#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=go-env.sh
source "$SCRIPT_DIR/go-env.sh"

secret_pattern='-----BEGIN ([A-Z0-9 ]+ )?PRIVATE KEY-----|(Authorization|Cookie|X-SDS-AUTH-TOKEN)[\"'\'']?[[:space:]]*[:=][[:space:]]*[\"'\'']?((Basic|Bearer)[[:space:]]+)?[A-Za-z0-9._~+/=-]{12,}'
if rg -n --hidden \
  --glob '!.git/**' \
  --glob '!coverage/**' \
  --glob '!dist/**' \
  --glob '!test-results/**' \
  --glob '!go.sum' \
  -- "$secret_pattern" .; then
  printf 'error: credential-like material found\n' >&2
  exit 1
fi

go mod tidy -diff
go mod verify
go tool govulncheck ./...
