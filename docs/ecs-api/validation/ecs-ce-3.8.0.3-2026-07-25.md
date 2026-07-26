# ECS CE 3.8.0.3 Live Validation Record

- Changes: `ECS-005` initial smoke；`ECS-007` non-empty Bucket follow-up
- Executed at: `2026-07-25T13:58:44Z`–`2026-07-25T14:10:16Z`
- Non-empty Bucket follow-up: `2026-07-26`
- Environment: isolated, non-production ECS Community Edition OVA on ESXi
- Product version returned by `GET /vdc/nodes`:
  `3.8.0.3.138685.3a0a9b6bf3a`
- Selected Exporter Profile: `ecs-3.8.0`
- Result: partial live Management API evidence; Profile certification remains pending

This record is deliberately redacted. It contains no endpoint, credential, token, cookie,
Authorization header, node identifier, private address or raw response body.

## Environment Boundary

The environment used one ECS CE node, one storage pool, one VDC, one replication group and a
cleanable test Namespace. The ECS-005 initial smoke started without Bucket or object data.
The later ECS-007 follow-up created three isolated test Buckets and four dummy objects; only
aggregate, non-identifying values are retained below. Exporter authentication used a dedicated
`SYSTEM_MONITOR` management account.

ECS CE is a reduced-footprint trial/PoC edition. Dell's public ECS CE README states that it
does not include the production fabric layer and is not suitable for failure-scenario testing:
<https://github.com/EMCECS/ECS-CommunityEdition#feature-differences>.

The CE certificate was self-signed with `CN=localhost`; the isolated test configuration
therefore used `tls.verify: false`. This does not change the production requirement to use
certificate verification and a trusted CA.

## Live Results

| Contract | Live result | Evidence boundary |
|---|---|---|
| Login/token/whoami/logout | HTTP 200; role was exactly `SYSTEM_MONITOR`; logout HTTP 200 after every run | Token and headers were never retained in repository evidence |
| Version discovery | HTTP 200; exact build above; uniform single-node selection chose `ecs-3.8.0` | Mixed-version behavior remains component-tested only |
| Host Header | Accepted IP and configured node name reached authentication; an unaccepted hostname returned HTTP 403 | Direct connection only; proxy/LB persistence remains untested |
| Cluster health | HTTP 200; string-encoded node/disk counts parsed; resulting cluster health was `1` | One-node CE health only |
| Cluster capacity | HTTP 200; ECS 3.8.0.3 returned `totalProvisioned_gb` and `totalFree_gb` at the top level | Known-size object/unit assertion remains untested |
| Node inventory/health | HTTP 200; health records were under `_embedded._instances`; node IDs matched `/vdc/nodes` | CPU/memory/network Flux data unavailable in CE run |
| Namespace inventory | HTTP 200; one cleanable test Namespace | No production data |
| Namespace quota | HTTP 200; quota fields were top-level and both unset sentinels were `-1` | Configured quota size remains untested |
| Namespace billing | Initial empty Namespace reported `0 KB`; follow-up reported `9765.625 KB` and four objects for a known 10,000,000-byte total | Corrected Exporter returned 10,000,000 bytes/four objects/three Buckets |
| Bucket inventory | Global request returned HTTP 400 because `namespace` is required；namespace-scoped requests returned an empty list initially and three items after follow-up setup | Corrected Exporter Inventory returned all three items |
| Bucket quota | Configured Bucket returned top-level `blockSize=2` and `notificationSize=1`；two other Buckets had quota unset | Values are API GB fields；their byte multiplier was not independently proven |
| Bucket billing | Single responses were top-level；Bucket totals were `6835.9375 KB` for three objects, `2929.6875 KB` for one object and `0 KB` for the empty Bucket | Billing KB multiplier proven；other billing units remain untested |
| Batch Bucket billing | `{"id":[three Bucket names]}` -> HTTP 200 with three plural `bucket_billing_infos` items；sizes `0`/`2929.6875`/`6835.9375 KB` and objects `0`/`1`/`3` | Corrected Exporter batch HTTP 200 and Bucket collector succeeded at least twice；500/code999 does not fallback |
| Inventory API | Unauthenticated request returned 401; authenticated Cluster, Node, Namespace, Bucket and Replication collections returned valid envelopes | Local Exporter API only; reverse proxy auth remains untested |
| Prometheus | Exposition parsed as text and included required self/domain families; unsupported Bucket performance families were absent | No external Prometheus server was deployed |

## Flux and Readiness Limitation

`POST /flux/api/external/v2/query` returned HTTP 503 with ECS code `6503` for every retry and
polling interval. The CE VM had no separate Flux/Influx container or executable; only the
StorageOS image and an Influx client library were present. This is evidence that Flux was
unavailable in this CE environment, not proof that the query contract is invalid on a
production ECS appliance.

ECS-005 therefore split Node Management and Node Resources collection. Node inventory and
health initialize independently, while the Flux failure remains observable as:

- `node-resources` collector error;
- `ecs_exporter_api_*` HTTP 503 telemetry;
- HTTP 200 readiness with `DEGRADED` / `collector_error`;
- omitted CPU, memory and network samples instead of fabricated values.

The final sanitized smoke run reported:

```text
liveness=UP
readiness_http=200 status=DEGRADED reason=collector_error
inventory_without_token_http=401
clusters_count=1
nodes_count=1
namespaces_count=1
buckets_count=0
replications_count=0
selected_profile=ecs-3.8.0 mixed_version=false
prometheus_exposition=valid
unsupported_bucket_performance=absent
```

