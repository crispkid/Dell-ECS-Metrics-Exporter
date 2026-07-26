#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
ROOT_DIR="${HARNESS_ROOT_DIR:-$(cd "$SCRIPT_DIR/.." && pwd -P)}"
readonly PORTABLE_HARNESS_VERSION="1.1.0"
CONFIG_EXPLICIT="false"
if [[ -n "${HARNESS_CONFIG_FILE:-}" ]]; then
  CONFIG_EXPLICIT="true"
fi
CONFIG_FILE="${HARNESS_CONFIG_FILE:-$SCRIPT_DIR/config.env}"

if [[ -f "$CONFIG_FILE" ]]; then
  # shellcheck source=/dev/null
  source "$CONFIG_FILE"
elif [[ "$CONFIG_EXPLICIT" == "true" ]]; then
  echo "Configured harness file does not exist: $CONFIG_FILE" >&2
  exit 1
fi

: "${HARNESS_PROJECT_NAME:=}"
: "${HARNESS_REQUIRED_PATHS:=}"
: "${HARNESS_EXPECTED_BASELINE_VERSION:=}"
: "${HARNESS_REQUIRED_STAGES:=}"
: "${HARNESS_FAIL_ON_SKIP:=false}"
: "${HARNESS_GOVERNANCE_MODE:=auto}"
: "${HARNESS_REQUIRE_PLAN_APPROVAL:=false}"
: "${HARNESS_ACTIVE_CHANGE_ID:=}"
: "${HARNESS_ACTIVE_CHANGE_FILE:=HARNESS/ACTIVE_CHANGE}"
: "${HARNESS_CHANGE_ID_PATTERN:=[A-Z][A-Z0-9_-]*-[0-9]+}"
: "${HARNESS_ALLOW_LEGACY_CHANGE_DISCOVERY:=false}"
: "${HARNESS_ALLOW_LEGACY_APPROVAL_FORMAT:=false}"
: "${HARNESS_SPEC_FILE:=SPECIFICATION.md}"
: "${HARNESS_CHANGELOG_FILE:=SPEC_CHANGELOG.md}"
: "${HARNESS_PLAN_FILE:=DEVELOPMENT_PLAN.md}"
: "${HARNESS_TEST_PLAN_FILE:=TEST_PLAN.md}"
: "${HARNESS_TRACE_FILE:=TRACEABILITY.md}"
: "${HARNESS_COVERAGE_THRESHOLD:=80}"
: "${HARNESS_STRICT_REPO_HYGIENE:=false}"
: "${HARNESS_REQUIRED_TOOLS:=}"
: "${HARNESS_ENVIRONMENT_COMMAND:=}"
: "${HARNESS_COMMAND_TIMEOUT_SECONDS:=0}"
: "${HARNESS_NETWORK_POLICY:=declared}"
: "${HARNESS_NETWORK_STAGES:=integration,e2e,security,deploy,ci-policy,supply-chain,agent-regression,agent-capability}"
: "${HARNESS_WRITE_REPORTS:=true}"
: "${HARNESS_REPORT_DIR:=test-results/harness}"
: "${HARNESS_REPORT_FILE:=}"
: "${HARNESS_LINT_COMMAND:=}"
: "${HARNESS_FORMAT_COMMAND:=}"
: "${HARNESS_TYPECHECK_COMMAND:=}"
: "${HARNESS_TEST_COMMAND:=}"
: "${HARNESS_COVERAGE_COMMAND:=}"
: "${HARNESS_BUILD_COMMAND:=}"
: "${HARNESS_INTEGRATION_COMMAND:=}"
: "${HARNESS_E2E_COMMAND:=}"
: "${HARNESS_SECURITY_COMMAND:=}"
: "${HARNESS_DEPLOY_COMMAND:=}"
: "${HARNESS_CI_POLICY_COMMAND:=}"
: "${HARNESS_SUPPLY_CHAIN_COMMAND:=}"
: "${HARNESS_AGENT_EVAL_DIR:=HARNESS/evals}"
: "${HARNESS_AGENT_REGRESSION_COMMAND:=}"
: "${HARNESS_AGENT_CAPABILITY_COMMAND:=}"
: "${HARNESS_AGENT_TRIALS:=3}"

