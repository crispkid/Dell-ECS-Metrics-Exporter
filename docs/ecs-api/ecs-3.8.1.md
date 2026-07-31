# ECS 3.8.1.x Profile Mapping

- Profile: `profiles/ecs-3.8.1.json`
- Range: `>=3.8.1.0, <3.8.2.0`
- Base contract: `common-contract.md`
- Mapping evidence: official ECS 3.8 index and Dell Host Header/Flux correction articles
- REST API ZIP review: pending authenticated Dell Support access
- Partial live evidence: ECS CE `3.8.1.4.140200.8103892f11b` and authorized
  appliance `3.8.1.1.140118.8d698782e5d`
- Sandbox certification: not complete
- Shared feature validation: `validated-shared` under `feature-validation.md`

## Official Sources

- [ECS 3.8 Product Documentation Index](https://www.dell.com/support/kbdoc/en-us/000205234/ecs-3-8-product-documentation-index-info-hub)
- [Dell KB 000205031 - ECS 3.8 Host Header](https://www.dell.com/support/kbdoc/en-us/000205031/ecs-how-to-perform-host-header-injection-for-3-8-x)
- [Dell KB 000211906 - Flux defect addressed in ECS 3.8.1](https://www.dell.com/support/kbdoc/en-us/000211906/ecs-different-values-from-flux-api-since-upgrading-to-3-7-3-8)
- [ECS 3.6.1 Administration Guide - Monitoring list of metrics: Performance](https://www.dell.com/support/manuals/en-us/ecs-appliance-/ecs_p_adminguide_3_6_1/monitoring-list-of-metrics-performance?guid=guid-261296f5-2e00-439a-9afe-a838db6672f4&lang=en-us)
- [Dell KB 000021869 - Grafana latency percentile representation](https://www.dell.com/support/kbdoc/en-us/000021869/ecs-grafana-dashboard-percentile-representation-explained)

## Coverage

| Mapping family | Status | ECS 3.8.1 decision |
|---|---|---|
| Authentication/version discovery | validated-shared | Exact build selected `ecs-3.8.1`; monitor login/whoami/logout returned HTTP 200 |
| Cluster/node inventory and health | validated-shared | Management-backed inventory/health succeeded; split Flux CPU/Memory/Network queries returned HTTP 200 on five appliance Nodes |
| Namespace/bucket inventory/quota/billing | validated-shared | Top-level configured/unset quota and plural batch billing passed known-size assertions |
| Flux latest snapshot | validated-shared | Separate `keep()`/`last()` queries preserve `cpu`, `interface`, `id`, and `namespace`; combined queries are prohibited because the appliance omitted required dimensions |
| Flux interval rates/deltas | conditional, sandbox-pending | Dell states the range defect is addressed in 3.8.1; the latest-snapshot probe passed but the disjoint range gate remains pending |
| VDC/namespace performance | validated-shared, conditional | VDC throughput/latency/request-rate and Namespace request-rate shapes observed; Namespace throughput/latency do not exist in the documented/live contract |
| Bucket performance | unsupported | No bucket-scope evidence |
| Replication/recovery | pending | Single-VDC RG transport parsed, but no target version supplied live status/RPO/recovery fields |

## ECS CE 3.8.1.4 Partial Live Evidence

The isolated CE run returned exact product version
`3.8.1.4.140200.8103892f11b` from `/vdc/nodes`. The Exporter selected this Profile,
completed all Management-backed collectors and emitted exact Inventory/Prometheus usage for
three Buckets and four known-size objects. Configured soft/hard quota values were
`1,000,000,000`/`2,000,000,000` bytes, while unset quotas remained `null` and omitted their
metrics.

Readiness was HTTP 200 `DEGRADED` only because the CE Flux endpoint returned HTTP 503 for
`node-resources`. See the
[redacted validation record](validation/ecs-ce-3.8.1.4-2026-07-26.md) and
`testdata/ecs/ecs-3.8.1.4-live/`.

This evidence changes candidate Management mappings to partial live observations for the
exact CE build only. It does not populate `tested_builds`, set `sandbox_certified=true` or
certify production appliance Flux behavior.

## ECS 3.8.1.1 Appliance Partial Live Evidence

The authorized read-only run returned exact product build
`3.8.1.1.140118.8d698782e5d` from five healthy Nodes and selected `ecs-3.8.1`.
Management-backed Cluster, Node, Namespace, quota, billing, Bucket and logout calls returned
HTTP 200. The appliance Flux endpoint was available and established these contracts:

- CPU, Memory and Network must be queried separately; CPU values require `cpu=cpu-total`,
  while Network values require the preserved `interface` column.
- `cq_performance_latency` uses `id=ttfb_read|ttlb_write` and fields `p50|p99`.
- VDC throughput/transaction/error measurements have no VDC tag and use the Management
  Cluster VDC identity.
- Only `cq_performance_transaction_ns` and `cq_performance_error_ns` provide the
  `namespace` tag; there is no evidenced Namespace throughput or latency mapping.
- A no-data window can return an all-null `Series` placeholder with HTTP 200; the parser treats
  that exact shape as an empty result rather than a missing-column error.
- Non-`*_delta` values are rates. Transaction success maps to 2xx, user errors to 4xx and
  system errors to 5xx. Aggregate failed transactions are omitted to prevent double counting.

The certificate was self-signed for localhost and did not match the management IP, so the
authorized run explicitly used `tls.verify: false`. This is supported configuration and emits
a startup warning, but it does not prove verified TLS identity. No endpoint, credential,
token, raw response, Node ID or Namespace name is retained in repository evidence.
See the
[redacted appliance validation record](validation/ecs-appliance-3.8.1.1-2026-07-30.md)
for the corrected Exporter series counts and remaining evidence boundary.

## Flux Re-enablement Gate

An exact 3.8.1 build may enable interval-derived rates only after a sandbox query proves:

1. every `_time` is inside the requested `_start`/`_stop`;
2. two disjoint ranges return disjoint samples;
3. counter resets are detected;
4. `*_delta` measurements are kept as interval Gauges;
5. no stale sample outside the freshness window is used.

Until those tests pass, runtime behavior must fall back to `last()` snapshots even though the
Profile documents the Dell fix.

## Host Header Contract

Fresh 3.8.1 systems can accept Management API calls without an accepted server name, but a
configuration created on 3.8.0 may persist across upgrade. Therefore the transport continues to
preserve hostname, SNI and Host consistently and reports rejected Host values as non-retryable.

## Certification Gaps

- Compare ECS 3.8.1 REST API ZIP with the common contract.
- Rerun the corrected Exporter and the Flux range-boundary re-enablement gate on every
  certified appliance build.
- Test both fresh install and upgrade-from-3.8.0 Host Header configurations.
- Validate multi-VDC replication status, lag and recovery fields.
- Rerun the protected certification workflow against production ECS 3.8.1.4 from the
  immutable release candidate.

## 2026-08-01 Validation Position

The shared split mapping passed the `ecs-3.8.1` fixture replay and preserves
`flux_interval_rates=conditional`. The existing ECS 3.8.1.1 appliance run remains the only
physical Flux observation; it proved Node CPU/Memory/Network shapes and Performance response
shape but not non-zero workload, range boundary or formal certification. Future exact-build
runs should use the [redacted appliance probe](flux-probe.md) so the same production parser and
redaction contract can be reviewed without collecting raw identities or values.
