package ecs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strings"
	"time"
	"unicode"

	"dell-ecs-metrics-exporter/internal/model"
)

const (
	binaryKB  = 1_024
	decimalGB = 1_000_000_000
)

type Capacity struct {
	TotalBytes     float64
	UsedBytes      float64
	AvailableBytes float64
}

type Quota struct {
	SoftBytes *float64
	HardBytes *float64
}

type Billing struct {
	UsedBytes   *float64
	ObjectCount *float64
	SampleTime  *time.Time
}

type NodeResource struct {
	CPUUsageRatio    *float64
	MemoryUsedBytes  *float64
	MemoryTotalBytes *float64
	DiskUsedBytes    *float64
	DiskTotalBytes   *float64
	Network          []model.Network
}

type NodeResourcePolicy struct {
	Filesystems          []string
	NetworkInterfaces    []string
	MaxNetworkInterfaces int
	PreferBondInterfaces bool
}

type NodeStatus struct {
	Health   float64
	Services []model.NodeService
}

type Page struct {
	NextMarker string
}

func ParseWhoAmI(data []byte) (string, []string, error) {
	var response map[string]any
	if err := decodeJSON(data, &response); err != nil {
		return "", nil, err
	}
	user := response
	if nested, ok := response["user"].(map[string]any); ok {
		user = nested
	}
	name := firstString(user, "common_name", "username", "name")
	if name == "" {
		return "", nil, fmt.Errorf("whoami identity is required")
	}
	roles := collectStrings(user["roles"])
	if roleObject, ok := user["roles"].(map[string]any); ok {
		roles = collectStrings(roleObject["role"])
	}
	if len(roles) == 0 {
		roles = collectStrings(user["role"])
	}
	allowed := slices.ContainsFunc(roles, func(role string) bool {
		return strings.EqualFold(role, "SYSTEM_MONITOR") ||
			strings.EqualFold(role, "SYSTEM_ADMIN")
	})
	if !allowed {
		return "", nil, fmt.Errorf("whoami requires SYSTEM_MONITOR or SYSTEM_ADMIN role")
	}
	slices.Sort(roles)
	roles = slices.Compact(roles)
	return name, roles, nil
}

func ParseCapacity(data []byte) (Capacity, error) {
	type rawCapacity struct {
		Total Number `json:"totalProvisioned_gb"`
		Free  Number `json:"totalFree_gb"`
	}
	var response struct {
		Capacity rawCapacity `json:"cluster_capacity"`
		Total    Number      `json:"totalProvisioned_gb"`
		Free     Number      `json:"totalFree_gb"`
	}
	if err := decodeJSON(data, &response); err != nil {
		return Capacity{}, err
	}
	topLevel := response.Total.valid || response.Free.valid
	nested := response.Capacity.Total.valid || response.Capacity.Free.valid
	if topLevel && nested {
		return Capacity{}, fmt.Errorf("cluster capacity contains ambiguous envelopes")
	}
	raw := response.Capacity
	fieldPrefix := "cluster_capacity."
	if topLevel {
		raw = rawCapacity{Total: response.Total, Free: response.Free}
		fieldPrefix = ""
	}
	total, err := raw.Total.Required(fieldPrefix + "totalProvisioned_gb")
	if err != nil {
		return Capacity{}, err
	}
	free, err := raw.Free.Required(fieldPrefix + "totalFree_gb")
	if err != nil {
		return Capacity{}, err
	}
	if total < 0 || free < 0 || free > total {
		return Capacity{}, fmt.Errorf("cluster capacity values are inconsistent")
	}
	totalBytes, err := scaleFinite(total, decimalGB, "cluster total capacity")
	if err != nil {
		return Capacity{}, err
	}
	freeBytes, err := scaleFinite(free, decimalGB, "cluster free capacity")
	if err != nil {
		return Capacity{}, err
	}
	return Capacity{
		TotalBytes: totalBytes, UsedBytes: totalBytes - freeBytes,
		AvailableBytes: freeBytes,
	}, nil
}

func ParseClusterHealth(data []byte) (string, float64, error) {
	var response struct {
		Name      string `json:"name"`
		Nodes     Number `json:"numNodes"`
		GoodNodes Number `json:"numGoodNodes"`
		BadNodes  Number `json:"numBadNodes"`
		Disks     Number `json:"numDisks"`
		GoodDisks Number `json:"numGoodDisks"`
		BadDisks  Number `json:"numBadDisks"`
	}
	if err := decodeJSON(data, &response); err != nil {
		return "", 0, err
	}
	if response.Name == "" {
		return "", 0, fmt.Errorf("cluster health name is required")
	}
	values := []struct {
		name string
		raw  Number
	}{
		{"numNodes", response.Nodes}, {"numGoodNodes", response.GoodNodes},
		{"numBadNodes", response.BadNodes}, {"numDisks", response.Disks},
		{"numGoodDisks", response.GoodDisks}, {"numBadDisks", response.BadDisks},
	}
	parsed := make(map[string]float64, len(values))
	for _, value := range values {
		number, err := value.raw.Required(value.name)
		if err != nil || number < 0 || math.Trunc(number) != number {
			return "", 0, fmt.Errorf("cluster health %s is invalid", value.name)
		}
		parsed[value.name] = number
	}
	healthy := parsed["numBadNodes"] == 0 && parsed["numBadDisks"] == 0 &&
		parsed["numNodes"] == parsed["numGoodNodes"] &&
		parsed["numDisks"] == parsed["numGoodDisks"]
	if healthy {
		return response.Name, 1, nil
	}
	return response.Name, 0, nil
}

