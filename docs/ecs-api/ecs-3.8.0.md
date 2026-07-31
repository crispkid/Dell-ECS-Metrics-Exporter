# ECS 3.8.0.x Profile Mapping

- Profile: `profiles/ecs-3.8.0.json`
- Range: `>=3.8.0.0, <3.8.1.0`
- Base contract: `common-contract.md`
- Mapping evidence: official ECS 3.8 index and Dell Host Header/Flux known-issue articles
- REST API ZIP review: pending authenticated Dell Support access
- Sandbox certification: partial ECS CE 3.8.0.3 Management API run; not certified
- Live record: `validation/ecs-ce-3.8.0.3-2026-07-25.md`
- Shared feature validation: `validated-shared` under `feature-validation.md`

## Official Sources

- [ECS 3.8 Product Documentation Index](https://www.dell.com/support/kbdoc/en-us/000205234/ecs-3-8-product-documentation-index-info-hub)
- [Dell KB 000205031 - ECS 3.8 Host Header](https://www.dell.com/support/kbdoc/en-us/000205031/ecs-how-to-perform-host-header-injection-for-3-8-x)
- [Dell KB 000211906 - Flux values in ECS 3.7/3.8](https://www.dell.com/support/kbdoc/en-us/000211906/ecs-different-values-from-flux-api-since-upgrading-to-3-7-3-8)

## Coverage

| Mapping family | Status | ECS 3.8.0 decision |
|---|---|---|
| Authentication/version discovery | validated-shared | Login/whoami/logout and exact build selection passed with `SYSTEM_MONITOR` |
| Cluster/node inventory and health | validated-shared | Top-level capacity and HAL Node health passed; Flux resource evidence inherited from appliance 3.8.1.1 |
| Namespace inventory/quota/billing | validated-shared | Top-level unset quota and non-zero `9765.625 KB`/four-object billing were observed |
| Bucket inventory/quota/billing | validated-shared | Corrected Exporter returned three Bucket Inventory items and matching quota/billing metrics；batch HTTP 200 and collector succeeded repeatedly |
| Flux latest snapshot | validated-shared | Use `last()` only; appliance evidence is shared across target versions |
| Flux interval rates/deltas | unsupported for this Profile | Same known range defect as ECS 3.7 |
| VDC/namespace performance | validated-shared, conditional | Query/parser feature is validated; interval-derived rates stay unavailable |
| Bucket performance | unsupported | No bucket-scope evidence |
| Replication/recovery | pending | No target version has live status/lag/recovery field evidence |

## Host Header Contract

When Management API traffic passes through a proxy or load balancer:

- configured URL hostname, TLS SNI and HTTP `Host` must remain consistent;
- ECS accepted server names must contain that hostname;
- HTTP 403 is non-retryable and should report logical error
  `host_not_accepted` without echoing the private URL;
- the adapter must not work around the check by replacing the configured hostname with an IP.

This is a deployment prerequisite rather than a schema fallback.

## Flux Workaround

Use the same `last()`-only policy as ECS 3.7. Monotonic source counters remain usable, but
Exporter-derived range rates and interval deltas are disabled.

The ECS CE 3.8.0.3 run returned HTTP 503/code 6503 for the Flux query and contained no
separate Flux/Influx backend. Exporter therefore reports the independent `node-resources`
collector as failed and readiness as `DEGRADED`, while retaining Management Node
inventory/health. This CE observation is not production Flux evidence.

## Partial Live Evidence

The redacted `validation/ecs-ce-3.8.0.3-2026-07-25.md` record covers exact version discovery,
least-privilege authentication, Host acceptance/rejection, Cluster/Node Management,
Namespace, namespace-scoped empty and non-empty Bucket scenarios, response envelope
differences, a known-size billing unit assertion, Exporter Inventory and Prometheus output.

It does not populate Profile `tested_builds` or change `sandbox_certified`.

The non-empty follow-up used three Buckets. Two Buckets contained 7,000,000 and
3,000,000 bytes respectively and one remained empty. The Namespace billing total was
`9765.625 KB` for four objects, proving billing `KB = 1024 bytes`. Bucket quota and single
billing fields were top-level. The Namespace batch billing endpoint returned HTTP 500 with
ECS code 999 for JSON `null`, while `{}` returned HTTP 200 with plural
`bucket_billing_infos: []` and single-bucket billing succeeded. No JSON entity returned 415
and `[]` returned 400/code1013. These observations prove that code999 is a request-body
validation result, not an unsupported-endpoint signal. The built-in class and non-empty probe
confirmed `{"id":[bucket names...]}` and returned three plural `bucket_billing_infos` items.
Fallback is limited to 404/405/501 or a missing requested item；500/code999 does not fallback.
The corrected Exporter live rerun passed with three Buckets, four objects and
10,000,000 used bytes. Health remained HTTP 200 `DEGRADED` only because the known CE Flux
node-resources query returned 503. This is still partial exact-build evidence, not Profile
certification.

## Remaining Version-Specific Assurance

These items do not change the shared feature validation state:

- Compare ECS 3.8.0 REST API ZIP with the common contract.
- Test direct, proxy and load-balanced Management API calls.
- Test proxy/load-balanced accepted and rejected Host values without logging the endpoint/token.
- Confirm Flux window defect on the exact build and verify all interval-derived metrics are absent.
- Verify capacity/quota GB and billing MB/GB/TB multipliers separately; the live run proves
  only billing KB.
- Test replication/recovery and representative failure/timeout/token-expiry scenarios.

## 2026-08-01 Validation Position

The official ECS 3.8 index exposes separate 3.8.0 Administration, Monitoring and REST API
documents, and Dell KB 000211906 keeps interval-derived Flux rates disabled for this Profile.
The shared split mapping passed the `ecs-3.8.0` fixture replay. ECS CE 3.8.0.3 still returned
Flux HTTP 503, but ECS-011 inherits the successful appliance 3.8.1.1 latest-snapshot feature
evidence. A [redacted appliance probe](flux-probe.md) on an exact 3.8.0 build is optional
version-specific regression evidence; the documented interval-rate prohibition remains binding.
