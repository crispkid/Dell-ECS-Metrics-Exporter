package metrics

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"dell-ecs-metrics-exporter/internal/cache"
	"dell-ecs-metrics-exporter/internal/model"
	"dell-ecs-metrics-exporter/internal/profile"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type BuildInfo struct {
	Version   string
	Commit    string
	BuildDate string
}

type Metrics struct {
	registry           *prometheus.Registry
	apiRequests        *prometheus.CounterVec
	apiErrors          *prometheus.CounterVec
	apiDuration        *prometheus.HistogramVec
	apiResponseSize    *prometheus.HistogramVec
	collectorRuns      *prometheus.CounterVec
	collectorErrors    *prometheus.CounterVec
	collectorDuration  *prometheus.HistogramVec
	cacheRefresh       *prometheus.CounterVec
	cacheRefreshErrors *prometheus.CounterVec
}

func New(
	store *cache.Store,
	catalog *profile.Catalog,
	build BuildInfo,
	maxStale time.Duration,
) *Metrics {
	registry := prometheus.NewRegistry()
	value := &Metrics{
		registry: registry,
		apiRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "ecs_exporter_api_requests_total", Help: "Dell ECS logical API requests.",
		}, []string{"cluster", "api", "result"}),
		apiErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "ecs_exporter_api_errors_total", Help: "Dell ECS logical API errors.",
		}, []string{"cluster", "api"}),
		apiDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "ecs_exporter_api_request_duration_seconds",
			Help: "Dell ECS logical API request duration.", Buckets: prometheus.DefBuckets,
		}, []string{"cluster", "api"}),
		apiResponseSize: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "ecs_exporter_api_response_size_bytes",
			Help:    "Dell ECS logical API response size.",
			Buckets: prometheus.ExponentialBuckets(1_024, 4, 9),
		}, []string{"cluster", "api", "result"}),
		collectorRuns: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "ecs_exporter_collector_runs_total", Help: "Collector executions.",
		}, []string{"cluster", "collector", "result"}),
		collectorErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "ecs_exporter_collector_errors_total", Help: "Collector errors.",
		}, []string{"cluster", "collector"}),
		collectorDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "ecs_exporter_collector_duration_seconds",
			Help: "Collector execution duration.", Buckets: prometheus.DefBuckets,
		}, []string{"cluster", "collector"}),
		cacheRefresh: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "ecs_exporter_cache_refresh_total", Help: "Successful cache refreshes.",
		}, []string{"cluster", "collector"}),
		cacheRefreshErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "ecs_exporter_cache_refresh_errors_total", Help: "Failed cache refreshes.",
		}, []string{"cluster", "collector"}),
	}
	registry.MustRegister(
		value.apiRequests, value.apiErrors, value.apiDuration, value.apiResponseSize,
		value.collectorRuns, value.collectorErrors, value.collectorDuration,
		value.cacheRefresh, value.cacheRefreshErrors,
		newSnapshotCollector(store, catalog, build, maxStale),
	)
	return value
}

func (m *Metrics) ObserveAPIResponseSize(cluster, logical, result string, bytes int64) {
	if bytes < 0 {
		return
	}
	m.apiResponseSize.WithLabelValues(label(cluster), label(logical), label(result)).
		Observe(float64(bytes))
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{
		ErrorHandling: promhttp.ContinueOnError,
	})
}

func (m *Metrics) ObserveAPI(
	cluster, logical, result, _ string, _ string,
	status, _ int,
	duration time.Duration,
) {
	cluster = label(cluster)
	logical = label(logical)
	result = label(result)
	m.apiRequests.WithLabelValues(cluster, logical, result).Inc()
	m.apiDuration.WithLabelValues(cluster, logical).Observe(duration.Seconds())
	if result != "success" || status >= 400 {
		m.apiErrors.WithLabelValues(cluster, logical).Inc()
	}
}

func (m *Metrics) ObserveCollector(
	cluster, collector, result, _ string, _ string,
	duration time.Duration,
) {
	cluster, collector, result = label(cluster), label(collector), label(result)
	m.collectorRuns.WithLabelValues(cluster, collector, result).Inc()
	m.collectorDuration.WithLabelValues(cluster, collector).Observe(duration.Seconds())
	if result == "error" {
		m.collectorErrors.WithLabelValues(cluster, collector).Inc()
	}
}

