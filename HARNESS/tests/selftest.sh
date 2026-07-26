#!/usr/bin/env bash
set -euo pipefail

HARNESS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
HARNESS="$HARNESS_DIR/harness.sh"
TEST_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/portable-harness.XXXXXX")"
trap 'rm -rf "$TEST_ROOT"' EXIT
EMPTY_CONFIG="$TEST_ROOT/empty-config.env"
touch "$EMPTY_CONFIG"

expect_failure() {
  if "$@" >/dev/null 2>&1; then
    echo "Expected command to fail: $*" >&2
    exit 1
  fi
}

write_approved_plan() {
  cat >"$1/DEVELOPMENT_PLAN.md" <<'EOF'
# Development Plan
## CHG-101 Portable governance
- Gate 2 Status: complete
- Gate 4 Status: approved
- Approved By: Example Maintainer
- Approved On: 2026-01-02
EOF
}

write_pending_plan() {
  cat >"$1/DEVELOPMENT_PLAN.md" <<'EOF'
# Development Plan
## CHG-101 Portable governance
- Gate 2 Status: complete
- Gate 4 Status: pending
- Approved By: Example Maintainer
- Approved On: 2026-01-02
EOF
}

bash -n \
  "$HARNESS" \
  "$HARNESS_DIR/lib/adapters.sh" \
  "$HARNESS_DIR/lib/governance.sh" \
  "$HARNESS_DIR/lib/reporting.sh" \
  "${BASH_SOURCE[0]}"

EMPTY_ROOT="$TEST_ROOT/empty"
mkdir -p "$EMPTY_ROOT"
HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" "$HARNESS" doctor >/dev/null
HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" "$HARNESS" governance:doctor >/dev/null
expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" "$HARNESS" verify
expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" HARNESS_REQUIRED_STAGES=test "$HARNESS" test
expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" HARNESS_FAIL_ON_SKIP=true "$HARNESS" lint
expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" HARNESS_EXPECTED_BASELINE_VERSION=9.9.9 "$HARNESS" doctor
expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" HARNESS_REQUIRED_STAGES=unknown "$HARNESS" doctor
expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" HARNESS_NETWORK_STAGES=unknown "$HARNESS" doctor
expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" HARNESS_COMMAND_TIMEOUT_SECONDS=invalid "$HARNESS" doctor
expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" HARNESS_REQUIRED_TOOLS=portable-harness-tool-that-does-not-exist "$HARNESS" doctor
if ! command -v timeout >/dev/null 2>&1 && ! command -v gtimeout >/dev/null 2>&1; then
  expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" HARNESS_COMMAND_TIMEOUT_SECONDS=1 "$HARNESS" doctor
fi
HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" HARNESS_REQUIRED_STAGES=none "$HARNESS" verify >/dev/null
[[ -f "$EMPTY_ROOT/test-results/harness/verify.json" ]] || { echo "Verify evidence was not written." >&2; exit 1; }
grep -Fq '"result": "passed"' "$EMPTY_ROOT/test-results/harness/verify.json"
grep -Fq '"stage":"doctor"' "$EMPTY_ROOT/test-results/harness/verify.json"
grep -Fq '"stage":"repo-doctor","status":"skipped"' "$EMPTY_ROOT/test-results/harness/verify.json"
grep -Fq '"stage":"governance-doctor","status":"skipped"' "$EMPTY_ROOT/test-results/harness/verify.json"
grep -Fq '"toolchain": [' "$EMPTY_ROOT/test-results/harness/verify.json"

FAIL_REPORT_ROOT="$TEST_ROOT/failure-report"
mkdir -p "$FAIL_REPORT_ROOT"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$FAIL_REPORT_ROOT" \
  HARNESS_REQUIRED_STAGES=test \
  HARNESS_TEST_COMMAND=false \
  "$HARNESS" verify
[[ -f "$FAIL_REPORT_ROOT/test-results/harness/verify.json" ]] || { echo "Failure evidence was not written." >&2; exit 1; }
grep -Fq '"result": "failed"' "$FAIL_REPORT_ROOT/test-results/harness/verify.json"
grep -Fq '"stage":"test","status":"failed"' "$FAIL_REPORT_ROOT/test-results/harness/verify.json"

