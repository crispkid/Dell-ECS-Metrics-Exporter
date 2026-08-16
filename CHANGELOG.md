# Changelog

All notable public changes are documented here. The project follows Semantic
Versioning. Exact release dates and links are added when a release is actually
published; this file does not imply that an unreleased build is production
approved.

## Unreleased

## [1.0.0] - 2026-08-16

### Added

- First stable Dell ECS Metrics Exporter release, approved from the
  user-confirmed physical ECS 3.8.0.x and 3.8.1.x compatibility tests plus the
  automated deterministic, race, container, deployment, synthetic scale,
  vulnerability, license, image scan, signing and release artifact gates.
- A transparent machine-readable stable validation record that preserves the
  limits of the user-attested appliance testing without inventing exact builds
  or replayable reports.
- A signed private-repository attestation boundary asset when GitHub-native
  artifact attestations are unavailable; checksums, SBOMs, BuildKit OCI
  provenance and keyless OCI signatures remain mandatory.

### Changed

- Record the user-confirmed physical ECS 3.8.0.x and 3.8.1.x
  compatibility test status. Exact four-part builds and redacted reports remain
  required before these runs can be represented as machine-verifiable
  `tested_builds` or formal Profile certification.
- Update the pinned build toolchain and Docker builder to Go 1.26.6, which
  contains the standard-library fixes required by the release vulnerability
  gate.

## [1.0.0-rc.1] - 2026-08-01

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
- Explicit prerelease workflow policy: RC tags retain deterministic, race,
  container, Kubernetes schema, synthetic scale, dependency/license, image
  scan, SBOM, signing and OCI provenance gates. Private RCs record a signed
  GitHub-attestation boundary when the repository plan lacks native support;
  stable tags additionally require native attestation, protected exact-build
  ECS, deployed E2E and deployed performance certification.

### Fixed

- Explicitly install the Harness-required `ripgrep` CLI in GitHub-hosted CI and
  release validation jobs so runner image changes cannot stop verification
  before tests run.
- Install the official kubeconform v0.7.0 release binary with a pinned SHA-256
  digest; `go install` builds report `development` and therefore cannot satisfy
  the fail-closed deployment validator version gate.
- Add a package-specific OSV license override for Go's synthetic `stdlib`
  package. This preserves the strict license allowlist and vulnerability scan
  while recording its actual BSD-3-Clause license instead of accepting unknown
  licenses globally.
- Capture Helm 3.18 OCI push output from stderr before validating and exporting
  the immutable Chart digest; the digest format check remains fail-closed.

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

### RC limitations

- This release is for evaluation and integration testing, not stable
  production approval.
- Exact physical ECS 3.8.1.4, deployed target-scale, tagged deployed E2E and
  named maintainer/security reviewer evidence remain required for v1.0.0.
- Node service/process, Disk, Flux interval-derived rates, replication
  status/lag and recovery progress do not yet have qualifying live evidence on
  any target version; Bucket Performance is unavailable by design.

### Fixed

- Portable Harness self-test now follows annotated source files and documents
  intentional cross-file globals/deferred command strings, so ShellCheck-enabled
  GitHub runners and local behavior agree.
