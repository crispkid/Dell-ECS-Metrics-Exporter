package fluxprobe

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode"

	"dell-ecs-metrics-exporter/internal/cache"
	"dell-ecs-metrics-exporter/internal/collector"
	"dell-ecs-metrics-exporter/internal/config"
	"dell-ecs-metrics-exporter/internal/ecs"
	"dell-ecs-metrics-exporter/internal/model"
	"dell-ecs-metrics-exporter/internal/profile"
)

const (
	SchemaVersion  = "1.0"
	Classification = "live-read-only-redacted"

	ResultPassed  = "passed"
	ResultPartial = "partial"
	ResultFailed  = "failed"
)

// API is the read-only ECS client surface used by the compatibility probe.
type API interface {
	GetBytes(context.Context, string, string, url.Values) ([]byte, error)
	PostBytes(context.Context, string, string, url.Values, any) ([]byte, error)
	Close(context.Context) error
}

type Options struct {
	EnablePerformance bool
	EnableDisk        bool
	GeneratedAt       time.Time
	Build             BuildInfo
}

type BuildInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
}

type Report struct {
	SchemaVersion     string            `json:"schema_version"`
	Classification    string            `json:"classification"`
	GeneratedAt       time.Time         `json:"generated_at"`
	Probe             BuildInfo         `json:"probe"`
	Result            string            `json:"result"`
	ObservedBuilds    []string          `json:"observed_builds,omitempty"`
	SelectedProfiles  []string          `json:"selected_profiles,omitempty"`
	MixedVersion      bool              `json:"mixed_version"`
	CapabilityPolicy  CapabilityPolicy  `json:"capability_policy"`
	Checks            []Check           `json:"checks"`
	NodeSeries        NodeSeries        `json:"node_series"`
	PerformanceSeries PerformanceSeries `json:"performance_series"`
	Redaction         Redaction         `json:"redaction"`
	EvidenceBoundary  string            `json:"evidence_boundary"`
}

type CapabilityPolicy struct {
	FluxIntervalRates    profile.Support `json:"flux_interval_rates,omitempty"`
	VDCPerformance       profile.Support `json:"vdc_performance,omitempty"`
	NamespacePerformance profile.Support `json:"namespace_performance,omitempty"`
	NodeDiskCapacity     profile.Support `json:"node_disk_capacity,omitempty"`
}

type Check struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	ErrorType  string `json:"error_type,omitempty"`
	HTTPStatus int    `json:"http_status,omitempty"`
}

type NodeSeries struct {
	Nodes           int  `json:"nodes"`
	CPUUsage        int  `json:"cpu_usage"`
	MemoryUsed      int  `json:"memory_used"`
	MemoryTotal     int  `json:"memory_total"`
	DiskUsed        int  `json:"disk_used"`
	DiskTotal       int  `json:"disk_total"`
	NetworkReceive  int  `json:"network_receive"`
	NetworkTransmit int  `json:"network_transmit"`
	Empty           bool `json:"empty"`
}

type PerformanceSeries struct {
	Total              int  `json:"total"`
	VDCReadThroughput  int  `json:"vdc_read_throughput"`
	VDCWriteThroughput int  `json:"vdc_write_throughput"`
	VDCLatency         int  `json:"vdc_latency"`
	VDCRequests        int  `json:"vdc_requests"`
	NamespaceRequests  int  `json:"namespace_requests"`
	Empty              bool `json:"empty"`
}

type Redaction struct {
	CredentialsOmitted        bool `json:"credentials_omitted"`
	NetworkEndpointsOmitted   bool `json:"network_endpoints_omitted"`
	ResourceIdentitiesOmitted bool `json:"resource_identities_omitted"`
	MetricValuesOmitted       bool `json:"metric_values_omitted"`
	RawResponsesOmitted       bool `json:"raw_responses_omitted"`
}

// SelectCluster returns one configured cluster without exposing candidate names
// in its errors. A name is required when the configuration has multiple clusters.
func SelectCluster(settings config.Config, name string) (config.ClusterConfig, error) {
	if name == "" {
		if len(settings.ECS.Clusters) != 1 {
			return config.ClusterConfig{}, errors.New("cluster selection is required")
		}
		return settings.ECS.Clusters[0], nil
	}
	for _, candidate := range settings.ECS.Clusters {
		if candidate.Name == name {
			return candidate, nil
		}
	}
	return config.ClusterConfig{}, errors.New("cluster selection is invalid")
}