export HARNESS_COVERAGE_THRESHOLD
export HARNESS_AGENT_TRIALS

# shellcheck source=lib/reporting.sh
source "$SCRIPT_DIR/lib/reporting.sh"

section() {
  printf '\n== %s ==\n' "$1"
}

note() {
  printf '%s\n' "$*"
}

die() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    die "required command not found: $1"
  fi
}

run_with_timeout() {
  if [[ "$HARNESS_COMMAND_TIMEOUT_SECONDS" -eq 0 ]]; then
    "$@"
    return
  fi

  if command -v timeout >/dev/null 2>&1; then
    timeout "$HARNESS_COMMAND_TIMEOUT_SECONDS" "$@"
  elif command -v gtimeout >/dev/null 2>&1; then
    gtimeout "$HARNESS_COMMAND_TIMEOUT_SECONDS" "$@"
  else
    die "HARNESS_COMMAND_TIMEOUT_SECONDS requires timeout or gtimeout"
  fi
}

run() {
  printf '+ '
  printf '%q ' "$@"
  printf '\n'
  run_with_timeout "$@"
}

run_in() {
  local directory="$1"
  shift
  (
    cd "$directory"
    run "$@"
  )
}

run_shell() {
  local command_text="$1"
  printf '+ (cd %q && bash -o pipefail -c %q)\n' "$ROOT_DIR" "$command_text"
  (
    cd "$ROOT_DIR"
    run_with_timeout bash -o pipefail -c "$command_text"
  )
}

is_true() {
  case "$1" in
    true|TRUE|yes|YES|1) return 0 ;;
    *) return 1 ;;
  esac
}

validate_boolean() {
  case "$2" in
    true|TRUE|false|FALSE|yes|YES|no|NO|1|0) ;;
    *) die "$1 must be true or false, got: $2" ;;
  esac
}

stage_is_required() {
  local normalized="${HARNESS_REQUIRED_STAGES//[[:space:]]/}"
  case ",$normalized," in
    *",$1,"*) return 0 ;;
    *) return 1 ;;
  esac
}

report_skip() {
  local stage="$1"
  local reason="$2"
  if is_true "$HARNESS_FAIL_ON_SKIP" || stage_is_required "$stage"; then
    printf 'error: %s is required but was skipped: %s\n' "$stage" "$reason" >&2
    return 1
  fi
  note "skip: $reason"
  return 0
}

csv_contains() {
  local values="${1//[[:space:]]/}"
  case ",$values," in
    *",$2,"*) return 0 ;;
    *) return 1 ;;
  esac
}

stage_network_allowed() {
  local stage="$1"
  case "$HARNESS_NETWORK_POLICY" in
    allow) return 0 ;;
    deny) return 1 ;;
    declared) csv_contains "$HARNESS_NETWORK_STAGES" "$stage" ;;
  esac
}

stage_network_guard() {
  local stage="$1"
  if [[ "$HARNESS_NETWORK_POLICY" == "deny" ]] \
    && csv_contains "$HARNESS_NETWORK_STAGES" "$stage"; then
    printf 'error: %s is declared network-dependent but HARNESS_NETWORK_POLICY=deny\n' "$stage" >&2
    return 3
  fi
  return 0
}

validate_required_stages() {
  [[ -n "$HARNESS_REQUIRED_STAGES" ]] || return 0
  [[ "$HARNESS_REQUIRED_STAGES" == "none" ]] && return 0
  local stages=()
  local stage
  IFS=',' read -r -a stages <<<"$HARNESS_REQUIRED_STAGES"
  for stage in "${stages[@]}"; do
    stage="${stage//[[:space:]]/}"
    case "$stage" in
      lint|format|typecheck|test|coverage|build|integration|e2e|security|deploy|ci-policy|supply-chain) ;;
      *) die "unknown stage in HARNESS_REQUIRED_STAGES: $stage" ;;
    esac
  done
}

