# Changelog

All notable public changes are documented here. The project follows Semantic
Versioning. Exact release dates and links are added when a release is actually
published; this file does not imply that an unreleased build is production
approved.

## Unreleased

### Added

- Multi-cluster Dell ECS exporter runtime, Prometheus metrics, authenticated
  read-only Inventory API, cache-based health, and four compatibility Profiles
  for ECS 3.6, 3.7, 3.8.0, and 3.8.1 families.
- Docker/OCI, Kubernetes/Helm, and hardened Linux Bare Metal/systemd deployment
  assets.
- Deterministic release archives, checksums, SBOMs, vulnerability/license
  policy, keyless Sigstore signing, GitHub attestations, and signed OCI image
  and Helm chart delivery.
- Fail-closed exact-build live, deployed E2E, container, Kubernetes schema, and
  target-scale performance gates.
- Production Runbook, release checklist, Prometheus alert rules, security
  policy, and contribution guidance.

### Compatibility notes

- ECS CE 3.8.0.3 has partial-live Management API evidence, including corrected
  Bucket quota/billing envelopes and billing KB conversion.
- ECS CE 3.8.1.4 exact build `3.8.1.4.140200.8103892f11b` now has partial-live
  Profile selection, Management, known-size quota/billing, Inventory/metrics
  and single-VDC RG evidence with redacted regression fixtures.
- ECS CE Flux node-resources HTTP 503 was observed on both tested CE builds and
  remains an environment limitation; it is not generalized to physical ECS.
- Formal ECS 3.8.1.4 appliance certification and all Profile
  `tested_builds` remain pending external evidence and reviewer approval.
