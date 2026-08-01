#!/usr/bin/env bash

set -euo pipefail
export LC_ALL=C

base_url="${ECS_PERF_BASE_URL:-}"
token_file="${ECS_PERF_INVENTORY_TOKEN_FILE:-}"
process_id="${ECS_PERF_PID:-}"
process_id_file="${ECS_PERF_PID_FILE:-}"
ca_file="${ECS_PERF_CA_FILE:-}"
expected_commit="${ECS_PERF_EXPECTED_COMMIT:-}"
minimum_clusters="${ECS_PERF_MIN_CLUSTERS:-10}"
minimum_nodes="${ECS_PERF_MIN_NODES:-100}"
minimum_buckets="${ECS_PERF_MIN_BUCKETS:-10000}"
maximum_rss_bytes="${ECS_PERF_MAX_RSS_BYTES:-536870912}"
maximum_cpu_percent="${ECS_PERF_MAX_CPU_PERCENT:-100}"
metrics_p95_limit="${ECS_PERF_METRICS_P95_SECONDS:-3}"
inventory_p95_limit="${ECS_PERF_INVENTORY_P95_SECONDS:-2}"

if [[ ! "$base_url" =~ ^https://[A-Za-z0-9._-]+(:[0-9]+)?$ ]] &&
  [[ ! "$base_url" =~ ^http://(127\.0\.0\.1|localhost)(:[0-9]+)?$ ]]; then
  printf 'blocked: ECS_PERF_BASE_URL must be HTTPS without a path, or loopback HTTP\n' >&2
  exit 3
fi
if [[ -z "$token_file" || ! -r "$token_file" ]]; then
  printf 'blocked: ECS_PERF_INVENTORY_TOKEN_FILE must name a readable token file\n' >&2
  exit 3
fi
if [[ -z "$process_id" && -n "$process_id_file" && -r "$process_id_file" ]]; then
  process_id="$(tr -d '[:space:]' <"$process_id_file")"
fi
if [[ ! "$process_id" =~ ^[1-9][0-9]*$ ]] || ! kill -0 "$process_id" 2>/dev/null; then
  printf 'blocked: ECS_PERF_PID or ECS_PERF_PID_FILE must identify the deployed exporter process\n' >&2
  exit 3
fi
if [[ ! "$expected_commit" =~ ^[0-9a-f]{40}$ ]]; then
  printf 'blocked: ECS_PERF_EXPECTED_COMMIT must be a full lowercase Git SHA\n' >&2
  exit 3
fi
if [[ -n "$ca_file" && ! -r "$ca_file" ]]; then
  printf 'blocked: ECS_PERF_CA_FILE must name a readable CA bundle\n' >&2
  exit 3
fi
for value in \
  "$minimum_clusters" "$minimum_nodes" "$minimum_buckets" \
  "$maximum_rss_bytes" "$maximum_cpu_percent"; do
  if [[ ! "$value" =~ ^[1-9][0-9]*$ ]]; then
    printf 'error: performance counts and resource limits must be positive integers\n' >&2
    exit 2
  fi
done
for tool in curl jq awk sort ps seq; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    printf 'blocked: deployed performance gate requires %s\n' "$tool" >&2
    exit 3
  fi
done

temporary_dir="$(mktemp -d "${TMPDIR:-/tmp}/dell-ecs-deployed-performance.XXXXXX")"
trap 'rm -r -- "$temporary_dir"' EXIT
mkdir -p test-results/performance

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

curl_args=(--fail --silent --show-error --max-time 30 --header "@$auth_header")
if [[ -n "$ca_file" ]]; then
  curl_args+=(--cacert "$ca_file")
fi

sample_resources() {
  local rss cpu
  rss="$(ps -o rss= -p "$process_id" | awk '{ print int($1) }')"
  cpu="$(ps -o %cpu= -p "$process_id" | awk '{ print $1 + 0 }')"
  if [[ ! "$rss" =~ ^[0-9]+$ ]] ||
    [[ ! "$cpu" =~ ^[0-9]+([.][0-9]+)?$ ]]; then
    printf 'error: could not sample exporter RSS/CPU\n' >&2
    exit 1
  fi
  printf '%s\n' "$rss" >>"$temporary_dir/rss-samples.txt"
  printf '%s\n' "$cpu" >>"$temporary_dir/cpu-samples.txt"
}
sample_resources

for resource in clusters nodes buckets; do
  curl "${curl_args[@]}" "$base_url/api/v1/$resource?page=0&size=1" \
    --output "$temporary_dir/$resource.json"
  jq -e '.totalElements | type == "number"' \
    "$temporary_dir/$resource.json" >/dev/null
done
curl "${curl_args[@]}" "$base_url/api/v1/version" \
  --output "$temporary_dir/version.json"
jq -e --arg commit "$expected_commit" '.build.commit == $commit' \
  "$temporary_dir/version.json" >/dev/null
cluster_count="$(jq -r '.totalElements' "$temporary_dir/clusters.json")"
node_count="$(jq -r '.totalElements' "$temporary_dir/nodes.json")"
bucket_count="$(jq -r '.totalElements' "$temporary_dir/buckets.json")"
if ((cluster_count < minimum_clusters || node_count < minimum_nodes || bucket_count < minimum_buckets)); then
  printf 'blocked: scale is %s clusters/%s nodes/%s buckets; required minimum is %s/%s/%s\n' \
    "$cluster_count" "$node_count" "$bucket_count" \
    "$minimum_clusters" "$minimum_nodes" "$minimum_buckets" >&2
  exit 3
fi

for index in $(seq 1 20); do
  curl "${curl_args[@]}" \
    --output "$temporary_dir/metrics-$index.txt" \
    --write-out '%{time_total}\n' \
    "$base_url/metrics" >>"$temporary_dir/metrics-times.txt"
  sample_resources
done
for index in $(seq 1 100); do
  curl "${curl_args[@]}" \
    --output /dev/null \
    --write-out '%{time_total}\n' \
    "$base_url/api/v1/buckets?page=$((index % 20))&size=500&sort=name" \
    >>"$temporary_dir/inventory-times.txt"
  sample_resources
done

percentile95() {
  sort -n "$1" |
    awk '{ values[NR]=$1 } END {
      index=int((NR - 1) * 0.95) + 1
      print values[index]
    }'
}
metrics_p95="$(percentile95 "$temporary_dir/metrics-times.txt")"
inventory_p95="$(percentile95 "$temporary_dir/inventory-times.txt")"
rss_kib="$(sort -nr "$temporary_dir/rss-samples.txt" | head -n 1)"
cpu_percent="$(sort -nr "$temporary_dir/cpu-samples.txt" | head -n 1)"
rss_bytes=$((rss_kib * 1024))
metrics_bytes="$(wc -c <"$temporary_dir/metrics-20.txt" | tr -d ' ')"

awk -v actual="$metrics_p95" -v limit="$metrics_p95_limit" \
  'BEGIN { exit !(actual < limit) }'
awk -v actual="$inventory_p95" -v limit="$inventory_p95_limit" \
  'BEGIN { exit !(actual < limit) }'
if ((rss_bytes >= maximum_rss_bytes)); then
  printf 'error: RSS %s exceeds limit %s\n' "$rss_bytes" "$maximum_rss_bytes" >&2
  exit 1
fi
awk -v actual="$cpu_percent" -v limit="$maximum_cpu_percent" \
  'BEGIN { exit !(actual <= limit) }'

jq -n \
  --arg recordedAt "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" \
  --argjson clusters "$cluster_count" \
  --argjson nodes "$node_count" \
  --argjson buckets "$bucket_count" \
  --argjson metricsP95 "$metrics_p95" \
  --argjson inventoryP95 "$inventory_p95" \
  --argjson metricsBytes "$metrics_bytes" \
  --argjson rssBytes "$rss_bytes" \
  --argjson cpuPercent "$cpu_percent" \
  --arg exporterCommit "$expected_commit" \
  '{
    schemaVersion: "1.0",
    classification: "deployed-target-scale",
    recordedAt: $recordedAt,
    exporterCommit: $exporterCommit,
    scale: {clusters: $clusters, nodes: $nodes, buckets: $buckets},
    metricsP95Seconds: $metricsP95,
    inventoryP95Seconds: $inventoryP95,
    metricsResponseBytes: $metricsBytes,
    rssBytes: $rssBytes,
    cpuPercent: $cpuPercent,
    resourceSample: "peak-observed-during-load",
    redaction: {
      credentials: "not-recorded",
      endpoints: "not-recorded",
      rawInventory: "not-recorded"
    }
  }' >test-results/performance/deployed.json

printf 'deployed target-scale performance gate passed\n'
