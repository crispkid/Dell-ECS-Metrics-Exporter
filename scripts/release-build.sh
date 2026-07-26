#!/usr/bin/env bash

set -euo pipefail

if (($# != 1)); then
  printf 'Usage: ./scripts/release-build.sh vMAJOR.MINOR.PATCH[-PRERELEASE]\n' >&2
  exit 2
fi

release_tag="$1"
if [[ ! "$release_tag" =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)(-[0-9A-Za-z.-]+)?$ ]]; then
  printf 'error: release version must be a v-prefixed Semantic Version\n' >&2
  exit 2
fi
release_version="${release_tag#v}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"
# shellcheck source=go-env.sh
source "$SCRIPT_DIR/go-env.sh"
for tool in git go helm tar find; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    printf 'blocked: release build requires %s\n' "$tool" >&2
    exit 3
  fi
done

if [[ "${RELEASE_ALLOW_DIRTY:-false}" != "true" ]] &&
  [[ -n "$(git status --porcelain=v1 --untracked-files=all)" ]]; then
  printf 'error: release build requires a clean Git worktree\n' >&2
  exit 1
fi

build_commit="${BUILD_COMMIT:-$(git rev-parse HEAD)}"
if [[ ! "$build_commit" =~ ^[0-9a-f]{40}$ ]]; then
  printf 'error: BUILD_COMMIT must be a full lowercase Git SHA\n' >&2
  exit 1
fi
if [[ "${RELEASE_ALLOW_DIRTY:-false}" != "true" &&
  "$build_commit" != "$(git rev-parse HEAD)" ]]; then
  printf 'error: production release BUILD_COMMIT must equal the checked-out HEAD\n' >&2
  exit 1
fi
source_epoch="${SOURCE_DATE_EPOCH:-$(git show -s --format=%ct "$build_commit")}"
if [[ ! "$source_epoch" =~ ^[1-9][0-9]*$ ]]; then
  printf 'error: SOURCE_DATE_EPOCH must be a positive Unix timestamp\n' >&2
  exit 1
fi
if date -u -r "$source_epoch" '+%Y-%m-%dT%H:%M:%SZ' >/dev/null 2>&1; then
  build_date="$(date -u -r "$source_epoch" '+%Y-%m-%dT%H:%M:%SZ')"
else
  build_date="$(date -u -d "@$source_epoch" '+%Y-%m-%dT%H:%M:%SZ')"
fi

output_dir="$PROJECT_ROOT/dist/release"
stage_root="$output_dir/.stage"
cleanup() {
  rm -rf -- "$stage_root"
}
trap cleanup EXIT
rm -rf -- "$stage_root"
mkdir -p "$stage_root"
find "$output_dir" -maxdepth 1 -type f -delete

targets=(
  "linux amd64"
  "linux arm64"
  "darwin amd64"
  "darwin arm64"
)

for target in "${targets[@]}"; do
  read -r target_os target_arch <<<"$target"
  artifact_name="dell-ecs-metrics-exporter_${release_version}_${target_os}_${target_arch}"
  stage="$stage_root/$artifact_name"
  mkdir -p "$stage/profiles"
  CGO_ENABLED=0 GOOS="$target_os" GOARCH="$target_arch" go build \
    -trimpath \
    -ldflags "-s -w -X main.version=$release_version -X main.commit=$build_commit -X main.buildDate=$build_date" \
    -o "$stage/ecs-exporter" \
    ./cmd/ecs-exporter
  cp LICENSE README.md "$stage/"
  cp profiles/*.json "$stage/profiles/"
  if [[ "$target_os" == "linux" ]]; then
    mkdir -p "$stage/deploy"
    cp -R deploy/bare-metal "$stage/deploy/"
  fi
  SOURCE_DATE_EPOCH="$source_epoch" go run ./cmd/releasepack \
    -source="$stage" \
    -output="$output_dir/$artifact_name.tar.gz" \
    -prefix="$artifact_name"
done

helm_package_dir="$stage_root/helm-package"
helm_extract_dir="$stage_root/helm-extract"
mkdir -p "$helm_package_dir" "$helm_extract_dir"
helm package charts/dell-ecs-metrics-exporter \
  --version "$release_version" \
  --app-version "$release_version" \
  --destination "$helm_package_dir" >/dev/null
helm_package="$helm_package_dir/dell-ecs-metrics-exporter-$release_version.tgz"
tar -xzf "$helm_package" -C "$helm_extract_dir"
SOURCE_DATE_EPOCH="$source_epoch" go run ./cmd/releasepack \
  -source="$helm_extract_dir/dell-ecs-metrics-exporter" \
  -output="$output_dir/dell-ecs-metrics-exporter-$release_version.tgz" \
  -prefix="dell-ecs-metrics-exporter"
helm lint "$output_dir/dell-ecs-metrics-exporter-$release_version.tgz" >/dev/null

printf '{\n' >"$output_dir/release-metadata.json"
printf '  "version": "%s",\n' "$release_version" >>"$output_dir/release-metadata.json"
printf '  "commit": "%s",\n' "$build_commit" >>"$output_dir/release-metadata.json"
printf '  "buildDate": "%s",\n' "$build_date" >>"$output_dir/release-metadata.json"
printf '  "sourceDateEpoch": %s\n' "$source_epoch" >>"$output_dir/release-metadata.json"
printf '}\n' >>"$output_dir/release-metadata.json"

rm -rf -- "$stage_root"
trap - EXIT
"$SCRIPT_DIR/generate-checksums.sh" "$output_dir"
printf 'release artifacts built in %s\n' "$output_dir"