func ParseNodes(data []byte, cluster string, collectedAt time.Time) ([]model.Node, []string, error) {
	var response struct {
		Nodes []struct {
			ID      string `json:"nodeid"`
			Name    string `json:"nodename"`
			IP      string `json:"ip"`
			Rack    string `json:"rackId"`
			Version string `json:"version"`
			State   string `json:"status"`
		} `json:"node"`
	}
	if err := decodeJSON(data, &response); err != nil {
		return nil, nil, err
	}
	if len(response.Nodes) == 0 {
		return nil, nil, fmt.Errorf("node list is empty")
	}
	nodes := make([]model.Node, 0, len(response.Nodes))
	versions := make([]string, 0, len(response.Nodes))
	seen := make(map[string]struct{}, len(response.Nodes))
	seenNames := make(map[string]struct{}, len(response.Nodes))
	for _, raw := range response.Nodes {
		if raw.ID == "" || raw.Name == "" || raw.Version == "" {
			return nil, nil, fmt.Errorf("node id, name, and version are required")
		}
		if _, exists := seen[raw.ID]; exists {
			return nil, nil, fmt.Errorf("duplicate node id")
		}
		if _, exists := seenNames[raw.Name]; exists {
			return nil, nil, fmt.Errorf("duplicate node name")
		}
		seen[raw.ID] = struct{}{}
		seenNames[raw.Name] = struct{}{}
		value := model.Node{
			Cluster: cluster, ID: raw.ID, Name: raw.Name, Version: raw.Version,
			CollectedAt: collectedAt,
		}
		value.IP = stringPointer(raw.IP)
		value.Rack = stringPointer(raw.Rack)
		value.State = stringPointer(raw.State)
		nodes = append(nodes, value)
		versions = append(versions, raw.Version)
	}
	return nodes, versions, nil
}

func ParseNodeStatuses(data []byte) (map[string]NodeStatus, error) {
	type rawService struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	type rawNode struct {
		ID        string       `json:"nodeid"`
		IDAlt     string       `json:"id"`
		Health    string       `json:"healthStatus"`
		Services  []rawService `json:"services"`
		Processes []rawService `json:"processes"`
	}
	var response struct {
		Node     []rawNode `json:"node"`
		Nodes    []rawNode `json:"nodes"`
		Embedded struct {
			Instances []rawNode `json:"_instances"`
		} `json:"_embedded"`
	}
	if err := decodeJSON(data, &response); err != nil {
		return nil, err
	}
	values := append(response.Node, response.Nodes...)
	values = append(values, response.Embedded.Instances...)
	if len(values) == 0 {
		return nil, fmt.Errorf("node health list is empty")
	}
	result := make(map[string]NodeStatus, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		id := raw.ID
		if id == "" {
			id = raw.IDAlt
		}
		if id == "" || raw.Health == "" {
			return nil, fmt.Errorf("node health id and status are required")
		}
		if _, exists := seen[id]; exists {
			return nil, fmt.Errorf("duplicate node health id")
		}
		seen[id] = struct{}{}
		var status NodeStatus
		switch strings.ToLower(raw.Health) {
		case "good", "healthy", "ok":
			status.Health = 1
		case "bad", "unhealthy", "degraded", "faulted":
			status.Health = 0
		default:
			return nil, fmt.Errorf("unknown node health status")
		}
		serviceSeen := make(map[string]struct{}, len(raw.Services)+len(raw.Processes))
		for kind, entries := range map[string][]rawService{
			"service": raw.Services, "process": raw.Processes,
		} {
			for _, entry := range entries {
				name := normalizeDimension(entry.Name, 128)
				state := normalizeDimension(entry.Status, 64)
				if name == "" || state == "" {
					return nil, fmt.Errorf("node service/process name and status are required")
				}
				key := kind + "\x00" + name
				if _, exists := serviceSeen[key]; exists {
					return nil, fmt.Errorf("duplicate node service/process status")
				}
				serviceSeen[key] = struct{}{}
				health, err := statusHealth(state)
				if err != nil {
					return nil, err
				}
				status.Services = append(status.Services, model.NodeService{
					Name: name, Kind: kind, Status: state, Health: health,
				})
			}
		}
		slices.SortFunc(status.Services, func(left, right model.NodeService) int {
			if left.Kind != right.Kind {
				return strings.Compare(left.Kind, right.Kind)
			}
			return strings.Compare(left.Name, right.Name)
		})
		result[id] = status
	}
	return result, nil
}

func ParseNodeHealth(data []byte) (map[string]float64, error) {
	statuses, err := ParseNodeStatuses(data)
	if err != nil {
		return nil, err
	}
	result := make(map[string]float64, len(statuses))
	for id, status := range statuses {
		result[id] = status.Health
	}
	return result, nil
}

func ParseNamespaces(data []byte, cluster string, collectedAt time.Time) ([]model.Namespace, error) {
	var response struct {
		Namespaces []struct {
			Name             string `json:"name"`
			ID               string `json:"id"`
			Owner            string `json:"owner"`
			Audit            *bool  `json:"audit_enabled"`
			ReplicationGroup string `json:"default_data_services_vpool"`
			Hard             Number `json:"blockSize"`
			Soft             Number `json:"notificationSize"`
		} `json:"namespace"`
	}
	if err := decodeJSON(data, &response); err != nil {
		return nil, err
	}
	result := make([]model.Namespace, 0, len(response.Namespaces))
	seen := make(map[string]struct{}, len(response.Namespaces))
	for _, raw := range response.Namespaces {
		name := raw.Name
		if name == "" {
			name = raw.ID
		}
		if name == "" {
			return nil, fmt.Errorf("namespace name is required")
		}
		if _, exists := seen[name]; exists {
			return nil, fmt.Errorf("duplicate namespace")
		}
		seen[name] = struct{}{}
		value := model.Namespace{
			Cluster: cluster, Name: name, CollectedAt: collectedAt,
			Owner: stringPointer(raw.Owner), AuditEnabled: raw.Audit,
			ReplicationGroup: stringPointer(raw.ReplicationGroup),
		}
		if err := validateQuotaNumbers(raw.Soft, raw.Hard); err != nil {
			return nil, err
		}
		applyQuota(&value.SoftQuotaBytes, &value.SoftQuotaConfigured, raw.Soft)
		applyQuota(&value.HardQuotaBytes, &value.HardQuotaConfigured, raw.Hard)
		result = append(result, value)
	}
	return result, nil
}