func (m *Metrics) ObserveCacheRefresh(cluster, collector, result string) {
	cluster, collector = label(cluster), label(collector)
	if result == "success" {
		m.cacheRefresh.WithLabelValues(cluster, collector).Inc()
	} else if result == "error" {
		m.cacheRefreshErrors.WithLabelValues(cluster, collector).Inc()
	}
}

type snapshotCollector struct {
	store       *cache.Store
	catalog     *profile.Catalog
	build       BuildInfo
	maxStale    time.Duration
	now         func() time.Time
	descriptors map[string]*prometheus.Desc
}

func newSnapshotCollector(
	store *cache.Store,
	catalog *profile.Catalog,
	build BuildInfo,
	maxStale time.Duration,
) *snapshotCollector {
	descriptor := func(name, help string, labels ...string) *prometheus.Desc {
		return prometheus.NewDesc(name, help, labels, nil)
	}
	return &snapshotCollector{
		store: store, catalog: catalog, build: build, maxStale: maxStale, now: time.Now,
		descriptors: map[string]*prometheus.Desc{
			"build":              descriptor("ecs_exporter_build_info", "Exporter build information.", "version", "commit", "build_date"),
			"profile":            descriptor("ecs_exporter_profile_contract_info", "Loaded ECS profile contract.", "profile", "evidence_status", "sandbox_certified"),
			"last_success":       descriptor("ecs_exporter_last_success_timestamp_seconds", "Collector last success timestamp.", "cluster", "collector"),
			"cache_age":          descriptor("ecs_exporter_cache_age_seconds", "Collector cache age.", "cluster", "collector"),
			"cached":             descriptor("ecs_exporter_cached_resources", "Cached resources.", "cluster", "resource_type"),
			"cluster_health":     descriptor("ecs_cluster_health", "Dell ECS cluster health.", "cluster", "site", "environment", "vdc"),
			"cluster_total":      descriptor("ecs_cluster_capacity_total_bytes", "Dell ECS cluster total capacity.", "cluster", "site", "environment", "vdc"),
			"cluster_used":       descriptor("ecs_cluster_capacity_used_bytes", "Dell ECS cluster used capacity.", "cluster", "site", "environment", "vdc"),
			"cluster_available":  descriptor("ecs_cluster_capacity_available_bytes", "Dell ECS cluster available capacity.", "cluster", "site", "environment", "vdc"),
			"cluster_buckets":    descriptor("ecs_cluster_buckets", "Dell ECS cluster bucket count.", "cluster", "site", "environment", "vdc"),
			"cluster_namespaces": descriptor("ecs_cluster_namespaces", "Dell ECS cluster namespace count.", "cluster", "site", "environment", "vdc"),
			"cluster_objects":    descriptor("ecs_cluster_objects", "Dell ECS cluster object count.", "cluster", "site", "environment", "vdc"),
			"node_health":        descriptor("ecs_node_health", "Dell ECS node health.", "cluster", "node"),
			"node_cpu":           descriptor("ecs_node_cpu_usage_ratio", "Dell ECS node CPU usage.", "cluster", "node"),
			"node_mem_used":      descriptor("ecs_node_memory_used_bytes", "Dell ECS node used memory.", "cluster", "node"),
			"node_mem_total":     descriptor("ecs_node_memory_total_bytes", "Dell ECS node total memory.", "cluster", "node"),
			"node_disk_used":     descriptor("ecs_node_disk_used_bytes", "Dell ECS node used disk.", "cluster", "node"),
			"node_disk_total":    descriptor("ecs_node_disk_total_bytes", "Dell ECS node total disk.", "cluster", "node"),
			"node_rx":            descriptor("ecs_node_network_receive_bytes_total", "Dell ECS node cumulative received bytes.", "cluster", "node", "interface"),
			"node_tx":            descriptor("ecs_node_network_transmit_bytes_total", "Dell ECS node cumulative transmitted bytes.", "cluster", "node", "interface"),
			"node_service":       descriptor("ecs_node_service_health", "Dell ECS node service or process health.", "cluster", "node", "kind", "service"),
			"namespace_used":     descriptor("ecs_namespace_capacity_used_bytes", "Dell ECS namespace used capacity.", "cluster", "namespace"),
			"namespace_quota":    descriptor("ecs_namespace_quota_bytes", "Dell ECS namespace hard quota.", "cluster", "namespace"),
			"namespace_buckets":  descriptor("ecs_namespace_buckets", "Dell ECS namespace bucket count.", "cluster", "namespace"),
			"namespace_objects":  descriptor("ecs_namespace_objects", "Dell ECS namespace object count.", "cluster", "namespace"),
			"bucket_used":        descriptor("ecs_bucket_used_bytes", "Dell ECS bucket used capacity.", "cluster", "namespace", "bucket"),
			"bucket_soft":        descriptor("ecs_bucket_soft_quota_bytes", "Dell ECS bucket soft quota.", "cluster", "namespace", "bucket"),
			"bucket_hard":        descriptor("ecs_bucket_hard_quota_bytes", "Dell ECS bucket hard quota.", "cluster", "namespace", "bucket"),
			"bucket_objects":     descriptor("ecs_bucket_objects", "Dell ECS bucket object count.", "cluster", "namespace", "bucket"),
			"vdc_read":           descriptor("ecs_vdc_read_throughput_bytes_per_second", "Dell ECS VDC read throughput.", "cluster", "vdc"),
			"vdc_write":          descriptor("ecs_vdc_write_throughput_bytes_per_second", "Dell ECS VDC write throughput.", "cluster", "vdc"),
			"vdc_latency":        descriptor("ecs_vdc_request_latency_seconds", "Dell ECS VDC request latency.", "cluster", "vdc", "operation", "quantile"),
			"vdc_requests":       descriptor("ecs_vdc_requests", "Dell ECS VDC request rate per second.", "cluster", "vdc", "operation", "status_class"),
			"namespace_requests": descriptor("ecs_namespace_requests", "Dell ECS Namespace request rate per second.", "cluster", "vdc", "namespace", "operation", "status_class"),
			"replication_status": descriptor("ecs_replication_status", "Dell ECS replication status.", "cluster", "replication_group", "source_vdc", "target_vdc"),
			"replication_lag":    descriptor("ecs_replication_lag_seconds", "Dell ECS replication lag.", "cluster", "replication_group", "source_vdc", "target_vdc"),
			"recovery_progress":  descriptor("ecs_recovery_progress_ratio", "Dell ECS recovery progress.", "cluster", "replication_group", "source_vdc", "target_vdc", "kind"),
		}}
}

