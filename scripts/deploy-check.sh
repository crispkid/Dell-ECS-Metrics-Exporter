#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"
# shellcheck source=go-env.sh
source "$SCRIPT_DIR/go-env.sh"

chart="charts/dell-ecs-metrics-exporter"
temporary_dir="$(mktemp -d "${TMPDIR:-/tmp}/dell-ecs-deploy-check.XXXXXX")"
trap 'rm -r -- "$temporary_dir"' EXIT

for script in \
  deploy/bare-metal/install.sh \
  deploy/bare-metal/uninstall.sh \
  deploy/bare-metal/verify.sh; do
  if [[ ! -x "$script" ]]; then
    printf 'error: Bare Metal script is not executable: %s\n' "$script" >&2
    exit 1
  fi
  bash -n "$script"
done
for contract in \
  'User=ecs-exporter' \
  'NoNewPrivileges=true' \
  'ProtectSystem=strict' \
  'RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6' \
  'MemoryDenyWriteExecute=true'; do
  if ! rg -q -- "$contract" deploy/bare-metal/dell-ecs-metrics-exporter.service; then
    printf 'error: Bare Metal unit is missing contract %s\n' "$contract" >&2
    exit 1
  fi
done
for contract in \
  'systemctl restart dell-ecs-metrics-exporter.service' \
  'mv -- "$binary_target.previous" "$binary_target"' \
  'mv -- "$profiles_target.previous" "$profiles_target"' \
  'mv -- "$unit_target.previous" "$unit_target"'; do
  if ! rg -Fq -- "$contract" deploy/bare-metal/install.sh; then
    printf 'error: Bare Metal installer is missing upgrade/rollback contract %s\n' \
      "$contract" >&2
    exit 1
  fi
done
if rg -q 'OWNER|REPOSITORY|example.com' \
  deploy/bare-metal/dell-ecs-metrics-exporter.service; then
  printf 'error: Bare Metal unit contains an unresolved placeholder\n' >&2
  exit 1
fi
if command -v systemd-analyze >/dev/null 2>&1; then
  sed \
    -e '/^ExecStartPre=/s#=.*#=/bin/true#' \
    -e '/^ExecStart=/s#=.*#=/bin/true#' \
    deploy/bare-metal/dell-ecs-metrics-exporter.service \
    >"$temporary_dir/dell-ecs-metrics-exporter.service"
  systemd-analyze verify "$temporary_dir/dell-ecs-metrics-exporter.service"
else
  printf 'notice: systemd-analyze unavailable; Bare Metal syntax/static checks completed\n'
fi

go run ./cmd/rulescheck deploy/prometheus/alerts.yaml >/dev/null
if command -v promtool >/dev/null 2>&1; then
  promtool check rules deploy/prometheus/alerts.yaml
else
  printf 'notice: promtool unavailable; strict alert YAML/operator metadata checks completed\n'
fi

helm lint "$chart"
helm template development "$chart" >"$temporary_dir/default.yaml"
helm template development "$chart" \
  --set serviceMonitor.enabled=true \
  --set credentials.existingSecret=ecs-exporter-auth \
  >"$temporary_dir/full.yaml"
helm template production "$chart" \
  --values "$chart/values-production.example.yaml" \
  >"$temporary_dir/production.yaml"

for kind in Deployment Service ConfigMap ServiceAccount NetworkPolicy PodDisruptionBudget; do
  if ! rg -q "^kind: ${kind}$" "$temporary_dir/default.yaml"; then
    printf 'error: rendered chart is missing %s\n' "$kind" >&2
    exit 1
  fi
done
if ! rg -q '^kind: ServiceMonitor$' "$temporary_dir/full.yaml"; then
  printf 'error: ServiceMonitor option did not render\n' >&2
  exit 1
fi
if ! rg -q 'secretName: ecs-exporter-auth' "$temporary_dir/full.yaml"; then
  printf 'error: existing Secret reference did not render\n' >&2
  exit 1
fi
if rg -q '^kind: Secret$' "$temporary_dir/full.yaml"; then
  printf 'error: chart must not render credential-bearing Secret resources\n' >&2
  exit 1
fi
for contract in \
  '    - Egress' \
  '  replicas: 1' \
  '  maxUnavailable: 0' \
  'kubernetes.io/metadata.name: kube-system' \
  'cidr: 192.0.2.0/24' \
  'port: 4443' \
  'port: 53'; do
  if ! rg -q -- "$contract" "$temporary_dir/production.yaml"; then
    printf 'error: production NetworkPolicy is missing contract %s\n' "$contract" >&2
    exit 1
  fi
done
if ! rg -q 'digest: sha256:replace-with-verified-release-digest' \
  "$chart/values-production.example.yaml"; then
  printf 'error: production values must remain fail-closed until a release digest is supplied\n' >&2
  exit 1
fi
for contract in \
  'runAsNonRoot: true' \
  'readOnlyRootFilesystem: true' \
  'checksum/config:' \
  'defaultMode: 0440' \
  'livenessProbe:' \
  'readinessProbe:' \
  'resources:' \
  '-config=/etc/ecs-exporter/config.yaml' \
  'path: inventory-token' \
  'path: username' \
  'path: password'; do
  if ! rg -q -- "$contract" "$temporary_dir/default.yaml"; then
    printf 'error: rendered chart is missing deployment contract %s\n' "$contract" >&2
    exit 1
  fi
done

if command -v kubeconform >/dev/null 2>&1; then
  for kubernetes_version in 1.25.0 1.31.0; do
    kubeconform \
      -strict \
      -kubernetes-version "$kubernetes_version" \
      -ignore-missing-schemas \
      -summary \
      "$temporary_dir/default.yaml" \
      "$temporary_dir/full.yaml" \
      "$temporary_dir/production.yaml"
  done
else
  printf 'notice: kubeconform unavailable; static Helm policy checks completed\n'
fi