func ParseNamespaceQuota(data []byte) (Quota, error) {
	type rawQuota struct {
		Hard Number `json:"blockSize"`
		Soft Number `json:"notificationSize"`
	}
	var response struct {
		Details rawQuota `json:"namespace_quota_details"`
		Hard    Number   `json:"blockSize"`
		Soft    Number   `json:"notificationSize"`
	}
	if err := decodeJSON(data, &response); err != nil {
		return Quota{}, err
	}
	topLevel := response.Hard.valid || response.Soft.valid
	nested := response.Details.Hard.valid || response.Details.Soft.valid
	if topLevel && nested {
		return Quota{}, fmt.Errorf("namespace quota contains ambiguous envelopes")
	}
	if topLevel {
		return quotaFromNumbers(response.Soft, response.Hard)
	}
	return quotaFromNumbers(response.Details.Soft, response.Details.Hard)
}

func ParseNamespaceBilling(data []byte) (Billing, error) {
	var response struct {
		Size       Number `json:"total_size"`
		Unit       string `json:"total_size_unit"`
		Objects    Number `json:"total_objects"`
		SampleTime string `json:"sample_time"`
	}
	if err := decodeJSON(data, &response); err != nil {
		return Billing{}, err
	}
	return parseBilling(response.Size, response.Unit, response.Objects, response.SampleTime)
}

func ParseBuckets(data []byte, cluster string, collectedAt time.Time) ([]model.Bucket, error) {
	values, _, err := ParseBucketPage(data, cluster, collectedAt)
	return values, err
}

func ParseBucketPage(data []byte, cluster string, collectedAt time.Time) ([]model.Bucket, Page, error) {
	var response struct {
		Buckets []struct {
			Name              string `json:"name"`
			Namespace         string `json:"namespace"`
			Owner             string `json:"owner"`
			VPool             string `json:"vpool"`
			Created           string `json:"created"`
			LastModified      string `json:"last_modified"`
			LastModifiedCamel string `json:"lastModified"`
			Locked            *bool  `json:"locked"`
			Versioning        *bool  `json:"versioning_enabled"`
			Encryption        *bool  `json:"encryption_enabled"`
			Audit             *bool  `json:"audit_enabled"`
			Retention         Number `json:"default_retention"`
			Hard              Number `json:"block_size"`
			Soft              Number `json:"notification_size"`
		} `json:"object_bucket"`
		NextMarker      string `json:"next_marker"`
		NextMarkerCamel string `json:"nextMarker"`
	}
	if err := decodeJSON(data, &response); err != nil {
		return nil, Page{}, err
	}
	nextMarker := response.NextMarker
	if nextMarker == "" {
		nextMarker = response.NextMarkerCamel
	}
	if len(nextMarker) > 4096 || strings.ContainsAny(nextMarker, "\r\n") {
		return nil, Page{}, fmt.Errorf("bucket page marker is invalid")
	}
	result := make([]model.Bucket, 0, len(response.Buckets))
	seen := make(map[string]struct{}, len(response.Buckets))
	for _, raw := range response.Buckets {
		if raw.Name == "" || raw.Namespace == "" {
			return nil, Page{}, fmt.Errorf("bucket name and namespace are required")
		}
		key := raw.Namespace + "\x00" + raw.Name
		if _, exists := seen[key]; exists {
			return nil, Page{}, fmt.Errorf("duplicate bucket")
		}
		seen[key] = struct{}{}
		value := model.Bucket{
			Cluster: cluster, Namespace: raw.Namespace, Name: raw.Name,
			Owner: stringPointer(raw.Owner), ReplicationGroup: stringPointer(raw.VPool),
			VersioningEnabled: raw.Versioning, EncryptionEnabled: raw.Encryption,
			ObjectLockEnabled: raw.Locked, AuditEnabled: raw.Audit, CollectedAt: collectedAt,
		}
		if err := validateQuotaNumbers(raw.Soft, raw.Hard); err != nil {
			return nil, Page{}, err
		}
		if raw.Created != "" {
			parsed, err := time.Parse(time.RFC3339, raw.Created)
			if err != nil {
				return nil, Page{}, fmt.Errorf("bucket created timestamp is invalid")
			}
			value.CreatedAt = &parsed
		}
		lastModified := raw.LastModified
		if lastModified != "" && raw.LastModifiedCamel != "" &&
			lastModified != raw.LastModifiedCamel {
			return nil, Page{}, fmt.Errorf("bucket last modified timestamp is ambiguous")
		}
		if lastModified == "" {
			lastModified = raw.LastModifiedCamel
		}
		if lastModified != "" {
			parsed, err := time.Parse(time.RFC3339Nano, lastModified)
			if err != nil {
				return nil, Page{}, fmt.Errorf("bucket last modified timestamp is invalid")
			}
			value.LastModifiedAt = &parsed
		}
		if raw.Retention.valid {
			switch {
			case raw.Retention.value == -1 || raw.Retention.value == -2:
			case raw.Retention.value < 0 || math.Trunc(raw.Retention.value) != raw.Retention.value:
				return nil, Page{}, fmt.Errorf("bucket retention is invalid")
			default:
				retention := fmt.Sprintf("%.0fs", raw.Retention.value)
				value.Retention = &retention
			}
		}
		applyQuota(&value.SoftQuotaBytes, &value.SoftQuotaConfigured, raw.Soft)
		applyQuota(&value.HardQuotaBytes, &value.HardQuotaConfigured, raw.Hard)
		result = append(result, value)
	}
	return result, Page{NextMarker: nextMarker}, nil
}

func ParseBucketQuota(data []byte) (Quota, error) {
	type rawQuota struct {
		Hard Number `json:"blockSize"`
		Soft Number `json:"notificationSize"`
	}
	var envelope map[string]json.RawMessage
	if err := decodeJSON(data, &envelope); err != nil {
		return Quota{}, err
	}
	details, nested := envelope["bucket_quota_details"]
	topLevel := hasAnyJSONField(envelope, "blockSize", "notificationSize")
	if nested && topLevel {
		return Quota{}, fmt.Errorf("bucket quota contains ambiguous envelopes")
	}
	var raw rawQuota
	switch {
	case nested:
		if err := decodeJSON(details, &raw); err != nil {
			return Quota{}, fmt.Errorf("decode bucket quota details: %w", err)
		}
	case topLevel:
		if err := decodeJSON(data, &raw); err != nil {
			return Quota{}, err
		}
	default:
		return Quota{}, fmt.Errorf("bucket quota is missing")
	}
	return quotaFromNumbers(raw.Soft, raw.Hard)
}