func (c *snapshotCollector) Describe(channel chan<- *prometheus.Desc) {
	for _, descriptor := range c.descriptors {
		channel <- descriptor
	}
}

func (c *snapshotCollector) Collect(channel chan<- prometheus.Metric) {
	now := c.now()
	channel <- gauge(c.descriptors["build"], 1, c.build.Version, c.build.Commit, c.build.BuildDate)
	for _, value := range c.catalog.Profiles() {
		channel <- gauge(c.descriptors["profile"], 1, value.ProfileID, value.Evidence.Status, boolString(value.Evidence.SandboxCertified))
	}
	snapshot := c.store.Snapshot()
	c.collectState(channel, snapshot, now)
	for _, value := range snapshot.Clusters {
		if !c.domainFresh(snapshot, value.Name, "cluster", now) {
			continue
		}
		labels := []string{label(value.Name), label(value.Site), label(value.Environment), label(value.VDC)}
		emitOptional(channel, c.descriptors["cluster_health"], prometheus.GaugeValue, value.Health, labels...)
		emitOptional(channel, c.descriptors["cluster_total"], prometheus.GaugeValue, value.TotalBytes, labels...)
		emitOptional(channel, c.descriptors["cluster_used"], prometheus.GaugeValue, value.UsedBytes, labels...)
		emitOptional(channel, c.descriptors["cluster_available"], prometheus.GaugeValue, value.AvailableBytes, labels...)
		channel <- gauge(c.descriptors["cluster_buckets"], float64(value.BucketCount), labels...)
		channel <- gauge(c.descriptors["cluster_namespaces"], float64(value.NamespaceCount), labels...)
		emitOptional(channel, c.descriptors["cluster_objects"], prometheus.GaugeValue, value.ObjectCount, labels...)
	}
	for _, value := range snapshot.Nodes {
		if !c.domainFresh(snapshot, value.Cluster, "node", now) {
			continue
		}
		labels := []string{label(value.Cluster), label(value.Name)}
		emitOptional(channel, c.descriptors["node_health"], prometheus.GaugeValue, value.Health, labels...)
		for _, service := range value.Services {
			channel <- gauge(
				c.descriptors["node_service"], service.Health, labels[0], labels[1],
				label(service.Kind), label(service.Name),
			)
		}
		if c.domainFresh(snapshot, value.Cluster, "node-resources", now) {
			emitOptional(channel, c.descriptors["node_cpu"], prometheus.GaugeValue, value.CPUUsageRatio, labels...)
			emitOptional(channel, c.descriptors["node_mem_used"], prometheus.GaugeValue, value.MemoryUsedBytes, labels...)
			emitOptional(channel, c.descriptors["node_mem_total"], prometheus.GaugeValue, value.MemoryTotalBytes, labels...)
			emitOptional(channel, c.descriptors["node_disk_used"], prometheus.GaugeValue, value.DiskUsedBytes, labels...)
			emitOptional(channel, c.descriptors["node_disk_total"], prometheus.GaugeValue, value.DiskTotalBytes, labels...)
			for _, network := range value.Network {
				networkLabels := []string{labels[0], labels[1], label(network.Interface)}
				emitOptional(channel, c.descriptors["node_rx"], prometheus.CounterValue, network.ReceiveBytes, networkLabels...)
				emitOptional(channel, c.descriptors["node_tx"], prometheus.CounterValue, network.TransmitBytes, networkLabels...)
			}
		}
	}
	for _, value := range snapshot.Namespaces {
		if !c.domainFresh(snapshot, value.Cluster, "namespace", now) {
			continue
		}
		labels := []string{label(value.Cluster), label(value.Name)}
		emitOptional(channel, c.descriptors["namespace_used"], prometheus.GaugeValue, value.UsedBytes, labels...)
		emitOptional(channel, c.descriptors["namespace_quota"], prometheus.GaugeValue, value.HardQuotaBytes, labels...)
		channel <- gauge(c.descriptors["namespace_buckets"], float64(value.BucketCount), labels...)
		emitOptional(channel, c.descriptors["namespace_objects"], prometheus.GaugeValue, value.ObjectCount, labels...)
	}
	for _, value := range snapshot.Buckets {
		if !c.domainFresh(snapshot, value.Cluster, "bucket", now) {
			continue
		}
		labels := []string{label(value.Cluster), label(value.Namespace), label(value.Name)}
		emitOptional(channel, c.descriptors["bucket_used"], prometheus.GaugeValue, value.UsedBytes, labels...)
		emitOptional(channel, c.descriptors["bucket_soft"], prometheus.GaugeValue, value.SoftQuotaBytes, labels...)
		emitOptional(channel, c.descriptors["bucket_hard"], prometheus.GaugeValue, value.HardQuotaBytes, labels...)
		emitOptional(channel, c.descriptors["bucket_objects"], prometheus.GaugeValue, value.ObjectCount, labels...)
	}
	for _, value := range snapshot.Performances {
		if !c.domainFresh(snapshot, value.Cluster, "performance", now) {
			continue
		}
		c.collectPerformance(channel, value)
	}
	for _, value := range snapshot.Replications {
		if !c.domainFresh(snapshot, value.Cluster, "replication", now) {
			continue
		}
		source, target := optionalString(value.SourceVDC), optionalString(value.TargetVDC)
		labels := []string{label(value.Cluster), label(value.Group), label(source), label(target)}
		emitOptional(channel, c.descriptors["replication_status"], prometheus.GaugeValue, value.Status, labels...)
		emitOptional(channel, c.descriptors["replication_lag"], prometheus.GaugeValue, value.LagSeconds, labels...)
		if value.RecoveryKind != nil &&
			c.domainFresh(snapshot, value.Cluster, "recovery", now) {
			emitOptional(
				channel, c.descriptors["recovery_progress"], prometheus.GaugeValue,
				value.RecoveryProgress, labels[0], labels[1], labels[2], labels[3],
				label(*value.RecoveryKind),
			)
		}
	}
}

