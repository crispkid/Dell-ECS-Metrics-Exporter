#!/usr/bin/env bash

set -euo pipefail

if (($# != 1)); then
  printf 'Usage: ./scripts/release-kind.sh vMAJOR.MINOR.PATCH[-PRERELEASE]\n' >&2
  exit 2
fi

release_tag="$1"
if [[ ! "$release_tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
  printf 'error: release tag must be a v-prefixed Semantic Version\n' >&2
  exit 2
fi

if [[ "$release_tag" == *-* ]]; then
  printf 'prerelease\n'
else
  printf 'stable\n'
fi