func ParseBucketBilling(data []byte) (Billing, error) {
	var envelope map[string]json.RawMessage
	if err := decodeJSON(data, &envelope); err != nil {
		return Billing{}, err
	}
	details, nested := envelope["bucket_billing_info"]
	topLevel := hasAnyJSONField(
		envelope, "total_size", "total_size_unit", "total_objects", "sample_time",
	)
	if nested && topLevel {
		return Billing{}, fmt.Errorf("bucket billing contains ambiguous envelopes")
	}
	var raw rawBucketBilling
	switch {
	case nested:
		if err := decodeJSON(details, &raw); err != nil {
			return Billing{}, fmt.Errorf("decode bucket billing details: %w", err)
		}
	case topLevel:
		if err := decodeJSON(data, &raw); err != nil {
			return Billing{}, err
		}
	default:
		return Billing{}, fmt.Errorf("bucket billing is missing")
	}
	return parseBilling(raw.Size, raw.Unit, raw.Objects, raw.SampleTime)
}

type rawBucketBilling struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	Size       Number `json:"total_size"`
	Unit       string `json:"total_size_unit"`
	Objects    Number `json:"total_objects"`
	SampleTime string `json:"sample_time"`
}

func ParseBucketBillings(data []byte) (map[string]Billing, error) {
	var envelope map[string]json.RawMessage
	if err := decodeJSON(data, &envelope); err != nil {
		return nil, err
	}
	singular, hasSingular := envelope["bucket_billing_info"]
	plural, hasPlural := envelope["bucket_billing_infos"]
	if hasSingular && hasPlural {
		return nil, fmt.Errorf("bucket billing batch contains ambiguous envelopes")
	}
	var details json.RawMessage
	switch {
	case hasPlural:
		details = plural
	case hasSingular:
		details = singular
	}
	details = bytes.TrimSpace(details)
	if len(details) == 0 || bytes.Equal(details, []byte("null")) {
		return nil, fmt.Errorf("bucket billing batch is missing")
	}
	var values []rawBucketBilling
	if details[0] == '[' {
		if err := json.Unmarshal(details, &values); err != nil {
			return nil, fmt.Errorf("decode bucket billing batch: %w", err)
		}
	} else {
		var value rawBucketBilling
		if err := json.Unmarshal(details, &value); err != nil {
			return nil, fmt.Errorf("decode bucket billing batch item: %w", err)
		}
		values = append(values, value)
	}
	result := make(map[string]Billing, len(values))
	for _, value := range values {
		if normalizeDimension(value.Name, 255) == "" ||
			normalizeDimension(value.Namespace, 255) == "" {
			return nil, fmt.Errorf("bucket billing batch item name and namespace are required")
		}
		key := value.Namespace + "\x00" + value.Name
		if _, exists := result[key]; exists {
			return nil, fmt.Errorf("bucket billing batch contains duplicate item")
		}
		billing, err := parseBilling(value.Size, value.Unit, value.Objects, value.SampleTime)
		if err != nil {
			return nil, err
		}
		result[key] = billing
	}
	return result, nil
}

func ParseFluxNodeResources(data []byte) (map[string]NodeResource, error) {
	return ParseFluxNodeResourcesWithPolicy(data, NodeResourcePolicy{
		MaxNetworkInterfaces: 16, PreferBondInterfaces: true,
	})
}

