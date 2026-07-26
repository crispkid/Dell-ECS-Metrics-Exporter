package collector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"dell-ecs-metrics-exporter/internal/cache"
	"dell-ecs-metrics-exporter/internal/config"
	"dell-ecs-metrics-exporter/internal/ecs"
	"dell-ecs-metrics-exporter/internal/model"
	"dell-ecs-metrics-exporter/internal/profile"
)

func TestRunnerCollectsAllDomains(t *testing.T) {
	t.Parallel()
	api := newFixtureAPI(t, "ecs-3.6")
	store := cache.New()
	observer := &collectorObserver{}
	runner := NewRunner(
		testRunnerCluster(), config.Defaults().Collector, api, loadCollectorCatalog(t), store, observer,
	)
	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	runner.now = func() time.Time { return now }
	ctx := context.Background()
	steps := []struct {
		name string
		run  func(context.Context) error
	}{
		{"bootstrap", runner.Bootstrap},
		{"node", runner.CollectNodes},
		{"node-resources", runner.CollectNodeResources},
		{"namespace-1", runner.CollectNamespaces},
		{"bucket", runner.CollectBuckets},
		{"namespace-2", runner.CollectNamespaces},
		{"cluster", runner.CollectCluster},
		{"performance", runner.CollectPerformance},
		{"replication", runner.CollectReplication},
		{"recovery", runner.CollectRecovery},
	}
	for _, step := range steps {
		if err := step.run(ctx); err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}
	}
	snapshot := store.Snapshot()
	if len(snapshot.Clusters) != 1 || len(snapshot.Nodes) != 2 ||
		len(snapshot.Namespaces) != 1 || len(snapshot.Buckets) != 2 ||
		len(snapshot.Replications) != 2 || len(snapshot.Performances) != 8 {
		t.Fatalf("snapshot counts = clusters:%d nodes:%d namespaces:%d buckets:%d replications:%d performances:%d",
			len(snapshot.Clusters), len(snapshot.Nodes), len(snapshot.Namespaces),
			len(snapshot.Buckets), len(snapshot.Replications), len(snapshot.Performances))
	}
	cluster := snapshot.Clusters[0]
	if cluster.Profiles[0] != "ecs-3.6" || cluster.MixedVersion ||
		cluster.BucketCount != 2 || cluster.NamespaceCount != 1 ||
		cluster.ObjectCount == nil || *cluster.ObjectCount != 12 {
		t.Fatalf("cluster = %#v", cluster)
	}
	if snapshot.Nodes[0].Health == nil || snapshot.Nodes[0].CPUUsageRatio == nil ||
		len(snapshot.Nodes[0].Services) != 2 ||
		snapshot.Namespaces[0].BucketCount != 2 ||
		snapshot.Buckets[0].UsedBytes == nil || !snapshot.Buckets[0].HardQuotaConfigured {
		t.Fatalf("enriched snapshot = %#v", snapshot)
	}
	if api.callCount("whoami") != 1 || api.callCount("vdc-performance") != 1 ||
		api.callCount("replication-link") != 1 || api.callCount("recovery-link") != 1 ||
		api.callCount("bucket-billing-batch") != 1 || api.callCount("bucket-billing") != 0 {
		t.Fatalf("API calls = %#v", api.calls)
	}

	runner.Run(ctx, "manual", func(context.Context) error { return errors.New("safe failure") })
	state := findState(store.Snapshot(), "manual")
	if state.Errors != 1 || state.LastError != "safe failure" || observer.errors == 0 {
		t.Fatalf("runner state = %#v observer=%#v", state, observer)
	}
}