BASELINE_ROOT="$TEST_ROOT/baseline"
mkdir -p "$BASELINE_ROOT"
printf '# AGENTS.md\n\nShared baseline version: **0.9.0**\n' >"$BASELINE_ROOT/AGENTS.md"
expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$BASELINE_ROOT" HARNESS_EXPECTED_BASELINE_VERSION=1.1.0 "$HARNESS" doctor
printf '# AGENTS.md\n\nShared baseline version: **1.1.0**\n' >"$BASELINE_ROOT/AGENTS.md"
HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$BASELINE_ROOT" HARNESS_EXPECTED_BASELINE_VERSION=1.1.0 "$HARNESS" doctor >/dev/null

EXAMPLE_CONFIG_ROOT="$TEST_ROOT/example-config"
mkdir -p "$EXAMPLE_CONFIG_ROOT"
printf '# AGENTS.md\n\nShared baseline version: **1.1.0**\n' >"$EXAMPLE_CONFIG_ROOT/AGENTS.md"
printf '# Project Context\n' >"$EXAMPLE_CONFIG_ROOT/PROJECT.md"
HARNESS_CONFIG_FILE="$HARNESS_DIR/config.env.example" HARNESS_ROOT_DIR="$EXAMPLE_CONFIG_ROOT" "$HARNESS" doctor >/dev/null

CONFIG_ROOT="$TEST_ROOT/configured"
mkdir -p "$CONFIG_ROOT"
touch "$CONFIG_ROOT/expected"
HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
HARNESS_ROOT_DIR="$CONFIG_ROOT" \
HARNESS_REQUIRED_STAGES=test,integration,ci-policy,supply-chain \
HARNESS_TEST_COMMAND='test "$HARNESS_NETWORK_ALLOWED" = false && test -f expected' \
HARNESS_INTEGRATION_COMMAND='test "$HARNESS_NETWORK_ALLOWED" = true && test -f expected && touch integration-ran' \
HARNESS_CI_POLICY_COMMAND='test -f expected && touch ci-policy-ran' \
HARNESS_SUPPLY_CHAIN_COMMAND='test -f expected && touch supply-chain-ran' \
"$HARNESS" verify >/dev/null
[[ -f "$CONFIG_ROOT/integration-ran" ]] || { echo "Required integration stage did not run." >&2; exit 1; }
[[ -f "$CONFIG_ROOT/ci-policy-ran" ]] || { echo "Required CI policy stage did not run." >&2; exit 1; }
[[ -f "$CONFIG_ROOT/supply-chain-ran" ]] || { echo "Required supply-chain stage did not run." >&2; exit 1; }

EVAL_ROOT="$TEST_ROOT/agent-eval"
mkdir -p "$EVAL_ROOT/HARNESS/evals"
printf 'id: selftest-agent-eval\nsuite: regression\n' >"$EVAL_ROOT/HARNESS/evals/case.yaml"
HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
HARNESS_ROOT_DIR="$EVAL_ROOT" \
HARNESS_AGENT_REGRESSION_COMMAND='test "$HARNESS_NETWORK_ALLOWED" = true && test -f HARNESS/evals/case.yaml && touch regression-ran' \
HARNESS_AGENT_CAPABILITY_COMMAND='test "$HARNESS_NETWORK_ALLOWED" = true && touch capability-ran' \
HARNESS_AGENT_TRIALS=3 \
"$HARNESS" agent:eval >/dev/null
[[ -f "$EVAL_ROOT/regression-ran" ]] || { echo "Agent regression suite did not run." >&2; exit 1; }
[[ -f "$EVAL_ROOT/capability-ran" ]] || { echo "Agent capability suite did not run." >&2; exit 1; }
[[ -f "$EVAL_ROOT/test-results/harness/agent-eval.json" ]] || { echo "Agent eval evidence was not written." >&2; exit 1; }
grep -Fq '"stage":"agent-regression","status":"passed"' "$EVAL_ROOT/test-results/harness/agent-eval.json"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EVAL_ROOT" \
  HARNESS_NETWORK_POLICY=deny \
  HARNESS_AGENT_REGRESSION_COMMAND=true \
  "$HARNESS" agent:regression
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EVAL_ROOT" \
  HARNESS_NETWORK_POLICY=deny \
  HARNESS_AGENT_REGRESSION_COMMAND=true \
  "$HARNESS" agent:eval