// Run executes only the same read-only management and Flux queries used by the
// exporter. The report deliberately contains counts and policies, never raw
// metric values, ECS resource identities, endpoints, credentials, or responses.
func Run(
	ctx context.Context,
	clusterConfig config.ClusterConfig,
	settings config.CollectorConfig,
	catalog *profile.Catalog,
	api API,
	options Options,
) Report {
	generatedAt := options.GeneratedAt.UTC()
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}
	report := newReport(generatedAt, options.Build)
	if err := ValidateOptions(clusterConfig, options); err != nil {
		report.Checks = append(report.Checks, Check{
			Name: "setup", Status: "error", ErrorType: "configuration",
		})
		logoutContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		logoutErr := api.Close(logoutContext)
		cancel()
		report.Checks = append(report.Checks, checkResult("logout", logoutErr))
		report.Result = overallResult(report.Checks)
		return report
	}
	clusterConfig.Capabilities.EnabledConditional = slices.Clone(
		clusterConfig.Capabilities.EnabledConditional,
	)
	if options.EnablePerformance {
		clusterConfig.Capabilities.EnabledConditional = appendMissing(
			clusterConfig.Capabilities.EnabledConditional,
			"vdc_performance", "namespace_performance",
		)
	}
	if options.EnableDisk {
		clusterConfig.Capabilities.EnabledConditional = appendMissing(
			clusterConfig.Capabilities.EnabledConditional, "node_disk_capacity",
		)
	}

	store := cache.New()
	runner := collector.NewRunner(
		clusterConfig, settings, api, catalog, store, collector.NopObserver{},
	)
	bootstrapErr := runner.Bootstrap(ctx)
	report.Checks = append(report.Checks, checkResult("bootstrap", bootstrapErr))
	if bootstrapErr == nil {
		report.Checks = append(report.Checks,
			checkResult("cluster_context", runner.CollectCluster(ctx)),
			checkResult("node_inventory", runner.CollectNodes(ctx)),
			checkResult("node_resources", runner.CollectNodeResources(ctx)),
		)
		if options.EnablePerformance {
			report.Checks = append(
				report.Checks,
				checkResult("performance", runner.CollectPerformance(ctx)),
			)
		} else {
			report.Checks = append(report.Checks, skippedCheck("performance"))
		}
	} else {
		for _, name := range []string{
			"cluster_context", "node_inventory", "node_resources", "performance",
		} {
			report.Checks = append(report.Checks, skippedCheck(name))
		}
	}

	logoutContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	logoutErr := api.Close(logoutContext)
	cancel()
	report.Checks = append(report.Checks, checkResult("logout", logoutErr))

	snapshot := store.Snapshot()
	populateCompatibility(&report, catalog, snapshot)
	report.NodeSeries = countNodeSeries(snapshot.Nodes)
	report.PerformanceSeries = countPerformanceSeries(snapshot.Performances)
	report.Result = overallResult(report.Checks)
	return report
}

// SetupFailureReport produces the same redacted schema when execution cannot
// safely start. It intentionally omits the underlying configuration error.
func SetupFailureReport(generatedAt time.Time, build BuildInfo, errorType string) Report {
	report := newReport(generatedAt.UTC(), build)
	report.Result = ResultFailed
	report.Checks = []Check{{Name: "setup", Status: "error", ErrorType: safeType(errorType)}}
	return report
}

func newReport(generatedAt time.Time, build BuildInfo) Report {
	return Report{
		SchemaVersion:  SchemaVersion,
		Classification: Classification,
		GeneratedAt:    generatedAt,
		Probe:          build,
		Redaction: Redaction{
			CredentialsOmitted: true, NetworkEndpointsOmitted: true,
			ResourceIdentitiesOmitted: true, MetricValuesOmitted: true,
			RawResponsesOmitted: true,
		},
		EvidenceBoundary: "candidate-live-compatibility-evidence-not-certification",
	}
}

