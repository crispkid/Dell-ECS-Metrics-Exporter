#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=go-env.sh
source "$SCRIPT_DIR/go-env.sh"

threshold="${COVERAGE_THRESHOLD:-80}"
mkdir -p coverage
go test -timeout=2m -covermode=atomic -coverprofile=coverage/coverage.out ./internal/...

coverage="$(go tool cover -func=coverage/coverage.out | awk '/^total:/ {sub(/%$/, "", $3); print $3}')"
if [[ -z "$coverage" ]]; then
  printf 'error: could not determine total coverage\n' >&2
  exit 1
fi
awk -v actual="$coverage" -v required="$threshold" 'BEGIN {
  if (actual + 0 < required + 0) {
    printf "error: coverage %.1f%% is below required %.1f%%\n", actual, required > "/dev/stderr"
    exit 1
  }
}'
printf 'coverage %.1f%% meets required %.1f%%\n' "$coverage" "$threshold"
