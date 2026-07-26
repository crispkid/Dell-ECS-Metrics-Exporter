#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=go-env.sh
source "$SCRIPT_DIR/go-env.sh"

expected_go="go1.26.5"
actual_go="$(go env GOVERSION)"
if [[ "$actual_go" != "$expected_go" ]]; then
  printf 'error: Go version is %s, expected %s\n' "$actual_go" "$expected_go" >&2
  exit 1
fi

helm_major="$(helm version --template '{{.Version}}' | sed -E 's/^v([0-9]+).*/\1/')"
if [[ ! "$helm_major" =~ ^[0-9]+$ ]] || ((helm_major < 3)); then
  printf 'error: Helm 3 or newer is required\n' >&2
  exit 1
fi

go version
helm version --short
