package fluxprobe

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"dell-ecs-metrics-exporter/internal/config"
	"dell-ecs-metrics-exporter/internal/ecs"
	"dell-ecs-metrics-exporter/internal/profile"
)

func TestProbeReplaysAllCompatibilityProfiles(t *testing.T) {
	t.Parallel()
	tests := []struct {
		versionDir string
		profileID  string
		interval   profile.Support
	}{
		{"ecs-3.6", "ecs-3.6", profile.SupportNative},
		{"ecs-3.7", "ecs-3.7", profile.SupportUnavailable},
		{"ecs-3.8.0", "ecs-3.8.0", profile.SupportUnavailable},
		{"ecs-3.8.1", "ecs-3.8.1", profile.SupportConditional},
	}
	for _, test := range tests {
		test := test
		t.Run(test.profileID, func(t *testing.T) {
			t.Parallel()
			api := &fixtureAPI{t: t, versionDir: test.versionDir, calls: make(map[string]int)}
			report := Run(
				context.Background(), testCluster(), config.Defaults().Collector,
				loadCatalog(t), api, Options{
					EnablePerformance: true,
					GeneratedAt:       time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
					Build:             BuildInfo{Version: "test", Commit: "fixture", BuildDate: "fixed"},
				},
			)
			if report.Result != ResultPassed || len(report.SelectedProfiles) != 1 ||
				report.SelectedProfiles[0] != test.profileID || report.MixedVersion ||
				report.CapabilityPolicy.FluxIntervalRates != test.interval {
				t.Fatalf("compatibility report = %#v", report)
			}
			if report.NodeSeries.Nodes != 2 || report.NodeSeries.CPUUsage != 1 ||
				report.NodeSeries.MemoryUsed != 1 || report.NodeSeries.MemoryTotal != 1 ||
				report.NodeSeries.NetworkReceive != 1 ||
				report.NodeSeries.NetworkTransmit != 1 || report.NodeSeries.Empty {
				t.Fatalf("node series = %#v", report.NodeSeries)
			}
			if report.PerformanceSeries.Total != 12 ||
				report.PerformanceSeries.VDCReadThroughput != 1 ||
				report.PerformanceSeries.VDCWriteThroughput != 1 ||
				report.PerformanceSeries.VDCLatency != 4 ||
				report.PerformanceSeries.VDCRequests != 3 ||
				report.PerformanceSeries.NamespaceRequests != 3 ||
				report.PerformanceSeries.Empty {
				t.Fatalf("performance series = %#v", report.PerformanceSeries)
			}
			if api.calls["node-resources"] != 3 || api.calls["vdc-performance"] != 3 ||
				api.closeCalls != 1 {
				t.Fatalf("API calls = %#v close=%d", api.calls, api.closeCalls)
			}
			assertRedacted(t, report)
		})
	}
}

func TestProbeReportsSafePartialFailure(t *testing.T) {
	t.Parallel()
	api := &fixtureAPI{
		t: t, versionDir: "ecs-3.8.1", calls: make(map[string]int),
		fail: map[string]error{
			"node-resources": &ecs.APIError{
				Logical: "contains-private-host", Status: 503, Kind: "http_503",
			},
		},
	}
	report := Run(
		context.Background(), testCluster(), config.Defaults().Collector,
		loadCatalog(t), api, Options{EnablePerformance: true},
	)
	if report.Result != ResultPartial {
		t.Fatalf("result = %q, want partial", report.Result)
	}
	check := findCheck(t, report, "node_resources")
	if check.Status != "error" || check.ErrorType != "http_503" || check.HTTPStatus != 503 {
		t.Fatalf("node resource check = %#v", check)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "contains-private-host") {
		t.Fatalf("report leaked API logical name: %s", encoded)
	}
}

func TestProbeAcceptsEmptyFluxWindows(t *testing.T) {
	t.Parallel()
	api := &fixtureAPI{
		t: t, versionDir: "ecs-3.8.1", calls: make(map[string]int), emptyFlux: true,
	}
	report := Run(
		context.Background(), testCluster(), config.Defaults().Collector,
		loadCatalog(t), api, Options{EnablePerformance: true},
	)
	if report.Result != ResultPassed || !report.NodeSeries.Empty ||
		!report.PerformanceSeries.Empty || report.PerformanceSeries.Total != 0 {
		t.Fatalf("empty-window report = %#v", report)
	}
}

func TestProbeDiskRequiresAndUsesAllowlist(t *testing.T) {
	t.Parallel()
	cluster := testCluster()
	if err := ValidateOptions(cluster, Options{EnableDisk: true}); err == nil {
		t.Fatal("disk probe accepted an empty filesystem allowlist")
	}
	blockedAPI := &fixtureAPI{t: t, versionDir: "ecs-3.6", calls: make(map[string]int)}
	blocked := Run(
		context.Background(), cluster, config.Defaults().Collector,
		loadCatalog(t), blockedAPI, Options{EnableDisk: true},
	)
	if blocked.Result != ResultFailed || findCheck(t, blocked, "setup").ErrorType != "configuration" ||
		blockedAPI.calls["version-discovery"] != 0 || blockedAPI.closeCalls != 1 {
		t.Fatalf("blocked disk report = %#v calls=%#v", blocked, blockedAPI.calls)
	}
	cluster.NodeResources.Filesystems = []string{"/data"}
	api := &fixtureAPI{t: t, versionDir: "ecs-3.6", calls: make(map[string]int)}
	report := Run(
		context.Background(), cluster, config.Defaults().Collector,
		loadCatalog(t), api, Options{EnableDisk: true},
	)
	if report.Result != ResultPassed || report.NodeSeries.DiskUsed != 1 ||
		report.NodeSeries.DiskTotal != 1 || api.calls["node-resources"] != 4 {
		t.Fatalf("disk report = %#v calls=%#v", report.NodeSeries, api.calls)
	}
}

