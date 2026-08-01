package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"slices"
	"sync"
	"time"

	"dell-ecs-metrics-exporter/internal/cache"
	"dell-ecs-metrics-exporter/internal/config"
	"dell-ecs-metrics-exporter/internal/httpapi"
	"dell-ecs-metrics-exporter/internal/metrics"
	"dell-ecs-metrics-exporter/internal/model"
	"dell-ecs-metrics-exporter/internal/profile"

	"github.com/prometheus/common/expfmt"
	prommodel "github.com/prometheus/common/model"
)

const (
	clusterCount = 10
	nodeCount    = 100
	bucketCount  = 10_000
)

type result struct {
	Classification         string  `json:"classification"`
	Clusters               int     `json:"clusters"`
	Nodes                  int     `json:"nodes"`
	Buckets                int     `json:"buckets"`
	ConcurrentScrapes      int     `json:"concurrentScrapes"`
	MetricsP95Seconds      float64 `json:"metricsP95Seconds"`
	InventoryP95Seconds    float64 `json:"inventoryP95Seconds"`
	MetricsResponseBytes   int     `json:"metricsResponseBytes"`
	HeapAllocBytes         uint64  `json:"heapAllocBytes"`
	MetricsTargetSeconds   float64 `json:"metricsTargetSeconds"`
	InventoryTargetSeconds float64 `json:"inventoryTargetSeconds"`
	MemoryTargetBytes      uint64  `json:"memoryTargetBytes"`
}

func main() {
	catalog, err := profile.LoadDir("profiles")
	if err != nil {
		fail("load profiles", err)
	}
	store := buildStore()
	telemetry := metrics.New(
		store, catalog,
		metrics.BuildInfo{Version: "performance-check", Commit: "synthetic", BuildDate: "synthetic"},
		time.Hour,
	)
	handler, err := httpapi.NewHandler(httpapi.Options{
		Store: store, Catalog: catalog, Metrics: telemetry.Handler(),
		MetricsPath: "/metrics",
		Security: config.InventorySecurityConfig{
			Enabled: false, MaxPageSize: 500,
		},
		StaleTolerance: 15 * time.Minute,
		MaxStale:       time.Hour,
	})
	if err != nil {
		fail("create HTTP handler", err)
	}

	const scrapes = 16
	metricDurations := make([]time.Duration, scrapes)
	metricSizes := make([]int, scrapes)
	metricStatuses := make([]int, scrapes)
	var sampleMetrics []byte
	var wait sync.WaitGroup
	for index := range scrapes {
		wait.Add(1)
		go func() {
			defer wait.Done()
			started := time.Now()
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
			handler.ServeHTTP(recorder, request)
			metricDurations[index] = time.Since(started)
			metricSizes[index] = recorder.Body.Len()
			metricStatuses[index] = recorder.Code
			if index == 0 {
				sampleMetrics = bytes.Clone(recorder.Body.Bytes())
			}
		}()
	}
	wait.Wait()
	for _, status := range metricStatuses {
		if status != http.StatusOK {
			fmt.Fprintf(os.Stderr, "metrics status = %d\n", status)
			os.Exit(1)
		}
	}
	parser := expfmt.NewTextParser(prommodel.LegacyValidation)
	families, err := parser.TextToMetricFamilies(bytes.NewReader(sampleMetrics))
	if err != nil {
		fail("parse metrics response", err)
	}
	for _, name := range []string{
		"ecs_exporter_build_info",
		"ecs_exporter_cached_resources",
		"ecs_bucket_used_bytes",
	} {
		if _, ok := families[name]; !ok {
			fmt.Fprintf(os.Stderr, "required metric family is missing: %s\n", name)
			os.Exit(1)
		}
	}
	sampleMetrics = nil
	families = nil

	inventoryDurations := make([]time.Duration, 100)
	for index := range inventoryDurations {
		started := time.Now()
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(
			http.MethodGet, "/api/v1/buckets?page=0&size=100&sort=name", nil,
		)
		handler.ServeHTTP(recorder, request)
		inventoryDurations[index] = time.Since(started)
		if recorder.Code != http.StatusOK {
			fmt.Fprintf(os.Stderr, "inventory status = %d\n", recorder.Code)
			os.Exit(1)
		}
	}

	runtime.GC()
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	// Keep the handler and its backing cache live through the memory sample.
	// Otherwise compiler liveness can allow the synthetic 10,000-bucket store
	// to be collected before ReadMemStats and produce a misleadingly low value.
	runtime.KeepAlive(handler)
	value := result{
		Classification:         "synthetic-in-process-precheck",
		Clusters:               clusterCount,
		Nodes:                  nodeCount,
		Buckets:                bucketCount,
		ConcurrentScrapes:      scrapes,
		MetricsP95Seconds:      percentile(metricDurations, 0.95).Seconds(),
		InventoryP95Seconds:    percentile(inventoryDurations, 0.95).Seconds(),
		MetricsResponseBytes:   slices.Max(metricSizes),
		HeapAllocBytes:         memory.Alloc,
		MetricsTargetSeconds:   3,
		InventoryTargetSeconds: 2,
		MemoryTargetBytes:      512 * 1024 * 1024,
	}
	if err := json.NewEncoder(os.Stdout).Encode(value); err != nil {
		fail("encode result", err)
	}
	if value.MetricsP95Seconds >= value.MetricsTargetSeconds ||
		value.InventoryP95Seconds >= value.InventoryTargetSeconds ||
		value.HeapAllocBytes >= value.MemoryTargetBytes {
		os.Exit(1)
	}
}

