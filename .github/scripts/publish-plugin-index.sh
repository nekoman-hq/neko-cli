#!/usr/bin/env bash
set -euo pipefail

require_env() {
  local name="$1"
  if [[ -z "${!name:-}" ]]; then
    echo "::error::$name is required" >&2
    exit 1
  fi
}

require_env GITHUB_REPOSITORY
require_env INDEX_PATH

if [[ -z "${GH_TOKEN:-}" && -z "${GITHUB_TOKEN:-}" ]]; then
  echo "::error::GH_TOKEN or GITHUB_TOKEN is required" >&2
  exit 1
fi

export GH_TOKEN="${GH_TOKEN:-$GITHUB_TOKEN}"

if ! command -v gh >/dev/null 2>&1; then
  echo "::error::gh is required to publish plugin-index.json" >&2
  exit 1
fi

if [[ ! -s "$INDEX_PATH" ]]; then
  echo "::error::INDEX_PATH must point to a non-empty plugin-index.json file" >&2
  exit 1
fi

registry_tag="${PLUGIN_REGISTRY_TAG:-plugin-registry}"
asset_name="${PLUGIN_INDEX_ASSET_NAME:-plugin-index.json}"
target_sha="${PLUGIN_REGISTRY_TARGET_SHA:-${GITHUB_SHA:-}}"

if gh release view "$registry_tag" --repo "$GITHUB_REPOSITORY" >/dev/null 2>&1; then
  gh release upload "$registry_tag" "$INDEX_PATH#$asset_name" --clobber --repo "$GITHUB_REPOSITORY"
  echo "Updated $asset_name on $registry_tag."
  exit 0
fi

if [[ -z "$target_sha" ]]; then
  echo "::error::PLUGIN_REGISTRY_TARGET_SHA or GITHUB_SHA is required to create $registry_tag" >&2
  exit 1
fi

gh release create "$registry_tag" "$INDEX_PATH#$asset_name" \
  --repo "$GITHUB_REPOSITORY" \
  --title "Neko Plugin Registry" \
  --notes "Mutable registry release for neko plugin-index.json." \
  --target "$target_sha"

echo "Created $registry_tag and uploaded $asset_name."
