#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=go-env.sh
source "$SCRIPT_DIR/go-env.sh"

unformatted="$(find cmd internal -type f -name '*.go' -print0 | xargs -0 gofmt -l)"
if [[ -n "$unformatted" ]]; then
  printf 'error: gofmt is required for:\n%s\n' "$unformatted" >&2
  exit 1
fi
