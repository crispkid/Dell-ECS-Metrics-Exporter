#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

run() {
  printf '\n== %s ==\n' "$1"
  shift
  "$@"
}

run "Toolchain" ./scripts/check-toolchain.sh
run "Lint" ./scripts/lint.sh
run "Format check" ./scripts/format-check.sh
run "Type check" ./scripts/typecheck.sh
run "Tests" ./scripts/test.sh
run "Coverage" ./scripts/coverage.sh
run "Build" ./scripts/build.sh
run "CI policy" ./scripts/ci-policy-check.sh
run "Security" ./scripts/security-check.sh
run "Deployment check" ./scripts/deploy-check.sh

printf '\nproject verification passed\n'
