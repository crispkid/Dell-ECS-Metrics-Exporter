package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dell-ecs-metrics-exporter/internal/cache"
	"dell-ecs-metrics-exporter/internal/collector"
	"dell-ecs-metrics-exporter/internal/config"
	"dell-ecs-metrics-exporter/internal/ecs"
	"dell-ecs-metrics-exporter/internal/httpapi"
	"dell-ecs-metrics-exporter/internal/metrics"
	"dell-ecs-metrics-exporter/internal/profile"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		slog.New(slog.NewJSONHandler(os.Stderr, nil)).Error("exporter stopped", "error", err)
		os.Exit(1)
	}
}

func run(arguments []string, stdout, stderr *os.File) error {
	flags := flag.NewFlagSet("ecs-exporter", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String(
		"config", envOrDefault("ECS_EXPORTER_CONFIG", "config.yaml"), "YAML configuration file",
	)
	profilesDir := flags.String(
		"profiles-dir", envOrDefault("ECS_EXPORTER_PROFILES_DIR", "profiles"),
		"compatibility profile directory",
	)
	validateProfiles := flags.Bool("validate-profiles", false, "validate profiles and exit")
	validateConfig := flags.Bool("validate-config", false, "validate configuration and exit")
	if err := flags.Parse(arguments); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", flags.Args())
	}

	catalog, err := profile.LoadDir(*profilesDir)
	if err != nil {
		return fmt.Errorf("load compatibility profiles: %w", err)
	}
	if *validateProfiles && !*validateConfig {
		return json.NewEncoder(stdout).Encode(map[string]any{
			"profiles": catalog.Summary(), "status": "valid",
		})
	}
	settings, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	if *validateConfig {
		return json.NewEncoder(stdout).Encode(map[string]any{
			"clusters": len(settings.ECS.Clusters), "profiles": catalog.Summary(), "status": "valid",
		})
	}

	logger := slog.New(slog.NewJSONHandler(stderr, nil)).With("service", "dell-ecs-metrics-exporter")
	store := cache.New()
	telemetry := metrics.New(store, catalog, metrics.BuildInfo{
		Version: version, Commit: commit, BuildDate: buildDate,
	}, settings.Cache.MaxStale.Duration)
	observer := runtimeObserver{metrics: telemetry, logger: logger}
	var clients []*ecs.Client
	var runners []*collector.Runner
	for _, clusterConfig := range settings.ECS.Clusters {
		client, err := ecs.NewClient(clusterConfig, settings.Collector, observer)
		if err != nil {
			return fmt.Errorf("initialize cluster %q: %w", clusterConfig.Name, err)
		}
		clients = append(clients, client)
		runners = append(runners, collector.NewRunner(
			clusterConfig, settings.Collector, client, catalog, store, observer,
		))
	}
	handler, err := httpapi.NewHandler(httpapi.Options{
		Store: store, Catalog: catalog, Metrics: telemetry.Handler(),
		MetricsPath:      settings.Prometheus.Path,
		MetricsProtected: settings.Prometheus.Protected,
		Security:         settings.Security.InventoryAPI,
		StaleTolerance:   settings.Cache.StaleTolerance.Duration,
		MaxStale:         settings.Cache.MaxStale.Duration,
		Build:            httpapi.BuildInfo{Version: version, Commit: commit, BuildDate: buildDate},
	})
	if err != nil {
		return fmt.Errorf("initialize HTTP API: %w", err)
	}
	server := &http.Server{
		Addr: settings.Server.ListenAddress, Handler: accessLog(logger, handler),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second,
	}

	runContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	scheduler := collector.NewScheduler(runners, settings.Collector)
	scheduler.Start(runContext)
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info(
			"starting exporter", "address", settings.Server.ListenAddress,
			"clusters", len(settings.ECS.Clusters), "profiles", catalog.Summary(),
		)
		if settings.Server.TLS.CertFile != "" {
			serverErrors <- server.ListenAndServeTLS(settings.Server.TLS.CertFile, settings.Server.TLS.KeyFile)
		} else {
			serverErrors <- server.ListenAndServe()
		}
	}()

	var runErr error
	select {
	case <-runContext.Done():
	case serveErr := <-serverErrors:
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			stop()
			runErr = fmt.Errorf("serve HTTP: %w", serveErr)
		}
	}
	stop()
	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := server.Shutdown(shutdownContext); err != nil {
		logger.Error("HTTP shutdown failed", "error", err)
	}
	cancel()
	scheduler.Wait()
	logoutContext, cancelLogout := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelLogout()
	for _, client := range clients {
		if err := client.Close(logoutContext); err != nil {
			logger.Warn("ECS logout failed", "error", err)
		}
	}
	return runErr
}

type runtimeObserver struct {
	metrics *metrics.Metrics
	logger  *slog.Logger
}

func (o runtimeObserver) ObserveAPI(
	cluster, logical, result, errorType, correlationID string,
	status, attempt int,
	duration time.Duration,
) {
	o.metrics.ObserveAPI(
		cluster, logical, result, errorType, correlationID, status, attempt, duration,
	)
	o.logger.Info(
		"ECS API request",
		"cluster", cluster,
		"api", logical,
		"http_status", status,
		"duration_seconds", duration.Seconds(),
		"retry_count", max(attempt-1, 0),
		"result", result,
		"error_type", errorType,
		"correlation_id", correlationID,
	)
}

func (o runtimeObserver) ObserveAPIResponseSize(
	cluster, logical, result string,
	bytes int64,
) {
	o.metrics.ObserveAPIResponseSize(cluster, logical, result, bytes)
}

func (o runtimeObserver) ObserveCollector(
	cluster, name, result, errorType, correlationID string,
	duration time.Duration,
) {
	o.metrics.ObserveCollector(
		cluster, name, result, errorType, correlationID, duration,
	)
	log := o.logger.Info
	if result == "error" {
		log = o.logger.Warn
	}
	log(
		"collector execution",
		"cluster", cluster,
		"collector", name,
		"duration_seconds", duration.Seconds(),
		"result", result,
		"error_type", errorType,
		"correlation_id", correlationID,
	)
}

func (o runtimeObserver) ObserveCacheRefresh(cluster, name, result string) {
	o.metrics.ObserveCacheRefresh(cluster, name, result)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func accessLog(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: writer, status: http.StatusOK}
		next.ServeHTTP(recorder, request)
		logger.Info(
			"HTTP access", "method", request.Method, "path", request.URL.Path,
			"status", recorder.status, "duration_seconds", time.Since(started).Seconds(),
		)
	})
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