grep -Fq '"stage":"agent-regression","status":"blocked","exit_code":3' "$EVAL_ROOT/test-results/harness/agent-eval.json"

if command -v node >/dev/null 2>&1 && command -v npm >/dev/null 2>&1; then
  NODE_ROOT="$TEST_ROOT/node"
  mkdir -p "$NODE_ROOT"
  cat >"$NODE_ROOT/package.json" <<'EOF'
{
  "private": true,
  "scripts": {
    "test": "node -e \"process.exit(0)\""
  }
}
EOF
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$NODE_ROOT" HARNESS_REQUIRED_STAGES=test "$HARNESS" test >/dev/null

  cat >"$NODE_ROOT/package.json" <<'EOF'
{
  "private": true,
  "scripts": {
    "test": "node -e \"process.exit(9)\""
  }
}
EOF
  expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$NODE_ROOT" HARNESS_REQUIRED_STAGES=test "$HARNESS" test

  NESTED_NODE_ROOT="$TEST_ROOT/nested-node"
  mkdir -p "$NESTED_NODE_ROOT/apps/web"
  cat >"$NESTED_NODE_ROOT/apps/web/package.json" <<'EOF'
{
  "private": true,
  "scripts": {
    "test": "node -e \"process.exit(0)\""
  }
}
EOF
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$NESTED_NODE_ROOT" HARNESS_REQUIRED_STAGES=test "$HARNESS" test >/dev/null

  MANAGER_ROOT="$TEST_ROOT/package-manager"
  FAKE_NODE_BIN="$TEST_ROOT/fake-node-bin"
  mkdir -p "$MANAGER_ROOT" "$FAKE_NODE_BIN"
  cat >"$MANAGER_ROOT/package.json" <<'EOF'
{
  "private": true,
  "packageManager": "pnpm@9.0.0",
  "scripts": {
    "test": "unused"
  }
}
EOF
  cat >"$FAKE_NODE_BIN/pnpm" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
  cat >"$FAKE_NODE_BIN/npm" <<'EOF'
#!/usr/bin/env bash
exit 9
EOF
  chmod +x "$FAKE_NODE_BIN/pnpm" "$FAKE_NODE_BIN/npm"
  PATH="$FAKE_NODE_BIN:$PATH" HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$MANAGER_ROOT" HARNESS_REQUIRED_STAGES=test "$HARNESS" test >/dev/null
fi

if command -v make >/dev/null 2>&1; then
  MAKE_ROOT="$TEST_ROOT/make"
  mkdir -p "$MAKE_ROOT"
  cat >"$MAKE_ROOT/Makefile" <<'EOF'
test:
	@true
EOF
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$MAKE_ROOT" HARNESS_REQUIRED_STAGES=test "$HARNESS" test >/dev/null

  cat >"$MAKE_ROOT/Makefile" <<'EOF'
test:
	@exit 7
EOF
  expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$MAKE_ROOT" HARNESS_REQUIRED_STAGES=test "$HARNESS" test
fi

PYTHON_ROOT="$TEST_ROOT/python"
mkdir -p "$PYTHON_ROOT/tests" "$PYTHON_ROOT/.venv/bin"
cat >"$PYTHON_ROOT/pyproject.toml" <<'EOF'
[project]
name = "harness-selftest"
version = "0.0.0"
EOF
cat >"$PYTHON_ROOT/.venv/bin/python" <<'EOF'
#!/usr/bin/env bash
if [[ "$*" == "-m pytest" ]]; then
  exit "${FAKE_PYTHON_STATUS:-0}"
fi
exit 8
EOF
chmod +x "$PYTHON_ROOT/.venv/bin/python"
HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$PYTHON_ROOT" HARNESS_REQUIRED_STAGES=test "$HARNESS" test >/dev/null
expect_failure env FAKE_PYTHON_STATUS=7 HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$PYTHON_ROOT" HARNESS_REQUIRED_STAGES=test "$HARNESS" test

