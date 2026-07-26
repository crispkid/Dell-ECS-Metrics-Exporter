package collector

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"dell-ecs-metrics-exporter/internal/cache"
	"dell-ecs-metrics-exporter/internal/config"
	"dell-ecs-metrics-exporter/internal/ecs"
	"dell-ecs-metrics-exporter/internal/model"
	"dell-ecs-metrics-exporter/internal/profile"
)

type API interface {
	GetBytes(context.Context, string, string, url.Values) ([]byte, error)
	PostBytes(context.Context, string, string, url.Values, any) ([]byte, error)
}

type Observer interface {
	ObserveCollector(
		cluster, collector, result, errorType, correlationID string,
		duration time.Duration,
	)
	ObserveCacheRefresh(cluster, collector, result string)
}

type NopObserver struct{}

func (NopObserver) ObserveCollector(string, string, string, string, string, time.Duration) {}
func (NopObserver) ObserveCacheRefresh(string, string, string)                             {}

type bucketListParam struct {
	IDs []string `json:"id"`
}

type Runner struct {
	config   config.ClusterConfig
	settings config.CollectorConfig
	api      API
	profiles *profile.Catalog
	store    *cache.Store
	observer Observer
	now      func() time.Time

	resolutionMu sync.RWMutex
	resolution   profile.Resolution
	bootstrapMu  sync.Mutex
	nodeMu       sync.Mutex
}

func NewRunner(
	cluster config.ClusterConfig,
	settings config.CollectorConfig,
	api API,
	profiles *profile.Catalog,
	store *cache.Store,
	observer Observer,
) *Runner {
	if observer == nil {
		observer = NopObserver{}
	}
	return &Runner{
		config: cluster, settings: settings, api: api, profiles: profiles,
		store: store, observer: observer, now: time.Now,
	}
}

func (r *Runner) Bootstrap(ctx context.Context) error {
	r.bootstrapMu.Lock()
	defer r.bootstrapMu.Unlock()
	if len(r.currentResolution().ProfileIDs) != 0 {
		return nil
	}
	data, err := r.api.GetBytes(ctx, "version-discovery", "/vdc/nodes", nil)
	if err != nil {
		return err
	}
	nodes, versions, err := ecs.ParseNodes(data, r.config.Name, r.now().UTC())
	if err != nil {
		return err
	}
	resolution, err := r.profiles.Resolve(versions)
	if err != nil {
		return err
	}
	if data, err = r.api.GetBytes(ctx, "whoami", "/user/whoami", nil); err != nil {
		return err
	} else if _, _, err := ecs.ParseWhoAmI(data); err != nil {
		return err
	}
	r.resolutionMu.Lock()
	r.resolution = resolution
	r.resolutionMu.Unlock()
	r.store.ReplaceNodes(r.config.Name, nodes)
	return nil
}

func (r *Runner) CollectCluster(ctx context.Context) error {
	if err := r.ensureBootstrap(ctx); err != nil {
		return err
	}
	now := r.now().UTC()
	healthData, err := r.api.GetBytes(
		ctx, "cluster-health", "/dashboard/zones/localzone",
		url.Values{"dataType": []string{"current"}},
	)
	if err != nil {
		return err
	}
	vdc, health, err := ecs.ParseClusterHealth(healthData)
	if err != nil {
		return err
	}
	capacityData, err := r.api.GetBytes(ctx, "cluster-capacity", "/object/capacity", nil)
	if err != nil {
		return err
	}
	capacity, err := ecs.ParseCapacity(capacityData)
	if err != nil {
		return err
	}
	snapshot := r.store.Snapshot()
	var bucketCount, namespaceCount int
	var objectCount float64
	var hasObjectCount bool
	for _, value := range snapshot.Buckets {
		if value.Cluster == r.config.Name {
			bucketCount++
		}
	}
	for _, value := range snapshot.Namespaces {
		if value.Cluster != r.config.Name {
			continue
		}
		namespaceCount++
		if value.ObjectCount != nil {
			objectCount += *value.ObjectCount
			hasObjectCount = true
		}
	}
	resolution := r.currentResolution()
	cluster := model.Cluster{
		Name: r.config.Name, Site: r.config.Site, Environment: r.config.Environment,
		VDC: vdc, Profiles: slices.Clone(resolution.ProfileIDs), MixedVersion: resolution.Mixed,
		Health: &health, TotalBytes: &capacity.TotalBytes, UsedBytes: &capacity.UsedBytes,
		AvailableBytes: &capacity.AvailableBytes, BucketCount: bucketCount,
		NamespaceCount: namespaceCount, CollectedAt: now,
	}
	for _, value := range snapshot.Nodes {
		if value.Cluster == r.config.Name {
			cluster.Versions = append(cluster.Versions, value.Version)
		}
	}
	if hasObjectCount {
		cluster.ObjectCount = &objectCount
	}
	r.store.ReplaceCluster(r.config.Name, cluster)
	return nil
}

