#!/usr/bin/env bash
set -euo pipefail

require_env() {
  local name="$1"
  if [[ -z "${!name:-}" ]]; then
    echo "::error::$name is required" >&2
    exit 1
  fi
}

require_env INDEX_OUTPUT
require_env GITHUB_REPOSITORY
require_env RELEASE_UNIT
require_env RELEASE_VERSION
require_env RELEASE_TAG

if ! command -v go >/dev/null 2>&1; then
  echo "::error::go is required to generate plugin-index.json" >&2
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "::error::jq is required to validate plugin-index.json" >&2
  exit 1
fi

mkdir -p "$(dirname "$INDEX_OUTPUT")"

temp_root="${RUNNER_TEMP:-/tmp}"
plugin_dir="$(mktemp -d "$temp_root/neko-plugin-index.XXXXXX")"
cleanup() {
  rm -rf "$plugin_dir"
}
trap cleanup EXIT

mkdir -p "$plugin_dir/release"
go build -o "$plugin_dir/release/plugin-release" ./plugin/release
cp plugin/release/manifest.json "$plugin_dir/release/manifest.json"

NEKO_PLUGIN_DIR="$plugin_dir" \
  go run . release plugin-index \
  --output "$INDEX_OUTPUT" \
  --repository "$GITHUB_REPOSITORY"

if [[ ! -s "$INDEX_OUTPUT" ]]; then
  echo "::error::generated plugin-index.json is empty: $INDEX_OUTPUT" >&2
  exit 1
fi

if ! jq -e \
  --arg repository "$GITHUB_REPOSITORY" \
  --arg unit "$RELEASE_UNIT" \
  --arg version "$RELEASE_VERSION" \
  --arg tag "$RELEASE_TAG" '
    .schemaVersion == 1 and
    .repository == $repository and
    any(.plugins[]; .unit == $unit and
      .version == $version and
      .tag == $tag and
      .tag == (.tagPrefix + .version))
  ' "$INDEX_OUTPUT" >/dev/null; then
  echo "::error::plugin-index.json must contain $RELEASE_UNIT $RELEASE_VERSION at $RELEASE_TAG" >&2
  exit 1
fi

if jq -e '.. | strings | select(startswith("/"))' "$INDEX_OUTPUT" >/dev/null; then
  echo "::error::plugin-index.json must not contain absolute local paths" >&2
  exit 1
fi

for forbidden in "${GITHUB_TOKEN:-}" "${GH_TOKEN:-}" "${RUNNER_TEMP:-}" "${GITHUB_WORKSPACE:-}"; do
  if [[ -n "$forbidden" ]] && grep -Fq -- "$forbidden" "$INDEX_OUTPUT"; then
    echo "::error::plugin-index.json contains forbidden runtime or secret data" >&2
    exit 1
  fi
done

if grep -Eq 'ghp_|github_pat_|GITHUB_TOKEN|GH_TOKEN' "$INDEX_OUTPUT"; then
  echo "::error::plugin-index.json contains secret-looking content" >&2
  exit 1
fi

echo "Generated and validated plugin-index.json at $INDEX_OUTPUT"
