# ECS 3.6.x Profile Mapping

- Profile: `profiles/ecs-3.6.json`
- Range: `>=3.6.0.0, <3.7.0.0`
- Mapping evidence: Dell ECS REST API `3.6.0.0.126369.160d4eb` and ECS 3.6.1
  Monitoring Guide
- Fixture class: synthetic, document-derived
- Sandbox certification: not run

## Official Sources

- [ECS 3.6 REST API Reference ZIP](https://dl.dell.com/content/docu101271_ecs-3-6-rest-api-reference.zip)
- [ECS 3.6.1 Monitoring Guide](https://dl.dell.com/content/docu102929_ecs-3-6-1-monitoring-guide.pdf)
- [ECS 3.6 Dashboard API changes](https://www.dell.com/support/manuals/en-us/ecs-appliance-/ecs_p_adminguide_3_6_1/dashboard-apis?guid=guid-6dea9a0d-66b1-4f26-8aaf-f3b9a3c335da&lang=en-us)

## Coverage

| Mapping | Status | ECS 3.6 decision |
|---|---|---|
| `MAP-AUTH-001` | documented | Token flow on port 4443 |
| `MAP-VERSION-001` | documented | `/vdc/nodes` exposes per-node version |
| `MAP-CLUSTER-HEALTH-001` | documented/derived | Good/bad node and disk counts remain in Dashboard |
| `MAP-CLUSTER-CAPACITY-001` | documented | Provisioned/free GB; used derived |
| `MAP-NODE-INFO-001` | documented | Node identity/version; lockdown status is not health |
| `MAP-NODE-HEALTH-001` | documented | Dashboard health only; removed performance fields ignored |
| `MAP-NODE-RESOURCE-001` | documented | Flux `cpu`, `mem`, `net`; network counters preserve interface |
| `MAP-NODE-DISK-001` | conditional | Requires filesystem/device allowlist |
| Namespace/Bucket inventory/quota/billing | documented | `-1` quota means unset; billing is sampled |
| VDC/Namespace performance | documented | Flux only; rate vs delta semantics preserved |
| Bucket performance | unsupported | No bucket-scope operation/latency mapping |
| Replication | documented/derived | Dashboard RG/RG link; lag from RPO timestamp |
| Recovery | conditional | Operation kind must distinguish failover/bootstrap/recovery |

## Critical Version Rule

ECS 3.6 removed CPU、memory、NIC、transaction and disk performance fields from:

- `/dashboard/zones/localzone`
- `/dashboard/zones/localzone/nodes`
- `/dashboard/nodes/{id}`
- `/dashboard/storagepools/{id}/nodes`

The REST API reference still contains broad Dashboard schemas/examples. The adapter must follow
the newer Administration/Monitoring guidance and source these values from Flux. Fixtures
therefore intentionally omit removed Dashboard performance fields.

## Certification Gaps

- Test exact patch/builds actually deployed, including the oldest and newest supported patch.
- Confirm decimal unit multipliers against a known object/quota/capacity.
- Confirm valid node health enums and RG link status enums.
- Confirm batch billing endpoint response and pagination on a large namespace.
- Confirm `net` interface names and configure an allowlist; never combine bond and member
  interfaces without evidence.