func (r *Runner) CollectNodes(ctx context.Context) error {
	r.nodeMu.Lock()
	defer r.nodeMu.Unlock()
	if err := r.ensureBootstrap(ctx); err != nil {
		return err
	}
	now := r.now().UTC()
	nodeData, err := r.api.GetBytes(ctx, "node-inventory", "/vdc/nodes", nil)
	if err != nil {
		return err
	}
	nodes, versions, err := ecs.ParseNodes(nodeData, r.config.Name, now)
	if err != nil {
		return err
	}
	resolution, err := r.profiles.Resolve(versions)
	if err != nil {
		return err
	}
	healthData, err := r.api.GetBytes(
		ctx, "node-health", "/dashboard/zones/localzone/nodes",
		url.Values{"dataType": []string{"current"}},
	)
	if err != nil {
		return err
	}
	statuses, err := ecs.ParseNodeStatuses(healthData)
	if err != nil {
		return err
	}
	preserveNodeResources(nodes, r.store.Snapshot().Nodes)
	ecs.ApplyNodeStatuses(
		nodes, statuses, r.capabilityEnabledFor(resolution, "node_service_process"),
	)
	r.resolutionMu.Lock()
	r.resolution = resolution
	r.resolutionMu.Unlock()
	r.store.ReplaceNodes(r.config.Name, nodes)
	return nil
}

func (r *Runner) CollectNodeResources(ctx context.Context) error {
	r.nodeMu.Lock()
	defer r.nodeMu.Unlock()
	if err := r.ensureBootstrap(ctx); err != nil {
		return err
	}
	cpuMemoryEnabled := r.capabilityEnabled("node_cpu_memory")
	networkEnabled := r.capabilityEnabled("node_network_counters")
	diskEnabled := r.capabilityEnabled("node_disk_capacity")
	if !cpuMemoryEnabled && !networkEnabled && !diskEnabled {
		return nil
	}
	fluxData, err := r.api.PostBytes(
		ctx, "node-resources", "/flux/api/external/v2/query", nil,
		map[string]string{"query": nodeResourceFluxQuery(
			cpuMemoryEnabled, diskEnabled, networkEnabled,
		)},
	)
	if err != nil {
		return err
	}
	policy := ecs.NodeResourcePolicy{
		NetworkInterfaces:    r.config.NodeResources.NetworkInterfaces,
		MaxNetworkInterfaces: r.config.NodeResources.EffectiveMaxNetworkInterfaces(),
		PreferBondInterfaces: r.config.NodeResources.BondPreferenceEnabled(),
	}
	if diskEnabled {
		policy.Filesystems = r.config.NodeResources.Filesystems
	}
	resources, err := ecs.ParseFluxNodeResourcesWithPolicy(fluxData, policy)
	if err != nil {
		return err
	}
	snapshot := r.store.Snapshot()
	nodes := make([]model.Node, 0, len(snapshot.Nodes))
	for _, node := range snapshot.Nodes {
		if node.Cluster == r.config.Name {
			nodes = append(nodes, node)
		}
	}
	if len(nodes) == 0 {
		return fmt.Errorf("node cache is empty")
	}
	for index := range nodes {
		if cpuMemoryEnabled {
			nodes[index].CPUUsageRatio = nil
			nodes[index].MemoryUsedBytes = nil
			nodes[index].MemoryTotalBytes = nil
		}
		if diskEnabled {
			nodes[index].DiskUsedBytes = nil
			nodes[index].DiskTotalBytes = nil
		}
		if networkEnabled {
			nodes[index].Network = nil
		}
	}
	ecs.ApplyNodeResourcesForCapabilities(
		nodes, resources, cpuMemoryEnabled, diskEnabled, networkEnabled,
	)
	r.store.ReplaceNodeResources(r.config.Name, nodes)
	return nil
}