func ParseFluxNodeResourcesWithPolicy(
	data []byte,
	policy NodeResourcePolicy,
) (map[string]NodeResource, error) {
	var response struct {
		Series []struct {
			Columns []string   `json:"Columns"`
			Values  [][]string `json:"Values"`
		} `json:"Series"`
	}
	if err := decodeJSON(data, &response); err != nil {
		return nil, err
	}
	if policy.MaxNetworkInterfaces < 1 || policy.MaxNetworkInterfaces > 128 {
		return nil, fmt.Errorf("node resource max network interfaces is invalid")
	}
	result := make(map[string]NodeResource)
	type diskResource struct {
		used  *float64
		total *float64
	}
	disks := make(map[string]map[string]diskResource)
	for _, series := range response.Series {
		index := make(map[string]int, len(series.Columns))
		for columnIndex, name := range series.Columns {
			if _, exists := index[name]; exists {
				return nil, fmt.Errorf("Flux series contains duplicate column %s", name)
			}
			index[name] = columnIndex
		}
		required := []string{"_measurement", "_field", "_value", "_time", "node_id"}
		for _, name := range required {
			if _, exists := index[name]; !exists {
				return nil, fmt.Errorf("Flux series is missing required column %s", name)
			}
		}
		for _, row := range series.Values {
			if len(row) != len(series.Columns) {
				return nil, fmt.Errorf("Flux row length does not match columns")
			}
			if _, err := time.Parse(time.RFC3339Nano, row[index["_time"]]); err != nil {
				return nil, fmt.Errorf("Flux timestamp is invalid")
			}
			if err := validateFluxRange(index, row); err != nil {
				return nil, err
			}
			number, err := parseFinite(row[index["_value"]])
			if err != nil || number < 0 {
				return nil, fmt.Errorf("Flux value is invalid")
			}
			nodeID := row[index["node_id"]]
			if nodeID == "" {
				return nil, fmt.Errorf("Flux node_id is empty")
			}
			resource := result[nodeID]
			measurement := row[index["_measurement"]]
			field := row[index["_field"]]
			switch {
			case measurement == "cpu" && field == "usage_idle":
				cpuIndex, exists := index["cpu"]
				if !exists || row[cpuIndex] != "cpu-total" {
					return nil, fmt.Errorf("Flux CPU series must identify cpu-total")
				}
				if number > 100 {
					return nil, fmt.Errorf("Flux CPU percent is outside 0..100")
				}
				if resource.CPUUsageRatio != nil {
					return nil, fmt.Errorf("Flux CPU series is duplicated")
				}
				value := (100 - number) / 100
				resource.CPUUsageRatio = &value
			case measurement == "mem" && field == "used":
				if resource.MemoryUsedBytes != nil {
					return nil, fmt.Errorf("Flux memory used series is duplicated")
				}
				resource.MemoryUsedBytes = pointer(number)
			case measurement == "mem" && field == "total":
				if resource.MemoryTotalBytes != nil {
					return nil, fmt.Errorf("Flux memory total series is duplicated")
				}
				resource.MemoryTotalBytes = pointer(number)
			case measurement == "disk" && (field == "used" || field == "total"):
				filesystem, ok := fluxDimension(index, row, "path", "filesystem", "device")
				if !ok {
					return nil, fmt.Errorf("Flux disk series must identify a filesystem")
				}
				filesystem = normalizeDimension(filesystem, 128)
				if filesystem == "" {
					return nil, fmt.Errorf("Flux disk filesystem is invalid")
				}
				if len(policy.Filesystems) == 0 || !slices.Contains(policy.Filesystems, filesystem) {
					break
				}
				if disks[nodeID] == nil {
					disks[nodeID] = make(map[string]diskResource)
				}
				disk := disks[nodeID][filesystem]
				if field == "used" {
					if disk.used != nil {
						return nil, fmt.Errorf("Flux disk used series is duplicated")
					}
					disk.used = pointer(number)
				} else {
					if disk.total != nil {
						return nil, fmt.Errorf("Flux disk total series is duplicated")
					}
					disk.total = pointer(number)
				}
				disks[nodeID][filesystem] = disk
			case measurement == "net" && (field == "bytes_recv" || field == "bytes_sent"):
				interfaceIndex, exists := index["interface"]
				if !exists || row[interfaceIndex] == "" {
					return nil, fmt.Errorf("Flux network interface is required")
				}
				interfaceName := normalizeDimension(row[interfaceIndex], 64)
				if interfaceName == "" {
					return nil, fmt.Errorf("Flux network interface is invalid")
				}
				if len(policy.NetworkInterfaces) != 0 &&
					!slices.Contains(policy.NetworkInterfaces, interfaceName) {
					break
				}
				position := slices.IndexFunc(resource.Network, func(value model.Network) bool {
					return value.Interface == interfaceName
				})
				if position < 0 {
					if len(resource.Network) >= policy.MaxNetworkInterfaces {
						return nil, fmt.Errorf("Flux network interface limit exceeded")
					}
					resource.Network = append(resource.Network, model.Network{Interface: interfaceName})
					position = len(resource.Network) - 1
				}
				if field == "bytes_recv" {
					if resource.Network[position].ReceiveBytes != nil {
						return nil, fmt.Errorf("Flux network receive series is duplicated")
					}
					resource.Network[position].ReceiveBytes = pointer(number)
				} else {
					if resource.Network[position].TransmitBytes != nil {
						return nil, fmt.Errorf("Flux network transmit series is duplicated")
					}
					resource.Network[position].TransmitBytes = pointer(number)
				}
			}
			result[nodeID] = resource
		}
	}
	for nodeID, resource := range result {
		if resource.MemoryUsedBytes != nil && resource.MemoryTotalBytes != nil &&
			*resource.MemoryUsedBytes > *resource.MemoryTotalBytes {
			return nil, fmt.Errorf("Flux memory used exceeds total")
		}
		for _, disk := range disks[nodeID] {
			if disk.used == nil || disk.total == nil {
				return nil, fmt.Errorf("Flux disk series is incomplete")
			}
			if *disk.used > *disk.total {
				return nil, fmt.Errorf("Flux disk used exceeds total")
			}
			used := optionalFloat(resource.DiskUsedBytes) + *disk.used
			total := optionalFloat(resource.DiskTotalBytes) + *disk.total
			if !finite(used) || !finite(total) {
				return nil, fmt.Errorf("Flux disk aggregate is too large")
			}
			resource.DiskUsedBytes = pointer(used)
			resource.DiskTotalBytes = pointer(total)
		}
		if policy.PreferBondInterfaces &&
			slices.ContainsFunc(resource.Network, func(value model.Network) bool {
				return strings.HasPrefix(strings.ToLower(value.Interface), "bond")
			}) {
			resource.Network = slices.DeleteFunc(resource.Network, func(value model.Network) bool {
				return likelyBondMember(value.Interface)
			})
		}
		slices.SortFunc(resource.Network, func(left, right model.Network) int {
			return strings.Compare(left.Interface, right.Interface)
		})
		result[nodeID] = resource
	}
	return result, nil
}

