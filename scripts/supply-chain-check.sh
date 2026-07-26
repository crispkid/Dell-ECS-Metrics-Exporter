#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

missing=()
for tool in syft grype osv-scanner; do
  command -v "$tool" >/dev/null 2>&1 || missing+=("$tool")
done
if ((${#missing[@]} > 0)); then
  printf 'blocked: supply-chain tools are missing: %s\n' "${missing[*]}" >&2
  exit 3
fi

require_version() {
  local tool="$1"
  local expected="$2"
  shift 2
  local output
  if ! output="$("$@" 2>&1)"; then
    printf 'blocked: could not determine %s version\n' "$tool" >&2
    exit 3
  fi
  if ! grep -Eq "(^|[^0-9])${expected//./\\.}([^0-9]|$)" <<<"$output"; then
    printf 'blocked: %s %s is required; detected: %s\n' \
      "$tool" "$expected" "$(printf '%s' "$output" | head -n 1)" >&2
    exit 3
  fi
}
require_version syft 1.44.0 syft version
require_version grype 0.112.0 grype version
require_version osv-scanner 2.3.8 osv-scanner --version

release_tag="${RELEASE_VERSION:-v0.0.0-dev}"
RELEASE_ALLOW_DIRTY="${RELEASE_ALLOW_DIRTY:-true}" \
  "$SCRIPT_DIR/release-build.sh" "$release_tag"

evidence_dir="$PROJECT_ROOT/test-results/supply-chain"
mkdir -p "$evidence_dir"
syft dir:"$PROJECT_ROOT" \
  -o "spdx-json=$evidence_dir/source.spdx.json"
syft dir:"$PROJECT_ROOT/dist/release" \
  -o "cyclonedx-json=$PROJECT_ROOT/dist/release/artifacts.cdx.json"
grype "sbom:$evidence_dir/source.spdx.json" \
  --fail-on high \
  -o json >"$evidence_dir/grype.json"
osv-scanner \
  --licenses="Apache-2.0,MIT,BSD-2-Clause,BSD-3-Clause,ISC" \
  --format=json \
  --output-file="$evidence_dir/osv-license.json" \
  "$PROJECT_ROOT"

"$SCRIPT_DIR/generate-checksums.sh" "$PROJECT_ROOT/dist/release"
printf 'supply-chain artifact, SBOM, checksum, and vulnerability gates passed\n'