## Runtime Corrections Proven by This Run

- Accept legacy nested and ECS 3.8.0 top-level cluster capacity envelopes.
- Accept legacy nested and ECS 3.8.0 top-level Namespace quota envelopes.
- Accept Node health from `node[]`, `nodes[]` or `_embedded._instances[]`.
- Enumerate Namespaces before issuing namespace-scoped Bucket list requests.
- Preserve Node inventory/health when the independent Flux resource source fails.
- Reject ambiguous envelopes and cross-Namespace Bucket responses.

## ECS-007 Non-empty Bucket Follow-up

The follow-up deliberately covered three quota and usage shapes:

| Scenario | Quota | Objects and known bytes | ECS billing sample |
|---|---:|---:|---:|
| Configured quota | soft 1 GB；hard 2 GB | three objects；7,000,000 bytes total | `6835.9375 KB` |
| Quota unset | unset | one object；3,000,000 bytes | `2929.6875 KB` |
| Empty | unset | zero objects；0 bytes | `0 KB` |
| Namespace total | n/a | four objects；10,000,000 bytes | `9765.625 KB` |

The known-size equation is exact:

```text
6835.9375 + 2929.6875 + 0 = 9765.625 KB
9765.625 * 1024 = 10,000,000 bytes
```

This proves the multiplier only for billing responses whose unit is `KB`. It does not prove
that billing MB/GB/TB or capacity/quota GB use binary multipliers, so those existing decimal
mappings remain unchanged.

The pre-fix Exporter did not complete the Bucket refresh:

- Bucket quota parsing required the inherited `bucket_quota_details` wrapper.
- Single-bucket billing parsing required the inherited `bucket_billing_info` wrapper.
- Batch POST sent no JSON entity and received HTTP 415.
- Billing KB was multiplied by 1000, producing `9,765,625` instead of
  `10,000,000` bytes.
- Readiness was HTTP 503 `DOWN` / `cache_not_initialized`; Bucket inventory and Bucket metrics
  remained unavailable even though the ECS data existed.

Manual batch probes produced this request/response matrix:

| JSON body | Result |
|---|---|
| no entity | HTTP 415 |
| `{}` | HTTP 200；`bucket_billing_infos: []` |
| `[]` | HTTP 400；ECS code 1013 |
| `null` | HTTP 500；ECS code 999 |
| `{"id":[three redacted Bucket names]}` | HTTP 200；three `bucket_billing_infos` items with sizes `0`/`2929.6875`/`6835.9375 KB` and objects `0`/`1`/`3` |

The built-in ECS CE `BucketListParam.class` and non-empty probe confirm that the request body is
`{"id":[bucket names...]}`. The matrix also proves that 500/code999 results from `null`; it is
not evidence that the endpoint is unsupported and does not trigger fallback. ECS-007 therefore
requires both nested and top-level quota/single-billing envelopes, ambiguity rejection,
plural batch response support, the confirmed `id` request array and `KB * 1024` billing
conversion. Fallback is limited to HTTP 404/405/501 or a missing requested item. The corrected
Exporter result is recorded below.

## ECS-007 Corrected Exporter Live Rerun

The corrected Exporter completed batch billing with HTTP 200 and the Bucket collector
succeeded in at least two consecutive refreshes. Authenticated Inventory returned HTTP 200
and all three redacted scenarios:

| Scenario | Used bytes | Objects | Soft quota bytes | Hard quota bytes |
|---|---:|---:|---:|---:|
| Empty | 0 | 0 | unset | unset |
| Configured quota | 7,000,000 | 3 | 1,000,000,000 | 2,000,000,000 |
| Quota unset | 3,000,000 | 1 | unset | unset |

The aggregate results were:

```text
cluster buckets=3 namespaces=1 objects=4
namespace capacity_used_bytes=10000000 objects=4 buckets=3
bucket total_used_bytes=10000000 total_objects=4
```

Health returned HTTP 200 `DEGRADED`. The only collector error was the already documented
ECS CE Flux `node-resources` HTTP 503; the Management-backed Bucket collector was successful.
Version output reported the supported `ecs-3.8.0` Profile, while
`sandboxCertifiedProfiles` remained empty.

This rerun proves the corrected Bucket path on this exact ECS CE build. It does not validate
production Flux, a formal appliance, the complete 3.8.0.x range or Profile certification.

## Certification Decision

This run does not set `tested_builds` and does not set `sandbox_certified=true`. Remaining
certification gates include:

- authenticated Dell 3.8.0 REST API package comparison;
- minimum and primary Profile build coverage;
- production-appliance Flux snapshot and known range-defect checks;
- token expiry, 401 renewal, 429, representative 4xx/5xx and timeout injection;
- proxy/load-balancer accepted-server-name behavior;
- known-size capacity/quota GB and billing MB/GB/TB assertions；billing KB is proven;
- configured replication/recovery responses and representative duration/size measurements;
- independent Project Maintainer and Security Reviewer approval.

## Local Verification

The ECS-005 initial live run was followed by its regression tests, the Go race detector, the
full Harness gate and a credential-pattern scan. ECS-007 corrected live rerun passed as
described above，and its race suite passed. Harness lint/format/typecheck/test/84.5% coverage/
build/CI policy and a separate deployment check passed；complete `verify` remains failed
because the execution policy did not authorize `govulncheck` to fetch the external
vulnerability database. Raw live responses were stored only in a local mode-`0600` temporary
directory and are excluded from Git.
