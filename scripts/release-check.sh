#!/usr/bin/env bash

set -euo pipefail

if [[ ! "${RELEASE_VERSION:-}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
  printf 'blocked: RELEASE_VERSION must be vMAJOR.MINOR.PATCH[-PRERELEASE]\n' >&2
  exit 3
fi
export RELEASE_VERSION

run() {
  printf '\n== %s ==\n' "$1"
  shift
  "$@"
}

run "Release policy" ./scripts/release-policy-check.sh
run "Deterministic verification" ./HARNESS/harness.sh verify
run "Fresh race suite" bash -lc \
  'source scripts/go-env.sh; go test -race -count=1 -timeout=3m ./...'
run "Supply chain" ./HARNESS/harness.sh supply-chain
run "Container startup" ./scripts/container-check.sh
run "Deployment policy and Kubernetes schemas" ./scripts/deployment-release-check.sh
run "Synthetic target-scale precheck" ./scripts/performance-check.sh
run "Deployed target-scale performance" ./scripts/deployed-performance-check.sh
run "Live ECS integration" ./HARNESS/harness.sh integration
run "Deployed end-to-end" ./HARNESS/harness.sh e2e

printf '\nproduction release gates passed\n'
