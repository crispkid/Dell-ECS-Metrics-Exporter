#!/usr/bin/env bash

set -euo pipefail

purge_config=false
if [[ "${1:-}" == "--purge-config" ]]; then
  purge_config=true
  shift
fi
if (($# != 0)); then
  printf 'Usage: sudo ./uninstall.sh [--purge-config]\n' >&2
  exit 2
fi
if [[ "$(id -u)" -ne 0 ]]; then
  printf 'error: uninstall.sh must run as root\n' >&2
  exit 1
fi

systemctl disable --now dell-ecs-metrics-exporter.service 2>/dev/null || true
rm -f -- /etc/systemd/system/dell-ecs-metrics-exporter.service
rm -f -- /etc/systemd/system/dell-ecs-metrics-exporter.service.previous
rm -f -- /usr/lib/sysusers.d/dell-ecs-metrics-exporter.conf
rm -f -- /usr/lib/tmpfiles.d/dell-ecs-metrics-exporter.conf
rm -f -- /usr/local/bin/ecs-exporter
rm -f -- /usr/local/bin/ecs-exporter.previous
rm -rf -- /usr/share/dell-ecs-metrics-exporter
systemctl daemon-reload

if [[ "$purge_config" == true ]]; then
  rm -rf -- /etc/dell-ecs-metrics-exporter
  printf 'removed configuration and secrets (--purge-config)\n'
else
  printf 'preserved /etc/dell-ecs-metrics-exporter\n'
fi

printf 'Dell ECS Metrics Exporter uninstalled\n'
