#!/usr/bin/env bash

set -euo pipefail

binary_source=""
profiles_source=""
config_source=""
start_service=true

usage() {
  cat <<'EOF'
Usage:
  sudo ./install.sh --binary PATH --profiles DIR [--config PATH] [--no-start]

Installs or upgrades Dell ECS Metrics Exporter on a systemd-based Linux host.
Existing config.yaml and secret files are preserved unless --config is supplied
for an initial installation. The script never creates credential values.
EOF
}

while (($# > 0)); do
  case "$1" in
    --binary)
      [[ $# -ge 2 ]] || { printf 'error: --binary requires a path\n' >&2; exit 2; }
      binary_source="$2"
      shift 2
      ;;
    --profiles)
      [[ $# -ge 2 ]] || { printf 'error: --profiles requires a directory\n' >&2; exit 2; }
      profiles_source="$2"
      shift 2
      ;;
    --config)
      [[ $# -ge 2 ]] || { printf 'error: --config requires a path\n' >&2; exit 2; }
      config_source="$2"
      shift 2
      ;;
    --no-start)
      start_service=false
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      printf 'error: unknown argument %s\n' "$1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ "$(id -u)" -ne 0 ]]; then
  printf 'error: install.sh must run as root\n' >&2
  exit 1
fi
if [[ -z "$binary_source" || ! -f "$binary_source" || ! -x "$binary_source" ]]; then
  printf 'error: --binary must name an executable file\n' >&2
  exit 1
fi
if [[ -z "$profiles_source" || ! -d "$profiles_source" ]]; then
  printf 'error: --profiles must name a profile directory\n' >&2
  exit 1
fi
if [[ -n "$config_source" && ! -f "$config_source" ]]; then
  printf 'error: --config must name a readable file\n' >&2
  exit 1
fi
if ! command -v systemctl >/dev/null 2>&1; then
  printf 'error: systemd/systemctl is required\n' >&2
  exit 1
fi

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
binary_target="/usr/local/bin/ecs-exporter"
share_root="/usr/share/dell-ecs-metrics-exporter"
profiles_target="$share_root/profiles"
config_root="/etc/dell-ecs-metrics-exporter"
config_target="$config_root/config.yaml"
unit_target="/etc/systemd/system/dell-ecs-metrics-exporter.service"
binary_stage=""
profiles_stage=""
had_complete_install=false
service_was_active=false
service_was_enabled=false
cleanup() {
  if [[ -n "$binary_stage" && -f "$binary_stage" ]]; then
    rm -f -- "$binary_stage"
  fi
  if [[ -n "$profiles_stage" && -d "$profiles_stage" ]]; then
    rm -r -- "$profiles_stage"
  fi
}
trap cleanup EXIT

if [[ -f "$binary_target" && -d "$profiles_target" ]]; then
  had_complete_install=true
fi
if systemctl is-active --quiet dell-ecs-metrics-exporter.service 2>/dev/null; then
  service_was_active=true
fi
if systemctl is-enabled --quiet dell-ecs-metrics-exporter.service 2>/dev/null; then
  service_was_enabled=true
fi

if command -v systemd-sysusers >/dev/null 2>&1; then
  install -D -m 0644 \
    "$script_dir/dell-ecs-metrics-exporter.sysusers" \
    /usr/lib/sysusers.d/dell-ecs-metrics-exporter.conf
  systemd-sysusers /usr/lib/sysusers.d/dell-ecs-metrics-exporter.conf
elif ! getent passwd ecs-exporter >/dev/null 2>&1; then
  nologin_shell="/usr/sbin/nologin"
  if [[ ! -x "$nologin_shell" ]]; then
    nologin_shell="/sbin/nologin"
  fi
  if [[ ! -x "$nologin_shell" ]]; then
    printf 'error: systemd-sysusers is unavailable and no nologin shell was found\n' >&2
    exit 1
  fi
  useradd --system --user-group --home-dir /nonexistent --shell "$nologin_shell" \
    --comment "Dell ECS Metrics Exporter" ecs-exporter
fi

install -d -m 0750 -o root -g ecs-exporter "$config_root" "$config_root/secrets"
install -d -m 0755 -o root -g root "$share_root"

profiles_stage="$(mktemp -d "$share_root/.profiles.XXXXXX")"
chmod 0755 "$profiles_stage"
while IFS= read -r -d '' profile; do
  install -m 0644 -o root -g root "$profile" "$profiles_stage/$(basename "$profile")"
done < <(find "$profiles_source" -maxdepth 1 -type f -name '*.json' -print0)
if [[ ! -f "$profiles_stage/profile.schema.json" ]]; then
  printf 'error: profile.schema.json is missing from --profiles\n' >&2
  exit 1
fi

if [[ ! -f "$config_target" ]]; then
  initial_config="${config_source:-$script_dir/config.yaml.example}"
  install -m 0640 -o root -g ecs-exporter "$initial_config" "$config_target"
elif [[ -n "$config_source" ]]; then
  printf 'notice: preserving existing %s; supplied --config was not applied\n' "$config_target"
fi

binary_stage="$(mktemp /usr/local/bin/.ecs-exporter.XXXXXX)"
install -m 0755 -o root -g root "$binary_source" "$binary_stage"
"$binary_stage" -profiles-dir="$profiles_stage" -validate-profiles >/dev/null
if [[ "$start_service" == true ]]; then
  "$binary_stage" \
    -config="$config_target" \
    -profiles-dir="$profiles_stage" \
    -validate-config >/dev/null
fi

rm -f -- "$binary_target.previous"
if [[ -f "$binary_target" ]]; then
  mv -- "$binary_target" "$binary_target.previous"
fi
mv -- "$binary_stage" "$binary_target"
binary_stage=""
rm -rf -- "$profiles_target.previous"
if [[ -d "$profiles_target" ]]; then
  mv -- "$profiles_target" "$profiles_target.previous"
fi
mv -- "$profiles_stage" "$profiles_target"
profiles_stage=""

install -m 0644 -o root -g root \
  "$script_dir/dell-ecs-metrics-exporter.service" "$unit_target.new"
rm -f -- "$unit_target.previous"
if [[ -f "$unit_target" ]]; then
  mv -- "$unit_target" "$unit_target.previous"
fi
mv -- "$unit_target.new" "$unit_target"
if command -v systemd-tmpfiles >/dev/null 2>&1; then
  install -D -m 0644 \
    "$script_dir/dell-ecs-metrics-exporter.tmpfiles" \
    /usr/lib/tmpfiles.d/dell-ecs-metrics-exporter.conf
  systemd-tmpfiles --create /usr/lib/tmpfiles.d/dell-ecs-metrics-exporter.conf
fi

systemctl daemon-reload
if [[ "$start_service" == true ]]; then
  systemctl enable dell-ecs-metrics-exporter.service
  if ! systemctl restart dell-ecs-metrics-exporter.service; then
    printf 'error: installed service failed to start; attempting rollback\n' >&2
    systemctl stop dell-ecs-metrics-exporter.service 2>/dev/null || true
    if [[ "$had_complete_install" == true &&
      -f "$binary_target.previous" &&
      -d "$profiles_target.previous" ]]; then
      rm -f -- "$binary_target"
      mv -- "$binary_target.previous" "$binary_target"
      rm -rf -- "$profiles_target"
      mv -- "$profiles_target.previous" "$profiles_target"
      if [[ -f "$unit_target.previous" ]]; then
        rm -f -- "$unit_target"
        mv -- "$unit_target.previous" "$unit_target"
      fi
      systemctl daemon-reload
      if [[ "$service_was_active" == true ]]; then
        systemctl start dell-ecs-metrics-exporter.service || {
          printf 'error: rollback files restored, but the previous service could not be restarted\n' >&2
          exit 1
        }
      fi
      printf 'notice: previous binary and profiles were restored\n' >&2
    fi
    if [[ "$service_was_enabled" != true ]]; then
      systemctl disable dell-ecs-metrics-exporter.service 2>/dev/null || true
    fi
    exit 1
  fi
else
  printf 'notice: service installed but not restarted; config validation is deferred (--no-start)\n'
fi

printf 'installed: %s\n' "$binary_target"
printf 'configuration: %s\n' "$config_target"
printf 'secrets directory: %s\n' "$config_root/secrets"
