#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

if ! command -v docker >/dev/null 2>&1; then
  printf 'blocked: Docker is required for the container gate\n' >&2
  exit 3
fi
if ! docker info >/dev/null 2>&1; then
  printf 'blocked: Docker daemon is unavailable\n' >&2
  exit 3
fi

image="${CONTAINER_CHECK_IMAGE:-dell-ecs-metrics-exporter:release-check}"
container="dell-ecs-exporter-check-$$"
temporary_dir="$(mktemp -d "${TMPDIR:-/tmp}/dell-ecs-container-check.XXXXXX")"
cleanup() {
  docker rm -f -- "$container" >/dev/null 2>&1 || true
  rm -r -- "$temporary_dir"
}
trap cleanup EXIT

mkdir -p "$temporary_dir/secrets"
printf 'monitor-user\n' >"$temporary_dir/secrets/username"
printf 'synthetic-password\n' >"$temporary_dir/secrets/password"
printf 'synthetic-inventory-token\n' >"$temporary_dir/secrets/inventory-token"
chmod 0644 "$temporary_dir/secrets/"*

cat >"$temporary_dir/config.yaml" <<'EOF'
server:
  listenAddress: ":8080"
prometheus:
  path: /metrics
  protected: false
security:
  inventoryApi:
    enabled: true
    authentication: token
    tokenFile: /run/secrets/inventory-token
ecs:
  clusters:
    - name: container-smoke
      site: local
      environment: development
      endpoint: https://127.0.0.1:1
      usernameFile: /run/secrets/username
      passwordFile: /run/secrets/password
      tls:
        verify: false
EOF
chmod 0644 "$temporary_dir/config.yaml"

docker build \
  --build-arg VERSION=release-check \
  --build-arg COMMIT=0000000000000000000000000000000000000000 \
  --build-arg BUILD_DATE=1970-01-01T00:00:00Z \
  --tag "$image" \
  .

image_user="$(docker image inspect --format '{{.Config.User}}' "$image")"
if [[ "$image_user" != "65532:65532" ]]; then
  printf 'error: image user is %s, expected 65532:65532\n' "$image_user" >&2
  exit 1
fi

docker run -d \
  --name "$container" \
  --read-only \
  --tmpfs /tmp:rw,noexec,nosuid,nodev,size=16m \
  --cap-drop ALL \
  --security-opt no-new-privileges \
  --user 65532:65532 \
  -p 127.0.0.1::8080 \
  -v "$temporary_dir/config.yaml:/etc/ecs-exporter/config.yaml:ro" \
  -v "$temporary_dir/secrets:/run/secrets:ro" \
  "$image" >/dev/null

host_port="$(docker port "$container" 8080/tcp | awk -F: 'NR == 1 {print $NF}')"
if [[ ! "$host_port" =~ ^[0-9]+$ ]]; then
  printf 'error: could not determine mapped exporter port\n' >&2
  exit 1
fi
for _ in 1 2 3 4 5 6 7 8 9 10; do
  if curl --fail --silent --show-error --max-time 2 \
    "http://127.0.0.1:$host_port/health" >/dev/null; then
    break
  fi
  sleep 1
done
curl --fail --silent --show-error --max-time 2 \
  "http://127.0.0.1:$host_port/health" >/dev/null
curl --fail --silent --show-error --max-time 2 \
  "http://127.0.0.1:$host_port/api/v1/version" >/dev/null
curl --fail --silent --show-error --max-time 2 \
  "http://127.0.0.1:$host_port/metrics" >/dev/null

printf 'container build/start/read-only/non-root smoke passed\n'
