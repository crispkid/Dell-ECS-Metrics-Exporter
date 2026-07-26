# ECS 3.8.1.x Profile Mapping

- Profile: `profiles/ecs-3.8.1.json`
- Range: `>=3.8.1.0, <3.8.2.0`
- Base contract: `common-contract.md`
- Mapping evidence: official ECS 3.8 index and Dell Host Header/Flux correction articles
- REST API ZIP review: pending authenticated Dell Support access
- Partial live evidence: ECS CE `3.8.1.4.140200.8103892f11b`
- Sandbox certification: not complete

## Official Sources

- [ECS 3.8 Product Documentation Index](https://www.dell.com/support/kbdoc/en-us/000205234/ecs-3-8-product-documentation-index-info-hub)
- [Dell KB 000205031 - ECS 3.8 Host Header](https://www.dell.com/support/kbdoc/en-us/000205031/ecs-how-to-perform-host-header-injection-for-3-8-x)
- [Dell KB 000211906 - Flux defect addressed in ECS 3.8.1](https://www.dell.com/support/kbdoc/en-us/000211906/ecs-different-values-from-flux-api-since-upgrading-to-3-7-3-8)

## Coverage

| Mapping family | Status | ECS 3.8.1 decision |
|---|---|---|
| Authentication/version discovery | CE-live-observed | Exact build selected `ecs-3.8.1`; monitor login/whoami/logout returned HTTP 200 |
| Cluster/node inventory and health | partial CE-live-observed | Management-backed inventory/health succeeded; Flux-backed Node resources returned HTTP 503 |
| Namespace/bucket inventory/quota/billing | CE-live-observed | Top-level configured/unset quota and plural batch billing passed known-size assertions |
| Flux latest snapshot | CE-blocked | The ECS CE Flux query endpoint returned HTTP 503; validate on an appliance |
| Flux interval rates/deltas | conditional, sandbox-pending | Dell states the range defect is addressed in 3.8.1; CE could not execute the gate |
| VDC/namespace performance | conditional, sandbox-pending | Same unavailable CE Flux endpoint; enable only after the range-boundary contract passes |
| Bucket performance | unsupported | No bucket-scope evidence |
| Replication/recovery | partial CE-live-observed | Single-VDC RG shape parsed with zero pending bytes and no status/RPO; multi-VDC links remain pending |

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
- Run Flux latest and the range-boundary re-enablement gate on every certified appliance build.
- Test both fresh install and upgrade-from-3.8.0 Host Header configurations.
- Validate multi-VDC replication status, lag and recovery fields.
- Rerun the protected certification workflow against production ECS 3.8.1.4 from the
  immutable release candidate.
