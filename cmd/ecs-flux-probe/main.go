package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"dell-ecs-metrics-exporter/internal/config"
	"dell-ecs-metrics-exporter/internal/ecs"
	"dell-ecs-metrics-exporter/internal/fluxprobe"
	"dell-ecs-metrics-exporter/internal/profile"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

var errProbeDidNotPass = errors.New("flux compatibility probe did not pass")

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if !errors.Is(err, errProbeDidNotPass) {
			fmt.Fprintln(os.Stderr, "flux compatibility probe could not start safely")
		}
		os.Exit(1)
	}
}

func run(arguments []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("ecs-flux-probe", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String(
		"config", envOrDefault("ECS_EXPORTER_CONFIG", "config.yaml"),
		"YAML configuration file",
	)
	profilesDir := flags.String(
		"profiles-dir", envOrDefault("ECS_EXPORTER_PROFILES_DIR", "profiles"),
		"compatibility profile directory",
	)
	clusterName := flags.String(
		"cluster", "", "configured cluster name; required when multiple clusters exist",
	)
	performance := flags.Bool(
		"performance", true, "probe VDC and namespace performance mappings",
	)
	disk := flags.Bool(
		"disk", false, "probe conditional disk mapping using the configured allowlist",
	)
	timeout := flags.Duration("timeout", 2*time.Minute, "overall probe timeout (5s to 10m)")
	if err := flags.Parse(arguments); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("positional arguments are not accepted")
	}

	build := fluxprobe.BuildInfo{Version: version, Commit: commit, BuildDate: buildDate}
	if *timeout < 5*time.Second || *timeout > 10*time.Minute {
		return writeSetupFailure(stdout, build, "configuration")
	}
	settings, err := config.Load(*configPath)
	if err != nil {
		return writeSetupFailure(stdout, build, "configuration")
	}
	catalog, err := profile.LoadDir(*profilesDir)
	if err != nil {
		return writeSetupFailure(stdout, build, "profile")
	}
	cluster, err := fluxprobe.SelectCluster(settings, *clusterName)
	if err != nil {
		return writeSetupFailure(stdout, build, "cluster_selection")
	}
	options := fluxprobe.Options{
		EnablePerformance: *performance,
		EnableDisk:        *disk,
		Build:             build,
	}
	if err := fluxprobe.ValidateOptions(cluster, options); err != nil {
		return writeSetupFailure(stdout, build, "configuration")
	}
	client, err := ecs.NewClient(cluster, settings.Collector, ecs.NopObserver{})
	if err != nil {
		return writeSetupFailure(stdout, build, "client")
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	report := fluxprobe.Run(ctx, cluster, settings.Collector, catalog, client, options)
	cancel()
	if err := writeReport(stdout, report); err != nil {
		return err
	}
	if report.Result != fluxprobe.ResultPassed {
		return errProbeDidNotPass
	}
	return nil
}

func writeSetupFailure(stdout io.Writer, build fluxprobe.BuildInfo, errorType string) error {
	report := fluxprobe.SetupFailureReport(time.Now().UTC(), build, errorType)
	if err := writeReport(stdout, report); err != nil {
		return err
	}
	return errProbeDidNotPass
}

func writeReport(writer io.Writer, report fluxprobe.Report) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