func ParseFluxPerformance(
	data []byte,
	cluster, defaultVDC string,
	collectedAt time.Time,
) ([]model.Performance, error) {
	var response struct {
		Series []struct {
			Columns []string   `json:"Columns"`
			Values  [][]string `json:"Values"`
		} `json:"Series"`
	}
	if err := decodeJSON(data, &response); err != nil {
		return nil, err
	}
	var result []model.Performance
	seen := make(map[string]struct{})
	for _, series := range response.Series {
		index := make(map[string]int, len(series.Columns))
		for columnIndex, name := range series.Columns {
			if _, exists := index[name]; exists {
				return nil, fmt.Errorf("performance Flux series contains duplicate column %s", name)
			}
			index[name] = columnIndex
		}
		for _, required := range []string{"_measurement", "_field", "_value", "_time"} {
			if _, exists := index[required]; !exists {
				return nil, fmt.Errorf("performance Flux series is missing required column %s", required)
			}
		}
		for _, row := range series.Values {
			if len(row) != len(series.Columns) {
				return nil, fmt.Errorf("performance Flux row length does not match columns")
			}
			sampleTime, err := time.Parse(time.RFC3339Nano, row[index["_time"]])
			if err != nil {
				return nil, fmt.Errorf("performance Flux timestamp is invalid")
			}
			if err := validateFluxRange(index, row); err != nil {
				return nil, err
			}
			number, err := parseFinite(row[index["_value"]])
			if err != nil || number < 0 {
				return nil, fmt.Errorf("performance Flux value is invalid")
			}
			vdc, _ := fluxDimension(index, row, "vdc", "vdc_name", "zone")
			if vdc == "" {
				vdc = defaultVDC
			}
			vdc = normalizeDimension(vdc, 128)
			if cluster == "" || vdc == "" {
				return nil, fmt.Errorf("performance Flux cluster and VDC are required")
			}
			namespace, _ := fluxDimension(index, row, "namespace")
			namespace = normalizeDimension(namespace, 128)
			operation, _ := fluxDimension(index, row, "operation")
			statusClass, _ := fluxDimension(index, row, "status_class")
			quantile, _ := fluxDimension(index, row, "quantile", "percentile")
			unit, _ := fluxDimension(index, row, "unit")
			field := strings.ToLower(row[index["_field"]])
			measurement := strings.ToLower(row[index["_measurement"]])
			value := model.Performance{
				Cluster: cluster, VDC: vdc, Namespace: namespace,
				Operation:   normalizeOperation(operation),
				StatusClass: normalizeStatusClass(statusClass),
				Quantile:    normalizeQuantile(quantile), CollectedAt: sampleTime,
			}
			switch measurement {
			case "cq_performance_throughput":
				scaled, scaleErr := performanceThroughput(number, unit)
				if scaleErr != nil {
					return nil, scaleErr
				}
				switch field {
				case "read", "read_rate", "read_bytes_per_second":
					value.Metric = model.PerformanceReadThroughput
				case "write", "write_rate", "write_bytes_per_second":
					value.Metric = model.PerformanceWriteThroughput
				default:
					return nil, fmt.Errorf("performance throughput field is unsupported")
				}
				value.Operation, value.StatusClass, value.Quantile = "", "", ""
				value.Value = scaled
			case "cq_performance_latency":
				if value.Operation == "" {
					value.Operation = normalizeOperation(field)
				}
				if value.Operation == "" || value.Quantile == "" {
					return nil, fmt.Errorf("performance latency operation and quantile are required")
				}
				scaled, scaleErr := performanceLatency(number, unit)
				if scaleErr != nil {
					return nil, scaleErr
				}
				value.Metric = model.PerformanceLatency
				value.StatusClass = ""
				value.Value = scaled
			case "cq_performance_transaction", "cq_performance_transaction_ns":
				if value.Operation == "" {
					value.Operation = operationFromField(field)
				}
				if value.Operation == "" {
					value.Operation = "all"
				}
				if value.StatusClass == "" {
					value.StatusClass = statusClassFromField(field)
				}
				value.Metric = model.PerformanceRequests
				value.Quantile = ""
				value.Value = number
			case "cq_performance_error", "cq_performance_error_ns":
				if value.Operation == "" {
					value.Operation = operationFromField(field)
				}
				if value.Operation == "" {
					value.Operation = "all"
				}
				if value.StatusClass == "" {
					value.StatusClass = statusClassFromField(field)
				}
				if value.StatusClass == "" {
					return nil, fmt.Errorf("performance error status class is required")
				}
				value.Metric = model.PerformanceRequests
				value.Quantile = ""
				value.Value = number
			default:
				return nil, fmt.Errorf("performance Flux measurement is unsupported")
			}
			if value.CollectedAt.IsZero() {
				value.CollectedAt = collectedAt.UTC()
			}
			key := strings.Join([]string{
				value.Cluster, value.VDC, value.Namespace, string(value.Metric),
				value.Operation, value.StatusClass, value.Quantile,
			}, "\x00")
			if _, exists := seen[key]; exists {
				return nil, fmt.Errorf("performance Flux series contains duplicate metric labels")
			}
			seen[key] = struct{}{}
			result = append(result, value)
		}
	}
	return result, nil
}

func ParseReplication(data []byte, cluster string, now time.Time) (model.Replication, error) {
	var response struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		RGName    string `json:"rgName"`
		Source    string `json:"zone1Name"`
		Target    string `json:"zone2Name"`
		Status    string `json:"rglinkStatus"`
		Repo      Number `json:"chunksRepoPendingReplicationTotalSize"`
		Journal   Number `json:"chunksJournalPendingReplicationTotalSize"`
		XOR       Number `json:"chunksPendingXorTotalSize"`
		RPO       Number `json:"replicationRpoTimestamp"`
		FailState string `json:"FailoverState"`
		FailPct   Number `json:"FailoverProgressPercent"`
		BootState string `json:"BootstrapState"`
		BootPct   Number `json:"BootstrapProgressPercent"`
	}
	if err := decodeJSON(data, &response); err != nil {
		return model.Replication{}, err
	}
	name := response.RGName
	if name == "" {
		name = response.Name
	}
	if response.ID == "" || name == "" {
		return model.Replication{}, fmt.Errorf("replication id and group name are required")
	}
	result := model.Replication{
		Cluster: cluster, ID: response.ID, Group: name,
		SourceVDC: stringPointer(response.Source), TargetVDC: stringPointer(response.Target),
		CollectedAt: now,
	}
	if response.Status != "" {
		switch strings.ToLower(response.Status) {
		case "good", "healthy", "ok":
			result.Status = pointer(1)
		case "bad", "unhealthy", "degraded", "faulted", "failed", "broken":
			result.Status = pointer(0)
		default:
			return model.Replication{}, fmt.Errorf("unknown replication status")
		}
	}
	var pending float64
	hasPending := false
	for _, value := range []Number{response.Repo, response.Journal, response.XOR} {
		if value.valid {
			if value.value < 0 {
				return model.Replication{}, fmt.Errorf("replication pending bytes are negative")
			}
			if pending > math.MaxFloat64-value.value {
				return model.Replication{}, fmt.Errorf("replication pending bytes overflow")
			}
			pending += value.value
			hasPending = true
		}
	}
	if hasPending {
		result.PendingBytes = pointer(pending)
	}
	if response.RPO.valid && hasPending && pending > 0 {
		if response.RPO.value < 0 || math.Trunc(response.RPO.value) != response.RPO.value ||
			response.RPO.value > float64(now.Unix()) {
			return model.Replication{}, fmt.Errorf("replication RPO timestamp is invalid or in the future")
		}
		lag := now.Sub(time.Unix(int64(response.RPO.value), 0)).Seconds()
		result.LagSeconds = &lag
	}
	switch {
	case response.BootState != "":
		result.RecoveryKind = stringPointer("bootstrap")
		result.RecoveryStatus = stringPointer(response.BootState)
		progress, err := ratioPercent(response.BootPct)
		if err != nil {
			return model.Replication{}, err
		}
		result.RecoveryProgress = progress
	case response.FailState != "":
		result.RecoveryKind = stringPointer("failover")
		result.RecoveryStatus = stringPointer(response.FailState)
		progress, err := ratioPercent(response.FailPct)
		if err != nil {
			return model.Replication{}, err
		}
		result.RecoveryProgress = progress
	}
	return result, nil
}

