#!/usr/bin/env bash

set -euo pipefail

if ! command -v kubeconform >/dev/null 2>&1; then
  printf 'blocked: kubeconform is required for the production deployment gate\n' >&2
  exit 3
fi
version="$(kubeconform -v 2>&1)" || {
  printf 'blocked: could not determine kubeconform version\n' >&2
  exit 3
}
if ! grep -Eq '(^|[^0-9])0[.]7[.]0([^0-9]|$)' <<<"$version"; then
  printf 'blocked: kubeconform 0.7.0 is required; detected: %s\n' "$version" >&2
  exit 3
fi

./scripts/deploy-check.sh