validate_network_stages() {
  [[ -n "$HARNESS_NETWORK_STAGES" ]] || return 0
  local stages=()
  local stage
  IFS=',' read -r -a stages <<<"$HARNESS_NETWORK_STAGES"
  for stage in "${stages[@]}"; do
    stage="${stage//[[:space:]]/}"
    case "$stage" in
      lint|format|typecheck|test|coverage|build|integration|e2e|security|deploy|ci-policy|supply-chain|agent-regression|agent-capability) ;;
      *) die "unknown stage in HARNESS_NETWORK_STAGES: $stage" ;;
    esac
  done
}

validate_settings() {
  case "$HARNESS_GOVERNANCE_MODE" in
    auto|required|off) ;;
    *) die "HARNESS_GOVERNANCE_MODE must be auto, required, or off" ;;
  esac
  validate_boolean HARNESS_REQUIRE_PLAN_APPROVAL "$HARNESS_REQUIRE_PLAN_APPROVAL"
  validate_boolean HARNESS_STRICT_REPO_HYGIENE "$HARNESS_STRICT_REPO_HYGIENE"
  validate_boolean HARNESS_FAIL_ON_SKIP "$HARNESS_FAIL_ON_SKIP"
  validate_boolean HARNESS_ALLOW_LEGACY_CHANGE_DISCOVERY "$HARNESS_ALLOW_LEGACY_CHANGE_DISCOVERY"
  validate_boolean HARNESS_ALLOW_LEGACY_APPROVAL_FORMAT "$HARNESS_ALLOW_LEGACY_APPROVAL_FORMAT"
  validate_boolean HARNESS_WRITE_REPORTS "$HARNESS_WRITE_REPORTS"
  validate_required_stages
  validate_network_stages
  if [[ ! "$HARNESS_COVERAGE_THRESHOLD" =~ ^[0-9]+([.][0-9]+)?$ ]]; then
    die "HARNESS_COVERAGE_THRESHOLD must be numeric"
  fi
  if [[ ! "$HARNESS_COMMAND_TIMEOUT_SECONDS" =~ ^[0-9]+$ ]]; then
    die "HARNESS_COMMAND_TIMEOUT_SECONDS must be a non-negative integer"
  fi
  if [[ ! "$HARNESS_AGENT_TRIALS" =~ ^[1-9][0-9]*$ ]]; then
    die "HARNESS_AGENT_TRIALS must be a positive integer"
  fi
  case "$HARNESS_NETWORK_POLICY" in
    deny|declared|allow) ;;
    *) die "HARNESS_NETWORK_POLICY must be deny, declared, or allow" ;;
  esac
  if [[ -n "$HARNESS_EXPECTED_BASELINE_VERSION" && "$HARNESS_EXPECTED_BASELINE_VERSION" != "$PORTABLE_HARNESS_VERSION" ]]; then
    die "harness baseline mismatch: expected $HARNESS_EXPECTED_BASELINE_VERSION, found $PORTABLE_HARNESS_VERSION"
  fi
  if [[ -n "$HARNESS_EXPECTED_BASELINE_VERSION" && -d "$ROOT_DIR" ]]; then
    [[ -f "$ROOT_DIR/AGENTS.md" ]] || die "AGENTS.md is required by the shared baseline version contract"
    if ! grep -Fq "Shared baseline version: **$HARNESS_EXPECTED_BASELINE_VERSION**" "$ROOT_DIR/AGENTS.md"; then
      die "AGENTS.md baseline version does not match $HARNESS_EXPECTED_BASELINE_VERSION"
    fi
  fi
}