COMPILED_BIN="$TEST_ROOT/compiled-bin"
mkdir -p "$COMPILED_BIN"
cat >"$COMPILED_BIN/go" <<'EOF'
#!/usr/bin/env bash
exit "${FAKE_GO_STATUS:-0}"
EOF
cat >"$COMPILED_BIN/cargo" <<'EOF'
#!/usr/bin/env bash
exit "${FAKE_CARGO_STATUS:-0}"
EOF
chmod +x "$COMPILED_BIN/go" "$COMPILED_BIN/cargo"

GO_ROOT="$TEST_ROOT/go"
mkdir -p "$GO_ROOT"
printf 'module example.invalid/selftest\n' >"$GO_ROOT/go.mod"
PATH="$COMPILED_BIN:$PATH" HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$GO_ROOT" HARNESS_REQUIRED_STAGES=test "$HARNESS" test >/dev/null
expect_failure env PATH="$COMPILED_BIN:$PATH" FAKE_GO_STATUS=7 HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$GO_ROOT" HARNESS_REQUIRED_STAGES=test "$HARNESS" test

RUST_ROOT="$TEST_ROOT/rust"
mkdir -p "$RUST_ROOT"
cat >"$RUST_ROOT/Cargo.toml" <<'EOF'
[package]
name = "harness-selftest"
version = "0.0.0"
EOF
PATH="$COMPILED_BIN:$PATH" HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$RUST_ROOT" HARNESS_REQUIRED_STAGES=test "$HARNESS" test >/dev/null
expect_failure env PATH="$COMPILED_BIN:$PATH" FAKE_CARGO_STATUS=7 HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$RUST_ROOT" HARNESS_REQUIRED_STAGES=test "$HARNESS" test

GOV_ROOT="$TEST_ROOT/governed"
mkdir -p "$GOV_ROOT"
cat >"$GOV_ROOT/SPECIFICATION.md" <<'EOF'
# Specification
## CHG-101 Portable governance
EOF
cat >"$GOV_ROOT/SPEC_CHANGELOG.md" <<'EOF'
# Specification Changelog
Historical example: CHG-999
Active Change: CHG-101
| Date | Change ID | Status |
|---|---|---|
| 2026-01-02 | CHG-101 | Gate 4 approved |
EOF
write_approved_plan "$GOV_ROOT"
cat >"$GOV_ROOT/TEST_PLAN.md" <<'EOF'
# Test Plan
CHG-101 is covered by the harness self-test.
EOF
cat >"$GOV_ROOT/TRACEABILITY.md" <<'EOF'
| Requirement | Change | Test | Status |
|---|---|---|---|
| GOV-101 | CHG-101 | selftest | Pass |
EOF

HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
HARNESS_ROOT_DIR="$GOV_ROOT" \
HARNESS_GOVERNANCE_MODE=required \
HARNESS_REQUIRE_PLAN_APPROVAL=true \
"$HARNESS" governance:doctor >/dev/null

write_pending_plan "$GOV_ROOT"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$GOV_ROOT" \
  HARNESS_GOVERNANCE_MODE=required \
  HARNESS_REQUIRE_PLAN_APPROVAL=true \
  "$HARNESS" governance:doctor

write_approved_plan "$GOV_ROOT"
sed 's/CHG-101/CHG-102/g' "$GOV_ROOT/TRACEABILITY.md" >"$GOV_ROOT/TRACEABILITY.wrong"
mv "$GOV_ROOT/TRACEABILITY.wrong" "$GOV_ROOT/TRACEABILITY.md"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$GOV_ROOT" \
  HARNESS_GOVERNANCE_MODE=required \
  "$HARNESS" governance:doctor

