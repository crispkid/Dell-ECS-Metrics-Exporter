#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"
# shellcheck source=go-env.sh
source "$SCRIPT_DIR/go-env.sh"

evidence_dir="$PROJECT_ROOT/test-results/performance"
mkdir -p "$evidence_dir"
go run ./cmd/perfcheck >"$evidence_dir/synthetic-precheck.json"

printf 'synthetic 10-cluster/100-node/10,000-bucket precheck passed\n'
printf 'note: deployed RSS/CPU/API-latency evidence is still required for production certification\n'
