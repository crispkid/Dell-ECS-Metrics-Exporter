#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=go-env.sh
source "$SCRIPT_DIR/go-env.sh"

go test -timeout=2m ./...