func buildStore() *cache.Store {
	store := cache.New()
	now := time.Now().UTC()
	one, total, used, available := 1.0, 1_000_000_000_000.0, 500_000_000_000.0, 500_000_000_000.0
	nodePerCluster := nodeCount / clusterCount
	bucketPerCluster := bucketCount / clusterCount
	for clusterIndex := range clusterCount {
		clusterName := fmt.Sprintf("cluster-%02d", clusterIndex)
		store.ReplaceCluster(clusterName, model.Cluster{
			Name: clusterName, Site: "site", Environment: "test", VDC: "vdc",
			Versions: []string{"3.8.1.4.synthetic"}, Profiles: []string{"ecs-3.8.1"},
			Health: &one, TotalBytes: &total, UsedBytes: &used, AvailableBytes: &available,
			BucketCount: bucketPerCluster, NamespaceCount: 10, CollectedAt: now,
		})
		nodes := make([]model.Node, nodePerCluster)
		for index := range nodes {
			nodes[index] = model.Node{
				Cluster: clusterName,
				ID:      fmt.Sprintf("node-%02d-%02d", clusterIndex, index),
				Name:    fmt.Sprintf("node-%02d-%02d", clusterIndex, index),
				Version: "3.8.1.4.synthetic", Health: &one, CollectedAt: now,
			}
		}
		store.ReplaceNodes(clusterName, nodes)
		namespaces := make([]model.Namespace, 10)
		for index := range namespaces {
			namespaces[index] = model.Namespace{
				Cluster: clusterName, Name: fmt.Sprintf("namespace-%02d", index),
				BucketCount: bucketPerCluster / 10, CollectedAt: now,
			}
		}
		store.ReplaceNamespaces(clusterName, namespaces)
		buckets := make([]model.Bucket, bucketPerCluster)
		for index := range buckets {
			buckets[index] = model.Bucket{
				Cluster: clusterName,
				Namespace: fmt.Sprintf(
					"namespace-%02d", index%len(namespaces),
				),
				Name:        fmt.Sprintf("bucket-%02d-%04d", clusterIndex, index),
				UsedBytes:   &used,
				ObjectCount: &one,
				CollectedAt: now,
			}
		}
		store.ReplaceBuckets(clusterName, buckets)
		for _, collector := range []string{"cluster", "node", "namespace", "bucket"} {
			store.Start(clusterName, collector, now)
			store.Finish(clusterName, collector, now, nil)
		}
	}
	return store
}

func percentile(values []time.Duration, quantile float64) time.Duration {
	copyValues := slices.Clone(values)
	slices.Sort(copyValues)
	index := int(float64(len(copyValues)-1) * quantile)
	return copyValues[index]
}

func fail(action string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", action, err)
	os.Exit(1)
}
