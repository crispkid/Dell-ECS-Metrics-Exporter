#!/usr/bin/env bash

set -euo pipefail

artifact_dir="${1:-dist/release}"
if [[ ! -d "$artifact_dir" ]]; then
  printf 'error: artifact directory does not exist: %s\n' "$artifact_dir" >&2
  exit 1
fi
if ! command -v sha256sum >/dev/null 2>&1 &&
  ! command -v shasum >/dev/null 2>&1; then
  printf 'blocked: sha256sum or shasum is required\n' >&2
  exit 3
fi

checksum_file="$artifact_dir/SHA256SUMS"
temporary_file="$(mktemp "$artifact_dir/.SHA256SUMS.XXXXXX.tmp")"
trap 'rm -f -- "$temporary_file"' EXIT

hash_file() {
  local path="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$path" | awk '{print $1}'
  else
    shasum -a 256 "$path" | awk '{print $1}'
  fi
}

while IFS= read -r path; do
  name="${path#"$artifact_dir"/}"
  case "$name" in
    .SHA256SUMS.*.tmp|SHA256SUMS|SHA256SUMS.*|*.sig|*.pem|*.bundle) continue ;;
  esac
  printf '%s  %s\n' "$(hash_file "$path")" "$name" >>"$temporary_file"
done < <(find "$artifact_dir" -maxdepth 1 -type f -print | LC_ALL=C sort)

mv -- "$temporary_file" "$checksum_file"
trap - EXIT
if command -v sha256sum >/dev/null 2>&1; then
  (cd "$artifact_dir" && sha256sum --check SHA256SUMS >/dev/null)
else
  (cd "$artifact_dir" && shasum -a 256 --check SHA256SUMS >/dev/null)
fi
printf 'checksums: %s\n' "$checksum_file"
