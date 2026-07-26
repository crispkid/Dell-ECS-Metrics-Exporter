#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"
# shellcheck source=go-env.sh
source "$SCRIPT_DIR/go-env.sh"

base_url="${ECS_E2E_BASE_URL:-}"
token_file="${ECS_E2E_INVENTORY_TOKEN_FILE:-}"
expected_ecs_version="${ECS_E2E_EXPECTED_ECS_VERSION:-}"
expected_commit="${ECS_E2E_EXPECTED_COMMIT:-}"
ca_file="${ECS_E2E_CA_FILE:-}"

if [[ -z "$base_url" ]]; then
  printf 'blocked: ECS_E2E_BASE_URL is required\n' >&2
  exit 3
fi
if [[ ! "$base_url" =~ ^https://[A-Za-z0-9._-]+(:[0-9]+)?$ ]] &&
  [[ ! "$base_url" =~ ^http://(127\.0\.0\.1|localhost)(:[0-9]+)?$ ]]; then
  printf 'error: ECS_E2E_BASE_URL must be HTTPS without a path, or loopback HTTP\n' >&2
  exit 1
fi
if [[ -z "$token_file" || ! -r "$token_file" ]]; then
  printf 'blocked: ECS_E2E_INVENTORY_TOKEN_FILE must name a readable token file\n' >&2
  exit 3
fi
if [[ ! "$expected_ecs_version" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  printf 'blocked: ECS_E2E_EXPECTED_ECS_VERSION must be an exact four-part version\n' >&2
  exit 3
fi
if [[ ! "$expected_commit" =~ ^[0-9a-f]{40}$ ]]; then
  printf 'blocked: ECS_E2E_EXPECTED_COMMIT must be a full lowercase Git SHA\n' >&2
  exit 3
fi
if [[ -n "$ca_file" && ! -r "$ca_file" ]]; then
  printf 'blocked: ECS_E2E_CA_FILE must name a readable CA bundle\n' >&2
  exit 3
fi
for tool in curl jq go; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    printf 'blocked: deployed E2E requires %s\n' "$tool" >&2
    exit 3
  fi
done
if ! command -v sha256sum >/dev/null 2>&1 &&
  ! command -v shasum >/dev/null 2>&1; then
  printf 'blocked: deployed E2E requires sha256sum or shasum\n' >&2
  exit 3
fi

temporary_dir="$(mktemp -d "${TMPDIR:-/tmp}/dell-ecs-e2e.XXXXXX")"
trap 'rm -r -- "$temporary_dir"' EXIT

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

curl_args=(--fail --silent --show-error --max-time 15 --header "@$auth_header")
if [[ -n "$ca_file" ]]; then
  curl_args+=(--cacert "$ca_file")
fi

curl "${curl_args[@]}" "$base_url/health" >/dev/null
curl "${curl_args[@]}" "$base_url/api/v1/health" \
  --output "$temporary_dir/health.json"
jq -e '.status == "UP"' "$temporary_dir/health.json" >/dev/null
curl "${curl_args[@]}" "$base_url/api/v1/version" \
  --output "$temporary_dir/version.json"
jq -e '.build.version | type == "string" and length > 0' \
  "$temporary_dir/version.json" >/dev/null
jq -e --arg commit "$expected_commit" '.build.commit == $commit' \
  "$temporary_dir/version.json" >/dev/null

unauthenticated_args=(--silent --show-error --max-time 15)
if [[ -n "$ca_file" ]]; then
  unauthenticated_args+=(--cacert "$ca_file")
fi
unauthenticated_code="$(
  curl "${unauthenticated_args[@]}" \
    --output /dev/null --write-out '%{http_code}' \
    "$base_url/api/v1/clusters"
)"
if [[ "$unauthenticated_code" != "401" && "$unauthenticated_code" != "403" ]]; then
  printf 'error: Inventory API accepted an unauthenticated request (HTTP %s)\n' \
    "$unauthenticated_code" >&2
  exit 1
fi

resources=(clusters nodes namespaces buckets replications)
for resource in "${resources[@]}"; do
  curl "${curl_args[@]}" \
    "$base_url/api/v1/$resource?page=0&size=500" \
    --output "$temporary_dir/$resource.json"
  jq -e '
    (.items | type == "array") and
    (.page | type == "number") and
    (.size | type == "number") and
    (.totalElements | type == "number")
  ' "$temporary_dir/$resource.json" >/dev/null
done

jq -e --arg version "$expected_ecs_version" '
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

curl "${curl_args[@]}" "$base_url/metrics" \
  --output "$temporary_dir/metrics.txt"
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
go run ./cmd/metricscheck "${required_metrics[@]}" \
  <"$temporary_dir/metrics.txt" >/dev/null

mkdir -p test-results/e2e
if command -v sha256sum >/dev/null 2>&1; then
  metrics_sha="$(sha256sum "$temporary_dir/metrics.txt" | awk '{print $1}')"
else
  metrics_sha="$(shasum -a 256 "$temporary_dir/metrics.txt" | awk '{print $1}')"
fi
jq -n \
  --arg recordedAt "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" \
  --arg expectedECSVersion "$expected_ecs_version" \
  --arg exporterVersion "$(jq -r '.build.version' "$temporary_dir/version.json")" \
  --arg exporterCommit "$expected_commit" \
  --arg metricsSHA256 "$metrics_sha" \
  --argjson metricsBytes "$(wc -c <"$temporary_dir/metrics.txt" | tr -d ' ')" \
  --argjson clusters "$(jq '.totalElements' "$temporary_dir/clusters.json")" \
  --argjson nodes "$(jq '.totalElements' "$temporary_dir/nodes.json")" \
  --argjson namespaces "$(jq '.totalElements' "$temporary_dir/namespaces.json")" \
  --argjson buckets "$(jq '.totalElements' "$temporary_dir/buckets.json")" \
  --argjson replications "$(jq '.totalElements' "$temporary_dir/replications.json")" \
  '{
    schemaVersion: "1.0",
    classification: "deployed-end-to-end",
    recordedAt: $recordedAt,
    expectedECSVersion: $expectedECSVersion,
    exporterVersion: $exporterVersion,
    exporterCommit: $exporterCommit,
    readiness: "UP",
    unauthenticatedInventory: "rejected",
    metrics: {bytes: $metricsBytes, sha256: $metricsSHA256},
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
  }' >test-results/e2e/result.json

printf 'deployed exporter E2E passed for ECS %s\n' "$expected_ecs_version"