func TestSetupFailureReportIsRedacted(t *testing.T) {
	t.Parallel()
	report := SetupFailureReport(
		time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), BuildInfo{},
		"bad value containing secret.example",
	)
	if report.Result != ResultFailed || report.Checks[0].ErrorType != "unknown" {
		t.Fatalf("setup report = %#v", report)
	}
	assertRedacted(t, report)
}

func findCheck(t *testing.T, report Report, name string) Check {
	t.Helper()
	for _, check := range report.Checks {
		if check.Name == name {
			return check
		}
	}
	t.Fatalf("check %q not found", name)
	return Check{}
}

func assertRedacted(t *testing.T, report Report) {
	t.Helper()
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		"alpha", "192.0.2.11", "node-a.example.invalid",
		"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "vdc-a", "namespace-a", "125500000",
	} {
		if strings.Contains(string(encoded), forbidden) {
			t.Errorf("report contains redacted value %q: %s", forbidden, encoded)
		}
	}
	if !report.Redaction.CredentialsOmitted || !report.Redaction.NetworkEndpointsOmitted ||
		!report.Redaction.ResourceIdentitiesOmitted || !report.Redaction.MetricValuesOmitted ||
		!report.Redaction.RawResponsesOmitted {
		t.Errorf("redaction declaration = %#v", report.Redaction)
	}
}

type fixtureAPI struct {
	t          *testing.T
	versionDir string
	calls      map[string]int
	fail       map[string]error
	emptyFlux  bool
	closeCalls int
}

func (a *fixtureAPI) GetBytes(
	_ context.Context, logical, _ string, _ url.Values,
) ([]byte, error) {
	a.calls[logical]++
	if err := a.fail[logical]; err != nil {
		return nil, err
	}
	switch logical {
	case "version-discovery", "node-inventory":
		return fixture(tPath(a.t, "testdata", "ecs", a.versionDir, "nodes.json")), nil
	case "whoami":
		return []byte(`{"user":{"common_name":"monitor","roles":{"role":["SYSTEM_MONITOR"]}}}`), nil
	case "cluster-health":
		return fixture(tPath(a.t, "testdata", "ecs", "common", "localzone-health.json")), nil
	case "cluster-capacity":
		return fixture(tPath(a.t, "testdata", "ecs", "common", "capacity.json")), nil
	case "node-health":
		return []byte(`{"node":[
			{"nodeid":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","healthStatus":"Good"},
			{"nodeid":"bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb","healthStatus":"Good"}]}`), nil
	default:
		return nil, errors.New("unexpected GET")
	}
}

func (a *fixtureAPI) PostBytes(
	_ context.Context, logical, _ string, _ url.Values, body any,
) ([]byte, error) {
	a.calls[logical]++
	if err := a.fail[logical]; err != nil {
		return nil, err
	}
	request, ok := body.(map[string]string)
	if !ok {
		return nil, errors.New("unexpected request body")
	}
	query := request["query"]
	var name string
	switch logical {
	case "node-resources":
		switch {
		case strings.Contains(query, `r._measurement == "cpu"`):
			name = "flux-node-cpu.json"
		case strings.Contains(query, `r._measurement == "mem"`):
			name = "flux-node-memory.json"
		case strings.Contains(query, `r._measurement == "disk"`):
			name = "flux-node-disk.json"
		case strings.Contains(query, `r._measurement == "net"`):
			name = "flux-node-network.json"
		}
	case "vdc-performance":
		switch {
		case strings.Contains(query, `"cq_performance_transaction_ns"`):
			name = "flux-namespace-performance.json"
		case strings.Contains(query, `"cq_performance_latency"`):
			name = "flux-vdc-latency.json"
		case strings.Contains(query, `"cq_performance_throughput"`):
			name = "flux-vdc-core-performance.json"
		}
	}
	if name == "" {
		return nil, errors.New("unexpected Flux query")
	}
	data := fixture(tPath(a.t, "testdata", "ecs", "common", name))
	if a.emptyFlux {
		return []byte(`{"Series":[{"Datatypes":null,"Columns":null,"Values":null}]}`), nil
	}
	return data, nil
}

func (a *fixtureAPI) Close(context.Context) error {
	a.closeCalls++
	return a.fail["logout"]
}

func fixture(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return data
}

func tPath(t *testing.T, parts ...string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate repository root")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	return filepath.Join(append([]string{root}, parts...)...)
}

func loadCatalog(t *testing.T) *profile.Catalog {
	t.Helper()
	catalog, err := profile.LoadDir(tPath(t, "profiles"))
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}

func testCluster() config.ClusterConfig {
	return config.ClusterConfig{Name: "alpha", Site: "site-a", Environment: "test"}
}