func (c *snapshotCollector) collectPerformance(
	channel chan<- prometheus.Metric,
	value model.Performance,
) {
	cluster, vdc := label(value.Cluster), label(value.VDC)
	namespace := label(value.Namespace)
	if namespace == "" {
		switch value.Metric {
		case model.PerformanceReadThroughput:
			channel <- gauge(c.descriptors["vdc_read"], value.Value, cluster, vdc)
		case model.PerformanceWriteThroughput:
			channel <- gauge(c.descriptors["vdc_write"], value.Value, cluster, vdc)
		case model.PerformanceLatency:
			channel <- gauge(
				c.descriptors["vdc_latency"], value.Value, cluster, vdc,
				label(value.Operation), label(value.Quantile),
			)
		case model.PerformanceRequests:
			channel <- gauge(
				c.descriptors["vdc_requests"], value.Value, cluster, vdc,
				label(value.Operation), label(value.StatusClass),
			)
		}
		return
	}
	switch value.Metric {
	case model.PerformanceRequests:
		channel <- gauge(
			c.descriptors["namespace_requests"], value.Value, cluster, vdc, namespace,
			label(value.Operation), label(value.StatusClass),
		)
	}
}

func (c *snapshotCollector) domainFresh(
	snapshot model.Snapshot,
	cluster, collector string,
	now time.Time,
) bool {
	found := false
	for _, state := range snapshot.States {
		if state.Cluster != cluster || state.Collector != collector {
			continue
		}
		found = true
		if state.LastSuccessAt.IsZero() {
			return false
		}
		return c.maxStale <= 0 || now.Sub(state.LastSuccessAt) <= c.maxStale
	}
	return !found
}