func TestNodeResourceFailureKeepsManagementInventory(t *testing.T) {
	t.Parallel()
	api := newFixtureAPI(t, "ecs-3.8.0")
	api.fail["node-resources"] = &ecs.APIError{
		Logical: "node-resources", Status: 503, Kind: "http_503", Retryable: true,
	}
	store := cache.New()
	runner := NewRunner(
		testRunnerCluster(), config.Defaults().Collector, api, loadCollectorCatalog(t), store, nil,
	)
	ctx := context.Background()
	runner.Run(ctx, "node", runner.CollectNodes)
	runner.Run(ctx, "node-resources", runner.CollectNodeResources)

	snapshot := store.Snapshot()
	if len(snapshot.Nodes) != 2 || snapshot.Nodes[0].Health == nil {
		t.Fatalf("management node inventory was not retained: %#v", snapshot.Nodes)
	}
	nodeState := findState(snapshot, "node")
	resourceState := findState(snapshot, "node-resources")
	if nodeState.LastSuccessAt.IsZero() || nodeState.LastError != "" ||
		resourceState.LastError == "" || resourceState.Errors != 1 {
		t.Fatalf("node states = management:%#v resources:%#v", nodeState, resourceState)
	}
}

func TestNodeResourceSnapshotClearsMissingSeries(t *testing.T) {
	t.Parallel()
	api := newFixtureAPI(t, "ecs-3.6")
	store := cache.New()
	runner := NewRunner(
		testRunnerCluster(), config.Defaults().Collector, api, loadCollectorCatalog(t), store, nil,
	)
	if err := runner.CollectNodes(context.Background()); err != nil {
		t.Fatal(err)
	}
	nodes := store.Snapshot().Nodes
	stale := 0.5
	nodes[1].CPUUsageRatio = &stale
	nodes[1].MemoryUsedBytes = &stale
	nodes[1].MemoryTotalBytes = &stale
	nodes[1].Network = []model.Network{{Interface: "stale", ReceiveBytes: &stale}}
	store.ReplaceNodes("alpha", nodes)

	if err := runner.CollectNodeResources(context.Background()); err != nil {
		t.Fatal(err)
	}
	nodes = store.Snapshot().Nodes
	if nodes[0].CPUUsageRatio == nil || nodes[1].CPUUsageRatio != nil ||
		nodes[1].MemoryUsedBytes != nil || len(nodes[1].Network) != 0 {
		t.Fatalf("resource snapshot retained stale series: %#v", nodes)
	}
}

func TestBucketPaginationAndAtomicFailure(t *testing.T) {
	t.Parallel()
	api := newFixtureAPI(t, "ecs-3.6")
	api.bucketPages = true
	store := cache.New()
	runner := NewRunner(
		testRunnerCluster(), config.Defaults().Collector, api, loadCollectorCatalog(t), store, nil,
	)
	runner.now = func() time.Time { return time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC) }
	if err := runner.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := runner.CollectBuckets(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.Snapshot().Buckets) != 2 || api.callCount("bucket-inventory") != 2 ||
		api.callCount("bucket-namespace-inventory") != 1 {
		t.Fatalf(
			"paged buckets=%#v page_calls=%d namespace_calls=%d",
			store.Snapshot().Buckets, api.callCount("bucket-inventory"),
			api.callCount("bucket-namespace-inventory"),
		)
	}

	store.ReplaceBuckets("alpha", []model.Bucket{{
		Cluster: "alpha", Namespace: "old", Name: "last-success",
	}})
	api.fail["bucket-quota"] = &ecs.APIError{
		Logical: "bucket-quota", Status: 503, Kind: "http_503", Retryable: true,
	}
	if err := runner.CollectBuckets(context.Background()); err == nil {
		t.Fatal("bucket failure returned nil")
	}
	buckets := store.Snapshot().Buckets
	if len(buckets) != 1 || buckets[0].Name != "last-success" {
		t.Fatalf("failed page replaced last success: %#v", buckets)
	}
}

func TestBucketPaginationRejectsRepeatedMarkerAndDuplicate(t *testing.T) {
	t.Parallel()
	runner := &Runner{
		config:   config.ClusterConfig{Name: "alpha"},
		settings: config.Defaults().Collector,
		now:      func() time.Time { return time.Now() },
	}
	tests := []struct {
		name string
		api  *scriptedAPI
	}{
		{
			name: "repeated marker",
			api: &scriptedAPI{get: func(_ string, _ url.Values) ([]byte, error) {
				return []byte(`{"object_bucket":[],"next_marker":"same"}`), nil
			}},
		},
		{
			name: "duplicate bucket",
			api: &scriptedAPI{get: func(_ string, query url.Values) ([]byte, error) {
				if query.Get("marker") == "" {
					return []byte(`{"object_bucket":[{"name":"b","namespace":"n"}],"next_marker":"next"}`), nil
				}
				return []byte(`{"object_bucket":[{"name":"b","namespace":"n"}]}`), nil
			}},
		},
		{
			name: "different namespace",
			api: &scriptedAPI{get: func(_ string, _ url.Values) ([]byte, error) {
				return []byte(`{"object_bucket":[{"name":"b","namespace":"other"}]}`), nil
			}},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			runner.api = test.api
			if _, err := runner.collectBucketPages(context.Background(), time.Now(), "n"); err == nil {
				t.Fatal("invalid pagination was accepted")
			}
		})
	}
}