func (r *Runner) CollectNamespaces(ctx context.Context) error {
	if err := r.ensureBootstrap(ctx); err != nil {
		return err
	}
	now := r.now().UTC()
	data, err := r.api.GetBytes(ctx, "namespace-inventory", "/object/namespaces", nil)
	if err != nil {
		return err
	}
	namespaces, err := ecs.ParseNamespaces(data, r.config.Name, now)
	if err != nil {
		return err
	}
	for index := range namespaces {
		name := namespaces[index].Name
		escaped := url.PathEscape(name)
		quotaData, err := r.api.GetBytes(
			ctx, "namespace-quota", "/object/namespaces/namespace/"+escaped+"/quota", nil,
		)
		if err != nil {
			return err
		}
		quota, err := ecs.ParseNamespaceQuota(quotaData)
		if err != nil {
			return err
		}
		billingData, err := r.api.GetBytes(
			ctx, "namespace-billing", "/object/billing/namespace/"+escaped+"/info",
			url.Values{"sizeunit": []string{"KB"}},
		)
		if err != nil {
			return err
		}
		billing, err := ecs.ParseNamespaceBilling(billingData)
		if err != nil {
			return err
		}
		ecs.ApplyQuotaToNamespace(&namespaces[index], quota)
		ecs.ApplyBillingToNamespace(&namespaces[index], billing)
	}
	snapshot := r.store.Snapshot()
	for index := range namespaces {
		for _, bucket := range snapshot.Buckets {
			if bucket.Cluster == r.config.Name && bucket.Namespace == namespaces[index].Name {
				namespaces[index].BucketCount++
			}
		}
	}
	r.store.ReplaceNamespaces(r.config.Name, namespaces)
	return nil
}

func (r *Runner) CollectBuckets(ctx context.Context) error {
	if err := r.ensureBootstrap(ctx); err != nil {
		return err
	}
	now := r.now().UTC()
	namespaceData, err := r.api.GetBytes(
		ctx, "bucket-namespace-inventory", "/object/namespaces", nil,
	)
	if err != nil {
		return err
	}
	namespaces, err := ecs.ParseNamespaces(namespaceData, r.config.Name, now)
	if err != nil {
		return err
	}
	var buckets []model.Bucket
	for _, namespace := range namespaces {
		values, collectErr := r.collectBucketPages(ctx, now, namespace.Name)
		if collectErr != nil {
			return collectErr
		}
		buckets = append(buckets, values...)
	}
	if err := r.enrichBuckets(ctx, buckets); err != nil {
		return err
	}
	r.store.ReplaceBuckets(r.config.Name, buckets)
	return nil
}

func (r *Runner) CollectPerformance(ctx context.Context) error {
	if err := r.ensureBootstrap(ctx); err != nil {
		return err
	}
	vdcEnabled := r.capabilityEnabled("vdc_performance")
	namespaceEnabled := r.capabilityEnabled("namespace_performance")
	if !vdcEnabled && !namespaceEnabled {
		return nil
	}
	data, err := r.api.PostBytes(
		ctx, "vdc-performance", "/flux/api/external/v2/query", nil,
		map[string]string{"query": performanceFluxQuery(vdcEnabled, namespaceEnabled)},
	)
	if err != nil {
		return err
	}
	defaultVDC := ""
	for _, value := range r.store.Snapshot().Clusters {
		if value.Name == r.config.Name {
			defaultVDC = value.VDC
			break
		}
	}
	values, err := ecs.ParseFluxPerformance(data, r.config.Name, defaultVDC, r.now().UTC())
	if err != nil {
		return err
	}
	values = slices.DeleteFunc(values, func(value model.Performance) bool {
		if value.Namespace == "" {
			return !vdcEnabled
		}
		return !namespaceEnabled
	})
	r.store.ReplacePerformances(r.config.Name, values)
	return nil
}

