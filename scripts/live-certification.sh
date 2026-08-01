#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"
# shellcheck source=go-env.sh
source "$SCRIPT_DIR/go-env.sh"

config="${ECS_CERT_CONFIG:-}"
token_file="${ECS_CERT_INVENTORY_TOKEN_FILE:-}"
expected_version="${ECS_CERT_EXPECTED_VERSION:-}"
base_url="${ECS_CERT_BASE_URL:-http://127.0.0.1:18080}"
timeout_seconds="${ECS_CERT_TIMEOUT_SECONDS:-300}"
allow_degraded="${ECS_CERT_ALLOW_DEGRADED:-false}"

if [[ -z "$config" || ! -r "$config" ]]; then
  printf 'blocked: ECS_CERT_CONFIG must name a readable protected config file\n' >&2
  exit 3
fi
if [[ -z "$token_file" || ! -r "$token_file" ]]; then
  printf 'blocked: ECS_CERT_INVENTORY_TOKEN_FILE must name a readable token file\n' >&2
  exit 3
fi
if [[ ! "$expected_version" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  printf 'blocked: ECS_CERT_EXPECTED_VERSION must be an exact four-part ECS version\n' >&2
  exit 3
fi
if [[ ! "$base_url" =~ ^https?://(127\.0\.0\.1|localhost|\[::1\]):[0-9]+$ ]]; then
  printf 'error: ECS_CERT_BASE_URL must be a loopback exporter URL with an explicit port\n' >&2
  exit 1
fi
if [[ ! "$timeout_seconds" =~ ^[1-9][0-9]*$ ]] || ((timeout_seconds > 1800)); then
  printf 'error: ECS_CERT_TIMEOUT_SECONDS must be between 1 and 1800\n' >&2
  exit 1
fi
if [[ "$allow_degraded" != "true" && "$allow_degraded" != "false" ]]; then
  printf 'error: ECS_CERT_ALLOW_DEGRADED must be true or false\n' >&2
  exit 1
fi
for tool in curl jq go; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    printf 'blocked: live certification requires %s\n' "$tool" >&2
    exit 3
  fi
done
if [[ -n "$(git status --porcelain=v1 --untracked-files=all)" ]]; then
  printf 'blocked: live certification requires a clean, fully committed worktree\n' >&2
  exit 3
fi
if ! command -v sha256sum >/dev/null 2>&1 &&
  ! command -v shasum >/dev/null 2>&1; then
  printf 'blocked: live certification requires sha256sum or shasum\n' >&2
  exit 3
fi

evidence_dir="$PROJECT_ROOT/test-results/certification"
mkdir -p "$evidence_dir"
temporary_dir="$(mktemp -d "${TMPDIR:-/tmp}/dell-ecs-certification.XXXXXX")"
exporter_pid=""
cleanup() {
  if [[ -n "$exporter_pid" ]] && kill -0 "$exporter_pid" 2>/dev/null; then
    kill -TERM "$exporter_pid" 2>/dev/null || true
    wait "$exporter_pid" 2>/dev/null || true
  fi
  rm -r -- "$temporary_dir"
}
trap cleanup EXIT

binary="$temporary_dir/ecs-exporter"
build_date="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
build_commit="$(git rev-parse HEAD)"
if [[ ! "$build_commit" =~ ^[0-9a-f]{40}$ ]]; then
  printf 'error: certification source must resolve to a full lowercase Git SHA\n' >&2
  exit 1
fi
CGO_ENABLED=0 go build -trimpath \
  -ldflags "-s -w -X main.version=certification -X main.commit=$build_commit -X main.buildDate=$build_date" \
  -o "$binary" ./cmd/ecs-exporter
"$binary" -profiles-dir=profiles -validate-profiles >/dev/null
"$binary" -config="$config" -profiles-dir=profiles -validate-config >/dev/null

"$binary" -config="$config" -profiles-dir=profiles \
  >"$temporary_dir/stdout.log" 2>"$temporary_dir/stderr.log" &
exporter_pid=$!

deadline=$((SECONDS + timeout_seconds))
while ((SECONDS < deadline)); do
  if curl --fail --silent --show-error --max-time 3 "$base_url/health" >/dev/null 2>&1; then
    break
  fi
  if ! kill -0 "$exporter_pid" 2>/dev/null; then
    printf 'error: exporter exited during certification startup\n' >&2
    exit 1
  fi
  sleep 2
done
if ! curl --fail --silent --show-error --max-time 3 "$base_url/health" >/dev/null; then
  printf 'error: exporter liveness did not become available\n' >&2
  exit 1
fi

readiness="$temporary_dir/readiness.json"
while ((SECONDS < deadline)); do
  if curl --silent --show-error --max-time 5 "$base_url/api/v1/health" \
    --output "$readiness"; then
    status="$(jq -r '.status // empty' "$readiness")"
    if [[ "$status" == "UP" || "$status" == "DEGRADED" ]]; then
      break
    fi
  fi
  if ! kill -0 "$exporter_pid" 2>/dev/null; then
    printf 'error: exporter exited while waiting for cache readiness\n' >&2
    exit 1
  fi
  sleep 5
done

status="$(jq -r '.status // empty' "$readiness")"
if [[ "$status" != "UP" && "$status" != "DEGRADED" ]]; then
  printf 'error: exporter readiness did not become serviceable\n' >&2
  exit 1
fi
if [[ "$status" == "DEGRADED" && "$allow_degraded" != "true" ]]; then
  printf 'error: production certification does not allow DEGRADED readiness\n' >&2
  exit 1
fi
if [[ "$status" == "DEGRADED" ]]; then
  jq -e '
    all(.clusters[];
      .status == "UP" or
      (.status == "DEGRADED" and .reason == "collector_error")
    )
  ' "$readiness" >/dev/null
fi

token="$(<"$token_file")"
while [[ "$token" == *$'\r' || "$token" == *$'\n' ]]; do
  token="${token%?}"
done
if [[ ${#token} -lt 16 || ${#token} -gt 4096 ]] ||
  [[ "$token" =~ [[:space:]] ]]; then
  printf 'error: inventory token must contain 16 to 4096 non-whitespace characters\n' >&2
  exit 1
fi
auth_header="$temporary_dir/auth-header"
printf 'Authorization: Bearer %s\n' "$token" >"$auth_header"
chmod 0600 "$auth_header"
unset token

metrics_file="$temporary_dir/metrics.txt"
curl --fail --silent --show-error --max-time 10 "$base_url/metrics" \
  --header "@$auth_header" --output "$metrics_file"
curl --fail --silent --show-error --max-time 10 "$base_url/api/v1/version" \
  --header "@$auth_header" --output "$temporary_dir/version.json"
jq -e --arg commit "$build_commit" '.build.commit == $commit' \
  "$temporary_dir/version.json" >/dev/null

resources=(clusters nodes namespaces buckets replications)
for resource in "${resources[@]}"; do
  curl --fail --silent --show-error --max-time 10 \
    "$base_url/api/v1/$resource?page=0&size=500" \
    --header "@$auth_header" \
    --output "$temporary_dir/$resource.json"
  jq -e '.items | type == "array"' "$temporary_dir/$resource.json" >/dev/null
done

required_metrics=(-require ecs_cluster_health)
if (($(jq '.totalElements' "$temporary_dir/nodes.json") > 0)); then
  required_metrics+=(-require ecs_node_health)
fi
if (($(jq '.totalElements' "$temporary_dir/namespaces.json") > 0)); then
  required_metrics+=(-require ecs_namespace_capacity_used_bytes)
fi
if (($(jq '.totalElements' "$temporary_dir/buckets.json") > 0)); then
  required_metrics+=(-require ecs_bucket_used_bytes)
fi
if (($(jq '.totalElements' "$temporary_dir/replications.json") > 0)); then
  required_metrics+=(-require ecs_replication_status)
fi
if [[ "$status" == "DEGRADED" ]]; then
  required_metrics+=(
    -require ecs_exporter_collector_errors_total
    -allow-collector-error node-resources
  )
fi
go run ./cmd/metricscheck "${required_metrics[@]}" <"$metrics_file" >/dev/null

jq -e --arg version "$expected_version" '
  [.items[].versions[]] as $versions |
  ($versions | length) > 0 and
  all($versions[]; . == $version or startswith($version + "."))
' "$temporary_dir/clusters.json" >/dev/null
for quota_resource in namespaces buckets; do
  jq -e '
    all(.items[];
      ((.softQuotaConfigured != false) or (.softQuotaBytes == null)) and
      ((.hardQuotaConfigured != false) or (.hardQuotaBytes == null))
    )
  ' "$temporary_dir/$quota_resource.json" >/dev/null
done

recorded_at="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
metrics_bytes="$(wc -c <"$metrics_file" | tr -d ' ')"
if command -v sha256sum >/dev/null 2>&1; then
  metrics_sha="$(sha256sum "$metrics_file" | awk '{print $1}')"
else
  metrics_sha="$(shasum -a 256 "$metrics_file" | awk '{print $1}')"
fi
jq -n \
  --arg recordedAt "$recorded_at" \
  --arg expectedVersion "$expected_version" \
  --arg exporterCommit "$build_commit" \
  --arg readiness "$status" \
  --arg metricsSHA256 "$metrics_sha" \
  --argjson metricsBytes "$metrics_bytes" \
  --argjson clusters "$(jq '.totalElements' "$temporary_dir/clusters.json")" \
  --argjson nodes "$(jq '.totalElements' "$temporary_dir/nodes.json")" \
  --argjson namespaces "$(jq '.totalElements' "$temporary_dir/namespaces.json")" \
  --argjson buckets "$(jq '.totalElements' "$temporary_dir/buckets.json")" \
  --argjson replications "$(jq '.totalElements' "$temporary_dir/replications.json")" \
  '{
    schemaVersion: "1.0",
    classification: "partial-live-read-only",
    recordedAt: $recordedAt,
    expectedVersion: $expectedVersion,
    exporterCommit: $exporterCommit,
    readiness: $readiness,
    response: {
      metricsBytes: $metricsBytes,
      metricsSHA256: $metricsSHA256
    },
    resourceCounts: {
      clusters: $clusters,
      nodes: $nodes,
      namespaces: $namespaces,
      buckets: $buckets,
      replications: $replications
    },
    redaction: {
      credentials: "not-recorded",
      endpoints: "not-recorded",
      rawResponses: "not-recorded",
      inventoryNames: "not-recorded"
    }
  }' >"$evidence_dir/result.json"

printf 'read-only live certification smoke passed; evidence is partial-live, not formal Profile certification\n'