func TestBucketBillingBatchFallsBackWhenUnsupported(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		apiError *ecs.APIError
	}{
		{
			name: "not found",
			apiError: &ecs.APIError{
				Logical: "bucket-billing-batch",
				Status:  http.StatusNotFound,
				Kind:    "http_404",
			},
		},
		{
			name: "method not allowed",
			apiError: &ecs.APIError{
				Logical: "bucket-billing-batch",
				Status:  http.StatusMethodNotAllowed,
				Kind:    "http_405",
			},
		},
		{
			name: "not implemented",
			apiError: &ecs.APIError{
				Logical: "bucket-billing-batch",
				Status:  http.StatusNotImplemented,
				Kind:    "http_501",
			},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			singleCalls := 0
			runner := &Runner{
				api: &scriptedAPI{
					get: func(logical string, _ url.Values) ([]byte, error) {
						if logical != "bucket-billing" {
							return nil, errors.New("unexpected GET")
						}
						singleCalls++
						return collectorFixture(t, "common", "bucket-billing-info.json"), nil
					},
					post: func(logical string, _ url.Values, _ any) ([]byte, error) {
						if logical != "bucket-billing-batch" {
							return nil, errors.New("unexpected POST")
						}
						return nil, test.apiError
					},
				},
			}
			buckets := []model.Bucket{
				{Cluster: "alpha", Namespace: "namespace-a", Name: "bucket-a"},
				{Cluster: "alpha", Namespace: "namespace-a", Name: "bucket-b"},
			}
			if err := runner.enrichBucketBillings(context.Background(), buckets); err != nil {
				t.Fatal(err)
			}
			if singleCalls != 2 ||
				buckets[0].UsageSampleAt == nil ||
				buckets[1].UsedBytes == nil {
				t.Fatalf("single calls=%d buckets=%#v", singleCalls, buckets)
			}
		})
	}
}

func TestBucketBillingBatchDoesNotFallbackForServerError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		code int
	}{
		{name: "generic", code: 0},
		{name: "code 999", code: 999},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			singleCalls := 0
			batchError := &ecs.APIError{
				Logical: "bucket-billing-batch",
				Status:  http.StatusInternalServerError,
				Kind:    "http_500",
				Code:    test.code,
			}
			runner := &Runner{
				api: &scriptedAPI{
					get: func(string, url.Values) ([]byte, error) {
						singleCalls++
						return nil, errors.New("single bucket billing must not be called")
					},
					post: func(logical string, _ url.Values, _ any) ([]byte, error) {
						if logical != "bucket-billing-batch" {
							return nil, errors.New("unexpected POST")
						}
						return nil, batchError
					},
				},
			}
			buckets := []model.Bucket{{
				Cluster: "alpha", Namespace: "namespace-a", Name: "bucket-a",
			}}
			err := runner.enrichBucketBillings(context.Background(), buckets)
			if !errors.Is(err, batchError) {
				t.Fatalf("error = %v, want original batch error", err)
			}
			if singleCalls != 0 {
				t.Fatalf("single bucket billing calls = %d, want 0", singleCalls)
			}
		})
	}
}