func (r *Runner) CollectReplication(ctx context.Context) error {
	if err := r.ensureBootstrap(ctx); err != nil {
		return err
	}
	now := r.now().UTC()
	var values []model.Replication
	for _, id := range r.config.Replication.Groups {
		data, err := r.api.GetBytes(
			ctx, "replication-group", "/dashboard/replicationgroups/"+url.PathEscape(id),
			url.Values{"dataType": []string{"current"}},
		)
		if err != nil {
			return err
		}
		value, err := ecs.ParseReplication(data, r.config.Name, now)
		if err != nil {
			return err
		}
		values = append(values, value)
	}
	for _, id := range r.config.Replication.Links {
		data, err := r.api.GetBytes(
			ctx, "replication-link", "/dashboard/rglinks/"+url.PathEscape(id),
			url.Values{"dataType": []string{"current"}},
		)
		if err != nil {
			return err
		}
		value, err := ecs.ParseReplication(data, r.config.Name, now)
		if err != nil {
			return err
		}
		values = append(values, value)
	}
	r.store.ReplaceReplications(r.config.Name, values)
	return nil
}

func (r *Runner) CollectRecovery(ctx context.Context) error {
	if err := r.ensureBootstrap(ctx); err != nil {
		return err
	}
	if !r.capabilityEnabled("recovery_progress") {
		return nil
	}
	now := r.now().UTC()
	values := make([]model.Replication, 0, len(r.config.Replication.Links))
	for _, id := range r.config.Replication.Links {
		data, err := r.api.GetBytes(
			ctx, "recovery-link", "/dashboard/rglinks/"+url.PathEscape(id),
			url.Values{"dataType": []string{"current"}},
		)
		if err != nil {
			return err
		}
		value, err := ecs.ParseReplication(data, r.config.Name, now)
		if err != nil {
			return err
		}
		values = append(values, value)
	}
	r.store.MergeRecovery(r.config.Name, values)
	return nil
}

func (r *Runner) Run(ctx context.Context, name string, function func(context.Context) error) {
	started := r.now()
	if !r.store.Start(r.config.Name, name, started.UTC()) {
		return
	}
	generation := r.store.Generation(r.config.Name, name)
	ctx = ecs.WithCorrelationID(ctx, r.config.Name)
	err := function(ctx)
	finished := r.now()
	result := "success"
	if err != nil {
		result = "error"
	} else if r.store.Generation(r.config.Name, name) == generation {
		result = "skipped"
	}
	if result == "skipped" {
		r.store.FinishSkipped(r.config.Name, name)
	} else {
		r.store.Finish(r.config.Name, name, finished.UTC(), safeCollectorError(err))
	}
	r.observer.ObserveCollector(
		r.config.Name, name, result, collectorErrorType(err), ecs.CorrelationID(ctx),
		finished.Sub(started),
	)
	r.observer.ObserveCacheRefresh(r.config.Name, name, result)
}

func collectorErrorType(err error) string {
	if err == nil {
		return "none"
	}
	var apiError *ecs.APIError
	if errors.As(err, &apiError) {
		return apiError.Kind
	}
	return "mapping"
}

func (r *Runner) currentResolution() profile.Resolution {
	r.resolutionMu.RLock()
	defer r.resolutionMu.RUnlock()
	return profile.Resolution{
		ProfileIDs: slices.Clone(r.resolution.ProfileIDs), Mixed: r.resolution.Mixed,
		Capabilities: cloneCapabilities(r.resolution.Capabilities),
	}
}

func (r *Runner) capabilityEnabled(name string) bool {
	return r.capabilityEnabledFor(r.currentResolution(), name)
}

func (r *Runner) capabilityEnabledFor(resolution profile.Resolution, name string) bool {
	switch resolution.Capabilities[name] {
	case profile.SupportNative, profile.SupportDerived:
		return true
	case profile.SupportConditional:
		return slices.Contains(r.config.Capabilities.EnabledConditional, name)
	default:
		return false
	}
}

func (r *Runner) ensureBootstrap(ctx context.Context) error {
	if len(r.currentResolution().ProfileIDs) != 0 {
		return nil
	}
	return r.Bootstrap(ctx)
}

