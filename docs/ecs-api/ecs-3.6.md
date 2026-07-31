# ECS 3.6.x Profile Mapping

- Profile: `profiles/ecs-3.6.json`
- Range: `>=3.6.0.0, <3.7.0.0`
- Mapping evidence: Dell ECS REST API `3.6.0.0.126369.160d4eb` and ECS 3.6.1
  Monitoring Guide
- Fixture class: synthetic, document-derived
- Sandbox certification: not run
- Shared feature validation: `validated-shared` under `feature-validation.md`

## Official Sources

- [ECS 3.6 REST API Reference ZIP](https://dl.dell.com/content/docu101271_ecs-3-6-rest-api-reference.zip)
- [ECS 3.6.1 Monitoring Guide](https://dl.dell.com/content/docu102929_ecs-3-6-1-monitoring-guide.pdf)
- [ECS 3.6 Dashboard API changes](https://www.dell.com/support/manuals/en-us/ecs-appliance-/ecs_p_adminguide_3_6_1/dashboard-apis?guid=guid-6dea9a0d-66b1-4f26-8aaf-f3b9a3c335da&lang=en-us)
- [ECS 3.6.1 Administration Guide - Flux API](https://www.dell.com/support/manuals/en-us/ecs-appliance-/ecs_p_adminguide_3_6_1/flux-api?guid=guid-48afcdc8-d89d-48b0-9b9d-7764bbc4d42b&lang=en-us)
- [ECS 3.6.1 Administration Guide - Flux replacements for deprecated Dashboard API](https://www.dell.com/support/manuals/en-us/ecs-appliance-/ecs_p_adminguide_3_6_1/flux-api-replacements-for-deprecated-dashboard-api?guid=guid-9436e3d5-9954-47bf-92d3-6a1adb78804f&lang=en-us)

## Coverage

| Mapping | Status | ECS 3.6 decision |
|---|---|---|
| `MAP-AUTH-001` | validated-shared | Token flow on port 4443 |
| `MAP-VERSION-001` | validated-shared | `/vdc/nodes` exposes per-node version |
| `MAP-CLUSTER-HEALTH-001` | validated-shared | Good/bad node and disk counts remain in Dashboard |
| `MAP-CLUSTER-CAPACITY-001` | validated-shared | Provisioned/free GB; used derived |
| `MAP-NODE-INFO-001` | validated-shared | Node identity/version; lockdown status is not health |
| `MAP-NODE-HEALTH-001` | validated-shared | Dashboard health only; removed performance fields ignored |
| `MAP-NODE-RESOURCE-001` | validated-shared | Flux `cpu`, `mem`, `net`; network counters preserve interface |
| `MAP-NODE-DISK-001` | pending | Requires filesystem/device allowlist; no target version has live Disk evidence |
| Namespace/Bucket inventory/quota/billing | validated-shared | `-1` quota means unset; billing is sampled |
| VDC/Namespace performance | validated-shared | Flux only; rate vs delta semantics preserved |
| Bucket performance | unsupported | No bucket-scope operation/latency mapping |
| Replication | pending | Dashboard RG/RG link implemented; status/lag have no qualifying live evidence |
| Recovery | pending | Operation kind must distinguish failover/bootstrap/recovery |

## Critical Version Rule

ECS 3.6 removed CPU、memory、NIC、transaction and disk performance fields from:

- `/dashboard/zones/localzone`
- `/dashboard/zones/localzone/nodes`
- `/dashboard/nodes/{id}`
- `/dashboard/storagepools/{id}/nodes`

The REST API reference still contains broad Dashboard schemas/examples. The adapter must follow
the newer Administration/Monitoring guidance and source these values from Flux. Fixtures
therefore intentionally omit removed Dashboard performance fields.

## 2026-08-01 Contract Reconciliation

Dell's published Flux pages explicitly identify `POST /flux/api/external/v2/query`,
`monitoring_op`, CPU `usage_idle` with `cpu-total`, `keep()`, Memory and Node identity tags.
These items agree with the split latest-snapshot query contract implemented by ECS-009.
The four-profile fixture replay passed for `ecs-3.6`. ECS-011 additionally treats the live
3.8.1.1 split Flux result as shared feature evidence, so CPU/Memory/Network latest snapshot and
supported VDC/Namespace Performance are `validated-shared` for ECS 3.6. Exact 3.6 appliance
execution remains useful regression evidence but is not required for feature status.
See [the probe procedure](flux-probe.md) and
[fixture replay record](validation/fixture-replay-2026-08-01.md).

## Remaining Version-Specific Assurance

These items do not change the shared feature validation state:

- Test exact patch/builds actually deployed, including the oldest and newest supported patch.
- Confirm decimal unit multipliers against a known object/quota/capacity.
- Confirm valid node health enums and RG link status enums.
- Confirm batch billing endpoint response and pagination on a large namespace.
- Confirm `net` interface names and configure an allowlist; never combine bond and member
  interfaces without evidence.
