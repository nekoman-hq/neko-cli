#!/usr/bin/env bash
set -euo pipefail

REPO="${NEKO_REPOSITORY:-nekoman-hq/neko-cli}"
API_BASE="${NEKO_GITHUB_API_BASE:-https://api.github.com}"
OS_OVERRIDE="${NEKO_OS:-}"
ARCH_OVERRIDE="${NEKO_ARCH:-}"
INSTALL_DIR=""

# Release listing is paginated; the repository publishes CLI and plugin units to
# the same release list, so a single page is not a safe assumption.
RELEASES_PER_PAGE=100
MAX_RELEASE_PAGES=20
STABLE_CLI_TAG_PATTERN='^v[0-9]+\.[0-9]+\.[0-9]+$'

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Error: required command '$1' not found" >&2
    exit 1
  fi
}

resolve_install_dir() {
  local effective_uid="$1"
  local user_home="$2"

  if [ "${NEKO_INSTALL_DIR+x}" = "x" ]; then
    if [ -z "$NEKO_INSTALL_DIR" ]; then
      echo "Error: NEKO_INSTALL_DIR must not be empty" >&2
      return 1
    fi
    printf '%s\n' "$NEKO_INSTALL_DIR"
    return
  fi

  if [ "$effective_uid" = "0" ]; then
    printf '%s\n' "/usr/local/bin"
    return
  fi
  if [ -z "$user_home" ]; then
    echo "Error: HOME must be set when NEKO_INSTALL_DIR is not provided" >&2
    return 1
  fi
  printf '%s/.local/bin\n' "$user_home"
}

normalize_version() {
  local value="$1"

  if [[ "$value" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    value="v${value}"
  fi

  if [[ ! "$value" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "Error: NEKO_VERSION must look like 3.0.4 or v3.0.4" >&2
    exit 1
  fi

  printf '%s\n' "$value"
}

curl_headers=(
  -H "Accept: application/vnd.github+json"
  -H "X-GitHub-Api-Version: 2022-11-28"
)

if [ -n "${GITHUB_TOKEN:-}" ]; then
  curl_headers+=(-H "Authorization: Bearer ${GITHUB_TOKEN}")
fi

github_get() {
  curl -fsSL "${curl_headers[@]}" "$1"
}

# stable_cli_tags reads a GitHub releases page on stdin and prints the tags of
# every published stable CLI release. Drafts, prereleases, plugin unit tags, the
# plugin-registry tag, and malformed tags are ignored. Release titles and the
# GitHub "Latest" label are never consulted.
stable_cli_tags() {
  jq -r --arg pattern "$STABLE_CLI_TAG_PATTERN" '
    if type != "array" then error("release response is not a JSON array") else . end
    | .[]
    | select(type == "object")
    | select(((.draft // false) | not) and ((.prerelease // false) | not))
    | .tag_name // empty
    | select(type == "string")
    | select(test($pattern))
  '
}

# greatest_cli_tag reads candidate tags on stdin and prints the greatest one by
# numeric major/minor/patch order, so v3.1.10 sorts after v3.1.9.
greatest_cli_tag() {
  jq -Rrn '
    [inputs | select(length > 0)]
    | sort_by(
        capture("^v(?<major>[0-9]+)\\.(?<minor>[0-9]+)\\.(?<patch>[0-9]+)$")
        | [.major, .minor, .patch]
        | map(tonumber)
      )
    | last // empty
  '
}

resolve_latest_cli_version() {
  local page=1
  local page_json
  local page_length
  local page_tags
  local tags=""
  local version
  local reached_last_page=0

  while [ "$page" -le "$MAX_RELEASE_PAGES" ]; do
    if ! page_json=$(github_get "${API_BASE}/repos/${REPO}/releases?per_page=${RELEASES_PER_PAGE}&page=${page}"); then
      echo "Error: unable to determine the latest CLI release: failed to fetch releases for ${REPO}" >&2
      exit 1
    fi

    if ! page_length=$(jq -r 'if type == "array" then length else error("release response is not a JSON array") end' <<<"$page_json" 2>/dev/null); then
      echo "Error: unable to determine the latest CLI release: malformed release response for ${REPO}" >&2
      exit 1
    fi

    if ! page_tags=$(stable_cli_tags <<<"$page_json" 2>/dev/null); then
      echo "Error: unable to determine the latest CLI release: malformed release response for ${REPO}" >&2
      exit 1
    fi

    if [ -n "$page_tags" ]; then
      tags="${tags}${page_tags}"$'\n'
    fi

    if [ "$page_length" -lt "$RELEASES_PER_PAGE" ]; then
      reached_last_page=1
      break
    fi
    page=$((page + 1))
  done

  # A full final page means the release list was truncated, so the greatest tag
  # seen so far is not provably the greatest tag published. Refuse rather than
  # install a non-maximum CLI release.
  if [ "$reached_last_page" -ne 1 ]; then
    echo "Error: unable to determine the latest CLI release: ${REPO} has more than ${MAX_RELEASE_PAGES} release pages ($((MAX_RELEASE_PAGES * RELEASES_PER_PAGE)) releases) and the list was truncated; pin a version with NEKO_VERSION" >&2
    exit 1
  fi

  version=$(greatest_cli_tag <<<"$tags")

  if [ -z "$version" ]; then
    echo "Error: unable to determine the latest CLI release: ${REPO} publishes no stable CLI release matching vX.Y.Z" >&2
    exit 1
  fi

  printf '%s\n' "$version"
}

platform_asset() {
  local os="$1"
  local arch="$2"
  local os_label
  local arch_label
  local extension

  case "$os" in
    Darwin|darwin)
      os_label="Darwin"
      extension="tar.gz"
      ;;
    Linux|linux)
      os_label="Linux"
      extension="tar.gz"
      ;;
    MINGW*|MSYS*|CYGWIN*|Windows|windows)
      os_label="Windows"
      extension="zip"
      ;;
    *)
      echo "Error: unsupported operating system: ${os}" >&2
      exit 1
      ;;
  esac

  case "$arch" in
    x86_64|amd64)
      arch_label="x86_64"
      ;;
    arm64|aarch64)
      arch_label="arm64"
      ;;
    i386|386)
      arch_label="i386"
      ;;
    *)
      echo "Error: unsupported architecture: ${arch}" >&2
      exit 1
      ;;
  esac

  if [ "$os_label" = "Darwin" ] && [ "$arch_label" = "i386" ]; then
    echo "Error: unsupported platform: Darwin/i386" >&2
    exit 1
  fi

  printf 'neko-cli_%s_%s.%s\n' "$os_label" "$arch_label" "$extension"
}