func (r *Runner) enrichBuckets(ctx context.Context, buckets []model.Bucket) error {
	workers := min(r.settings.Concurrency.MaxConcurrentRequestsPerCluster, len(buckets))
	if workers == 0 {
		return nil
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	jobs := make(chan int)
	errs := make(chan error, 1)
	var group sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for index := range jobs {
				if err := r.enrichBucketQuota(ctx, &buckets[index]); err != nil {
					select {
					case errs <- err:
						cancel()
					default:
					}
					return
				}
			}
		}()
	}
sendJobs:
	for index := range buckets {
		select {
		case jobs <- index:
		case <-ctx.Done():
			break sendJobs
		}
	}
	close(jobs)
	group.Wait()
	select {
	case err := <-errs:
		return err
	default:
		return r.enrichBucketBillings(ctx, buckets)
	}
}

func (r *Runner) collectBucketPages(
	ctx context.Context,
	now time.Time,
	namespace string,
) ([]model.Bucket, error) {
	if namespace == "" {
		return nil, fmt.Errorf("bucket namespace is required")
	}
	const maxPages = 1000
	var buckets []model.Bucket
	marker := ""
	seenMarkers := make(map[string]struct{})
	seenBuckets := make(map[string]struct{})
	for pageNumber := 0; pageNumber < maxPages; pageNumber++ {
		query := url.Values{
			"namespace": []string{namespace},
			"limit":     []string{fmt.Sprint(r.settings.Concurrency.BucketPageSize)},
		}
		if marker != "" {
			query.Set("marker", marker)
		}
		data, err := r.api.GetBytes(ctx, "bucket-inventory", "/object/bucket", query)
		if err != nil {
			return nil, err
		}
		pageBuckets, page, err := ecs.ParseBucketPage(data, r.config.Name, now)
		if err != nil {
			return nil, err
		}
		if len(pageBuckets) > r.settings.Concurrency.BucketPageSize {
			return nil, fmt.Errorf("bucket page exceeds configured page size")
		}
		for _, bucket := range pageBuckets {
			if bucket.Namespace != namespace {
				return nil, fmt.Errorf("bucket inventory returned a different namespace")
			}
			key := bucket.Namespace + "\x00" + bucket.Name
			if _, exists := seenBuckets[key]; exists {
				return nil, fmt.Errorf("bucket inventory contains duplicate across pages")
			}
			seenBuckets[key] = struct{}{}
			buckets = append(buckets, bucket)
		}
		if page.NextMarker == "" {
			return buckets, nil
		}
		if _, exists := seenMarkers[page.NextMarker]; exists || page.NextMarker == marker {
			return nil, fmt.Errorf("bucket pagination marker repeated")
		}
		seenMarkers[page.NextMarker] = struct{}{}
		marker = page.NextMarker
	}
	return nil, fmt.Errorf("bucket pagination exceeded %d pages", maxPages)
}

func (r *Runner) enrichBucketQuota(ctx context.Context, bucket *model.Bucket) error {
	bucketPath := url.PathEscape(bucket.Name)
	quotaData, err := r.api.GetBytes(
		ctx, "bucket-quota", "/object/bucket/"+bucketPath+"/quota",
		url.Values{"namespace": []string{bucket.Namespace}},
	)
	if err != nil {
		return err
	}
	quota, err := ecs.ParseBucketQuota(quotaData)
	if err != nil {
		return err
	}
	ecs.ApplyQuotaToBucket(bucket, quota)
	return nil
}

func (r *Runner) enrichBucketBillings(ctx context.Context, buckets []model.Bucket) error {
	byNamespace := make(map[string][]int)
	for index := range buckets {
		byNamespace[buckets[index].Namespace] = append(byNamespace[buckets[index].Namespace], index)
	}
	for namespace, indexes := range byNamespace {
		escaped := url.PathEscape(namespace)
		ids := make([]string, 0, len(indexes))
		for _, index := range indexes {
			ids = append(ids, buckets[index].Name)
		}
		data, err := r.api.PostBytes(
			ctx, "bucket-billing-batch", "/object/billing/buckets/"+escaped+"/info",
			url.Values{"sizeunit": []string{"KB"}}, bucketListParam{IDs: ids},
		)
		var billings map[string]ecs.Billing
		if err == nil {
			billings, err = ecs.ParseBucketBillings(data)
			if err != nil {
				return err
			}
			requested := make(map[string]struct{}, len(indexes))
			for _, index := range indexes {
				requested[namespace+"\x00"+buckets[index].Name] = struct{}{}
			}
			for key := range billings {
				if _, exists := requested[key]; !exists {
					return fmt.Errorf("bucket billing batch contains unrequested item")
				}
			}
		} else if !batchBillingUnsupported(err) {
			return err
		}
		for _, index := range indexes {
			key := namespace + "\x00" + buckets[index].Name
			billing, exists := billings[key]
			if !exists {
				billing, err = r.collectSingleBucketBilling(ctx, buckets[index])
				if err != nil {
					return err
				}
			}
			ecs.ApplyBillingToBucket(&buckets[index], billing)
		}
	}
	return nil
}

