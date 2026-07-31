# ECS CE 3.8.1.4 Live Validation Record

- Executed: `2026-07-26`
- Environment: isolated, non-production ECS Community Edition OVA on ESXi
- Upstream release:
  [ECS Community Edition Install Node 2.9.1 / ECS 3.8.1.4](https://github.com/EMCECS/ECS-CommunityEdition/releases/tag/v2.9.1-3.8.1.4)
- Product version returned by `GET /vdc/nodes`:
  `3.8.1.4.140200.8103892f11b`
- Selected Exporter Profile: `ecs-3.8.1`
- Result: partial live Management API and Exporter evidence; Profile certification remains
  pending
- ECS-011 disposition: the successfully exercised functions listed in
  `../feature-validation.md` are `validated-shared` across all four target Profiles

This record is deliberately redacted. It contains no endpoint, credential, token, cookie,
Authorization header, private address, node identifier or unredacted raw response body.

## Image and Environment Boundary

The OVA was downloaded from the release asset linked above. Its exact size was
`4,942,754,816` bytes and its SHA-512 matched the release checksum:

```text
2fde0d7c59357e874dc5cef27a3dbc67024b3db4e042da3cbd49e8724e313b401e9a0eabfb3af1bdb67e0679c2d712ef9a5ee79036841948c15b3b5fb86e1cf2
```

The OVA manifest and both VMDK chains were independently verified before the VM was started.
The test VM used 8 vCPU, 24 GiB RAM, a 100 GiB system disk and a thin-provisioned 1 TiB ECS
data disk. The environment contained one ECS node, one storage pool, one VDC, one
single-VDC replication group, one test Namespace, three Buckets and four dummy objects.
Exporter authentication used a dedicated `SYSTEM_MONITOR` management account.

ECS CE is a reduced-footprint trial/PoC edition. It is not evidence for appliance fabric,
multi-node failure handling or production performance. The CE certificate was self-signed,
so this isolated run used `tls.verify: false`. ECS-009 now permits this explicit setting in
every environment and emits a startup warning; this CE run did not establish verified TLS
identity.

## Installer Result

`ova-step1` completed with exit code `0`; every Ansible recap had `failed=0`. The data disk
was partitioned, formatted, mounted and assigned to the StorageOS container.

During `ova-step2`, the first Data Store request reached the newly listening API before its
internal Directory Table handler had initialized. The API returned HTTP 500 and the StorageOS
log recorded `directory handler not initialized yet`. After the API reported all expected
Directory Tables ready, the same Data Store request succeeded. The resulting Data Store was
`readytouse`, the storage pool status was `1`, and the final diagnostic state was:

```text
dt_total=416
dt_unready=0
dt_unknown=0
```

VDC, replication group, management user, Namespace, object user and Bucket creation then
completed successfully. This was a first-start readiness race in the CE installer flow, not
an Exporter compatibility failure.

## Live API and Exporter Results

| Contract | Live result | Evidence boundary |
|---|---|---|
| Login/token/whoami/logout | HTTP 200 with the dedicated monitor account; graceful Exporter shutdown logged logout HTTP 200 | Tokens and headers were not retained |
| Version discovery | HTTP 200; exact build above selected `ecs-3.8.1`; `mixedVersion=false` | Single-node only |
| Host/transport | Direct HTTPS Management connection by configured endpoint succeeded | Proxy/LB and upgraded accepted-server-name persistence remain untested |
| Cluster health/capacity | HTTP 200; health `1`; capacity fields parsed successfully | One-node CE health and capacity only |
| Node inventory/health | HTTP 200; one healthy Node | Flux-backed Node resources unavailable, as described below |
| Namespace inventory/quota/billing | HTTP 200; top-level unset quota sentinels parsed; billing returned `10240 KB` and four objects | Known-size KB conversion proven |
| Bucket inventory/quota/billing | HTTP 200; three Bucket inventory items; configured and unset top-level quota shapes parsed; plural batch billing parsed | Corrected batch path and `{"id":[...]}` request proven |
| Inventory API | Liveness and readiness HTTP 200; unauthenticated Inventory HTTP 401; authenticated collections returned valid envelopes | Local token authentication only |
| Prometheus | 29 metric families passed `metricscheck`; configured quota and exact known usage values were present; unset quota metrics were omitted | No external Prometheus server was deployed |
| Replication | Single-VDC RG dashboard response parsed with pending bytes `0`, no link status and no RPO | Multi-VDC link status, lag and recovery remain untested |

The sanitized Exporter summary was:

```text
liveness_http=200
readiness_http=200 status=DEGRADED reason=collector_error
unauthenticated_inventory_http=401
clusters=1 nodes=1 namespaces=1 buckets=3 replications=0
selected_profile=ecs-3.8.1 mixed_version=false
prometheus_metric_families=29
logout_http=200
```

`replications=0` reflects the validation configuration: the single-VDC RG has no link status
and was probed separately rather than configured as a production replication target.

## Known-Size Data Assertions

| Scenario | Quota | Objects | Exact bytes | ECS billing |
|---|---:|---:|---:|---:|
| Configured quota | soft 1 GB; hard 2 GB | 3 | 7,340,032 | `7168 KB` |
| Quota unset | unset | 1 | 3,145,728 | `3072 KB` |
| Empty | unset | 0 | 0 | `0 KB` |
| Namespace total | unset | 4 | 10,485,760 | `10240 KB` |

The Exporter returned the same object counts and exact byte values in both Inventory and
Prometheus output. It emitted configured quota as decimal API GB:
`1,000,000,000` and `2,000,000,000` bytes. Unset Namespace/Bucket quotas remained JSON
`null` with configured flags `false`, and no quota metric was fabricated.

## Flux Limitation

`POST /flux/api/external/v2/query` returned HTTP 503 on every retry for the native
`node-resources` collector. Management-backed Node inventory and health stayed available,
while readiness correctly reported HTTP 200 `DEGRADED` / `collector_error`.

Because Node resources and conditional performance use the same unavailable CE Flux endpoint,
this run does not validate:

- Node CPU, memory, disk or network metrics;
- VDC or Namespace performance;
- the ECS 3.8.1 Flux range-boundary re-enablement gate;
- interval-derived rates, counter reset handling or freshness behavior.

The 503 is evidence about this ECS CE deployment, not proof that a production ECS 3.8.1.4
appliance lacks Flux.

## Evidence and Certification Decision

The redacted fixtures in `testdata/ecs/ecs-3.8.1.4-live/` preserve only the exact version,
top-level quota shapes, known-size billing values and single-VDC RG shape required for
regression tests. Raw responses and credentials remained in mode-`0600` temporary storage
outside the repository.

This partial CE run does not populate Profile `tested_builds` and does not set
`sandbox_certified=true`. Remaining certification gates include:

- the authenticated Dell ECS 3.8.1 REST API ZIP comparison;
- Flux latest/range-boundary validation on an appliance exposing Flux;
- production ECS 3.8.1.4 protected read-only rerun from one immutable release candidate;
- proxy/load-balancer Host/SNI behavior, token renewal and representative failure injection;
- multi-VDC replication/link/recovery behavior;
- target-scale, duration and response-size measurements;
- independent Project Maintainer and Security Reviewer approval.
