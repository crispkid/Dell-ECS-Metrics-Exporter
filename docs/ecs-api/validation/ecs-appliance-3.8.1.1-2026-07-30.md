# ECS Appliance 3.8.1.1 Live Validation Record

- Executed: `2026-07-30`
- Environment: authorized physical Dell ECS appliance
- Product version returned by `GET /vdc/nodes`:
  `3.8.1.1.140118.8d698782e5d`
- Selected Exporter Profile: `ecs-3.8.1`
- Result: corrected Node resource and Performance mappings partially live-verified;
  formal Profile certification remains pending
- ECS-011 disposition: the successfully exercised functions listed in
  `../feature-validation.md` are `validated-shared` across all four target Profiles

This record is deliberately redacted. It contains no endpoint, credential, token,
Authorization header, private address, Node identifier, Namespace name or raw response body.

## Environment and Security Boundary

The read-only validation observed five healthy Nodes. The available account had an accepted
monitoring role, but least-privilege `SYSTEM_MONITOR` behavior was not independently proven.
No workload, Namespace, Bucket or appliance configuration was changed.

The appliance certificate was self-signed for a name that did not match the configured
management address. The user explicitly authorized `tls.verify: false`. The Exporter accepted
that setting and emitted a redacted startup warning containing only the configured
Cluster/environment labels. This proves the configured exception works; it does not prove TLS
server identity. TLS verification remains enabled by default.

## Corrected Exporter Result

| Contract | Live result | Evidence boundary |
|---|---|---|
| Login/version/whoami/logout | HTTP 200; exact build selected `ecs-3.8.1`; graceful shutdown returned logout HTTP 200 | Token expiry/renewal and least privilege remain untested |
| Cluster/Node Management API | Cluster health/capacity and five-Node inventory/health parsed successfully | Failure injection and mixed-version behavior remain untested |
| Namespace/Bucket Management API | Inventory, quota and billing response shapes parsed successfully | No known-size workload was created in this production appliance |
| Node CPU | Separate Flux query preserved `cpu=cpu-total`; five `ecs_node_cpu_usage_ratio` series exported | Latest-snapshot behavior only |
| Node memory | Separate Flux query returned used/total; five series of each metric exported | Latest-snapshot behavior only |
| Node network | Separate Flux query preserved `interface`; ten receive and ten transmit series exported across five Nodes | Two observed interfaces per Node; reset/range behavior remains untested |
| Node disk | Query and mapping are implemented and fixture-tested | Conditional disk collection was not enabled in this run |
| Performance | Three exact split queries repeatedly returned HTTP 200 and the collector completed successfully; VDC latency returned four valid rows during the probe | The final zero-load window contained no VDC core or Namespace rows, so non-zero workload values remain a certification gap |
| Empty Flux result | HTTP 200 with `Columns:null` and `Values:null` was accepted as an empty result | Malformed non-empty responses still fail closed |
| Prometheus/readiness | 32 metric families passed `metricscheck`; liveness and readiness were HTTP 200, readiness `UP` | No external Prometheus server was deployed |

The final sanitized Exporter summary was:

```text
liveness_http=200
readiness_http=200 status=UP
selected_profile=ecs-3.8.1 mixed_version=false
nodes=5
node_cpu_series=5
node_memory_used_series=5
node_memory_total_series=5
node_network_receive_series=10
node_network_transmit_series=10
prometheus_metric_families=32
```

Performance source-shape discovery also established:

- VDC throughput fields are `total_read_requests_size` and
  `total_write_requests_size`;
- latency uses `id=ttfb_read|ttlb_write` and fields `p50|p99`;
- VDC transaction/error measurements have no VDC tag and therefore use the
  Management Cluster VDC identity;
- Namespace request measurements preserve the `namespace` tag;
- there is no observed or documented Namespace throughput/latency measurement;
- non-`*_delta` transaction/error values are request rates.

The corrected collector uses separate, narrowly filtered queries so ECS cannot remove the
dimensions required by an unrelated measurement. All split responses are parsed and merged
before the cache is replaced; a failed request or invalid response leaves the prior complete
snapshot intact.

## Evidence and Certification Decision

Only sanitized, official-shape fixtures and this aggregate record are committed. Protected
temporary credentials, request/response bodies, logs and metrics snapshots are deleted after
validation.

This partial appliance run does not populate Profile `tested_builds` and does not set
`sandbox_certified=true`. Remaining certification gates include:

- authenticated Dell REST API ZIP comparison;
- verified-CA/SAN testing and proxy/load-balancer Host/SNI behavior;
- token expiry/renewal, representative API failures and atomic-cache failure injection live;
- disjoint Flux range, counter reset and freshness boundary validation;
- non-zero representative S3 workload for VDC/Namespace performance;
- conditional disk, multi-VDC replication/link/recovery and target-scale duration testing;
- rerun from the immutable release candidate plus independent maintainer/security approval.
