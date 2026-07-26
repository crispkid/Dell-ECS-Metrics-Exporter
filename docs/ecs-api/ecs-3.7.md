# ECS 3.7.x Profile Mapping

- Profile: `profiles/ecs-3.7.json`
- Range: `>=3.7.0.0, <3.8.0.0`
- Base contract: `common-contract.md`
- Mapping evidence: official ECS 3.7 documentation index plus Dell Flux known-issue article
- REST API ZIP review: pending authenticated Dell Support access
- Sandbox certification: not run

## Official Sources

- [ECS 3.7 Product Documentation Index](https://www.dell.com/support/kbdoc/en-us/000195190/ecs-3-7-product-documentation-index)
- [Dell KB 000211906 - Flux values in ECS 3.7/3.8](https://www.dell.com/support/kbdoc/en-us/000211906/ecs-different-values-from-flux-api-since-upgrading-to-3-7-3-8)

## Coverage

| Mapping family | Status | ECS 3.7 decision |
|---|---|---|
| Authentication/version discovery | candidate-inherited | Same logical flow as common contract; verify REST ZIP/live |
| Cluster/node inventory and health | candidate-inherited | Use common endpoints; verify fields/enums |
| Namespace/bucket inventory/quota/billing | candidate-inherited | Use common endpoints; verify batch path, units and pagination |
| Flux latest snapshot | documented | Use `last()` only for CPU/memory/network counters |
| Flux interval rates/deltas | unsupported for this Profile | Dell reports time-range behavior can return cumulative totals |
| VDC/namespace performance | conditional | Do not expose interval-derived rates until an exact build proves correct |
| Bucket performance | unsupported | No bucket-scope evidence |
| Replication/recovery | candidate-inherited | Verify status enum and progress fields |

## Required Workaround

For ECS 3.7, queries must be shaped to return current snapshots:

```flux
from(bucket: "monitoring_op")
  |> range(start: -10m)
  |> filter(fn: (r) => r._measurement == "net")
  |> last()
```

The Exporter may publish `bytes_recv`/`bytes_sent` as reset-aware counters because they are
cumulative source values. It must not calculate a range-based rate or treat the query window
as proof that the returned value is a window delta.

## Certification Gaps

Every `candidate-inherited` record remains blocked until:

1. ECS 3.7 REST API ZIP is compared with the common contract.
2. Redacted live fixtures are captured from at least one exact 3.7 build.
3. Known-size unit assertions and token expiry behavior pass.
4. Flux test proves `last()` freshness and confirms interval-rate collectors stay disabled.