find_asset_url() {
  local version="$1"
  local asset_name="$2"
  local release_json
  local url

  if ! release_json=$(github_get "${API_BASE}/repos/${REPO}/releases/tags/${version}"); then
    echo "Error: failed to fetch release ${version} for ${REPO}" >&2
    exit 1
  fi

  url=$(
    jq -r --arg name "$asset_name" '
      .assets[]?
      | select(.name == $name)
      | .browser_download_url // empty
    ' <<<"$release_json" | head -n 1
  )

  if [ -z "$url" ]; then
    echo "Error: asset ${asset_name} not found in release ${version}" >&2
    exit 1
  fi

  printf '%s\n' "$url"
}

extract_archive() {
  local archive="$1"
  local extract_dir="$2"

  case "$archive" in
    *.tar.gz)
      tar -xzf "$archive" -C "$extract_dir"
      ;;
    *.zip)
      require_command unzip
      unzip -q "$archive" -d "$extract_dir"
      ;;
    *)
      echo "Error: unsupported archive format: ${archive}" >&2
      exit 1
      ;;
  esac
}

prepare_install_dir() {
  if ! mkdir -p "$INSTALL_DIR"; then
    echo "Error: cannot create installation directory: ${INSTALL_DIR}" >&2
    echo "Choose a user-owned directory with NEKO_INSTALL_DIR, for example: ${HOME:-\$HOME}/.local/bin" >&2
    return 1
  fi
  if [ ! -d "$INSTALL_DIR" ] || [ ! -w "$INSTALL_DIR" ]; then
    echo "Error: installation directory is not writable: ${INSTALL_DIR}" >&2
    echo "The installer never invokes sudo. Choose a user-owned directory with NEKO_INSTALL_DIR." >&2
    return 1
  fi
}

install_binary() {
  local source="$1"
  local destination="${INSTALL_DIR}/neko"

  install -m 0755 "$source" "$destination"
}

path_guidance() {
  case ":${PATH:-}:" in
    *":${INSTALL_DIR}:"*)
      return
      ;;
  esac
  echo "Add ${INSTALL_DIR} to PATH to run neko from a new shell."
}

main() {
  require_command curl
  require_command jq
  require_command tar
  require_command find
  require_command install
  require_command id

  INSTALL_DIR=$(resolve_install_dir "$(id -u)" "${HOME:-}")
  prepare_install_dir

  local version="${NEKO_VERSION:-}"
  if [ -n "$version" ]; then
    version=$(normalize_version "$version")
  else
    version=$(resolve_latest_cli_version)
  fi

  local os="${OS_OVERRIDE:-$(uname -s)}"
  local arch="${ARCH_OVERRIDE:-$(uname -m)}"
  local asset_name
  local asset_url
  asset_name=$(platform_asset "$os" "$arch")
  asset_url=$(find_asset_url "$version" "$asset_name")

  local tmp_dir
  local archive
  local extract_dir
  local binary_path
  tmp_dir=$(mktemp -d)
  trap 'rm -rf -- "${tmp_dir:-}"' EXIT

  archive="${tmp_dir}/${asset_name}"
  extract_dir="${tmp_dir}/extract"
  mkdir -p "$extract_dir"

  echo "Downloading ${asset_name} from ${REPO} ${version}..."
  if ! github_get "$asset_url" >"$archive"; then
    echo "Error: failed to download ${asset_name}" >&2
    return 1
  fi
  extract_archive "$archive" "$extract_dir"

  binary_path=$(find "$extract_dir" -type f \( -name neko-cli -o -name neko-cli.exe -o -name neko -o -name neko.exe \) | head -n 1)
  if [ -z "$binary_path" ]; then
    echo "Error: downloaded archive did not contain neko-cli binary" >&2
    return 1
  fi

  install_binary "$binary_path"

  echo "neko-cli ${version} installed to ${INSTALL_DIR}/neko"
  path_guidance
}

# The installer is a standalone script with a single entry point. It runs the
# same main flow whether it is executed from a file (./install.sh, bash
# install.sh) or from stdin (curl ... | bash). No script path is consulted, and
# no repository-relative helper file is sourced, so stdin execution needs nothing
# that a piped shell leaves unset. Bash expands "$@" to nothing when there are no
# positional parameters, so this stays safe under `set -u`.
main "$@"
