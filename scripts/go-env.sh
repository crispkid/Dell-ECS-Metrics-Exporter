#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TASK_CACHE_ROOT="/tmp/dell-ecs-metrics-exporter-${UID:-unknown}"

export GOCACHE="$TASK_CACHE_ROOT/go-build"
export GOMODCACHE="$TASK_CACHE_ROOT/go-mod"
export GOFLAGS="-mod=readonly"

mkdir -p "$GOCACHE" "$GOMODCACHE"
cd "$PROJECT_ROOT"
