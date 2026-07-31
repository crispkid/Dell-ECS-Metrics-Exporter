package ecs

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"dell-ecs-metrics-exporter/internal/model"
)

func TestManagementFixtureMappings(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 1, 15, 13, 0, 0, 0, time.UTC)
	capacity, err := ParseCapacity(fixture(t, "common", "capacity.json"))
	if err != nil || capacity.TotalBytes != 750e9 || capacity.UsedBytes != 550e9 ||
		capacity.AvailableBytes != 200e9 {
		t.Fatalf("capacity = %#v err=%v", capacity, err)
	}
	vdc, clusterHealth, err := ParseClusterHealth(fixture(t, "common", "localzone-health.json"))
	if err != nil || vdc != "vdc-a" || clusterHealth != 1 {
		t.Fatalf("cluster health = %q %v err=%v", vdc, clusterHealth, err)
	}
	nodes, versions, err := ParseNodes(fixture(t, "ecs-3.6", "nodes.json"), "alpha", now)
	if err != nil || len(nodes) != 2 || versions[0] != "3.6.2.6.123456.synthetic" {
		t.Fatalf("nodes = %#v versions=%v err=%v", nodes, versions, err)
	}
	nodeHealth, err := ParseNodeHealth([]byte(`{"nodes":[
		{"id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","healthStatus":"Good"},
		{"id":"bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb","healthStatus":"degraded"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	ApplyNodeHealth(nodes, nodeHealth)
	if *nodes[0].Health != 1 || *nodes[1].Health != 0 {
		t.Fatalf("applied health = %#v", nodes)
	}

	namespaces, err := ParseNamespaces(fixture(t, "common", "namespaces.json"), "alpha", now)
	if err != nil || len(namespaces) != 1 || namespaces[0].HardQuotaConfigured {
		t.Fatalf("namespaces = %#v err=%v", namespaces, err)
	}
	namespaceQuota, err := ParseNamespaceQuota(fixture(t, "common", "namespace-quota-configured.json"))
	if err != nil || *namespaceQuota.HardBytes != 1000e9 || *namespaceQuota.SoftBytes != 800e9 {
		t.Fatalf("namespace quota = %#v err=%v", namespaceQuota, err)
	}
	namespaceBilling, err := ParseNamespaceBilling(fixture(t, "common", "namespace-billing-info.json"))
	if err != nil || *namespaceBilling.UsedBytes != 3_145_728 || *namespaceBilling.ObjectCount != 12 {
		t.Fatalf("namespace billing = %#v err=%v", namespaceBilling, err)
	}
	ApplyQuotaToNamespace(&namespaces[0], namespaceQuota)
	ApplyBillingToNamespace(&namespaces[0], namespaceBilling)
	if !namespaces[0].HardQuotaConfigured || namespaces[0].CollectedAt.Hour() != 13 ||
		namespaces[0].UsageSampleAt == nil || namespaces[0].UsageSampleAt.Hour() != 12 ||
		namespaces[0].Owner == nil || *namespaces[0].Owner != "synthetic-owner" ||
		namespaces[0].AuditEnabled == nil || !*namespaces[0].AuditEnabled {
		t.Fatalf("enriched namespace = %#v", namespaces[0])
	}

	buckets, err := ParseBuckets(fixture(t, "common", "buckets.json"), "alpha", now)
	if err != nil || len(buckets) != 2 || buckets[0].HardQuotaConfigured ||
		!buckets[1].HardQuotaConfigured {
		t.Fatalf("buckets = %#v err=%v", buckets, err)
	}
	bucketQuota, err := ParseBucketQuota(fixture(t, "common", "bucket-quota-configured.json"))
	if err != nil || *bucketQuota.HardBytes != 500e9 || *bucketQuota.SoftBytes != 400e9 {
		t.Fatalf("bucket quota = %#v err=%v", bucketQuota, err)
	}
	bucketBilling, err := ParseBucketBilling(fixture(t, "common", "bucket-billing-info.json"))
	if err != nil || *bucketBilling.UsedBytes != 2_097_152 || *bucketBilling.ObjectCount != 8 {
		t.Fatalf("bucket billing = %#v err=%v", bucketBilling, err)
	}
	ApplyQuotaToBucket(&buckets[0], bucketQuota)
	ApplyBillingToBucket(&buckets[0], bucketBilling)
	if !buckets[0].SoftQuotaConfigured || buckets[0].UsageSampleAt == nil ||
		buckets[0].LastModifiedAt == nil ||
		buckets[0].Retention == nil || *buckets[0].Retention != "3600s" {
		t.Fatalf("enriched bucket = %#v", buckets[0])
	}
}

func TestECS380LiveEnvelopeVariants(t *testing.T) {
	t.Parallel()

	capacity, err := ParseCapacity([]byte(`{
		"totalProvisioned_gb": 1010,
		"totalFree_gb": 999
	}`))
	if err != nil || capacity.TotalBytes != 1010e9 ||
		capacity.AvailableBytes != 999e9 || capacity.UsedBytes != 11e9 {
		t.Fatalf("top-level capacity = %#v err=%v", capacity, err)
	}

	health, err := ParseNodeHealth([]byte(`{
		"_embedded": {
			"_instances": [
				{"id": "node-a", "displayName": "luna", "healthStatus": "Good"}
			]
		}
	}`))
	if err != nil || health["node-a"] != 1 {
		t.Fatalf("embedded node health = %#v err=%v", health, err)
	}

	quota, err := ParseNamespaceQuota([]byte(`{
		"namespace": "exporter-validation",
		"blockSize": -1,
		"notificationSize": -1
	}`))
	if err != nil || quota.HardBytes != nil || quota.SoftBytes != nil {
		t.Fatalf("top-level namespace quota = %#v err=%v", quota, err)
	}

	bucketQuota, err := ParseBucketQuota(
		fixture(t, "ecs-3.8.0.3-live", "bucket-quota.json"),
	)
	if err != nil || bucketQuota.HardBytes == nil || *bucketQuota.HardBytes != 2e9 ||
		bucketQuota.SoftBytes == nil || *bucketQuota.SoftBytes != 1e9 {
		t.Fatalf("top-level bucket quota = %#v err=%v", bucketQuota, err)
	}

	bucketBilling, err := ParseBucketBilling(
		fixture(t, "ecs-3.8.0.3-live", "bucket-billing-info.json"),
	)
	if err != nil || bucketBilling.UsedBytes == nil ||
		*bucketBilling.UsedBytes != 7_000_000 ||
		bucketBilling.ObjectCount == nil || *bucketBilling.ObjectCount != 3 {
		t.Fatalf("top-level bucket billing = %#v err=%v", bucketBilling, err)
	}

	namespaceBilling, err := ParseNamespaceBilling(
		fixture(t, "ecs-3.8.0.3-live", "namespace-billing-info.json"),
	)
	if err != nil || namespaceBilling.UsedBytes == nil ||
		*namespaceBilling.UsedBytes != 10_000_000 ||
		namespaceBilling.ObjectCount == nil || *namespaceBilling.ObjectCount != 4 {
		t.Fatalf("top-level namespace billing = %#v err=%v", namespaceBilling, err)
	}
}

func TestECS3814LiveEnvelopeVariants(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 26, 10, 5, 0, 0, time.UTC)
	nodes, versions, err := ParseNodes(
		fixture(t, "ecs-3.8.1.4-live", "nodes.json"), "alpha", now,
	)
	if err != nil || len(nodes) != 1 ||
		len(versions) != 1 || versions[0] != "3.8.1.4.140200.8103892f11b" {
		t.Fatalf("3.8.1.4 nodes = %#v versions=%v err=%v", nodes, versions, err)
	}

	namespaceQuota, err := ParseNamespaceQuota(
		fixture(t, "ecs-3.8.1.4-live", "namespace-quota-unset.json"),
	)
	if err != nil || namespaceQuota.HardBytes != nil || namespaceQuota.SoftBytes != nil {
		t.Fatalf("3.8.1.4 namespace quota = %#v err=%v", namespaceQuota, err)
	}

	bucketQuota, err := ParseBucketQuota(
		fixture(t, "ecs-3.8.1.4-live", "bucket-quota-configured.json"),
	)
	if err != nil || bucketQuota.HardBytes == nil || *bucketQuota.HardBytes != 2e9 ||
		bucketQuota.SoftBytes == nil || *bucketQuota.SoftBytes != 1e9 {
		t.Fatalf("3.8.1.4 configured bucket quota = %#v err=%v", bucketQuota, err)
	}
	unsetBucketQuota, err := ParseBucketQuota(
		fixture(t, "ecs-3.8.1.4-live", "bucket-quota-unset.json"),
	)
	if err != nil || unsetBucketQuota.HardBytes != nil || unsetBucketQuota.SoftBytes != nil {
		t.Fatalf("3.8.1.4 unset bucket quota = %#v err=%v", unsetBucketQuota, err)
	}

	namespaceBilling, err := ParseNamespaceBilling(
		fixture(t, "ecs-3.8.1.4-live", "namespace-billing-info.json"),
	)
	if err != nil || namespaceBilling.UsedBytes == nil ||
		*namespaceBilling.UsedBytes != 10_485_760 ||
		namespaceBilling.ObjectCount == nil || *namespaceBilling.ObjectCount != 4 {
		t.Fatalf("3.8.1.4 namespace billing = %#v err=%v", namespaceBilling, err)
	}
	bucketBillings, err := ParseBucketBillings(
		fixture(t, "ecs-3.8.1.4-live", "bucket-billing-batch.json"),
	)
	if err != nil || len(bucketBillings) != 3 ||
		*bucketBillings["redacted-namespace\x00redacted-bucket-quota"].UsedBytes != 7_340_032 ||
		*bucketBillings["redacted-namespace\x00redacted-bucket-unset"].UsedBytes != 3_145_728 ||
		*bucketBillings["redacted-namespace\x00redacted-bucket-empty"].ObjectCount != 0 {
		t.Fatalf("3.8.1.4 bucket billings = %#v err=%v", bucketBillings, err)
	}

	replication, err := ParseReplication(
		fixture(t, "ecs-3.8.1.4-live", "replication-group-single-vdc.json"),
		"alpha",
		now,
	)
	if err != nil || replication.Status != nil || replication.PendingBytes == nil ||
		*replication.PendingBytes != 0 || replication.LagSeconds != nil {
		t.Fatalf("3.8.1.4 single-VDC replication = %#v err=%v", replication, err)
	}
}

func TestWhoAmIRoleValidation(t *testing.T) {
	t.Parallel()
	name, roles, err := ParseWhoAmI([]byte(`{"user":{"common_name":"monitor",
		"roles":{"role":["SYSTEM_MONITOR","AUDITOR"]}}}`))
	if err != nil || name != "monitor" || len(roles) != 2 {
		t.Fatalf("whoami = %q %v err=%v", name, roles, err)
	}
	name, roles, err = ParseWhoAmI([]byte(`{"name":"admin","roles":["SYSTEM_ADMIN"]}`))
	if err != nil || name != "admin" || roles[0] != "SYSTEM_ADMIN" {
		t.Fatalf("alternate whoami = %q %v err=%v", name, roles, err)
	}
	for _, data := range []string{
		`{"user":{"roles":{"role":"SYSTEM_MONITOR"}}}`,
		`{"user":{"common_name":"user","roles":{"role":"NAMESPACE_ADMIN"}}}`,
		`{"user":{"common_name":"user","roles":{"description":"SYSTEM_MONITOR"}}}`,
		`[]`,
	} {
		if _, _, err := ParseWhoAmI([]byte(data)); err == nil {
			t.Fatalf("invalid whoami accepted: %s", data)
		}
	}
}

func TestFluxAndReplicationFixtureMappings(t *testing.T) {
	t.Parallel()
	resources, err := ParseFluxNodeResources(fixture(t, "common", "flux-node-resources.json"))
	if err != nil {
		t.Fatal(err)
	}
	const nodeID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	resource := resources[nodeID]
	if math.Abs(*resource.CPUUsageRatio-0.275) > 1e-12 ||
		*resource.MemoryUsedBytes != 8589934592 || len(resource.Network) != 1 ||
		*resource.Network[0].ReceiveBytes != 123456789 {
		t.Fatalf("resource = %#v", resource)
	}
	nodes := []model.Node{{ID: nodeID}, {ID: "missing"}}
	ApplyNodeResources(nodes, resources)
	if nodes[0].CPUUsageRatio == nil || nodes[1].CPUUsageRatio != nil {
		t.Fatalf("applied resources = %#v", nodes)
	}

	rpo := time.Unix(1768478100, 0).UTC()
	replication, err := ParseReplication(
		fixture(t, "common", "rg-link.json"), "alpha", rpo.Add(5*time.Minute),
	)
	if err != nil || replication.Status == nil || *replication.Status != 1 ||
		*replication.PendingBytes != 1572864 || *replication.LagSeconds != 300 ||
		*replication.RecoveryKind != "bootstrap" || *replication.RecoveryProgress != 0.25 {
		t.Fatalf("replication = %#v err=%v", replication, err)
	}
	group, err := ParseReplication(
		fixture(t, "common", "replication-group.json"), "alpha", rpo.Add(time.Minute),
	)
	if err != nil || group.Status != nil || *group.PendingBytes != 1572864 {
		t.Fatalf("replication group = %#v err=%v", group, err)
	}
}

func TestPerformanceBatchBillingAndNodeStatusMappings(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	performance, err := ParseFluxPerformance(
		fixture(t, "common", "flux-vdc-performance.json"), "alpha", "fallback-vdc", now,
	)
	if err != nil || len(performance) != 12 {
		t.Fatalf("performance = %#v err=%v", performance, err)
	}
	if performance[0].Metric != model.PerformanceReadThroughput ||
		performance[0].Value != 125_500_000 ||
		performance[5].Metric != model.PerformanceLatency ||
		math.Abs(performance[5].Value-0.0125) > 1e-12 {
		t.Fatalf("performance conversion = %#v", performance)
	}
	var official []model.Performance
	for _, name := range []string{
		"flux-vdc-core-performance.json",
		"flux-vdc-latency.json",
		"flux-namespace-performance.json",
	} {
		values, parseErr := ParseFluxPerformance(
			fixture(t, "common", name), "alpha", "fallback-vdc", now,
		)
		if parseErr != nil {
			t.Fatalf("%s: %v", name, parseErr)
		}
		official = append(official, values...)
	}
	if len(official) != 12 ||
		official[0].Metric != model.PerformanceReadThroughput ||
		official[2].StatusClass != "2xx" ||
		official[3].StatusClass != "4xx" ||
		official[4].StatusClass != "5xx" ||
		official[5].Operation != "READ" ||
		official[5].Quantile != "0.5" ||
		math.Abs(official[5].Value-0.0125) > 1e-12 ||
		official[9].Namespace != "namespace-a" {
		t.Fatalf("official performance mapping = %#v", official)
	}

	billings, err := ParseBucketBillings(fixture(t, "common", "bucket-billing-batch.json"))
	if err != nil || len(billings) != 2 ||
		*billings["namespace-a\x00bucket-b"].UsedBytes != 1_048_576 {
		t.Fatalf("batch billing = %#v err=%v", billings, err)
	}
	liveBillings, err := ParseBucketBillings(
		fixture(t, "ecs-3.8.0.3-live", "bucket-billing-batch.json"),
	)
	if err != nil || len(liveBillings) != 3 ||
		*liveBillings["redacted-namespace\x00redacted-bucket"].UsedBytes != 7_000_000 ||
		*liveBillings["redacted-namespace\x00redacted-bucket-unset"].UsedBytes != 3_000_000 {
		t.Fatalf("plural live batch billing = %#v err=%v", liveBillings, err)
	}
	tbBilling, err := ParseNamespaceBilling([]byte(
		`{"total_size":1.5,"total_size_unit":"TB","total_objects":2}`,
	))
	if err != nil || *tbBilling.UsedBytes != 1_500_000_000_000 {
		t.Fatalf("TB billing = %#v err=%v", tbBilling, err)
	}

	statuses, err := ParseNodeStatuses([]byte(`{"node":[{
		"nodeid":"node-a","healthStatus":"Good",
		"services":[{"name":"blobsvc","status":"running"}],
		"processes":[{"name":"fabric-lifecycle","status":"stopped"}]
	}]}`))
	if err != nil || len(statuses["node-a"].Services) != 2 ||
		statuses["node-a"].Services[0].Health != 0 ||
		statuses["node-a"].Services[1].Health != 1 {
		t.Fatalf("node statuses = %#v err=%v", statuses, err)
	}
}

func TestFluxNullSeriesPlaceholderIsAnEmptyResult(t *testing.T) {
	t.Parallel()
	payload := []byte(`{"Series":[{"Datatypes":null,"Columns":null,"Values":null}]}`)
	resources, err := ParseFluxNodeResources(payload)
	if err != nil || len(resources) != 0 {
		t.Fatalf("node resources = %#v err=%v", resources, err)
	}
	performance, err := ParseFluxPerformance(
		payload, "alpha", "vdc-a", time.Now().UTC(),
	)
	if err != nil || len(performance) != 0 {
		t.Fatalf("performance = %#v err=%v", performance, err)
	}
}

func TestNodeResourceSelectionCardinalityAndCounterReset(t *testing.T) {
	t.Parallel()
	payload := []byte(`{"Series":[{"Columns":
		["_measurement","_field","_value","_time","node_id","path","interface"],
		"Values":[
			["disk","used","40","2026-01-01T00:00:00Z","node-a","/data",""],
			["disk","total","100","2026-01-01T00:00:00Z","node-a","/data",""],
			["disk","used","99","2026-01-01T00:00:00Z","node-a","/excluded",""],
			["disk","total","100","2026-01-01T00:00:00Z","node-a","/excluded",""],
			["net","bytes_recv","20","2026-01-01T00:00:00Z","node-a","","bond0"],
			["net","bytes_sent","30","2026-01-01T00:00:00Z","node-a","","bond0"],
			["net","bytes_recv","10","2026-01-01T00:00:00Z","node-a","","eth0"],
			["net","bytes_sent","15","2026-01-01T00:00:00Z","node-a","","eth0"]
		]}]}`)
	resources, err := ParseFluxNodeResourcesWithPolicy(payload, NodeResourcePolicy{
		Filesystems:          []string{"/data"},
		MaxNetworkInterfaces: 2,
		PreferBondInterfaces: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	resource := resources["node-a"]
	if *resource.DiskUsedBytes != 40 || *resource.DiskTotalBytes != 100 ||
		len(resource.Network) != 1 || resource.Network[0].Interface != "bond0" {
		t.Fatalf("selected resources = %#v", resource)
	}
	if _, err := ParseFluxNodeResourcesWithPolicy(payload, NodeResourcePolicy{
		Filesystems:          []string{"/data"},
		MaxNetworkInterfaces: 1,
	}); err == nil {
		t.Fatal("network interface cardinality limit was not enforced")
	}

	lowerCounter := []byte(strings.ReplaceAll(string(payload), `"20"`, `"2"`))
	lower, err := ParseFluxNodeResourcesWithPolicy(lowerCounter, NodeResourcePolicy{
		Filesystems:          []string{"/data"},
		NetworkInterfaces:    []string{"bond0"},
		MaxNetworkInterfaces: 1,
	})
	if err != nil || *lower["node-a"].Network[0].ReceiveBytes != 2 {
		t.Fatalf("counter reset sample = %#v err=%v", lower, err)
	}
}

func TestPaginationQuotaAndOptionalSemantics(t *testing.T) {
	t.Parallel()
	pageJSON := `{"object_bucket":[{"name":"b","namespace":"n"}],"nextMarker":"opaque"}`
	buckets, page, err := ParseBucketPage([]byte(pageJSON), "alpha", time.Now())
	if err != nil || len(buckets) != 1 || page.NextMarker != "opaque" {
		t.Fatalf("page = %#v buckets=%#v err=%v", page, buckets, err)
	}
	for _, path := range []string{"namespace-quota-unset.json", "bucket-quota-unset.json"} {
		var quota Quota
		if strings.HasPrefix(path, "namespace") {
			quota, err = ParseNamespaceQuota(fixture(t, "common", path))
		} else {
			quota, err = ParseBucketQuota(fixture(t, "common", path))
		}
		if err != nil || quota.HardBytes != nil || quota.SoftBytes != nil {
			t.Fatalf("%s quota = %#v err=%v", path, quota, err)
		}
	}
}

func TestParserRejectsInvalidResponses(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		run  func() error
	}{
		{"capacity free over total", func() error {
			_, err := ParseCapacity([]byte(`{"cluster_capacity":{"totalProvisioned_gb":1,"totalFree_gb":2}}`))
			return err
		}},
		{"capacity overflow", func() error {
			_, err := ParseCapacity([]byte(`{"cluster_capacity":{"totalProvisioned_gb":1e308,"totalFree_gb":0}}`))
			return err
		}},
		{"capacity ambiguous envelope", func() error {
			_, err := ParseCapacity([]byte(`{
				"totalProvisioned_gb":1,
				"totalFree_gb":0,
				"cluster_capacity":{"totalProvisioned_gb":1,"totalFree_gb":0}
			}`))
			return err
		}},
		{"health missing", func() error {
			_, _, err := ParseClusterHealth([]byte(`{"name":"vdc","numNodes":1}`))
			return err
		}},
		{"duplicate node", func() error {
			_, _, err := ParseNodes([]byte(`{"node":[
				{"nodeid":"a","nodename":"a","version":"3.6.0.0"},
				{"nodeid":"a","nodename":"b","version":"3.6.0.0"}]}`), "alpha", time.Now())
			return err
		}},
		{"unknown node health", func() error {
			_, err := ParseNodeHealth([]byte(`{"node":[{"nodeid":"a","healthStatus":"mystery"}]}`))
			return err
		}},
		{"duplicate node health", func() error {
			_, err := ParseNodeHealth([]byte(`{"node":[
				{"nodeid":"a","healthStatus":"good"},
				{"nodeid":"a","healthStatus":"bad"}]}`))
			return err
		}},
		{"duplicate namespace", func() error {
			_, err := ParseNamespaces([]byte(`{"namespace":[{"name":"n"},{"name":"n"}]}`), "alpha", time.Now())
			return err
		}},
		{"invalid quota", func() error {
			_, err := ParseBucketQuota([]byte(`{"bucket_quota_details":{"blockSize":-2,"notificationSize":1}}`))
			return err
		}},
		{"namespace quota ambiguous envelope", func() error {
			_, err := ParseNamespaceQuota([]byte(`{
				"blockSize":1,
				"notificationSize":-1,
				"namespace_quota_details":{"blockSize":1,"notificationSize":-1}
			}`))
			return err
		}},
		{"bucket quota ambiguous envelope", func() error {
			_, err := ParseBucketQuota([]byte(`{
				"blockSize":1,
				"notificationSize":-1,
				"bucket_quota_details":{"blockSize":1,"notificationSize":-1}
			}`))
			return err
		}},
		{"quota overflow", func() error {
			_, err := ParseBucketQuota([]byte(`{"bucket_quota_details":{
				"blockSize":1e308,"notificationSize":-1}}`))
			return err
		}},
		{"soft quota over hard", func() error {
			_, err := ParseBucketQuota([]byte(`{"bucket_quota_details":{"blockSize":1,"notificationSize":2}}`))
			return err
		}},
		{"unsupported billing unit", func() error {
			_, err := ParseNamespaceBilling([]byte(`{"total_size":1,"total_size_unit":"KiB","total_objects":1}`))
			return err
		}},
		{"bucket billing ambiguous envelope", func() error {
			_, err := ParseBucketBilling([]byte(`{
				"total_size":1,
				"total_size_unit":"KB",
				"total_objects":1,
				"bucket_billing_info":{
					"total_size":1,
					"total_size_unit":"KB",
					"total_objects":1
				}
			}`))
			return err
		}},
		{"billing overflow", func() error {
			_, err := ParseNamespaceBilling([]byte(`{
				"total_size":1e308,"total_size_unit":"GB","total_objects":1}`))
			return err
		}},
		{"bad bucket timestamp", func() error {
			_, err := ParseBuckets([]byte(`{"object_bucket":[{"name":"b","namespace":"n","created":"yesterday"}]}`), "alpha", time.Now())
			return err
		}},
		{"bad marker", func() error {
			_, _, err := ParseBucketPage([]byte("{\"next_marker\":\"bad\\nmarker\"}"), "alpha", time.Now())
			return err
		}},
		{"bad retention", func() error {
			_, err := ParseBuckets([]byte(`{"object_bucket":[{
				"name":"b","namespace":"n","default_retention":-3}]}`), "alpha", time.Now())
			return err
		}},
		{"bad batch billing identity", func() error {
			_, err := ParseBucketBillings([]byte(`{"bucket_billing_info":[{
				"name":"","namespace":"n","total_size":1,"total_size_unit":"KB","total_objects":1}]}`))
			return err
		}},
		{"ambiguous batch billing envelope", func() error {
			_, err := ParseBucketBillings([]byte(`{
				"bucket_billing_info":[],
				"bucket_billing_infos":[]
			}`))
			return err
		}},
		{"flux row", func() error {
			_, err := ParseFluxNodeResources([]byte(`{"Series":[{"Columns":["_time"],"Values":[["x","extra"]]}]}`))
			return err
		}},
		{"flux duplicate column", func() error {
			_, err := ParseFluxNodeResources([]byte(`{"Series":[{"Columns":
				["_measurement","_field","_value","_time","node_id","node_id"],"Values":[]}]}`))
			return err
		}},
		{"flux range", func() error {
			_, err := ParseFluxNodeResources([]byte(`{"Series":[{"Columns":
				["_measurement","_field","_value","_time","node_id","cpu","_start","_stop"],
				"Values":[["cpu","usage_idle","50","2026-01-01T01:00:00Z","n","cpu-total",
				"2026-01-01T00:00:00Z","2026-01-01T01:00:00Z"]]}]}`))
			return err
		}},
		{"flux memory", func() error {
			_, err := ParseFluxNodeResources([]byte(`{"Series":[{"Columns":
				["_measurement","_field","_value","_time","node_id"],
				"Values":[["mem","used","2","2026-01-01T00:00:00Z","n"],
				["mem","total","1","2026-01-01T00:00:00Z","n"]]}]}`))
			return err
		}},
		{"performance envelope", func() error {
			_, err := ParseFluxPerformance([]byte(`{"Series":[{"Columns":
				["_measurement","_field","_value","_time"],
				"Values":[["unknown","value","1","2026-01-01T00:00:00Z"]]}]}`),
				"alpha", "vdc", time.Now())
			return err
		}},
		{"unknown replication status", func() error {
			_, err := ParseReplication([]byte(`{"id":"x","rgName":"g","rglinkStatus":"mystery"}`), "alpha", time.Now())
			return err
		}},
		{"future RPO", func() error {
			_, err := ParseReplication([]byte(`{"id":"x","rgName":"g",
				"chunksRepoPendingReplicationTotalSize":1,"replicationRpoTimestamp":9999999999}`),
				"alpha", time.Now())
			return err
		}},
		{"fractional RPO", func() error {
			_, err := ParseReplication([]byte(`{"id":"x","rgName":"g",
				"chunksRepoPendingReplicationTotalSize":1,"replicationRpoTimestamp":1.5}`),
				"alpha", time.Now())
			return err
		}},
		{"pending overflow", func() error {
			_, err := ParseReplication([]byte(`{"id":"x","rgName":"g",
				"chunksRepoPendingReplicationTotalSize":1e308,
				"chunksJournalPendingReplicationTotalSize":1e308}`), "alpha", time.Now())
			return err
		}},
		{"bad progress", func() error {
			_, err := ParseReplication([]byte(`{"id":"x","rgName":"g",
				"BootstrapState":"active","BootstrapProgressPercent":101}`), "alpha", time.Now())
			return err
		}},
		{"trailing JSON", func() error {
			_, err := ParseBuckets([]byte(`{"object_bucket":[]} {}`), "alpha", time.Now())
			return err
		}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if err := test.run(); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestNumberForms(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{`42`, `"4.2e1"`} {
		var number Number
		if err := json.Unmarshal([]byte(raw), &number); err != nil {
			t.Fatal(err)
		}
		value := number.Optional()
		if value == nil || *value != 42 {
			t.Fatalf("%s = %v", raw, value)
		}
	}
	for _, raw := range []string{`null`, `""`, `"NaN"`, `"Infinity"`, `{}`} {
		var number Number
		err := json.Unmarshal([]byte(raw), &number)
		if raw == "null" {
			if err != nil {
				t.Fatal(err)
			}
			if _, err = number.Required("value"); err == nil {
				t.Fatal("null number was required successfully")
			}
		} else if err == nil {
			t.Fatalf("%s was accepted", raw)
		}
	}
}

func fixture(t *testing.T, directory, name string) []byte {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test source")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "ecs", directory, name)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return content
}
