package health

import (
	"slices"
	"time"

	"dell-ecs-metrics-exporter/internal/model"
)

type Status string

const (
	UP       Status = "UP"
	DEGRADED Status = "DEGRADED"
	DOWN     Status = "DOWN"
)

type ClusterStatus struct {
	Cluster       string    `json:"cluster"`
	Status        Status    `json:"status"`
	LastSuccessAt time.Time `json:"lastSuccessAt,omitempty"`
	Reason        string    `json:"reason,omitempty"`
}

type Report struct {
	Status    Status          `json:"status"`
	CheckedAt time.Time       `json:"checkedAt"`
	Clusters  []ClusterStatus `json:"clusters"`
}

func Evaluate(snapshot model.Snapshot, staleTolerance, maxStale time.Duration, now time.Time) Report {
	report := Report{Status: UP, CheckedAt: now.UTC()}
	clusterNames := make([]string, 0, len(snapshot.Clusters))
	for _, cluster := range snapshot.Clusters {
		clusterNames = append(clusterNames, cluster.Name)
	}
	for _, state := range snapshot.States {
		if !slices.Contains(clusterNames, state.Cluster) {
			clusterNames = append(clusterNames, state.Cluster)
		}
	}
	slices.Sort(clusterNames)
	clusterNames = slices.Compact(clusterNames)
	if len(clusterNames) == 0 {
		report.Status = DOWN
		return report
	}

	downCount := 0
	for _, name := range clusterNames {
		status := evaluateCluster(name, snapshot, staleTolerance, maxStale, now)
		report.Clusters = append(report.Clusters, status)
		if status.Status == DEGRADED && report.Status == UP {
			report.Status = DEGRADED
		}
		if status.Status == DOWN {
			downCount++
			report.Status = DEGRADED
		}
	}
	if downCount == len(clusterNames) {
		report.Status = DOWN
	}
	return report
}

func evaluateCluster(
	cluster string,
	snapshot model.Snapshot,
	staleTolerance, maxStale time.Duration,
	now time.Time,
) ClusterStatus {
	result := ClusterStatus{Cluster: cluster, Status: UP}
	required := map[string]bool{"cluster": false, "node": false, "namespace": false, "bucket": false}
	var newestSuccess time.Time
	var oldestSuccess time.Time
	hasError := false
	for _, state := range snapshot.States {
		if state.Cluster != cluster {
			continue
		}
		if _, tracked := required[state.Collector]; tracked && !state.LastSuccessAt.IsZero() {
			required[state.Collector] = true
			if newestSuccess.IsZero() || state.LastSuccessAt.After(newestSuccess) {
				newestSuccess = state.LastSuccessAt
			}
			if oldestSuccess.IsZero() || state.LastSuccessAt.Before(oldestSuccess) {
				oldestSuccess = state.LastSuccessAt
			}
		}
		if state.LastError != "" {
			hasError = true
		}
	}
	result.LastSuccessAt = newestSuccess
	for _, present := range required {
		if !present {
			result.Status = DOWN
			result.Reason = "cache_not_initialized"
			return result
		}
	}
	age := now.Sub(oldestSuccess)
	if age > maxStale {
		result.Status = DOWN
		result.Reason = "cache_too_old"
		return result
	}
	if age > staleTolerance {
		result.Status = DEGRADED
		result.Reason = "cache_stale"
		return result
	}
	for _, value := range snapshot.Clusters {
		if value.Name == cluster && value.Health != nil && *value.Health == 0 {
			result.Status = DEGRADED
			result.Reason = "ecs_unhealthy"
			return result
		}
	}
	if hasError {
		result.Status = DEGRADED
		result.Reason = "collector_error"
	}
	return result
}