func (r *Runner) collectSingleBucketBilling(
	ctx context.Context,
	bucket model.Bucket,
) (ecs.Billing, error) {
	data, err := r.api.GetBytes(
		ctx, "bucket-billing",
		"/object/billing/buckets/"+url.PathEscape(bucket.Namespace)+"/"+
			url.PathEscape(bucket.Name)+"/info",
		url.Values{"sizeunit": []string{"KB"}},
	)
	if err != nil {
		return ecs.Billing{}, err
	}
	return ecs.ParseBucketBilling(data)
}

func batchBillingUnsupported(err error) bool {
	var apiError *ecs.APIError
	if !errors.As(err, &apiError) {
		return false
	}
	return apiError.Status == 404 ||
		apiError.Status == 405 ||
		apiError.Status == 501
}

func safeCollectorError(err error) error {
	if err == nil {
		return nil
	}
	var apiError *ecs.APIError
	if errors.As(err, &apiError) {
		return errors.New(apiError.Error())
	}
	return errors.New(strings.ReplaceAll(err.Error(), "\n", " "))
}

func cloneCapabilities(source map[string]profile.Support) map[string]profile.Support {
	result := make(map[string]profile.Support, len(source))
	for name, value := range source {
		result[name] = value
	}
	return result
}

func preserveNodeResources(nodes, previous []model.Node) {
	byID := make(map[string]model.Node, len(previous))
	for _, node := range previous {
		byID[node.Cluster+"\x00"+node.ID] = node
	}
	for index := range nodes {
		previousNode, exists := byID[nodes[index].Cluster+"\x00"+nodes[index].ID]
		if !exists {
			continue
		}
		nodes[index].CPUUsageRatio = previousNode.CPUUsageRatio
		nodes[index].MemoryUsedBytes = previousNode.MemoryUsedBytes
		nodes[index].MemoryTotalBytes = previousNode.MemoryTotalBytes
		nodes[index].DiskUsedBytes = previousNode.DiskUsedBytes
		nodes[index].DiskTotalBytes = previousNode.DiskTotalBytes
		nodes[index].Network = slices.Clone(previousNode.Network)
	}
}

func nodeResourceFluxQuery(cpuMemory, disk, network bool) string {
	var measurements []string
	if cpuMemory {
		measurements = append(
			measurements, `r._measurement == "cpu"`, `r._measurement == "mem"`,
		)
	}
	if disk {
		measurements = append(measurements, `r._measurement == "disk"`)
	}
	if network {
		measurements = append(measurements, `r._measurement == "net"`)
	}
	query := `from(bucket: "monitoring_op")
  |> range(start: -10m)
  |> filter(fn: (r) => ` + strings.Join(measurements, " or ") + `)`
	if cpuMemory {
		query += `
  |> filter(fn: (r) => r._measurement != "cpu" or r.cpu == "cpu-total")`
	}
	return query + `
  |> last()`
}

func performanceFluxQuery(vdc, namespace bool) string {
	query := `from(bucket: "monitoring_vdc")
  |> range(start: -10m)
  |> filter(fn: (r) => r._measurement =~ /^cq_performance_/)`
	switch {
	case vdc && !namespace:
		query += `
  |> filter(fn: (r) => (not exists r.namespace) or r.namespace == "")`
	case !vdc && namespace:
		query += `
  |> filter(fn: (r) => exists r.namespace and r.namespace != "")`
	}
	return query + `
  |> last()`
}