func populateCompatibility(report *Report, catalog *profile.Catalog, snapshot model.Snapshot) {
	seen := make(map[string]struct{})
	for _, node := range snapshot.Nodes {
		if _, exists := seen[node.Version]; exists {
			continue
		}
		seen[node.Version] = struct{}{}
		report.ObservedBuilds = append(report.ObservedBuilds, node.Version)
	}
	slices.Sort(report.ObservedBuilds)
	if len(report.ObservedBuilds) == 0 {
		return
	}
	resolution, err := catalog.Resolve(report.ObservedBuilds)
	if err != nil {
		report.Checks = append(report.Checks, checkResult("profile_resolution", err))
		return
	}
	report.SelectedProfiles = resolution.ProfileIDs
	report.MixedVersion = resolution.Mixed
	report.CapabilityPolicy = CapabilityPolicy{
		FluxIntervalRates:    resolution.Capabilities["flux_interval_rates"],
		VDCPerformance:       resolution.Capabilities["vdc_performance"],
		NamespacePerformance: resolution.Capabilities["namespace_performance"],
		NodeDiskCapacity:     resolution.Capabilities["node_disk_capacity"],
	}
}

func countNodeSeries(nodes []model.Node) NodeSeries {
	result := NodeSeries{Nodes: len(nodes)}
	for _, node := range nodes {
		if node.CPUUsageRatio != nil {
			result.CPUUsage++
		}
		if node.MemoryUsedBytes != nil {
			result.MemoryUsed++
		}
		if node.MemoryTotalBytes != nil {
			result.MemoryTotal++
		}
		if node.DiskUsedBytes != nil {
			result.DiskUsed++
		}
		if node.DiskTotalBytes != nil {
			result.DiskTotal++
		}
		for _, network := range node.Network {
			if network.ReceiveBytes != nil {
				result.NetworkReceive++
			}
			if network.TransmitBytes != nil {
				result.NetworkTransmit++
			}
		}
	}
	result.Empty = result.CPUUsage+result.MemoryUsed+result.MemoryTotal+
		result.DiskUsed+result.DiskTotal+result.NetworkReceive+result.NetworkTransmit == 0
	return result
}

func countPerformanceSeries(values []model.Performance) PerformanceSeries {
	result := PerformanceSeries{Total: len(values)}
	for _, value := range values {
		if value.Namespace != "" {
			if value.Metric == model.PerformanceRequests {
				result.NamespaceRequests++
			}
			continue
		}
		switch value.Metric {
		case model.PerformanceReadThroughput:
			result.VDCReadThroughput++
		case model.PerformanceWriteThroughput:
			result.VDCWriteThroughput++
		case model.PerformanceLatency:
			result.VDCLatency++
		case model.PerformanceRequests:
			result.VDCRequests++
		}
	}
	result.Empty = result.Total == 0
	return result
}

func checkResult(name string, err error) Check {
	if err == nil {
		return Check{Name: name, Status: "pass"}
	}
	result := Check{Name: name, Status: "error", ErrorType: "mapping"}
	var apiError *ecs.APIError
	if errors.As(err, &apiError) {
		result.ErrorType = safeType(apiError.Kind)
		if apiError.Status >= 100 && apiError.Status <= 599 {
			result.HTTPStatus = apiError.Status
		}
	} else if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		result.ErrorType = "context"
	}
	return result
}

func skippedCheck(name string) Check {
	return Check{Name: name, Status: "skipped"}
}

func overallResult(checks []Check) string {
	result := ResultPassed
	for _, check := range checks {
		if check.Status != "error" {
			continue
		}
		if check.Name == "bootstrap" || check.Name == "setup" || check.Name == "profile_resolution" {
			return ResultFailed
		}
		result = ResultPartial
	}
	return result
}

func appendMissing(values []string, names ...string) []string {
	for _, name := range names {
		if !slices.Contains(values, name) {
			values = append(values, name)
		}
	}
	return values
}

func safeType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || len(value) > 64 || strings.ContainsFunc(value, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_')
	}) {
		return "unknown"
	}
	return value
}

func ValidateOptions(cluster config.ClusterConfig, options Options) error {
	if options.EnableDisk && len(cluster.NodeResources.Filesystems) == 0 {
		return fmt.Errorf("disk probe requires an explicit filesystem allowlist")
	}
	return nil
}
