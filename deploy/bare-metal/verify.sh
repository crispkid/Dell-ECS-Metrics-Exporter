#!/usr/bin/env bash

set -euo pipefail

binary="${ECS_EXPORTER_BINARY:-/usr/local/bin/ecs-exporter}"
config="${ECS_EXPORTER_CONFIG:-/etc/dell-ecs-metrics-exporter/config.yaml}"
profiles="${ECS_EXPORTER_PROFILES_DIR:-/usr/share/dell-ecs-metrics-exporter/profiles}"
health_url="${ECS_EXPORTER_HEALTH_URL:-http://127.0.0.1:8080/health}"
readiness_url="${ECS_EXPORTER_READINESS_URL:-http://127.0.0.1:8080/api/v1/health}"
version_url="${ECS_EXPORTER_VERSION_URL:-http://127.0.0.1:8080/api/v1/version}"
metrics_url="${ECS_EXPORTER_METRICS_URL:-http://127.0.0.1:8080/metrics}"
inventory_url="${ECS_EXPORTER_INVENTORY_URL:-http://127.0.0.1:8080/api/v1/clusters?page=0&size=1}"
token_file="${ECS_EXPORTER_INVENTORY_TOKEN_FILE:-/etc/dell-ecs-metrics-exporter/secrets/inventory-token}"
allow_degraded="${ECS_EXPORTER_ALLOW_DEGRADED:-false}"

if [[ "$allow_degraded" != "true" && "$allow_degraded" != "false" ]]; then
  printf 'error: ECS_EXPORTER_ALLOW_DEGRADED must be true or false\n' >&2
  exit 2
fi
if [[ ! -r "$token_file" ]]; then
  printf 'error: Inventory token is not readable: %s\n' "$token_file" >&2
  exit 1
fi

temporary_dir="$(mktemp -d "${TMPDIR:-/tmp}/dell-ecs-bare-metal-verify.XXXXXX")"
trap 'rm -r -- "$temporary_dir"' EXIT
token="$(<"$token_file")"
while [[ "$token" == *$'\r' || "$token" == *$'\n' ]]; do
  token="${token%?}"
done
if [[ ${#token} -lt 16 || ${#token} -gt 4096 ]] ||
  [[ "$token" =~ [[:space:]] ]]; then
  printf 'error: Inventory token must contain 16 to 4096 non-whitespace characters\n' >&2
  exit 1
fi
auth_header="$temporary_dir/auth-header"
printf 'Authorization: Bearer %s\n' "$token" >"$auth_header"
chmod 0600 "$auth_header"
unset token

"$binary" -profiles-dir="$profiles" -validate-profiles >/dev/null
"$binary" -config="$config" -profiles-dir="$profiles" -validate-config >/dev/null

if command -v systemd-analyze >/dev/null 2>&1; then
  systemd-analyze verify /etc/systemd/system/dell-ecs-metrics-exporter.service
fi
systemctl is-enabled --quiet dell-ecs-metrics-exporter.service
systemctl is-active --quiet dell-ecs-metrics-exporter.service

curl --fail --silent --show-error --max-time 5 "$health_url" >/dev/null
curl --fail --silent --show-error --max-time 5 "$readiness_url" \
  --output "$temporary_dir/readiness.json"
if ! grep -q '"status":"UP"' "$temporary_dir/readiness.json"; then
  if [[ "$allow_degraded" != "true" ]] ||
    ! grep -q '"status":"DEGRADED"' "$temporary_dir/readiness.json"; then
    printf 'error: readiness is neither UP nor an explicitly allowed DEGRADED\n' >&2
    exit 1
  fi
fi
curl --fail --silent --show-error --max-time 5 "$version_url" \
  --output "$temporary_dir/version.json"
grep -q '"version":' "$temporary_dir/version.json"
curl --fail --silent --show-error --max-time 10 \
  --header "@$auth_header" \
  "$metrics_url" \
  --output "$temporary_dir/metrics.txt"
grep -q '^ecs_exporter_build_info{' "$temporary_dir/metrics.txt"

unauthenticated_code="$(
  curl --silent --show-error --max-time 5 \
    --output /dev/null --write-out '%{http_code}' "$inventory_url"
)"
if [[ "$unauthenticated_code" != "401" && "$unauthenticated_code" != "403" ]]; then
  printf 'error: Inventory API did not reject unauthenticated access (HTTP %s)\n' \
    "$unauthenticated_code" >&2
  exit 1
fi
curl --fail --silent --show-error --max-time 10 \
  --header "@$auth_header" \
  "$inventory_url" \
  --output "$temporary_dir/inventory.json"
grep -q '"items":' "$temporary_dir/inventory.json"

printf 'bare-metal deployment verification passed\n'
