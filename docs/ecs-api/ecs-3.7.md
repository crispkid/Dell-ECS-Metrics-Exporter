# ECS 3.7.x Profile Mapping

- Profile: `profiles/ecs-3.7.json`
- Range: `>=3.7.0.0, <3.8.0.0`
- Base contract: `common-contract.md`
- Mapping evidence: official ECS 3.7 documentation index plus Dell Flux known-issue article
- REST API ZIP review: pending authenticated Dell Support access
- Sandbox certification: not run
- Shared feature validation: `validated-shared` under `feature-validation.md`

## Official Sources

- [ECS 3.7 Product Documentation Index](https://www.dell.com/support/kbdoc/en-us/000195190/ecs-3-7-product-documentation-index)
- [Dell KB 000211906 - Flux values in ECS 3.7/3.8](https://www.dell.com/support/kbdoc/en-us/000211906/ecs-different-values-from-flux-api-since-upgrading-to-3-7-3-8)

## Coverage

| Mapping family | Status | ECS 3.7 decision |
|---|---|---|
| Authentication/version discovery | validated-shared | Same production flow passed on target-family CE/appliance versions |
| Cluster/node inventory and health | validated-shared | Common endpoints and parsers passed live on other target versions |
| Namespace/bucket inventory/quota/billing | validated-shared | Non-empty, known-size quota/billing path passed live on other target versions |
| Flux latest snapshot | validated-shared | Use `last()` only for CPU/memory/network counters |
| Flux interval rates/deltas | unsupported for this Profile | Dell reports time-range behavior can return cumulative totals |
| VDC/namespace performance | validated-shared, conditional | Shared query/parser feature is validated; interval-derived rates remain disabled |
| Bucket performance | unsupported | No bucket-scope evidence |
| Replication/recovery | pending | No target version has live status/lag/recovery field evidence |

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

## Remaining Version-Specific Assurance

The following work increases exact-version assurance but no longer blocks shared feature status:

1. ECS 3.7 REST API ZIP is compared with the common contract.
2. Redacted live fixtures are captured from at least one exact 3.7 build.
3. Known-size unit assertions and token expiry behavior pass.
4. Flux test proves `last()` freshness and confirms interval-rate collectors stay disabled.

## 2026-08-01 Validation Position

The official Dell documentation index confirms that an ECS 3.7 Monitoring Guide and REST API
reference exist, while Dell KB 000211906 requires the existing interval-rate prohibition.
The shared split mapping passed the `ecs-3.7` fixture replay, including selection of
`flux_interval_rates=unavailable`. Under ECS-011, live evidence from other target versions makes
the common implemented functions `validated-shared`; an exact 3.7
[redacted appliance probe](flux-probe.md) is optional regression evidence for version-specific
assurance, not a prerequisite for those feature statuses.