func TestECS380LiveBucketCompatibilityPath(t *testing.T) {
	t.Parallel()
	singleCalls := 0
	api := &scriptedAPI{
		get: func(logical string, query url.Values) ([]byte, error) {
			switch logical {
			case "version-discovery":
				return collectorFixture(t, "ecs-3.8.0", "nodes.json"), nil
			case "whoami":
				return []byte(`{"user":{"common_name":"monitor",
					"roles":{"role":["SYSTEM_MONITOR"]}}}`), nil
			case "bucket-namespace-inventory":
				return []byte(`{"namespace":[{
					"name":"redacted-namespace","id":"redacted-namespace"
				}]}`), nil
			case "bucket-inventory":
				if query.Get("namespace") != "redacted-namespace" {
					return nil, errors.New("bucket namespace filter is required")
				}
				return []byte(`{"object_bucket":[
					{"name":"redacted-bucket","namespace":"redacted-namespace"},
					{"name":"redacted-bucket-empty","namespace":"redacted-namespace"},
					{"name":"redacted-bucket-unset","namespace":"redacted-namespace"}
				]}`), nil
			case "bucket-quota":
				return collectorFixture(t, "ecs-3.8.0.3-live", "bucket-quota.json"), nil
			case "bucket-billing":
				singleCalls++
				return collectorFixture(
					t, "ecs-3.8.0.3-live", "bucket-billing-info.json",
				), nil
			default:
				return nil, errors.New("unexpected GET logical API: " + logical)
			}
		},
		post: func(logical string, query url.Values, body any) ([]byte, error) {
			if logical != "bucket-billing-batch" {
				return nil, errors.New("unexpected POST logical API: " + logical)
			}
			encoded, err := json.Marshal(body)
			if err != nil || query.Get("sizeunit") != "KB" ||
				string(encoded) != `{"id":["redacted-bucket","redacted-bucket-empty","redacted-bucket-unset"]}` {
				return nil, fmt.Errorf("unexpected batch request: query=%v body=%#v", query, body)
			}
			return collectorFixture(
				t, "ecs-3.8.0.3-live", "bucket-billing-batch.json",
			), nil
		},
	}
	store := cache.New()
	runner := NewRunner(
		testRunnerCluster(), config.Defaults().Collector, api, loadCollectorCatalog(t), store, nil,
	)
	if err := runner.CollectBuckets(context.Background()); err != nil {
		t.Fatal(err)
	}
	buckets := store.Snapshot().Buckets
	if len(buckets) != 3 || singleCalls != 0 ||
		buckets[0].UsedBytes == nil || *buckets[0].UsedBytes != 7_000_000 ||
		buckets[0].ObjectCount == nil || *buckets[0].ObjectCount != 3 ||
		buckets[0].SoftQuotaBytes == nil || *buckets[0].SoftQuotaBytes != 1e9 ||
		buckets[0].HardQuotaBytes == nil || *buckets[0].HardQuotaBytes != 2e9 {
		t.Fatalf("live compatibility buckets=%#v single_calls=%d", buckets, singleCalls)
	}
}

func TestBucketBillingBatchRejectsUnrequestedItem(t *testing.T) {
	t.Parallel()
	runner := &Runner{
		api: &scriptedAPI{
			get: func(string, url.Values) ([]byte, error) {
				return nil, errors.New("single bucket billing must not be called")
			},
			post: func(string, url.Values, any) ([]byte, error) {
				return []byte(`{"bucket_billing_infos":[{
					"name":"other","namespace":"namespace-a",
					"total_size":1,"total_size_unit":"KB","total_objects":1
				}]}`), nil
			},
		},
	}
	buckets := []model.Bucket{{
		Cluster: "alpha", Namespace: "namespace-a", Name: "bucket-a",
	}}
	if err := runner.enrichBucketBillings(context.Background(), buckets); err == nil ||
		!strings.Contains(err.Error(), "unrequested") {
		t.Fatalf("unrequested batch item error = %v", err)
	}
}

