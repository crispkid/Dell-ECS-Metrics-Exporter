package metrics

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"dell-ecs-metrics-exporter/internal/cache"
	"dell-ecs-metrics-exporter/internal/model"
	"dell-ecs-metrics-exporter/internal/profile"

	"github.com/prometheus/common/expfmt"
	prometheusmodel "github.com/prometheus/common/model"
)

func TestMetricsExpositionAndObservers(t *testing.T) {
	t.Parallel()
	store := cache.New()
	now := time.Now().UTC().Add(-time.Minute)
	number, zero := 42.0, 0.0
	text := "vdc-b"
	store.ReplaceCluster("alpha", model.Cluster{
		Name: "alpha", Site: "site-a", Environment: "test", VDC: "vdc-a",
		Health: &number, TotalBytes: &number, UsedBytes: &number, AvailableBytes: &number,
		BucketCount: 2, NamespaceCount: 1, ObjectCount: &number,
	})
	store.ReplaceNodes("alpha", []model.Node{{
		Cluster: "alpha", ID: "node-id", Name: "node\n-a", Health: &number,
		CPUUsageRatio: &number, MemoryUsedBytes: &number, MemoryTotalBytes: &number,
		DiskUsedBytes: &number, DiskTotalBytes: &number,
		Services: []model.NodeService{{
			Name: "blobsvc", Kind: "service", Status: "running", Health: 1,
		}},
		Network: []model.Network{{
			Interface: "public", ReceiveBytes: &number, TransmitBytes: &number,
		}},
	}})
	store.ReplaceNamespaces("alpha", []model.Namespace{{
		Cluster: "alpha", Name: "namespace-a", UsedBytes: &number, HardQuotaBytes: &number,
		BucketCount: 2, ObjectCount: &number,
	}})
	store.ReplaceBuckets("alpha", []model.Bucket{
		{
			Cluster: "alpha", Namespace: "namespace-a", Name: "bucket-a",
			UsedBytes: &number, SoftQuotaBytes: &number, HardQuotaBytes: &number,
			ObjectCount: &number,
		},
		{Cluster: "alpha", Namespace: "namespace-a", Name: "bucket-without-quota"},
	})
	store.ReplaceReplications("alpha", []model.Replication{
		{
			Cluster: "alpha", ID: "link-a", Group: "rg-a", SourceVDC: &text,
			Status: &number, LagSeconds: &zero, RecoveryKind: stringValue("bootstrap"),
			RecoveryProgress: &number,
		},
		{Cluster: "alpha", ID: "group-b", Group: "rg-b"},
	})
	store.ReplacePerformances("alpha", []model.Performance{
		{
			Cluster: "alpha", VDC: "vdc-a", Metric: model.PerformanceReadThroughput,
			Value: 125_000_000,
		},
		{
			Cluster: "alpha", VDC: "vdc-a", Namespace: "namespace-a",
			Metric: model.PerformanceLatency, Operation: "PUT", Quantile: "p99", Value: 0.012,
		},
	})
	store.ReplaceNodes("orphan", []model.Node{{Cluster: "orphan", ID: "n", Name: "n"}})
	if !store.Start("alpha", "bucket", now) {
		t.Fatal("could not start state")
	}
	store.Finish("alpha", "bucket", now, nil)

	value := New(store, loadMetricsCatalog(t), BuildInfo{
		Version: "1.0.0", Commit: "abc123", BuildDate: "2026-07-25",
	}, time.Hour)
	value.ObserveAPI(
		"alpha", "bucket-inventory", "success", "none", "alpha-1",
		http.StatusOK, 1, time.Millisecond,
	)
	value.ObserveAPI(
		"alpha", "bucket-inventory", "error", "http_503", "alpha-1",
		http.StatusServiceUnavailable, 2, 2*time.Millisecond,
	)
	value.ObserveCollector("alpha", "bucket", "success", "none", "alpha-1", time.Millisecond)
	value.ObserveCollector("alpha", "bucket", "error", "mapping", "alpha-2", time.Millisecond)
	value.ObserveAPIResponseSize("alpha", "bucket-inventory", "success", 2048)
	value.ObserveCacheRefresh("alpha", "bucket", "success")
	value.ObserveCacheRefresh("alpha", "bucket", "error")
	value.ObserveCacheRefresh("alpha", "bucket", "skipped")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	value.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("metrics status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	parser := expfmt.NewTextParser(prometheusmodel.UTF8Validation)
	families, err := parser.TextToMetricFamilies(bytes.NewReader(recorder.Body.Bytes()))
	if err != nil {
		t.Fatalf("Prometheus parser rejected metrics: %v\n%s", err, recorder.Body.String())
	}
	for _, name := range []string{
		"ecs_exporter_build_info", "ecs_exporter_profile_contract_info",
		"ecs_exporter_api_requests_total", "ecs_exporter_api_errors_total",
		"ecs_exporter_api_response_size_bytes",
		"ecs_exporter_collector_runs_total", "ecs_exporter_cache_refresh_errors_total",
		"ecs_exporter_last_success_timestamp_seconds", "ecs_exporter_cache_age_seconds",
		"ecs_exporter_cached_resources", "ecs_cluster_health",
		"ecs_node_network_receive_bytes_total", "ecs_namespace_quota_bytes",
		"ecs_bucket_soft_quota_bytes", "ecs_replication_status", "ecs_recovery_progress_ratio",
		"ecs_node_service_health", "ecs_vdc_read_throughput_bytes_per_second",
		"ecs_namespace_request_latency_seconds",
	} {
		if families[name] == nil {
			t.Errorf("metric family %s is missing", name)
		}
	}
	body := recorder.Body.String()
	if strings.Contains(body, "node\\n-a") || strings.Contains(body, `bucket="bucket-without-quota"`) &&
		strings.Contains(body, "ecs_bucket_soft_quota_bytes") {
		t.Fatal("label sanitization or quota omission failed")
	}
	if !strings.Contains(
		body,
		`ecs_recovery_progress_ratio{cluster="alpha",kind="bootstrap",replication_group="rg-a",source_vdc="vdc-b",target_vdc=""}`,
	) {
		t.Fatal("recovery metric does not preserve link dimensions")
	}
}

func TestMaxStaleSuppressesDomainSeriesButRetainsSelfMetrics(t *testing.T) {
	t.Parallel()
	store := cache.New()
	number := 1.0
	store.ReplaceCluster("alpha", model.Cluster{Name: "alpha", Health: &number})
	store.ReplaceBuckets("alpha", []model.Bucket{{
		Cluster: "alpha", Namespace: "n", Name: "b", UsedBytes: &number,
	}})
	old := time.Now().UTC().Add(-2 * time.Hour)
	for _, collector := range []string{"cluster", "bucket"} {
		if !store.Start("alpha", collector, old) {
			t.Fatal("could not create collector state")
		}
		store.Finish("alpha", collector, old, nil)
	}
	value := New(
		store, loadMetricsCatalog(t), BuildInfo{Version: "test"}, time.Hour,
	)
	recorder := httptest.NewRecorder()
	value.Handler().ServeHTTP(
		recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil),
	)
	body := recorder.Body.String()
	if strings.Contains(body, "ecs_cluster_health") ||
		strings.Contains(body, "ecs_bucket_used_bytes") {
		t.Fatalf("stale domain metrics were exposed:\n%s", body)
	}
	for _, metric := range []string{
		"ecs_exporter_build_info", "ecs_exporter_last_success_timestamp_seconds",
		"ecs_exporter_cache_age_seconds", "ecs_exporter_cached_resources",
	} {
		if !strings.Contains(body, metric) {
			t.Fatalf("self metric %s is missing", metric)
		}
	}
}

func TestLabelNormalization(t *testing.T) {
	t.Parallel()
	input := " \x00" + strings.Repeat("a", 300) + " "
	got := label(input)
	if strings.ContainsRune(got, '\x00') || len(got) != 128 {
		t.Fatalf("normalized label length=%d value=%q", len(got), got)
	}
	if got == label(" \x00"+strings.Repeat("a", 299)+"b ") ||
		got != label(input) {
		t.Fatal("long label hashing is not stable or collision-resistant")
	}
	if label("node\n-a") == label("node-a") {
		t.Fatal("sanitization produced colliding labels")
	}
	if value := label(strings.Repeat("界", 100)); len(value) > 128 ||
		!utf8.ValidString(value) {
		t.Fatalf("Unicode label is invalid: length=%d value=%q", len(value), value)
	}
	if boolString(true) != "true" || boolString(false) != "false" ||
		optionalString(nil) != "" || optionalString(stringValue("x")) != "x" {
		t.Fatal("helper conversion failed")
	}
}

func loadMetricsCatalog(t *testing.T) *profile.Catalog {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate source")
	}
	catalog, err := profile.LoadDir(filepath.Join(filepath.Dir(file), "..", "..", "profiles"))
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}

func stringValue(value string) *string {
	return &value
}
