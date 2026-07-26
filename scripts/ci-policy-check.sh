#!/usr/bin/env bash

set -euo pipefail

workflow_dir=".github/workflows"
if [[ ! -d "$workflow_dir" ]]; then
  printf 'error: GitHub Actions workflow directory is missing\n' >&2
  exit 1
fi

workflow_files=()
while IFS= read -r path; do
  workflow_files+=("$path")
done < <(find "$workflow_dir" -type f \( -name '*.yaml' -o -name '*.yml' \) -print | sort)

if [[ ${#workflow_files[@]} -eq 0 ]]; then
  printf 'error: no GitHub Actions workflows found\n' >&2
  exit 1
fi

if rg -n '^[[:space:]]*pull_request_target:' "${workflow_files[@]}"; then
  printf 'error: pull_request_target is forbidden for this repository\n' >&2
  exit 1
fi
if ! rg -q '^[[:space:]]*permissions:' "${workflow_files[@]}" ||
  ! rg -q '^[[:space:]]*contents:[[:space:]]+read[[:space:]]*$' "${workflow_files[@]}"; then
  printf 'error: workflows must declare read-only contents permission\n' >&2
  exit 1
fi

uses_lines="$(rg -n '^[[:space:]]*uses:' "${workflow_files[@]}" || true)"
if [[ -n "$uses_lines" ]] &&
  printf '%s\n' "$uses_lines" | rg -v '@[0-9a-f]{40}([[:space:]]*#.*)?$'; then
  printf 'error: every action must be pinned to a full commit SHA\n' >&2
  exit 1
fi

if rg -n 'run:.*\$\{\{[[:space:]]*github\.event\.' "${workflow_files[@]}"; then
  printf 'error: untrusted event fields must not be interpolated into run commands\n' >&2
  exit 1
fi

printf 'GitHub Actions policy checks passed\n'