func TestBucketBillingBatchFallsBackForMissingItem(t *testing.T) {
	t.Parallel()
	singleCalls := 0
	runner := &Runner{
		api: &scriptedAPI{
			get: func(logical string, _ url.Values) ([]byte, error) {
				if logical != "bucket-billing" {
					return nil, errors.New("unexpected GET")
				}
				singleCalls++
				return collectorFixture(t, "common", "bucket-billing-info.json"), nil
			},
			post: func(logical string, _ url.Values, _ any) ([]byte, error) {
				if logical != "bucket-billing-batch" {
					return nil, errors.New("unexpected POST")
				}
				return []byte(`{"bucket_billing_info":[{
					"name":"bucket-a","namespace":"namespace-a",
					"total_size":1,"total_size_unit":"KB","total_objects":1
				}]}`), nil
			},
		},
	}
	buckets := []model.Bucket{
		{Cluster: "alpha", Namespace: "namespace-a", Name: "bucket-a"},
		{Cluster: "alpha", Namespace: "namespace-a", Name: "bucket-b"},
	}
	if err := runner.enrichBucketBillings(context.Background(), buckets); err != nil {
		t.Fatal(err)
	}
	if singleCalls != 1 || *buckets[0].UsedBytes != 1024 ||
		*buckets[1].UsedBytes != 2_097_152 {
		t.Fatalf("single calls=%d buckets=%#v", singleCalls, buckets)
	}
}

func TestConditionalPerformanceIsNotQueried(t *testing.T) {
	t.Parallel()
	api := newFixtureAPI(t, "ecs-3.8.1")
	runner := NewRunner(
		testRunnerCluster(), config.Defaults().Collector, api, loadCollectorCatalog(t), cache.New(), nil,
	)
	if err := runner.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := runner.CollectPerformance(context.Background()); err != nil {
		t.Fatal(err)
	}
	if api.callCount("vdc-performance") != 0 {
		t.Fatal("conditional VDC performance capability was queried")
	}
}

func TestConditionalPerformanceCanBeExplicitlyEnabled(t *testing.T) {
	t.Parallel()
	api := newFixtureAPI(t, "ecs-3.8.1")
	cluster := testRunnerCluster()
	cluster.Capabilities.EnabledConditional = append(
		cluster.Capabilities.EnabledConditional,
		"vdc_performance", "namespace_performance",
	)
	store := cache.New()
	runner := NewRunner(
		cluster, config.Defaults().Collector, api, loadCollectorCatalog(t), store, nil,
	)
	if err := runner.CollectPerformance(context.Background()); err != nil {
		t.Fatal(err)
	}
	if api.callCount("vdc-performance") != 1 || len(store.Snapshot().Performances) != 8 {
		t.Fatalf(
			"calls=%d performances=%#v",
			api.callCount("vdc-performance"), store.Snapshot().Performances,
		)
	}
}

func TestSchedulerRunsEachCollectorAndStops(t *testing.T) {
	t.Parallel()
	api := newFixtureAPI(t, "ecs-3.6")
	settings := config.Defaults().Collector
	for name := range settings.Intervals.Values() {
		override := config.Duration{Duration: time.Hour}
		switch name {
		case "cluster":
			settings.Intervals.Cluster = override
		case "node":
			settings.Intervals.Node = override
		case "performance":
			settings.Intervals.Performance = override
		case "replication":
			settings.Intervals.Replication = override
		case "recovery":
			settings.Intervals.Recovery = override
		case "namespace":
			settings.Intervals.Namespace = override
		case "bucket":
			settings.Intervals.Bucket = override
		}
	}
	runner := NewRunner(
		testRunnerCluster(), settings, api, loadCollectorCatalog(t), cache.New(), nil,
	)
	if err := runner.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	scheduler := NewScheduler([]*Runner{runner}, settings)
	scheduler.Start(ctx)
	scheduler.Wait()
	states := runner.store.Snapshot().States
	if len(states) != 0 || api.callCount("whoami") != 1 {
		t.Fatalf("scheduled states = %#v", states)
	}
}

func TestRunScheduledRepeatsWithoutOverlap(t *testing.T) {
	t.Parallel()
	store := cache.New()
	runner := &Runner{
		config: config.ClusterConfig{Name: "alpha"}, store: store,
		observer: NopObserver{}, now: time.Now,
	}
	ctx, cancel := context.WithCancel(context.Background())
	var mu sync.Mutex
	runs := 0
	runScheduled(ctx, runner, "test", time.Millisecond, 0, func(context.Context) error {
		mu.Lock()
		defer mu.Unlock()
		runs++
		if runs == 3 {
			cancel()
		}
		return nil
	})
	mu.Lock()
	defer mu.Unlock()
	if runs != 3 || findState(store.Snapshot(), "test").Runs != 3 {
		t.Fatalf("runs = %d state=%#v", runs, store.Snapshot().States)
	}
}

