package cache

import (
	"slices"
	"sync"
	"time"

	"dell-ecs-metrics-exporter/internal/model"
)

type clusterData struct {
	cluster      *model.Cluster
	nodes        []model.Node
	namespaces   []model.Namespace
	buckets      []model.Bucket
	performances []model.Performance
	replications []model.Replication
}

type Store struct {
	mu          sync.RWMutex
	clusters    map[string]*clusterData
	states      map[string]model.CollectorState
	generations map[string]uint64
}

func New() *Store {
	return &Store{
		clusters:    make(map[string]*clusterData),
		states:      make(map[string]model.CollectorState),
		generations: make(map[string]uint64),
	}
}

func (s *Store) ReplaceCluster(name string, value model.Cluster) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data := s.ensureCluster(name)
	copyValue := cloneCluster(value)
	data.cluster = &copyValue
	s.bump(name, "cluster")
}

func (s *Store) ReplaceNodes(name string, values []model.Node) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureCluster(name).nodes = cloneNodes(values)
	s.bump(name, "node")
}

func (s *Store) ReplaceNodeResources(name string, values []model.Node) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureCluster(name).nodes = cloneNodes(values)
	s.bump(name, "node-resources")
}

func (s *Store) ReplaceNamespaces(name string, values []model.Namespace) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureCluster(name).namespaces = cloneNamespaces(values)
	s.bump(name, "namespace")
}

func (s *Store) ReplaceBuckets(name string, values []model.Bucket) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureCluster(name).buckets = cloneBuckets(values)
	s.bump(name, "bucket")
}

func (s *Store) ReplacePerformances(name string, values []model.Performance) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureCluster(name).performances = slices.Clone(values)
	s.bump(name, "performance")
}

func (s *Store) ReplaceReplications(name string, values []model.Replication) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureCluster(name).replications = cloneReplications(values)
	s.bump(name, "replication")
}

func (s *Store) MergeRecovery(name string, values []model.Replication) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data := s.ensureCluster(name)
	positions := make(map[string]int, len(data.replications))
	for index, value := range data.replications {
		positions[value.ID] = index
	}
	for _, value := range values {
		if index, exists := positions[value.ID]; exists {
			data.replications[index].RecoveryKind = clonePointer(value.RecoveryKind)
			data.replications[index].RecoveryStatus = clonePointer(value.RecoveryStatus)
			data.replications[index].RecoveryProgress = clonePointer(value.RecoveryProgress)
			data.replications[index].CollectedAt = value.CollectedAt
			continue
		}
		data.replications = append(data.replications, cloneReplications([]model.Replication{value})[0])
	}
	s.bump(name, "recovery")
}

func (s *Store) Generation(name, resource string) uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.generations[stateKey(name, resource)]
}

func (s *Store) Start(cluster, collector string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := stateKey(cluster, collector)
	state := s.states[key]
	if state.Running {
		return false
	}
	state.Cluster = cluster
	state.Collector = collector
	state.LastAttemptAt = now
	state.Running = true
	state.Runs++
	s.states[key] = state
	return true
}

func (s *Store) Finish(cluster, collector string, now time.Time, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := stateKey(cluster, collector)
	state := s.states[key]
	state.Cluster = cluster
	state.Collector = collector
	state.Running = false
	if err == nil {
		state.LastSuccessAt = now
		state.LastError = ""
	} else {
		state.Errors++
		state.LastError = sanitizeError(err.Error())
	}
	s.states[key] = state
}

func (s *Store) FinishSkipped(cluster, collector string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := stateKey(cluster, collector)
	state := s.states[key]
	state.Cluster = cluster
	state.Collector = collector
	state.Running = false
	state.LastError = ""
	s.states[key] = state
}

func (s *Store) Snapshot() model.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result model.Snapshot
	for _, data := range s.clusters {
		if data.cluster != nil {
			value := cloneCluster(*data.cluster)
			result.Clusters = append(result.Clusters, value)
		}
		result.Nodes = append(result.Nodes, cloneNodes(data.nodes)...)
		result.Namespaces = append(result.Namespaces, cloneNamespaces(data.namespaces)...)
		result.Buckets = append(result.Buckets, cloneBuckets(data.buckets)...)
		result.Performances = append(result.Performances, slices.Clone(data.performances)...)
		result.Replications = append(result.Replications, cloneReplications(data.replications)...)
	}
	for _, state := range s.states {
		result.States = append(result.States, state)
	}
	sortSnapshot(&result)
	return result
}

func (s *Store) ensureCluster(name string) *clusterData {
	data := s.clusters[name]
	if data == nil {
		data = &clusterData{}
		s.clusters[name] = data
	}
	return data
}

func (s *Store) bump(name, resource string) {
	s.generations[stateKey(name, resource)]++
}

func cloneNodes(values []model.Node) []model.Node {
	result := slices.Clone(values)
	for index := range result {
		result[index].IP = clonePointer(values[index].IP)
		result[index].Rack = clonePointer(values[index].Rack)
		result[index].State = clonePointer(values[index].State)
		result[index].Health = clonePointer(values[index].Health)
		result[index].CPUUsageRatio = clonePointer(values[index].CPUUsageRatio)
		result[index].MemoryUsedBytes = clonePointer(values[index].MemoryUsedBytes)
		result[index].MemoryTotalBytes = clonePointer(values[index].MemoryTotalBytes)
		result[index].DiskUsedBytes = clonePointer(values[index].DiskUsedBytes)
		result[index].DiskTotalBytes = clonePointer(values[index].DiskTotalBytes)
		result[index].Network = slices.Clone(values[index].Network)
		result[index].Services = slices.Clone(values[index].Services)
		for networkIndex := range result[index].Network {
			result[index].Network[networkIndex].ReceiveBytes =
				clonePointer(values[index].Network[networkIndex].ReceiveBytes)
			result[index].Network[networkIndex].TransmitBytes =
				clonePointer(values[index].Network[networkIndex].TransmitBytes)
		}
	}
	return result
}