TEMPLATE_GOV_ROOT="$TEST_ROOT/template-governance"
TEMPLATE_GOV_DIR="$HARNESS_DIR/templates/governance"
mkdir -p "$TEMPLATE_GOV_ROOT/HARNESS"
cp "$TEMPLATE_GOV_DIR/ACTIVE_CHANGE.example" "$TEMPLATE_GOV_ROOT/HARNESS/ACTIVE_CHANGE"
cp "$TEMPLATE_GOV_DIR/SPECIFICATION.md.example" "$TEMPLATE_GOV_ROOT/SPECIFICATION.md"
cp "$TEMPLATE_GOV_DIR/SPEC_CHANGELOG.md.example" "$TEMPLATE_GOV_ROOT/SPEC_CHANGELOG.md"
cp "$TEMPLATE_GOV_DIR/TEST_PLAN.md.example" "$TEMPLATE_GOV_ROOT/TEST_PLAN.md"
cp "$TEMPLATE_GOV_DIR/TRACEABILITY.md.example" "$TEMPLATE_GOV_ROOT/TRACEABILITY.md"
sed \
  -e 's/Gate 2 Status: pending/Gate 2 Status: complete/' \
  -e 's/Gate 4 Status: pending/Gate 4 Status: approved/' \
  -e 's/Approved By: pending/Approved By: Example Maintainer/' \
  -e 's/Approved On: pending/Approved On: 2026-01-02/' \
  "$TEMPLATE_GOV_DIR/DEVELOPMENT_PLAN.md.example" >"$TEMPLATE_GOV_ROOT/DEVELOPMENT_PLAN.md"
HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
HARNESS_ROOT_DIR="$TEMPLATE_GOV_ROOT" \
HARNESS_GOVERNANCE_MODE=required \
HARNESS_REQUIRE_PLAN_APPROVAL=true \
"$HARNESS" governance:doctor >/dev/null

PARTIAL_GOV_ROOT="$TEST_ROOT/partial-governance"
mkdir -p "$PARTIAL_GOV_ROOT"
printf '# Specification\nCHG-101\n' >"$PARTIAL_GOV_ROOT/SPECIFICATION.md"
expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$PARTIAL_GOV_ROOT" "$HARNESS" governance:doctor

LEGACY_GOV_ROOT="$TEST_ROOT/legacy-governance"
mkdir -p "$LEGACY_GOV_ROOT"
printf '# Specification\nCHG-202\n' >"$LEGACY_GOV_ROOT/SPECIFICATION.md"
printf '| Date | Change ID |\n|---|---|\n| 2026-01-02 | CHG-202 |\n' >"$LEGACY_GOV_ROOT/SPEC_CHANGELOG.md"
cat >"$LEGACY_GOV_ROOT/DEVELOPMENT_PLAN.md" <<'EOF'
# Development Plan
## CHG-202 Legacy governance
- Gate 2: complete
- Gate 4: approved
- Example Maintainer approved Gate 4 on 2026-01-02.
EOF
printf '# Test Plan\nCHG-202\n' >"$LEGACY_GOV_ROOT/TEST_PLAN.md"
printf '# Traceability\nCHG-202\n' >"$LEGACY_GOV_ROOT/TRACEABILITY.md"
HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
HARNESS_ROOT_DIR="$LEGACY_GOV_ROOT" \
HARNESS_GOVERNANCE_MODE=required \
HARNESS_REQUIRE_PLAN_APPROVAL=true \
HARNESS_ALLOW_LEGACY_CHANGE_DISCOVERY=true \
HARNESS_ALLOW_LEGACY_APPROVAL_FORMAT=true \
"$HARNESS" governance:doctor >/dev/null

if command -v git >/dev/null 2>&1; then
  REPO_ROOT="$TEST_ROOT/repository"
  mkdir -p "$REPO_ROOT"
  git -C "$REPO_ROOT" init -q
  printf 'SECRET=placeholder\n' >"$REPO_ROOT/.env"
  git -C "$REPO_ROOT" add -f .env
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$REPO_ROOT" "$HARNESS" repo:doctor >/dev/null
  expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$REPO_ROOT" HARNESS_STRICT_REPO_HYGIENE=true "$HARNESS" repo:doctor
fi

expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" "$HARNESS" unknown-command

if command -v shellcheck >/dev/null 2>&1; then
  shellcheck \
    "$HARNESS" \
    "$HARNESS_DIR/lib/adapters.sh" \
    "$HARNESS_DIR/lib/governance.sh" \
    "$HARNESS_DIR/lib/reporting.sh" \
    "${BASH_SOURCE[0]}"
else
  echo "note: shellcheck unavailable; syntax and behavioral self-tests still ran"
fi

echo "ok: portable harness self-tests passed"
