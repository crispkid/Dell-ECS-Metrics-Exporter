#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=go-env.sh
source "$SCRIPT_DIR/go-env.sh"

build_version="${BUILD_VERSION:-dev}"
build_commit="${BUILD_COMMIT:-unknown}"
build_date="${BUILD_DATE:-unknown}"

mkdir -p dist
CGO_ENABLED=0 go build \
  -trimpath \
  -ldflags "-s -w -X main.version=$build_version -X main.commit=$build_commit -X main.buildDate=$build_date" \
  -o dist/ecs-exporter \
  ./cmd/ecs-exporter

CGO_ENABLED=0 go build \
  -trimpath \
  -ldflags "-s -w -X main.version=$build_version -X main.commit=$build_commit -X main.buildDate=$build_date" \
  -o dist/ecs-flux-probe \
  ./cmd/ecs-flux-probe

dist/ecs-exporter -profiles-dir profiles -validate-profiles