func TestCollectorValidationAndSafeErrors(t *testing.T) {
	t.Parallel()
	apiErr := &ecs.APIError{Logical: "bucket", Status: 503, Kind: "http_503"}
	safe := safeCollectorError(apiErr)
	if strings.Contains(safe.Error(), "https://") || !strings.Contains(safe.Error(), "bucket") {
		t.Fatalf("safe error = %v", safe)
	}
	if safeCollectorError(nil) != nil {
		t.Fatal("nil error changed")
	}
}

func TestFluxQueriesOnlyRequestEnabledCapabilityScopes(t *testing.T) {
	t.Parallel()
	networkOnly := nodeResourceFluxQuery(false, false, true)
	if !strings.Contains(networkOnly, `r._measurement == "net"`) ||
		strings.Contains(networkOnly, `"disk"`) ||
		strings.Contains(networkOnly, `"cpu"`) ||
		strings.Contains(networkOnly, `"mem"`) {
		t.Fatalf("network query = %s", networkOnly)
	}
	diskCPU := nodeResourceFluxQuery(true, true, false)
	if !strings.Contains(diskCPU, `"disk"`) || !strings.Contains(diskCPU, `"cpu"`) ||
		strings.Contains(diskCPU, `"net"`) {
		t.Fatalf("disk/cpu query = %s", diskCPU)
	}
	if query := performanceFluxQuery(true, false); !strings.Contains(query, "not exists r.namespace") {
		t.Fatalf("VDC-only query = %s", query)
	}
	if query := performanceFluxQuery(false, true); !strings.Contains(query, "exists r.namespace") ||
		strings.Contains(query, "not exists") {
		t.Fatalf("namespace-only query = %s", query)
	}
}

func TestRunReportsSkippedWithoutClaimingCacheRefresh(t *testing.T) {
	t.Parallel()
	store := cache.New()
	observer := &collectorObserver{}
	runner := &Runner{
		config: config.ClusterConfig{Name: "alpha"},
		store:  store, observer: observer, now: time.Now,
	}
	runner.Run(context.Background(), "conditional", func(context.Context) error { return nil })
	if observer.lastResult != "skipped" || observer.refreshResult != "skipped" {
		t.Fatalf("observer = %#v", observer)
	}
	state := findState(store.Snapshot(), "conditional")
	if !state.LastSuccessAt.IsZero() || state.Running || state.Runs != 1 {
		t.Fatalf("skipped state = %#v", state)
	}
}

type fixtureAPI struct {
	t           *testing.T
	versionDir  string
	mu          sync.Mutex
	calls       map[string]int
	fail        map[string]error
	bucketPages bool
}

func newFixtureAPI(t *testing.T, versionDir string) *fixtureAPI {
	return &fixtureAPI{
		t: t, versionDir: versionDir, calls: make(map[string]int), fail: make(map[string]error),
	}
}

func (a *fixtureAPI) GetBytes(
	_ context.Context, logical, _ string, query url.Values,
) ([]byte, error) {
	a.mu.Lock()
	a.calls[logical]++
	failure := a.fail[logical]
	a.mu.Unlock()
	if failure != nil {
		return nil, failure
	}
	switch logical {
	case "version-discovery", "node-inventory":
		return collectorFixture(a.t, a.versionDir, "nodes.json"), nil
	case "whoami":
		return []byte(`{"user":{"common_name":"monitor","roles":{"role":["SYSTEM_MONITOR"]}}}`), nil
	case "cluster-health":
		return collectorFixture(a.t, "common", "localzone-health.json"), nil
	case "cluster-capacity":
		return collectorFixture(a.t, "common", "capacity.json"), nil
	case "node-health":
		return []byte(`{"node":[
			{"nodeid":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","healthStatus":"Good",
			 "services":[{"name":"blobsvc","status":"running"}],
			 "processes":[{"name":"fabric-lifecycle","status":"healthy"}]},
			{"nodeid":"bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb","healthStatus":"Good"}]}`), nil
	case "namespace-inventory", "bucket-namespace-inventory":
		return collectorFixture(a.t, "common", "namespaces.json"), nil
	case "namespace-quota":
		return collectorFixture(a.t, "common", "namespace-quota-configured.json"), nil
	case "namespace-billing":
		return collectorFixture(a.t, "common", "namespace-billing-info.json"), nil
	case "bucket-inventory":
		if query.Get("namespace") != "namespace-a" {
			return nil, errors.New("bucket namespace filter is required")
		}
		if a.bucketPages {
			if query.Get("marker") == "" {
				return []byte(`{"object_bucket":[
					{"name":"bucket-a","namespace":"namespace-a"}],"next_marker":"page-2"}`), nil
			}
			return []byte(`{"object_bucket":[
				{"name":"bucket-b","namespace":"namespace-a"}]}`), nil
		}
		return collectorFixture(a.t, "common", "buckets.json"), nil
	case "bucket-quota":
		return collectorFixture(a.t, "common", "bucket-quota-configured.json"), nil
	case "bucket-billing":
		return collectorFixture(a.t, "common", "bucket-billing-info.json"), nil
	case "replication-group":
		return collectorFixture(a.t, "common", "replication-group.json"), nil
	case "replication-link":
		return collectorFixture(a.t, "common", "rg-link.json"), nil
	case "recovery-link":
		return collectorFixture(a.t, "common", "rg-link.json"), nil
	default:
		return nil, errors.New("unexpected GET logical API: " + logical)
	}
}