func (c *snapshotCollector) collectState(channel chan<- prometheus.Metric, snapshot model.Snapshot, now time.Time) {
	counts := make(map[string]map[string]int)
	for _, cluster := range snapshot.Clusters {
		counts[cluster.Name] = map[string]int{"cluster": 1}
	}
	for _, value := range snapshot.Nodes {
		increment(counts, value.Cluster, "node")
	}
	for _, value := range snapshot.Namespaces {
		increment(counts, value.Cluster, "namespace")
	}
	for _, value := range snapshot.Buckets {
		increment(counts, value.Cluster, "bucket")
	}
	for _, value := range snapshot.Replications {
		increment(counts, value.Cluster, "replication")
	}
	for _, value := range snapshot.Performances {
		increment(counts, value.Cluster, "performance")
	}
	for cluster, resources := range counts {
		for kind, count := range resources {
			channel <- gauge(c.descriptors["cached"], float64(count), label(cluster), kind)
		}
	}
	for _, state := range snapshot.States {
		if state.LastSuccessAt.IsZero() {
			continue
		}
		labels := []string{label(state.Cluster), label(state.Collector)}
		channel <- gauge(c.descriptors["last_success"], float64(state.LastSuccessAt.Unix()), labels...)
		age := max(now.Sub(state.LastSuccessAt).Seconds(), 0)
		channel <- gauge(c.descriptors["cache_age"], age, labels...)
	}
}

func emitOptional(channel chan<- prometheus.Metric, descriptor *prometheus.Desc, valueType prometheus.ValueType, value *float64, labels ...string) {
	if value != nil {
		channel <- prometheus.MustNewConstMetric(descriptor, valueType, *value, labels...)
	}
}

func gauge(descriptor *prometheus.Desc, value float64, labels ...string) prometheus.Metric {
	return prometheus.MustNewConstMetric(descriptor, prometheus.GaugeValue, value, labels...)
}

func label(value string) string {
	trimmed := strings.TrimSpace(value)
	var builder strings.Builder
	changed := trimmed != value
	for _, character := range trimmed {
		if unicode.IsControl(character) {
			changed = true
			continue
		}
		builder.WriteRune(character)
	}
	sanitized := builder.String()
	if !changed && len(sanitized) <= 128 {
		return sanitized
	}
	sum := sha256.Sum256([]byte(value))
	const prefixLimit = 63
	prefix := sanitized
	if len(prefix) > prefixLimit {
		prefix = prefix[:prefixLimit]
	}
	for !utf8.ValidString(prefix) {
		prefix = prefix[:len(prefix)-1]
	}
	return prefix + "-" + hex.EncodeToString(sum[:])
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func optionalString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func increment(counts map[string]map[string]int, cluster, kind string) {
	if counts[cluster] == nil {
		counts[cluster] = make(map[string]int)
	}
	counts[cluster][kind]++
}
