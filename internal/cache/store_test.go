package cache

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"dell-ecs-metrics-exporter/internal/model"
)

func TestStoreReplacementSnapshotIsolationAndOrdering(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	number, text, flag := 1.0, "value", true
	created := now.Add(-time.Hour)
	store := New()
	cluster := model.Cluster{
		Name: "beta", Versions: []string{"3.8.1.4"}, Profiles: []string{"ecs-3.8.1"},
		Health: &number, ObjectCount: &number,
	}
	nodes := []model.Node{{
		Cluster: "beta", ID: "node-b", IP: &text, Health: &number,
		Network: []model.Network{{Interface: "public", ReceiveBytes: &number}},
	}}
	namespaces := []model.Namespace{{
		Cluster: "beta", Name: "namespace-b", Owner: &text, AuditEnabled: &flag,
	}}
	buckets := []model.Bucket{{
		Cluster: "beta", Namespace: "namespace-b", Name: "bucket-b",
		Owner: &text, UsedBytes: &number, VersioningEnabled: &flag, CreatedAt: &created,
	}}
	replications := []model.Replication{{
		Cluster: "beta", ID: "link-b", Group: "rg-b", Status: &number, SourceVDC: &text,
	}}
	performances := []model.Performance{{
		Cluster: "beta", VDC: "vdc-b", Namespace: "namespace-b",
		Metric: model.PerformanceLatency, Operation: "GET", Quantile: "p99", Value: 0.1,
	}}
	store.ReplaceCluster("beta", cluster)
	store.ReplaceNodes("beta", nodes)
	store.ReplaceNamespaces("beta", namespaces)
	store.ReplaceBuckets("beta", buckets)
	store.ReplacePerformances("beta", performances)
	store.ReplaceReplications("beta", replications)
	store.ReplaceCluster("alpha", model.Cluster{Name: "alpha"})

	cluster.Versions[0], nodes[0].Network[0].Interface = "changed", "changed"
	*nodes[0].Health, *namespaces[0].AuditEnabled, *buckets[0].CreatedAt = 9, false, time.Time{}
	*replications[0].Status = 9

	snapshot := store.Snapshot()
	if len(snapshot.Clusters) != 2 || snapshot.Clusters[0].Name != "alpha" ||
		snapshot.Clusters[1].Versions[0] != "3.8.1.4" ||
		*snapshot.Nodes[0].Health != 1 || snapshot.Nodes[0].Network[0].Interface != "public" ||
		!*snapshot.Namespaces[0].AuditEnabled || snapshot.Buckets[0].CreatedAt.IsZero() ||
		*snapshot.Replications[0].Status != 1 ||
		len(snapshot.Performances) != 1 || snapshot.Performances[0].Value != 0.1 ||
		store.Generation("beta", "cluster") != 1 ||
		store.Generation("beta", "node") != 1 ||
		store.Generation("beta", "performance") != 1 {
		t.Fatalf("snapshot was not isolated or sorted: %#v", snapshot)
	}
	snapshot.Performances[0].Value = 99
	if store.Snapshot().Performances[0].Value != 0.1 {
		t.Fatal("performance snapshot mutation leaked into store")
	}
	*snapshot.Buckets[0].UsedBytes = 99
	if got := *store.Snapshot().Buckets[0].UsedBytes; got != 1 {
		t.Fatalf("snapshot mutation leaked into store: %v", got)
	}
}

func TestCollectorStateSingleFlightAndErrorLifecycle(t *testing.T) {
	t.Parallel()
	store := New()
	start := time.Now().UTC()
	if !store.Start("alpha", "bucket", start) {
		t.Fatal("first Start returned false")
	}
	if store.Start("alpha", "bucket", start.Add(time.Second)) {
		t.Fatal("overlapping Start returned true")
	}
	store.Finish("alpha", "bucket", start.Add(time.Second), errors.New(strings.Repeat("x", 300)))
	state := store.Snapshot().States[0]
	if state.Running || state.Runs != 1 || state.Errors != 1 || len(state.LastError) != 256 ||
		!state.LastSuccessAt.IsZero() {
		t.Fatalf("failed state = %#v", state)
	}
	if !store.Start("alpha", "bucket", start.Add(2*time.Second)) {
		t.Fatal("Start after failure returned false")
	}
	store.Finish("alpha", "bucket", start.Add(3*time.Second), nil)
	state = store.Snapshot().States[0]
	if state.Running || state.Runs != 2 || state.Errors != 1 || state.LastError != "" ||
		!state.LastSuccessAt.Equal(start.Add(3*time.Second)) {
		t.Fatalf("successful state = %#v", state)
	}
	if !store.Start("alpha", "bucket", start.Add(4*time.Second)) {
		t.Fatal("Start before skipped run returned false")
	}
	store.FinishSkipped("alpha", "bucket")
	state = store.Snapshot().States[0]
	if state.Running || state.Runs != 3 ||
		!state.LastSuccessAt.Equal(start.Add(3*time.Second)) || state.LastError != "" {
		t.Fatalf("skipped state = %#v", state)
	}
}

func TestMergeRecoveryPreservesReplicationDomain(t *testing.T) {
	t.Parallel()
	store := New()
	status, oldProgress, newProgress := 1.0, 0.1, 0.9
	store.ReplaceReplications("alpha", []model.Replication{
		{Cluster: "alpha", ID: "group", Group: "rg"},
		{
			Cluster: "alpha", ID: "link", Group: "rg", Status: &status,
			RecoveryProgress: &oldProgress,
		},
	})
	store.MergeRecovery("alpha", []model.Replication{
		{
			Cluster: "alpha", ID: "link", Group: "rg",
			RecoveryKind: stringPointer("bootstrap"), RecoveryProgress: &newProgress,
		},
		{
			Cluster: "alpha", ID: "new-link", Group: "rg",
			RecoveryKind: stringPointer("failover"), RecoveryProgress: &newProgress,
		},
	})
	values := store.Snapshot().Replications
	if len(values) != 3 || values[0].ID != "group" || values[1].ID != "link" ||
		*values[1].Status != 1 || *values[1].RecoveryProgress != 0.9 {
		t.Fatalf("merged recovery = %#v", values)
	}
}

func TestConcurrentReadAndReplace(t *testing.T) {
	t.Parallel()
	store := New()
	var group sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		group.Add(1)
		go func(worker int) {
			defer group.Done()
			for iteration := 0; iteration < 100; iteration++ {
				store.ReplaceNodes("alpha", []model.Node{{
					Cluster: "alpha", ID: "node", Name: "node",
				}})
				_ = store.Snapshot()
			}
		}(worker)
	}
	group.Wait()
	if len(store.Snapshot().Nodes) != 1 {
		t.Fatal("concurrent replacement produced an invalid snapshot")
	}
}

func stringPointer(value string) *string {
	return &value
}