func ApplyNodeResources(nodes []model.Node, resources map[string]NodeResource) {
	ApplyNodeResourcesForCapabilities(nodes, resources, true, true, true)
}

func ApplyNodeResourcesForCapabilities(
	nodes []model.Node,
	resources map[string]NodeResource,
	cpuMemory, disk, network bool,
) {
	for index := range nodes {
		resource, exists := resources[nodes[index].ID]
		if !exists {
			continue
		}
		if cpuMemory {
			nodes[index].CPUUsageRatio = resource.CPUUsageRatio
			nodes[index].MemoryUsedBytes = resource.MemoryUsedBytes
			nodes[index].MemoryTotalBytes = resource.MemoryTotalBytes
		}
		if disk {
			nodes[index].DiskUsedBytes = resource.DiskUsedBytes
			nodes[index].DiskTotalBytes = resource.DiskTotalBytes
		}
		if network {
			nodes[index].Network = slices.Clone(resource.Network)
		}
	}
}

func ApplyNodeStatuses(nodes []model.Node, statuses map[string]NodeStatus, includeServices bool) {
	for index := range nodes {
		status, exists := statuses[nodes[index].ID]
		if !exists {
			continue
		}
		nodes[index].Health = pointer(status.Health)
		if includeServices {
			nodes[index].Services = slices.Clone(status.Services)
		} else {
			nodes[index].Services = nil
		}
	}
}

func ApplyNodeHealth(nodes []model.Node, health map[string]float64) {
	for index := range nodes {
		if value, exists := health[nodes[index].ID]; exists {
			nodes[index].Health = pointer(value)
		}
	}
}

func ApplyQuotaToNamespace(value *model.Namespace, quota Quota) {
	value.SoftQuotaBytes = quota.SoftBytes
	value.HardQuotaBytes = quota.HardBytes
	value.SoftQuotaConfigured = quota.SoftBytes != nil
	value.HardQuotaConfigured = quota.HardBytes != nil
}

func ApplyBillingToNamespace(value *model.Namespace, billing Billing) {
	value.UsedBytes = billing.UsedBytes
	value.ObjectCount = billing.ObjectCount
	value.UsageSampleAt = billing.SampleTime
}

func ApplyQuotaToBucket(value *model.Bucket, quota Quota) {
	value.SoftQuotaBytes = quota.SoftBytes
	value.HardQuotaBytes = quota.HardBytes
	value.SoftQuotaConfigured = quota.SoftBytes != nil
	value.HardQuotaConfigured = quota.HardBytes != nil
}

func ApplyBillingToBucket(value *model.Bucket, billing Billing) {
	value.UsedBytes = billing.UsedBytes
	value.ObjectCount = billing.ObjectCount
	value.UsageSampleAt = billing.SampleTime
}

func decodeJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode ECS response: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("decode ECS response: multiple JSON values")
		}
		return fmt.Errorf("decode ECS response trailing data: %w", err)
	}
	return nil
}

func hasAnyJSONField(value map[string]json.RawMessage, fields ...string) bool {
	for _, field := range fields {
		if _, ok := value[field]; ok {
			return true
		}
	}
	return false
}

func validateFluxRange(index map[string]int, row []string) error {
	startIndex, hasStart := index["_start"]
	stopIndex, hasStop := index["_stop"]
	if !hasStart && !hasStop {
		return nil
	}
	if !hasStart || !hasStop {
		return fmt.Errorf("Flux range boundary columns are incomplete")
	}
	start, err := time.Parse(time.RFC3339Nano, row[startIndex])
	if err != nil {
		return fmt.Errorf("Flux _start timestamp is invalid")
	}
	stop, err := time.Parse(time.RFC3339Nano, row[stopIndex])
	if err != nil || !start.Before(stop) {
		return fmt.Errorf("Flux _stop timestamp is invalid")
	}
	sample, _ := time.Parse(time.RFC3339Nano, row[index["_time"]])
	if sample.Before(start) || !sample.Before(stop) {
		return fmt.Errorf("Flux timestamp is outside the half-open range")
	}
	return nil
}

func quotaFromNumbers(soft, hard Number) (Quota, error) {
	if err := validateQuotaNumbers(soft, hard); err != nil {
		return Quota{}, err
	}
	var result Quota
	for field, entry := range map[string]struct {
		source Number
		target **float64
	}{"soft": {soft, &result.SoftBytes}, "hard": {hard, &result.HardBytes}} {
		value, err := entry.source.Required(field + " quota")
		if err != nil {
			return Quota{}, err
		}
		if value == -1 {
			continue
		}
		if value < 0 {
			return Quota{}, fmt.Errorf("%s quota is invalid", field)
		}
		converted, err := scaleFinite(value, decimalGB, field+" quota")
		if err != nil {
			return Quota{}, err
		}
		*entry.target = &converted
	}
	return result, nil
}

func validateQuotaNumbers(soft, hard Number) error {
	if soft.valid && soft.value < -1 {
		return fmt.Errorf("soft quota is invalid")
	}
	if hard.valid && hard.value < -1 {
		return fmt.Errorf("hard quota is invalid")
	}
	if soft.valid && soft.value > math.MaxFloat64/decimalGB {
		return fmt.Errorf("soft quota is too large")
	}
	if hard.valid && hard.value > math.MaxFloat64/decimalGB {
		return fmt.Errorf("hard quota is too large")
	}
	if soft.valid && hard.valid && soft.value >= 0 && hard.value >= 0 &&
		soft.value > hard.value {
		return fmt.Errorf("soft quota exceeds hard quota")
	}
	return nil
}