func (a *fixtureAPI) PostBytes(
	_ context.Context, logical, _ string, _ url.Values, _ any,
) ([]byte, error) {
	a.mu.Lock()
	a.calls[logical]++
	failure := a.fail[logical]
	a.mu.Unlock()
	if failure != nil {
		return nil, failure
	}
	switch logical {
	case "node-resources":
		return collectorFixture(a.t, "common", "flux-node-resources.json"), nil
	case "vdc-performance":
		return collectorFixture(a.t, "common", "flux-vdc-performance.json"), nil
	case "bucket-billing-batch":
		return collectorFixture(a.t, "common", "bucket-billing-batch.json"), nil
	default:
		return nil, errors.New("unexpected POST logical API: " + logical)
	}
}

func (a *fixtureAPI) callCount(logical string) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.calls[logical]
}

type scriptedAPI struct {
	get  func(string, url.Values) ([]byte, error)
	post func(string, url.Values, any) ([]byte, error)
}

func (a *scriptedAPI) GetBytes(
	_ context.Context, logical, _ string, query url.Values,
) ([]byte, error) {
	return a.get(logical, query)
}

func (a *scriptedAPI) PostBytes(
	_ context.Context, logical, _ string, query url.Values, body any,
) ([]byte, error) {
	if a.post != nil {
		return a.post(logical, query, body)
	}
	return nil, errors.New("unexpected POST")
}

type collectorObserver struct {
	mu            sync.Mutex
	errors        int
	lastResult    string
	refreshResult string
}

func (o *collectorObserver) ObserveCollector(
	_, _, result, _, _ string, _ time.Duration,
) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.lastResult = result
	if result == "error" {
		o.errors++
	}
}

func (o *collectorObserver) ObserveCacheRefresh(_, _, result string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.refreshResult = result
}

func testRunnerCluster() config.ClusterConfig {
	return config.ClusterConfig{
		Name: "alpha", Site: "site-a", Environment: "test",
		Capabilities: config.CapabilityConfig{
			EnabledConditional: []string{"recovery_progress", "node_service_process"},
		},
		Replication: config.ReplicationTargets{Groups: []string{"rg-a"}, Links: []string{"rg-link-a"}},
	}
}

func loadCollectorCatalog(t *testing.T) *profile.Catalog {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate source")
	}
	value, err := profile.LoadDir(filepath.Join(filepath.Dir(file), "..", "..", "profiles"))
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func collectorFixture(t *testing.T, directory, name string) []byte {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate source")
	}
	content, err := os.ReadFile(filepath.Join(
		filepath.Dir(file), "..", "..", "testdata", "ecs", directory, name,
	))
	if err != nil {
		t.Fatal(err)
	}
	return content
}

func findState(snapshot model.Snapshot, collector string) model.CollectorState {
	for _, state := range snapshot.States {
		if state.Collector == collector {
			return state
		}
	}
	return model.CollectorState{}
}
