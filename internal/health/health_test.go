package health

import (
	"testing"
	"time"

	"dell-ecs-metrics-exporter/internal/model"
)

func TestEvaluateLifecycleAndPartialFailure(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	if report := Evaluate(model.Snapshot{}, time.Minute, 10*time.Minute, now); report.Status != DOWN {
		t.Fatalf("empty status = %s", report.Status)
	}

	healthy := 1.0
	snapshot := model.Snapshot{Clusters: []model.Cluster{
		{Name: "alpha", Health: &healthy},
		{Name: "beta", Health: &healthy},
	}}
	addStates := func(cluster string, success time.Time, failure string) {
		for _, collector := range []string{"cluster", "node", "namespace", "bucket"} {
			snapshot.States = append(snapshot.States, model.CollectorState{
				Cluster: cluster, Collector: collector, LastSuccessAt: success, LastError: failure,
			})
		}
	}
	addStates("alpha", now.Add(-30*time.Second), "")
	addStates("beta", now.Add(-2*time.Minute), "")
	report := Evaluate(snapshot, time.Minute, 10*time.Minute, now)
	if report.Status != DEGRADED || report.Clusters[0].Status != UP ||
		report.Clusters[1].Reason != "cache_stale" {
		t.Fatalf("partial report = %#v", report)
	}

	for index := range snapshot.States {
		snapshot.States[index].LastSuccessAt = now.Add(-11 * time.Minute)
	}
	if report = Evaluate(snapshot, time.Minute, 10*time.Minute, now); report.Status != DOWN {
		t.Fatalf("all stale report = %#v", report)
	}
}

func TestEvaluateMissingErrorAndECSHealth(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	unhealthy := 0.0
	snapshot := model.Snapshot{Clusters: []model.Cluster{{Name: "alpha", Health: &unhealthy}}}
	snapshot.States = append(snapshot.States, model.CollectorState{
		Cluster: "alpha", Collector: "cluster", LastSuccessAt: now,
	})
	report := Evaluate(snapshot, time.Minute, time.Hour, now)
	if report.Clusters[0].Reason != "cache_not_initialized" {
		t.Fatalf("missing report = %#v", report)
	}
	for _, collector := range []string{"node", "namespace", "bucket"} {
		snapshot.States = append(snapshot.States, model.CollectorState{
			Cluster: "alpha", Collector: collector, LastSuccessAt: now,
		})
	}
	report = Evaluate(snapshot, time.Minute, time.Hour, now)
	if report.Status != DEGRADED || report.Clusters[0].Reason != "ecs_unhealthy" {
		t.Fatalf("unhealthy report = %#v", report)
	}
	healthy := 1.0
	snapshot.Clusters[0].Health = &healthy
	snapshot.States[0].LastError = "safe error"
	report = Evaluate(snapshot, time.Minute, time.Hour, now)
	if report.Status != DEGRADED || report.Clusters[0].Reason != "collector_error" {
		t.Fatalf("error report = %#v", report)
	}
}
