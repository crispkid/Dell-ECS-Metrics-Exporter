#!/usr/bin/env bash

set -euo pipefail

required_files=(
  ".syft.yaml"
  ".github/workflows/release.yml"
  "CHANGELOG.md"
  "CONTRIBUTING.md"
  "deploy/bare-metal/dell-ecs-metrics-exporter.service"
  "deploy/bare-metal/install.sh"
  "deploy/prometheus/alerts.yaml"
  "docs/PRODUCTION_RUNBOOK.md"
  "docs/RC_RELEASE_CHECKLIST.md"
  "docs/RELEASE_CHECKLIST.md"
  "scripts/release-build.sh"
  "scripts/release-kind.sh"
  "scripts/supply-chain-check.sh"
)
for path in "${required_files[@]}"; do
  if [[ ! -f "$path" ]]; then
    printf 'error: production release asset is missing: %s\n' "$path" >&2
    exit 1
  fi
done

if ! rg -q '^FROM [^[:space:]]+@sha256:[0-9a-f]{64} AS build$' Dockerfile; then
  printf 'error: Docker build image must be pinned by digest\n' >&2
  exit 1
fi
if ! rg -q 'cosign sign --yes' .github/workflows/release.yml ||
  ! rg -Fq '"$HELM_REFERENCE@$HELM_DIGEST"' .github/workflows/release.yml ||
  ! rg -q 'actions/attest@' .github/workflows/release.yml ||
  ! rg -q 'sbom-action@' .github/workflows/release.yml; then
  printf 'error: release workflow is missing signing, attestation, or SBOM policy\n' >&2
  exit 1
fi
if [[ "$(./scripts/release-kind.sh v1.0.0)" != "stable" ]] ||
  [[ "$(./scripts/release-kind.sh v1.0.0-rc.1)" != "prerelease" ]] ||
  ! rg -Fq "if: needs.validate.outputs.release_kind == 'stable'" \
    .github/workflows/release.yml ||
  ! rg -Fq 'needs.validate.outputs.release_kind == '\''prerelease'\''' \
    .github/workflows/release.yml ||
  ! rg -Fq 'release_flags+=(--prerelease)' .github/workflows/release.yml ||
  [[ "$(rg -c "if: needs.validate.outputs.release_kind == 'stable' \\|\\| github.event.repository.private == false" .github/workflows/release.yml)" -ne 6 ]] ||
  ! rg -Fq 'github-attestation-boundary.txt' .github/workflows/release.yml; then
  printf 'error: release workflow does not preserve stable gates and GitHub prerelease marking\n' >&2
  exit 1
fi
if ! rg -q 'ECS_CERT_EXPECTED_VERSION: "3\.8\.1\.4"' .github/workflows/release.yml ||
  ! rg -q 'ECS_CERT_EXPECTED_VERSION: "3\.8\.0\.3"' .github/workflows/release.yml ||
  ! rg -q 'ECS_CERT_ALLOW_DEGRADED: "true"' .github/workflows/release.yml; then
  printf 'error: release workflow is missing exact production/CE compatibility gates\n' >&2
  exit 1
fi
if [[ "$(rg -c 'grype-version: v0\.112\.0' .github/workflows/release.yml)" -ne 3 ]] ||
  [[ "$(rg -c 'syft-version: v1\.44\.0' .github/workflows/release.yml)" -ne 3 ]] ||
  ! rg -q 'osv-scanner@v2\.3\.8' .github/workflows/release.yml ||
  ! rg -q 'require_version syft 1\.44\.0' scripts/supply-chain-check.sh ||
  ! rg -q 'require_version grype 0\.112\.0' scripts/supply-chain-check.sh ||
  ! rg -q 'require_version osv-scanner 2\.3\.8' scripts/supply-chain-check.sh; then
  printf 'error: release scanners must use the reviewed pinned versions\n' >&2
  exit 1
fi
if rg -n 'uses:[[:space:]]+[^[:space:]]+@(main|master|v[0-9]+([.][0-9]+)*)' \
  .github/workflows; then
  printf 'error: release actions must be pinned to full commit SHAs\n' >&2
  exit 1
fi
release_tag=""
if [[ "${GITHUB_REF_TYPE:-}" == "tag" ]]; then
  release_tag="${GITHUB_REF_NAME:-}"
  if [[ -z "$release_tag" ]]; then
    printf 'blocked: GITHUB_REF_NAME is required for a tag release\n' >&2
    exit 3
  fi
elif [[ "${RELEASE_REQUIRE_TAG:-false}" == "true" ]]; then
  release_tag="${RELEASE_VERSION:-}"
fi
if [[ -n "$release_tag" ]]; then
  if [[ ! "$release_tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
    printf 'blocked: release tag must be a v-prefixed Semantic Version\n' >&2
    exit 3
  fi
  tag_commit="$(git rev-list -n 1 "refs/tags/$release_tag" 2>/dev/null || true)"
  if [[ ! "$tag_commit" =~ ^[0-9a-f]{40}$ ]]; then
    printf 'blocked: release tag does not exist locally: %s\n' "$release_tag" >&2
    exit 3
  fi
  if [[ "$tag_commit" != "$(git rev-parse HEAD)" ]]; then
    printf 'blocked: release tag %s does not point to checked-out HEAD\n' "$release_tag" >&2
    exit 3
  fi
fi
if [[ "$(git ls-files | wc -l | tr -d ' ')" -le 2 ]]; then
  printf 'blocked: implementation is not committed to Git\n' >&2
  exit 3
fi
if [[ -n "$(git status --porcelain=v1 --untracked-files=all)" ]]; then
  printf 'blocked: production release requires a clean Git worktree\n' >&2
  exit 3
fi
if [[ -z "$(git remote)" ]]; then
  printf 'blocked: no Git remote is configured\n' >&2
  exit 3
fi
if ! command -v jq >/dev/null 2>&1; then
  printf 'blocked: jq is required to validate shared Profile feature evidence\n' >&2
  exit 3
fi
if ! jq -se '
  [
    "authentication",
    "version_discovery",
    "cluster_health",
    "cluster_capacity",
    "node_inventory",
    "node_health",
    "node_cpu_memory",
    "node_network_counters",
    "namespace_inventory",
    "namespace_capacity",
    "namespace_quota",
    "namespace_performance",
    "bucket_inventory",
    "bucket_capacity",
    "bucket_quota",
    "vdc_performance",
    "flux_latest_snapshot"
  ] as $required |
  all(.[];
    .evidence.shared_validated_capabilities as $validated |
    .evidence.feature_validation_policy == "shared-live-any-target-version" and
    all($required[]; . as $capability | ($validated | index($capability)) != null)
  )
' \
  profiles/ecs-3.6.json \
  profiles/ecs-3.7.json \
  profiles/ecs-3.8.0.json \
  profiles/ecs-3.8.1.json >/dev/null; then
  printf 'blocked: required cross-version shared feature validation evidence is incomplete\n' >&2
  exit 3
fi

printf 'release policy checks passed\n'