func parseBilling(size Number, unit string, objects Number, sample string) (Billing, error) {
	sizeValue, err := size.Required("billing total_size")
	if err != nil || sizeValue < 0 {
		return Billing{}, fmt.Errorf("billing total_size is invalid")
	}
	multiplier := float64(1)
	switch strings.ToUpper(unit) {
	case "B", "BYTES":
	case "KB":
		multiplier = binaryKB
	case "MB":
		multiplier = 1_000_000
	case "GB":
		multiplier = decimalGB
	case "TB":
		multiplier = 1_000_000_000_000
	default:
		return Billing{}, fmt.Errorf("billing unit is unsupported")
	}
	objectValue, err := objects.Required("billing total_objects")
	if err != nil || objectValue < 0 || math.Trunc(objectValue) != objectValue {
		return Billing{}, fmt.Errorf("billing total_objects is invalid")
	}
	usedBytes, err := scaleFinite(sizeValue, multiplier, "billing total_size")
	if err != nil {
		return Billing{}, err
	}
	result := Billing{UsedBytes: &usedBytes, ObjectCount: pointer(objectValue)}
	if sample != "" {
		parsed, err := time.Parse(time.RFC3339Nano, sample)
		if err != nil {
			return Billing{}, fmt.Errorf("billing sample_time is invalid")
		}
		result.SampleTime = &parsed
	}
	return result, nil
}

func applyQuota(target **float64, configured *bool, source Number) {
	if !source.valid || source.value == -1 {
		return
	}
	if source.value >= 0 {
		*target = pointer(source.value * decimalGB)
		*configured = true
	}
}

func ratioPercent(value Number) (*float64, error) {
	if !value.valid {
		return nil, nil
	}
	if value.value < 0 || value.value > 100 {
		return nil, fmt.Errorf("percentage is outside 0..100")
	}
	return pointer(value.value / 100), nil
}

func parseFinite(value string) (float64, error) {
	var number Number
	if err := number.UnmarshalJSON([]byte(value)); err == nil {
		return number.Required("value")
	}
	quoted, _ := json.Marshal(value)
	if err := number.UnmarshalJSON(quoted); err != nil {
		return 0, err
	}
	return number.Required("value")
}

func scaleFinite(value, multiplier float64, field string) (float64, error) {
	if value < 0 || multiplier <= 0 || value > math.MaxFloat64/multiplier {
		return 0, fmt.Errorf("%s is outside the supported range", field)
	}
	return value * multiplier, nil
}

func pointer(value float64) *float64 {
	return &value
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	copyValue := value
	return &copyValue
}

func normalizeDimension(value string, limit int) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > limit || strings.ContainsFunc(value, unicode.IsControl) {
		return ""
	}
	return value
}

func statusHealth(value string) (float64, error) {
	switch strings.ToLower(value) {
	case "active", "good", "healthy", "ok", "running", "started", "up":
		return 1, nil
	case "bad", "degraded", "down", "failed", "faulted", "inactive", "stopped", "unhealthy":
		return 0, nil
	default:
		return 0, fmt.Errorf("unknown node service/process status")
	}
}

func fluxDimension(index map[string]int, row []string, names ...string) (string, bool) {
	for _, name := range names {
		if position, exists := index[name]; exists && row[position] != "" {
			return row[position], true
		}
	}
	return "", false
}

func optionalFloat(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func likelyBondMember(value string) bool {
	lower := strings.ToLower(value)
	for _, prefix := range []string{"em", "eno", "enp", "ens", "eth"} {
		if strings.HasPrefix(lower, prefix) &&
			strings.ContainsAny(strings.TrimPrefix(lower, prefix), "0123456789") {
			return true
		}
	}
	return false
}

func performanceThroughput(value float64, unit string) (float64, error) {
	multiplier := float64(1)
	switch strings.ToUpper(strings.TrimSpace(unit)) {
	case "", "B/S", "BYTE/S", "BYTES/S":
	case "KB/S":
		multiplier = 1_000
	case "MB/S":
		multiplier = 1_000_000
	case "GB/S":
		multiplier = 1_000_000_000
	case "TB/S":
		multiplier = 1_000_000_000_000
	default:
		return 0, fmt.Errorf("performance throughput unit is unsupported")
	}
	result := value * multiplier
	if !finite(result) {
		return 0, fmt.Errorf("performance throughput is too large")
	}
	return result, nil
}

func performanceLatency(value float64, unit string) (float64, error) {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "", "ms", "millisecond", "milliseconds":
		return value / 1_000, nil
	case "s", "second", "seconds":
		return value, nil
	default:
		return 0, fmt.Errorf("performance latency unit is unsupported")
	}
}

func normalizeOperation(value string) string {
	upper := strings.ToUpper(normalizeDimension(value, 32))
	switch upper {
	case "ALL", "DELETE", "GET", "HEAD", "POST", "PUT", "READ", "WRITE":
		return upper
	default:
		return ""
	}
}

func operationFromField(field string) string {
	lower := strings.ToLower(field)
	for _, operation := range []string{"delete", "get", "head", "post", "put", "read", "write"} {
		if strings.Contains(lower, operation) {
			return strings.ToUpper(operation)
		}
	}
	return ""
}

func normalizeStatusClass(value string) string {
	switch strings.ToLower(normalizeDimension(value, 16)) {
	case "2xx", "success", "succeed":
		return "2xx"
	case "4xx", "client_error":
		return "4xx"
	case "5xx", "error", "server_error":
		return "5xx"
	default:
		return ""
	}
}

func statusClassFromField(field string) string {
	lower := strings.ToLower(field)
	switch {
	case strings.Contains(lower, "succeed"), strings.Contains(lower, "success"),
		strings.Contains(lower, "2xx"):
		return "2xx"
	case strings.Contains(lower, "client"), strings.Contains(lower, "4xx"):
		return "4xx"
	case strings.Contains(lower, "error"), strings.Contains(lower, "failed"),
		strings.Contains(lower, "server"), strings.Contains(lower, "5xx"):
		return "5xx"
	default:
		return ""
	}
}

func normalizeQuantile(value string) string {
	switch strings.ToLower(normalizeDimension(value, 16)) {
	case "p50", "0.5", "50":
		return "0.5"
	case "p95", "0.95", "95":
		return "0.95"
	case "p99", "0.99", "99":
		return "0.99"
	default:
		return ""
	}
}

func firstString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func collectStrings(value any) []string {
	var result []string
	switch typed := value.(type) {
	case string:
		if typed = strings.TrimSpace(typed); typed != "" {
			result = append(result, typed)
		}
	case []any:
		for _, entry := range typed {
			result = append(result, collectStrings(entry)...)
		}
	}
	return result
}
