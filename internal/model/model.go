package model

import "time"

type Cluster struct {
	Name           string    `json:"name"`
	Site           string    `json:"site"`
	Environment    string    `json:"environment"`
	VDC            string    `json:"vdc"`
	Versions       []string  `json:"versions"`
	Profiles       []string  `json:"profiles"`
	MixedVersion   bool      `json:"mixedVersion"`
	Health         *float64  `json:"health"`
	TotalBytes     *float64  `json:"totalBytes"`
	UsedBytes      *float64  `json:"usedBytes"`
	AvailableBytes *float64  `json:"availableBytes"`
	BucketCount    int       `json:"bucketCount"`
	NamespaceCount int       `json:"namespaceCount"`
	ObjectCount    *float64  `json:"objectCount"`
	CollectedAt    time.Time `json:"collectedAt"`
}

type Node struct {
	Cluster          string        `json:"cluster"`
	ID               string        `json:"id"`
	Name             string        `json:"name"`
	IP               *string       `json:"ip"`
	Rack             *string       `json:"rack"`
	Version          string        `json:"version"`
	Health           *float64      `json:"health"`
	State            *string       `json:"state"`
	CPUUsageRatio    *float64      `json:"cpuUsageRatio"`
	MemoryUsedBytes  *float64      `json:"memoryUsedBytes"`
	MemoryTotalBytes *float64      `json:"memoryTotalBytes"`
	DiskUsedBytes    *float64      `json:"diskUsedBytes"`
	DiskTotalBytes   *float64      `json:"diskTotalBytes"`
	Network          []Network     `json:"network"`
	Services         []NodeService `json:"services"`
	CollectedAt      time.Time     `json:"collectedAt"`
}

type Network struct {
	Interface     string   `json:"interface"`
	ReceiveBytes  *float64 `json:"receiveBytes"`
	TransmitBytes *float64 `json:"transmitBytes"`
}

type NodeService struct {
	Name   string  `json:"name"`
	Kind   string  `json:"kind"`
	Status string  `json:"status"`
	Health float64 `json:"health"`
}

type Namespace struct {
	Cluster             string     `json:"cluster"`
	Name                string     `json:"name"`
	Owner               *string    `json:"owner"`
	UsedBytes           *float64   `json:"usedBytes"`
	ObjectCount         *float64   `json:"objectCount"`
	BucketCount         int        `json:"bucketCount"`
	SoftQuotaBytes      *float64   `json:"softQuotaBytes"`
	HardQuotaBytes      *float64   `json:"hardQuotaBytes"`
	SoftQuotaConfigured bool       `json:"softQuotaConfigured"`
	HardQuotaConfigured bool       `json:"hardQuotaConfigured"`
	ReplicationGroup    *string    `json:"replicationGroup"`
	AuditEnabled        *bool      `json:"auditEnabled"`
	UsageSampleAt       *time.Time `json:"usageSampleAt"`
	CollectedAt         time.Time  `json:"collectedAt"`
}

type Bucket struct {
	Cluster             string     `json:"cluster"`
	Namespace           string     `json:"namespace"`
	Name                string     `json:"name"`
	Owner               *string    `json:"owner"`
	UsedBytes           *float64   `json:"usedBytes"`
	ObjectCount         *float64   `json:"objectCount"`
	SoftQuotaBytes      *float64   `json:"softQuotaBytes"`
	HardQuotaBytes      *float64   `json:"hardQuotaBytes"`
	SoftQuotaConfigured bool       `json:"softQuotaConfigured"`
	HardQuotaConfigured bool       `json:"hardQuotaConfigured"`
	VersioningEnabled   *bool      `json:"versioningEnabled"`
	EncryptionEnabled   *bool      `json:"encryptionEnabled"`
	ReplicationGroup    *string    `json:"replicationGroup"`
	ObjectLockEnabled   *bool      `json:"objectLockEnabled"`
	Retention           *string    `json:"retention"`
	AuditEnabled        *bool      `json:"auditEnabled"`
	CreatedAt           *time.Time `json:"createdAt"`
	LastModifiedAt      *time.Time `json:"lastModifiedAt"`
	UsageSampleAt       *time.Time `json:"usageSampleAt"`
	CollectedAt         time.Time  `json:"collectedAt"`
}

type PerformanceMetric string

const (
	PerformanceReadThroughput  PerformanceMetric = "read_throughput"
	PerformanceWriteThroughput PerformanceMetric = "write_throughput"
	PerformanceLatency         PerformanceMetric = "latency"
	PerformanceRequests        PerformanceMetric = "requests"
)

type Performance struct {
	Cluster     string            `json:"cluster"`
	VDC         string            `json:"vdc"`
	Namespace   string            `json:"namespace,omitempty"`
	Metric      PerformanceMetric `json:"metric"`
	Operation   string            `json:"operation,omitempty"`
	StatusClass string            `json:"statusClass,omitempty"`
	Quantile    string            `json:"quantile,omitempty"`
	Value       float64           `json:"value"`
	CollectedAt time.Time         `json:"collectedAt"`
}

type Replication struct {
	Cluster          string    `json:"cluster"`
	ID               string    `json:"id"`
	Group            string    `json:"replicationGroup"`
	SourceVDC        *string   `json:"sourceVdc"`
	TargetVDC        *string   `json:"targetVdc"`
	Status           *float64  `json:"status"`
	LagSeconds       *float64  `json:"lagSeconds"`
	PendingBytes     *float64  `json:"pendingBytes"`
	RecoveryKind     *string   `json:"recoveryKind"`
	RecoveryStatus   *string   `json:"recoveryStatus"`
	RecoveryProgress *float64  `json:"recoveryProgress"`
	CollectedAt      time.Time `json:"collectedAt"`
}

type CollectorState struct {
	Cluster       string    `json:"cluster"`
	Collector     string    `json:"collector"`
	LastAttemptAt time.Time `json:"lastAttemptAt"`
	LastSuccessAt time.Time `json:"lastSuccessAt"`
	LastError     string    `json:"lastError,omitempty"`
	Running       bool      `json:"running"`
	Runs          uint64    `json:"runs"`
	Errors        uint64    `json:"errors"`
}

type Snapshot struct {
	Clusters     []Cluster
	Nodes        []Node
	Namespaces   []Namespace
	Buckets      []Bucket
	Performances []Performance
	Replications []Replication
	States       []CollectorState
}
