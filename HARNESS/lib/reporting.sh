#!/usr/bin/env bash

# Machine-readable evidence support for the portable harness. This file expects
# ROOT_DIR, PORTABLE_HARNESS_VERSION, HARNESS_REPORT_DIR, and
# HARNESS_WRITE_REPORTS to be set by harness.sh.

HARNESS_REPORT_ACTIVE="false"
HARNESS_REPORT_FINALIZED="false"
HARNESS_REPORT_CURRENT_STAGE=""
HARNESS_REPORT_CURRENT_COMMAND=""
HARNESS_REPORT_CURRENT_NETWORK="false"
HARNESS_REPORT_CURRENT_STARTED=0
HARNESS_REPORT_STARTED_AT=""
HARNESS_REPORT_RUN_TYPE=""
HARNESS_REPORT_RECORDS=""
HARNESS_REPORT_OUTPUT=""

harness_json_escape() {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  value="${value//$'\n'/\\n}"
  value="${value//$'\r'/\\r}"
  value="${value//$'\t'/\\t}"
  printf '%s' "$value"
}

harness_report_is_enabled() {
  case "${HARNESS_WRITE_REPORTS:-true}" in
    true|TRUE|yes|YES|1) return 0 ;;
    *) return 1 ;;
  esac
}

harness_report_path() {
  local directory="${HARNESS_REPORT_DIR:-test-results/harness}"
  local filename="${HARNESS_REPORT_FILE:-${HARNESS_REPORT_RUN_TYPE}.json}"
  case "$directory" in
    /*) printf '%s/%s\n' "$directory" "$filename" ;;
    *) printf '%s/%s/%s\n' "$ROOT_DIR" "$directory" "$filename" ;;
  esac
}

harness_report_init() {
  local run_type="$1"
  harness_report_is_enabled || return 0

  HARNESS_REPORT_RUN_TYPE="$run_type"
  HARNESS_REPORT_STARTED_AT="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
  HARNESS_REPORT_RECORDS="$(mktemp "${TMPDIR:-/tmp}/portable-harness-report.XXXXXX")"
  HARNESS_REPORT_OUTPUT="$(harness_report_path)"
  HARNESS_REPORT_ACTIVE="true"
  HARNESS_REPORT_FINALIZED="false"
  trap 'harness_report_on_exit' EXIT
}

harness_report_stage_begin() {
  local stage="$1"
  local command_text="$2"
  local network_allowed="${3:-false}"
  [[ "$HARNESS_REPORT_ACTIVE" == "true" ]] || return 0

  HARNESS_REPORT_CURRENT_STAGE="$stage"
  HARNESS_REPORT_CURRENT_COMMAND="$command_text"
  HARNESS_REPORT_CURRENT_NETWORK="$network_allowed"
  HARNESS_REPORT_CURRENT_STARTED="$(date '+%s')"
}

harness_report_stage_finish() {
  local status="$1"
  local exit_code="$2"
  local detail="${3:-}"
  [[ "$HARNESS_REPORT_ACTIVE" == "true" ]] || return 0
  [[ -n "$HARNESS_REPORT_CURRENT_STAGE" ]] || return 0

  local finished duration stage command_text network_allowed
  finished="$(date '+%s')"
  duration=$((finished - HARNESS_REPORT_CURRENT_STARTED))
  stage="$(harness_json_escape "$HARNESS_REPORT_CURRENT_STAGE")"
  command_text="$(harness_json_escape "$HARNESS_REPORT_CURRENT_COMMAND")"
  network_allowed="$(harness_json_escape "$HARNESS_REPORT_CURRENT_NETWORK")"
  detail="$(harness_json_escape "$detail")"

  printf '{"stage":"%s","status":"%s","exit_code":%s,"duration_seconds":%s,"network_allowed":%s,"command":"%s","detail":"%s"}\n' \
    "$stage" "$status" "$exit_code" "$duration" "$network_allowed" \
    "$command_text" "$detail" >>"$HARNESS_REPORT_RECORDS"

  HARNESS_REPORT_CURRENT_STAGE=""
  HARNESS_REPORT_CURRENT_COMMAND=""
  HARNESS_REPORT_CURRENT_NETWORK="false"
  HARNESS_REPORT_CURRENT_STARTED=0
}

harness_report_revision() {
  if command -v git >/dev/null 2>&1 \
    && git -C "$ROOT_DIR" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    git -C "$ROOT_DIR" rev-parse HEAD 2>/dev/null || true
  fi
}

harness_report_write_toolchain() {
  local first="true"
  local tool path version
  local tools=(git make node npm python3 python go rustc cargo)
  printf '['
  for tool in "${tools[@]}"; do
    command -v "$tool" >/dev/null 2>&1 || continue
    if [[ "$tool" == "python" ]] && command -v python3 >/dev/null 2>&1; then
      continue
    fi
    path="$(command -v "$tool")"
    case "$tool" in
      go) version="$("$tool" version 2>&1 | head -n 1 || true)" ;;
      *) version="$("$tool" --version 2>&1 | head -n 1 || true)" ;;
    esac
    if [[ "$first" == "true" ]]; then
      first="false"
    else
      printf ','
    fi
    printf '{"name":"%s","path":"%s","version":"%s"}' \
      "$(harness_json_escape "$tool")" \
      "$(harness_json_escape "$path")" \
      "$(harness_json_escape "$version")"
  done
  printf ']'
}

harness_report_finalize() {
  local exit_code="$1"
  [[ "$HARNESS_REPORT_ACTIVE" == "true" ]] || return 0
  [[ "$HARNESS_REPORT_FINALIZED" == "false" ]] || return 0
  HARNESS_REPORT_FINALIZED="true"

  local result finished revision os bash_version output_dir temporary_output
  if [[ "$exit_code" -eq 0 ]]; then
    result="passed"
  else
    result="failed"
  fi
  finished="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
  revision="$(harness_json_escape "$(harness_report_revision)")"
  os="$(harness_json_escape "$(uname -srm 2>/dev/null || uname -a)")"
  bash_version="$(harness_json_escape "$BASH_VERSION")"
  output_dir="$(dirname "$HARNESS_REPORT_OUTPUT")"
  mkdir -p "$output_dir" || return 1
  temporary_output="${HARNESS_REPORT_OUTPUT}.tmp.$$"

  if ! {
    printf '{\n'
    printf '  "schema_version": "1",\n'
    printf '  "harness_version": "%s",\n' "$(harness_json_escape "$PORTABLE_HARNESS_VERSION")"
    printf '  "run_type": "%s",\n' "$(harness_json_escape "$HARNESS_REPORT_RUN_TYPE")"
    printf '  "started_at": "%s",\n' "$(harness_json_escape "$HARNESS_REPORT_STARTED_AT")"
    printf '  "finished_at": "%s",\n' "$(harness_json_escape "$finished")"
    printf '  "repository_revision": "%s",\n' "$revision"
    printf '  "environment": {"os":"%s","bash":"%s"},\n' "$os" "$bash_version"
    printf '  "toolchain": '
    harness_report_write_toolchain
    printf ',\n'
    printf '  "result": "%s",\n' "$result"
    printf '  "exit_code": %s,\n' "$exit_code"
    printf '  "stages": [\n'
    awk 'NR > 1 { printf ",\n" } { printf "    %s", $0 } END { if (NR > 0) printf "\n" }' "$HARNESS_REPORT_RECORDS"
    printf '  ]\n'
    printf '}\n'
  } >"$temporary_output"; then
    rm -f "$temporary_output"
    return 1
  fi
  if ! mv "$temporary_output" "$HARNESS_REPORT_OUTPUT"; then
    rm -f "$temporary_output"
    return 1
  fi

  rm -f "$HARNESS_REPORT_RECORDS"
  HARNESS_REPORT_ACTIVE="false"
  printf 'evidence: %s\n' "$HARNESS_REPORT_OUTPUT"
}

harness_report_on_exit() {
  local exit_code=$?
  trap - EXIT

  if [[ "$HARNESS_REPORT_ACTIVE" == "true" ]]; then
    if [[ -n "$HARNESS_REPORT_CURRENT_STAGE" ]]; then
      if [[ "$exit_code" -eq 3 ]]; then
        harness_report_stage_finish blocked "$exit_code" "command reported blocked/unavailable"
      else
        harness_report_stage_finish failed "$exit_code" "command terminated before completing"
      fi
    fi
    if ! harness_report_finalize "$exit_code"; then
      [[ "$exit_code" -ne 0 ]] || exit_code=1
    fi
  fi
  exit "$exit_code"
}