func cloneCluster(value model.Cluster) model.Cluster {
	value.Versions = slices.Clone(value.Versions)
	value.Profiles = slices.Clone(value.Profiles)
	value.Health = clonePointer(value.Health)
	value.TotalBytes = clonePointer(value.TotalBytes)
	value.UsedBytes = clonePointer(value.UsedBytes)
	value.AvailableBytes = clonePointer(value.AvailableBytes)
	value.ObjectCount = clonePointer(value.ObjectCount)
	return value
}

func cloneNamespaces(values []model.Namespace) []model.Namespace {
	result := slices.Clone(values)
	for index := range result {
		result[index].Owner = clonePointer(values[index].Owner)
		result[index].UsedBytes = clonePointer(values[index].UsedBytes)
		result[index].ObjectCount = clonePointer(values[index].ObjectCount)
		result[index].SoftQuotaBytes = clonePointer(values[index].SoftQuotaBytes)
		result[index].HardQuotaBytes = clonePointer(values[index].HardQuotaBytes)
		result[index].ReplicationGroup = clonePointer(values[index].ReplicationGroup)
		result[index].AuditEnabled = clonePointer(values[index].AuditEnabled)
		result[index].UsageSampleAt = clonePointer(values[index].UsageSampleAt)
	}
	return result
}

func cloneBuckets(values []model.Bucket) []model.Bucket {
	result := slices.Clone(values)
	for index := range result {
		result[index].Owner = clonePointer(values[index].Owner)
		result[index].UsedBytes = clonePointer(values[index].UsedBytes)
		result[index].ObjectCount = clonePointer(values[index].ObjectCount)
		result[index].SoftQuotaBytes = clonePointer(values[index].SoftQuotaBytes)
		result[index].HardQuotaBytes = clonePointer(values[index].HardQuotaBytes)
		result[index].VersioningEnabled = clonePointer(values[index].VersioningEnabled)
		result[index].EncryptionEnabled = clonePointer(values[index].EncryptionEnabled)
		result[index].ReplicationGroup = clonePointer(values[index].ReplicationGroup)
		result[index].ObjectLockEnabled = clonePointer(values[index].ObjectLockEnabled)
		result[index].Retention = clonePointer(values[index].Retention)
		result[index].AuditEnabled = clonePointer(values[index].AuditEnabled)
		result[index].CreatedAt = clonePointer(values[index].CreatedAt)
		result[index].LastModifiedAt = clonePointer(values[index].LastModifiedAt)
		result[index].UsageSampleAt = clonePointer(values[index].UsageSampleAt)
	}
	return result
}

func cloneReplications(values []model.Replication) []model.Replication {
	result := slices.Clone(values)
	for index := range result {
		result[index].SourceVDC = clonePointer(values[index].SourceVDC)
		result[index].TargetVDC = clonePointer(values[index].TargetVDC)
		result[index].Status = clonePointer(values[index].Status)
		result[index].LagSeconds = clonePointer(values[index].LagSeconds)
		result[index].PendingBytes = clonePointer(values[index].PendingBytes)
		result[index].RecoveryKind = clonePointer(values[index].RecoveryKind)
		result[index].RecoveryStatus = clonePointer(values[index].RecoveryStatus)
		result[index].RecoveryProgress = clonePointer(values[index].RecoveryProgress)
	}
	return result
}

func clonePointer[T any](value *T) *T {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}

func stateKey(cluster, collector string) string {
	return cluster + "\x00" + collector
}

func sanitizeError(value string) string {
	const limit = 256
	if len(value) > limit {
		return value[:limit]
	}
	return value
}

func sortSnapshot(snapshot *model.Snapshot) {
	slices.SortFunc(snapshot.Clusters, func(a, b model.Cluster) int { return compare(a.Name, b.Name) })
	slices.SortFunc(snapshot.Nodes, func(a, b model.Node) int {
		if value := compare(a.Cluster, b.Cluster); value != 0 {
			return value
		}
		return compare(a.ID, b.ID)
	})
	slices.SortFunc(snapshot.Namespaces, func(a, b model.Namespace) int {
		if value := compare(a.Cluster, b.Cluster); value != 0 {
			return value
		}
		return compare(a.Name, b.Name)
	})
	slices.SortFunc(snapshot.Buckets, func(a, b model.Bucket) int {
		if value := compare(a.Cluster, b.Cluster); value != 0 {
			return value
		}
		if value := compare(a.Namespace, b.Namespace); value != 0 {
			return value
		}
		return compare(a.Name, b.Name)
	})
	slices.SortFunc(snapshot.Replications, func(a, b model.Replication) int {
		if value := compare(a.Cluster, b.Cluster); value != 0 {
			return value
		}
		return compare(a.ID, b.ID)
	})
	slices.SortFunc(snapshot.Performances, func(a, b model.Performance) int {
		for _, value := range [][2]string{
			{a.Cluster, b.Cluster}, {a.VDC, b.VDC}, {a.Namespace, b.Namespace},
			{string(a.Metric), string(b.Metric)}, {a.Operation, b.Operation},
			{a.StatusClass, b.StatusClass}, {a.Quantile, b.Quantile},
		} {
			if order := compare(value[0], value[1]); order != 0 {
				return order
			}
		}
		return 0
	})
	slices.SortFunc(snapshot.States, func(a, b model.CollectorState) int {
		if value := compare(a.Cluster, b.Cluster); value != 0 {
			return value
		}
		return compare(a.Collector, b.Collector)
	})
}

func compare(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