is_ignored_path() {
  case "$1" in
    */.git/*|*/node_modules/*|*/vendor/*|*/.venv/*|*/venv/*|*/dist/*|*/build/*|*/.next/*)
      return 0
      ;;
    *) return 1 ;;
  esac
}

environment_doctor() {
  section "Environment doctor"
  note "os: $(uname -srm 2>/dev/null || uname -a)"
  note "bash: $BASH_VERSION"
  note "network policy: $HARNESS_NETWORK_POLICY"
  note "network-declared stages: ${HARNESS_NETWORK_STAGES:-none}"
  if [[ "$HARNESS_COMMAND_TIMEOUT_SECONDS" -eq 0 ]]; then
    note "command timeout: disabled"
  else
    note "command timeout: ${HARNESS_COMMAND_TIMEOUT_SECONDS}s"
    if ! command -v timeout >/dev/null 2>&1 && ! command -v gtimeout >/dev/null 2>&1; then
      die "configured command timeout requires timeout or gtimeout"
    fi
  fi

  if [[ -n "$HARNESS_REQUIRED_TOOLS" ]]; then
    local tools=()
    local tool
    IFS=',' read -r -a tools <<<"$HARNESS_REQUIRED_TOOLS"
    for tool in "${tools[@]}"; do
      tool="${tool//[[:space:]]/}"
      [[ -n "$tool" ]] || continue
      require_command "$tool"
      note "ok: required tool $tool"
    done
  fi

  if [[ -n "$HARNESS_ENVIRONMENT_COMMAND" ]]; then
    run_shell "$HARNESS_ENVIRONMENT_COMMAND"
  fi
}

doctor() {
  section "Harness doctor"

  [[ -d "$ROOT_DIR" ]] || die "repository root does not exist: $ROOT_DIR"

  note "root: $ROOT_DIR"
  note "project: ${HARNESS_PROJECT_NAME:-$(basename "$ROOT_DIR")}" 
  if [[ -f "$CONFIG_FILE" ]]; then
    note "config: $CONFIG_FILE"
  else
    note "config: automatic detection (no HARNESS/config.env)"
  fi
  note "governance: $HARNESS_GOVERNANCE_MODE"
  note "harness version: $PORTABLE_HARNESS_VERSION"
  note "coverage threshold: $HARNESS_COVERAGE_THRESHOLD%"
  note "evidence directory: $HARNESS_REPORT_DIR"
  if [[ -n "$HARNESS_REQUIRED_STAGES" ]]; then
    note "required stages: $HARNESS_REQUIRED_STAGES"
  else
    note "warning: no required stages declared; skipped stages will not fail"
  fi

  if [[ -n "$HARNESS_REQUIRED_PATHS" ]]; then
    local required_paths=()
    local required_path
    IFS=':' read -r -a required_paths <<<"$HARNESS_REQUIRED_PATHS"
    for required_path in "${required_paths[@]}"; do
      [[ -e "$ROOT_DIR/$required_path" ]] || die "missing required path: $required_path"
      note "ok: required path $required_path"
    done
  fi

  local detected=""
  [[ -f "$ROOT_DIR/package.json" ]] && detected="${detected} Node.js"
  [[ -f "$ROOT_DIR/pyproject.toml" || -f "$ROOT_DIR/requirements.txt" ]] && detected="${detected} Python"
  [[ -f "$ROOT_DIR/go.mod" ]] && detected="${detected} Go"
  [[ -f "$ROOT_DIR/Cargo.toml" ]] && detected="${detected} Rust"
  [[ -f "$ROOT_DIR/Makefile" || -f "$ROOT_DIR/makefile" ]] && detected="${detected} Make"
  note "detected:${detected:- none at repository root}"
  environment_doctor
}

# shellcheck source=lib/governance.sh
source "$SCRIPT_DIR/lib/governance.sh"
# shellcheck source=lib/adapters.sh
source "$SCRIPT_DIR/lib/adapters.sh"
execute_stage() {
  local stage="$1"
  local title="$2"
  local variable_name="$3"
  shift 3
  local configured="${!variable_name:-}"
  local network_allowed="false"
  local report_command="auto-detect:$stage"
  local status=0

  if stage_network_allowed "$stage"; then
    network_allowed="true"
  fi
  export HARNESS_NETWORK_ALLOWED="$network_allowed"
  [[ -n "$configured" ]] && report_command="$configured"

  section "$title"
  harness_report_stage_begin "$stage" "$report_command" "$network_allowed"
  if stage_network_guard "$stage"; then
    :
  else
    status=$?
    harness_report_stage_finish blocked "$status" "network policy blocked the stage"
    return "$status"
  fi

  if [[ -n "$configured" ]]; then
    if [[ "$configured" == "skip" ]]; then
      if report_skip "$stage" "explicitly disabled by $variable_name"; then
        harness_report_stage_finish skipped 0 "explicitly disabled by $variable_name"
        return 0
      fi
      harness_report_stage_finish failed 1 "required stage was explicitly disabled"
      return 1
    fi
    if run_shell "$configured"; then
      harness_report_stage_finish passed 0 "configured command"
      return 0
    else
      status=$?
    fi
    if [[ "$status" -eq 3 ]]; then
      harness_report_stage_finish blocked "$status" "configured command reported blocked/unavailable"
    else
      harness_report_stage_finish failed "$status" "configured command failed"
    fi
    return "$status"
  fi

  STAGE_RAN=0
  run_make_stage "$@"
  if [[ "$RUNNER_FOUND" -eq 1 ]]; then
    harness_report_stage_finish passed 0 "detected Make target"
    return 0
  fi
  run_node_stage "$@"
  [[ "$RUNNER_FOUND" -eq 1 ]] && STAGE_RAN=1
  run_python_stage "$stage"
  [[ "$RUNNER_FOUND" -eq 1 ]] && STAGE_RAN=1
  run_compiled_stage "$stage"
  [[ "$RUNNER_FOUND" -eq 1 ]] && STAGE_RAN=1

  if [[ "$STAGE_RAN" -eq 0 ]]; then
    if report_skip "$stage" "no configured or detected $stage command"; then
      harness_report_stage_finish skipped 0 "no configured or detected command"
      return 0
    fi
    harness_report_stage_finish failed 1 "required stage has no implementation"
    return 1
  fi
  harness_report_stage_finish passed 0 "auto-detected command"
}

lint_stage() {
  execute_stage lint "Lint" HARNESS_LINT_COMMAND lint
}

format_stage() {
  execute_stage format "Format check" HARNESS_FORMAT_COMMAND format-check format:check check-format check:format
}

typecheck_stage() {
  execute_stage typecheck "Type check" HARNESS_TYPECHECK_COMMAND typecheck type-check check-types check:types
}

test_stage() {
  execute_stage test "Tests" HARNESS_TEST_COMMAND test
}

coverage_stage() {
  execute_stage coverage "Coverage" HARNESS_COVERAGE_COMMAND test-coverage test:coverage coverage
}

build_stage() {
  execute_stage build "Build" HARNESS_BUILD_COMMAND build
}

integration_stage() {
  execute_stage integration "Integration tests" HARNESS_INTEGRATION_COMMAND test-integration test:integration integration-test integration:test
}

e2e_stage() {
  execute_stage e2e "End-to-end tests" HARNESS_E2E_COMMAND test-e2e test:e2e e2e
}

security_stage() {
  execute_stage security "Security" HARNESS_SECURITY_COMMAND security security-check security:check audit
}

deploy_stage() {
  execute_stage deploy "Deployment check" HARNESS_DEPLOY_COMMAND deploy-check deploy:check deploy-validate deploy:validate
}

ci_policy_stage() {
  execute_stage ci-policy "CI policy" HARNESS_CI_POLICY_COMMAND ci-policy ci:policy
}

supply_chain_stage() {
  execute_stage supply-chain "Supply-chain evidence" HARNESS_SUPPLY_CHAIN_COMMAND supply-chain supply:chain provenance sbom
}

configured_agent_stage() {
  local stage="$1"
  local title="$2"
  local variable_name="$3"
  local required="$4"
  local configured="${!variable_name:-}"
  local network_allowed="false"
  local status=0

  if stage_network_allowed "$stage"; then
    network_allowed="true"
  fi
  export HARNESS_NETWORK_ALLOWED="$network_allowed"

  section "$title"
  harness_report_stage_begin "$stage" "${configured:-unconfigured}" "$network_allowed"
  if stage_network_guard "$stage"; then
    :
  else
    status=$?
    harness_report_stage_finish blocked "$status" "network policy blocked the stage"
    return "$status"
  fi

  if [[ -z "$configured" || "$configured" == "skip" ]]; then
    if [[ "$required" == "true" ]]; then
      printf 'error: %s requires %s\n' "$stage" "$variable_name" >&2
      harness_report_stage_finish failed 1 "required agent evaluation command is missing"
      return 1
    fi
    note "skip: $stage is not configured"
    harness_report_stage_finish skipped 0 "optional agent evaluation command is not configured"
    return 0
  fi

  if run_shell "$configured"; then
    harness_report_stage_finish passed 0 "configured agent evaluation command"
    return 0
  else
    status=$?
  fi
  if [[ "$status" -eq 3 ]]; then
    harness_report_stage_finish blocked "$status" "agent evaluation command reported blocked/unavailable"
  else
    harness_report_stage_finish failed "$status" "agent evaluation command failed"
  fi
  return "$status"
}

agent_eval_directory() {
  case "$HARNESS_AGENT_EVAL_DIR" in
    /*) printf '%s\n' "$HARNESS_AGENT_EVAL_DIR" ;;
    *) printf '%s/%s\n' "$ROOT_DIR" "$HARNESS_AGENT_EVAL_DIR" ;;
  esac
}

agent_eval_doctor() {
  section "Agent evaluation doctor"
  local eval_directory case_count
  eval_directory="$(agent_eval_directory)"
  [[ -d "$eval_directory" ]] || die "agent evaluation directory does not exist: $eval_directory"
  case_count="$(find "$eval_directory" -maxdepth 1 -type f \( -name '*.yaml' -o -name '*.yml' -o -name '*.json' \) | awk 'END { print NR+0 }')"
  [[ "$case_count" -gt 0 ]] || die "agent evaluation directory has no .yaml, .yml, or .json cases: $eval_directory"
  [[ -n "$HARNESS_AGENT_REGRESSION_COMMAND" && "$HARNESS_AGENT_REGRESSION_COMMAND" != "skip" ]] \
    || die "HARNESS_AGENT_REGRESSION_COMMAND is required for agent:eval"
  note "cases: $case_count"
  note "trials: $HARNESS_AGENT_TRIALS"
  note "regression runner: configured"
  if [[ -n "$HARNESS_AGENT_CAPABILITY_COMMAND" && "$HARNESS_AGENT_CAPABILITY_COMMAND" != "skip" ]]; then
    note "capability runner: configured"
  else
    note "capability runner: optional and not configured"
  fi
}

agent_regression_stage() {
  configured_agent_stage agent-regression "Agent regression evaluation" HARNESS_AGENT_REGRESSION_COMMAND true
}

agent_capability_stage() {
  configured_agent_stage agent-capability "Agent capability evaluation" HARNESS_AGENT_CAPABILITY_COMMAND false
}

reported_internal_check() {
  local stage="$1"
  local command_text="$2"
  shift 2
  HARNESS_INTERNAL_CHECK_STATUS="passed"
  HARNESS_INTERNAL_CHECK_DETAIL=""
  harness_report_stage_begin "$stage" "$command_text" false
  "$@"
  harness_report_stage_finish "$HARNESS_INTERNAL_CHECK_STATUS" 0 "$HARNESS_INTERNAL_CHECK_DETAIL"
}

agent_eval() {
  harness_report_init agent-eval
  reported_internal_check agent-eval-doctor internal:agent-eval-doctor agent_eval_doctor
  agent_regression_stage
  agent_capability_stage
}

verify() {
  harness_report_init verify
  reported_internal_check doctor internal:doctor doctor
  [[ -n "$HARNESS_REQUIRED_STAGES" ]] || die "verify requires HARNESS_REQUIRED_STAGES; use 'none' only for an intentional no-gate project"
  reported_internal_check repo-doctor internal:repo-doctor repo_doctor
  reported_internal_check governance-doctor internal:governance-doctor governance_doctor
  lint_stage
  format_stage
  typecheck_stage
  test_stage
  coverage_stage
  build_stage
  stage_is_required integration && integration_stage
  stage_is_required e2e && e2e_stage
  stage_is_required ci-policy && ci_policy_stage
  stage_is_required supply-chain && supply_chain_stage
  stage_is_required security && security_stage
  stage_is_required deploy && deploy_stage
  return 0
}

usage() {
  cat <<'EOF'
Usage: ./HARNESS/harness.sh [command]

Commands:
  doctor                 Validate configuration and required paths
  environment:doctor     Show and validate the execution environment
  repo:doctor            Inspect tracked generated and secret-like files
  governance:doctor      Validate optional governance traceability
  governance:approved    Validate active-change approval records
  lint                   Run lint checks
  format:check           Check formatting without rewriting files
  typecheck              Run type checks
  test                   Run unit/component tests
  coverage               Run the configured coverage threshold gate
  build                  Build project artifacts
  integration            Run real integration checks
  e2e                    Run end-to-end checks
  security               Run project security checks
  ci:policy              Run the configured CI workflow policy check
  supply-chain           Run the configured SBOM/provenance gate
  deploy:check           Validate deployment artifacts/configuration
  verify                 Run the deterministic handoff gate (default)
  all                    Alias for verify
  agent:eval:doctor      Validate the agent evaluation dataset and runner
  agent:regression       Run required agent regression evaluations
  agent:capability       Run the optional agent capability benchmark
  agent:eval             Run configured agent regression and capability suites
  selftest               Run isolated harness fixtures
  version                Show the portable baseline version
  help                   Show this help
EOF
}

command_name="${1:-verify}"
case "$command_name" in
  version|help|-h|--help|selftest) ;;
  *) validate_settings ;;
esac
case "$command_name" in
  doctor) doctor ;;
  environment:doctor) environment_doctor ;;
  repo:doctor) repo_doctor ;;
  governance:doctor) governance_doctor ;;
  governance:approved) governance_approved ;;
  lint) lint_stage ;;
  format:check) format_stage ;;
  typecheck) typecheck_stage ;;
  test) test_stage ;;
  coverage) coverage_stage ;;
  build) build_stage ;;
  integration) integration_stage ;;
  e2e) e2e_stage ;;
  security) security_stage ;;
  ci:policy) ci_policy_stage ;;
  supply-chain) supply_chain_stage ;;
  deploy:check) deploy_stage ;;
  verify|all) verify ;;
  agent:eval:doctor) agent_eval_doctor ;;
  agent:regression) agent_regression_stage ;;
  agent:capability) agent_capability_stage ;;
  agent:eval) agent_eval ;;
  selftest) run "$SCRIPT_DIR/tests/selftest.sh" ;;
  version) printf '%s\n' "$PORTABLE_HARNESS_VERSION" ;;
  help|-h|--help) usage ;;
  *)
    printf 'Unknown harness command: %s\n' "$command_name" >&2
    usage >&2
    exit 2
    ;;
esac
