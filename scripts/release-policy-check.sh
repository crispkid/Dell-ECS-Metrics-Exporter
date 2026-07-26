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
  "docs/RELEASE_CHECKLIST.md"
  "scripts/release-build.sh"
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
  printf 'blocked: jq is required to validate Profile certification evidence\n' >&2
  exit 3
fi
if ! jq -e '
  (.version.tested_builds | index("3.8.1.4")) != null and
  .evidence.sandbox_certified == true and
  .evidence.status == "sandbox-verified" and
  .evidence.api_reference_access == "downloaded-and-reviewed" and
  .evidence.fixture_classification == "redacted-sandbox-derived"
' profiles/ecs-3.8.1.json >/dev/null; then
  printf 'blocked: ECS 3.8.1.4 Profile lacks reviewed formal certification evidence\n' >&2
  exit 3
fi

printf 'release policy checks passed\n'
