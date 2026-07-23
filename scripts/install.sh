#!/usr/bin/env bash
set -euo pipefail
set -f

readonly REPOSITORY="Architeg/gloss"
readonly OFFICIAL_RELEASE_ROOT="https://github.com/${REPOSITORY}"
readonly USER_AGENT="gloss-installer"
readonly BINARY_NAME="gloss"
readonly MAX_ARCHIVE_BYTES=$((64 * 1024 * 1024))
readonly MAX_CHECKSUM_BYTES=$((1024 * 1024))

temporary_dir=""
staged_executable=""

fail() {
  echo "gloss installer: $*" >&2
  return 1
}

cleanup() {
  if [[ -n "$staged_executable" && -f "$staged_executable" ]]; then
    rm -f -- "$staged_executable"
  fi
  if [[ -n "$temporary_dir" && -d "$temporary_dir" ]]; then
    rm -rf -- "$temporary_dir"
  fi
}

normalize_version() {
  local value="$1"
  if [[ "$value" != v* ]]; then
    value="v${value}"
  fi
  if [[ ! "$value" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]; then
    fail "version must match vMAJOR.MINOR.PATCH: $1"
    return 1
  fi
  printf '%s\n' "$value"
}

detect_platform() {
  local uname_s="$1"
  local uname_m="$2"
  local os
  local arch
  case "$uname_s" in
    Darwin) os="darwin" ;;
    Linux) os="linux" ;;
    *)
      fail "unsupported operating system: $uname_s"
      return 1
      ;;
  esac
  case "$uname_m" in
    x86_64 | amd64) arch="amd64" ;;
    arm64 | aarch64) arch="arm64" ;;
    *)
      fail "unsupported architecture: $uname_m"
      return 1
      ;;
  esac
  printf '%s %s gloss-%s-%s.zip gloss-%s-%s\n' "$os" "$arch" "$os" "$arch" "$os" "$arch"
}

safe_test_release_root() {
  local value="$1"
  case "$value" in
    http://127.0.0.1:* | http://localhost:* | http://[[]::1[]]:* | https://*)
      printf '%s\n' "${value%/}"
      ;;
    *)
      fail "testing release root must use HTTPS or loopback HTTP"
      return 1
      ;;
  esac
}

release_root() {
  if [[ -n "${GLOSS_RELEASE_ROOT:-}" ]]; then
    if [[ "${GLOSS_INSTALL_TESTING:-}" != "1" ]]; then
      fail "GLOSS_RELEASE_ROOT is available only with GLOSS_INSTALL_TESTING=1"
      return 1
    fi
    safe_test_release_root "$GLOSS_RELEASE_ROOT"
    return
  fi
  printf '%s\n' "$OFFICIAL_RELEASE_ROOT"
}

download_bounded() {
  local url="$1"
  local destination="$2"
  local limit="$3"
  local -a protocol_options=(--proto '=https' --proto-redir '=https')
  if [[ "$url" == http://* && "${GLOSS_INSTALL_TESTING:-}" == "1" ]]; then
    protocol_options=(--proto '=http,https' --proto-redir '=http,https')
  fi
  if ! curl --fail --silent --show-error --location \
    --connect-timeout 10 --max-time 60 --max-filesize "$limit" \
    "${protocol_options[@]}" \
    --user-agent "$USER_AGENT" \
    --output "$destination" "$url"; then
    fail "download failed: $url"
    return 1
  fi
  local size
  size="$(wc -c < "$destination" | tr -d '[:space:]')"
  if [[ ! "$size" =~ ^[0-9]+$ ]] || ((size == 0 || size > limit)); then
    fail "download has unsafe size: $url"
    return 1
  fi
}

resolve_latest_version() {
  local root="$1"
  local effective
  local -a protocol_options=(--proto '=https' --proto-redir '=https')
  if [[ "$root" == http://* && "${GLOSS_INSTALL_TESTING:-}" == "1" ]]; then
    protocol_options=(--proto '=http,https' --proto-redir '=http,https')
  fi
  if ! effective="$(curl --fail --silent --show-error --location \
    --connect-timeout 10 --max-time 30 \
    "${protocol_options[@]}" \
    --user-agent "$USER_AGENT" \
    --output /dev/null --write-out '%{url_effective}' \
    "$root/releases/latest")"; then
    fail "could not resolve the latest stable release"
    return 1
  fi
  local tag="${effective%%\?*}"
  tag="${tag%/}"
  tag="${tag##*/}"
  normalize_version "$tag"
}

safe_checksum_basename() {
  local name="$1"
  [[ -n "$name" && "$name" != "." && "$name" != ".." && "$name" != */* && "$name" != *\\* ]]
}

lookup_checksum() {
  local checksum_file="$1"
  local expected_name="$2"
  local matched=""
  local matches=0
  local entries=0
  local line
  local digest
  local name
  local extra

  while IFS= read -r line || [[ -n "$line" ]]; do
    [[ -z "${line//[[:space:]]/}" ]] && continue
    digest=""
    name=""
    extra=""
    read -r digest name extra <<< "$line"
    if [[ -n "$extra" || -z "$digest" || -z "$name" ]]; then
      fail "malformed checksum entry"
      return 1
    fi
    name="${name#\*}"
    if ! safe_checksum_basename "$name"; then
      fail "unsafe checksum filename: $name"
      return 1
    fi
    if [[ ! "$digest" =~ ^[0-9a-fA-F]{64}$ ]]; then
      fail "invalid SHA-256 for $name"
      return 1
    fi
    entries=$((entries + 1))
    if [[ "$name" == "$expected_name" ]]; then
      matches=$((matches + 1))
      matched="$digest"
    fi
  done < "$checksum_file"

  if ((entries != 4)); then
    fail "checksums.txt must contain exactly four entries"
    return 1
  fi
  if ((matches != 1)); then
    fail "expected exactly one checksum for $expected_name"
    return 1
  fi
  printf '%s\n' "$matched" | tr 'A-F' 'a-f'
}

compute_sha256() {
  local file="$1"
  local forced="${GLOSS_CHECKSUM_TOOL:-}"
  if [[ -n "$forced" && "${GLOSS_INSTALL_TESTING:-}" != "1" ]]; then
    fail "GLOSS_CHECKSUM_TOOL is available only in installer test mode"
    return 1
  fi
  if [[ "$forced" == "none" ]]; then
    fail "neither sha256sum nor shasum is available"
    return 1
  fi
  if [[ "$forced" == "sha256sum" ]] || { [[ -z "$forced" ]] && command -v sha256sum >/dev/null 2>&1; }; then
    sha256sum "$file" | awk '{print $1}'
    return
  fi
  if [[ "$forced" == "shasum" ]] || { [[ -z "$forced" ]] && command -v shasum >/dev/null 2>&1; }; then
    shasum -a 256 "$file" | awk '{print $1}'
    return
  fi
  fail "neither sha256sum nor shasum is available"
}

verify_checksum() {
  local checksum_file="$1"
  local archive="$2"
  local asset="$3"
  local expected
  local actual
  expected="$(lookup_checksum "$checksum_file" "$asset")" || return 1
  actual="$(compute_sha256 "$archive")" || return 1
  actual="$(printf '%s' "$actual" | tr 'A-F' 'a-f')"
  if [[ "$actual" != "$expected" ]]; then
    fail "SHA-256 mismatch for $asset"
    return 1
  fi
}

inspect_archive() {
  local archive="$1"
  local expected="$2"
  local entries
  local mode
  local size

  entries="$(unzip -Z1 "$archive")" || {
    fail "cannot inspect release ZIP"
    return 1
  }
  if [[ "$entries" != "$expected" ]]; then
    fail "ZIP must contain exactly one entry named $expected"
    return 1
  fi
  if ! safe_checksum_basename "$entries"; then
    fail "ZIP contains an unsafe entry name"
    return 1
  fi
  mode="$(zipinfo -l "$archive" | awk -v expected="$expected" '$NF == expected { print $1 }')"
  if [[ ! "$mode" =~ ^-[rwx-]{9}$ || "$mode" != *x* ]]; then
    fail "ZIP entry is not a regular executable"
    return 1
  fi
  size="$(zipinfo -l "$archive" | awk -v expected="$expected" '$NF == expected { print $4 }')"
  if [[ ! "$size" =~ ^[0-9]+$ ]] || ((size == 0 || size > MAX_ARCHIVE_BYTES)); then
    fail "ZIP executable has an unsafe size"
    return 1
  fi
}

is_homebrew_path() {
  local value="$1"
  case "$value/" in
    */Cellar/* | /usr/local/opt/* | /opt/homebrew/opt/* | */homebrew/opt/* | */.linuxbrew/opt/*) return 0 ;;
    *) return 1 ;;
  esac
}

is_protected_path() {
  local value="$1"
  case "$value" in
    /bin/* | /sbin/* | /usr/bin/* | /usr/sbin/* | /System/*) return 0 ;;
    *) return 1 ;;
  esac
}

file_identity() {
  local path="$1"
  if stat -f '%d:%i' "$path" >/dev/null 2>&1; then
    stat -f '%d:%i' "$path"
  else
    stat -c '%d:%i' "$path"
  fi
}

validate_destination() {
  local requested="$1"
  local default_dir="$HOME/.local/bin"
  if [[ "$requested" != /* ]]; then
    fail "installation directory must be absolute"
    return 1
  fi
  if [[ ! -e "$requested" ]]; then
    if [[ "$requested" != "$default_dir" ]]; then
      fail "explicit installation directory does not exist: $requested"
      return 1
    fi
    local local_parent="$HOME/.local"
    if [[ -e "$local_parent" ]]; then
      if [[ -L "$local_parent" || ! -d "$local_parent" ]]; then
        fail "user-local parent must be a non-symlink directory: $local_parent"
        return 1
      fi
    else
      mkdir -m 700 "$local_parent"
    fi
    mkdir -m 700 "$requested"
  fi
  if [[ -L "$requested" || ! -d "$requested" ]]; then
    fail "installation directory must be a non-symlink directory"
    return 1
  fi
  local resolved
  resolved="$(cd -- "$requested" && pwd -P)"
  local logical
  logical="$(cd -- "$requested" && pwd -L)"
  if [[ "$logical" != "$resolved" ]]; then
    fail "installation directory has a symlinked parent: $requested"
    return 1
  fi
  if is_homebrew_path "$requested" || is_homebrew_path "$resolved"; then
    fail "refusing Homebrew-managed destination; use: brew upgrade Architeg/tap/gloss"
    return 1
  fi
  if is_protected_path "$resolved/$BINARY_NAME"; then
    fail "refusing protected system destination: $resolved"
    return 1
  fi
  if [[ ! -w "$resolved" ]]; then
    fail "installation directory is not writable: $resolved"
    return 1
  fi
  local target="$resolved/$BINARY_NAME"
  if [[ -L "$target" ]]; then
    fail "refusing symlinked installation target: $target"
    return 1
  fi
  if [[ -e "$target" && ! -f "$target" ]]; then
    fail "existing installation target is not a regular file: $target"
    return 1
  fi
  if [[ -e "$target" && ! -w "$target" ]]; then
    fail "existing installation target is not writable: $target"
    return 1
  fi
  printf '%s\n' "$target"
}

install_atomically() {
  local source="$1"
  local target="$2"
  local before=""
  if [[ -e "$target" ]]; then
    before="$(file_identity "$target")"
  fi
  local target_dir
  target_dir="$(dirname -- "$target")"
  staged_executable="$(mktemp "$target_dir/.gloss-install.XXXXXX")"
  cp -- "$source" "$staged_executable"
  chmod 0755 "$staged_executable"
  if [[ ! -s "$staged_executable" || -L "$staged_executable" ]]; then
    fail "staged executable is invalid"
    return 1
  fi
  if [[ -n "$before" ]]; then
    if [[ -L "$target" || ! -f "$target" || "$(file_identity "$target")" != "$before" ]]; then
      fail "installation target changed during installation"
      return 1
    fi
  elif [[ -e "$target" || -L "$target" ]]; then
    fail "installation target appeared during installation"
    return 1
  fi
  mv -f -- "$staged_executable" "$target"
  staged_executable=""
}

main() {
  for command in curl unzip zipinfo awk mktemp; do
    if ! command -v "$command" >/dev/null 2>&1; then
      fail "required command is unavailable: $command"
      return 1
    fi
  done

  local uname_s
  local uname_m
  uname_s="${GLOSS_TEST_UNAME_S:-$(uname -s)}"
  uname_m="${GLOSS_TEST_UNAME_M:-$(uname -m)}"
  if [[ -n "${GLOSS_TEST_UNAME_S:-}${GLOSS_TEST_UNAME_M:-}" && "${GLOSS_INSTALL_TESTING:-}" != "1" ]]; then
    fail "platform overrides are available only in installer test mode"
    return 1
  fi

  local platform
  platform="$(detect_platform "$uname_s" "$uname_m")" || return 1
  local os
  local arch
  local asset
  local executable
  read -r os arch asset executable <<< "$platform"

  local root
  root="$(release_root)" || return 1
  local requested_version="${VERSION:-latest}"
  local tag
  if [[ "$requested_version" == "latest" ]]; then
    tag="$(resolve_latest_version "$root")" || return 1
  else
    tag="$(normalize_version "$requested_version")" || return 1
  fi

  temporary_dir="$(mktemp -d "${TMPDIR:-/tmp}/gloss-install.XXXXXX")"
  trap cleanup EXIT HUP INT TERM
  chmod 0700 "$temporary_dir"

  local checksums="$temporary_dir/checksums.txt"
  local archive="$temporary_dir/$asset"
  local release_url="$root/releases/download/$tag"
  echo "Downloading Gloss $tag for $os/$arch..."
  download_bounded "$release_url/checksums.txt" "$checksums" "$MAX_CHECKSUM_BYTES"
  download_bounded "$release_url/$asset" "$archive" "$MAX_ARCHIVE_BYTES"

  # The archive is not inspected or extracted until its exact checksum passes.
  verify_checksum "$checksums" "$archive" "$asset"
  inspect_archive "$archive" "$executable"

  local extract_dir="$temporary_dir/extracted"
  mkdir -m 700 "$extract_dir"
  unzip -qq "$archive" "$executable" -d "$extract_dir"
  local extracted="$extract_dir/$executable"
  if [[ -L "$extracted" || ! -f "$extracted" || ! -s "$extracted" ]]; then
    fail "extracted executable is invalid"
    return 1
  fi

  local install_dir="${INSTALL_DIR:-$HOME/.local/bin}"
  local target
  target="$(validate_destination "$install_dir")" || return 1
  install_atomically "$extracted" "$target"

  echo
  echo "Gloss ${tag#v} installed."
  echo "Destination: $target"
  case ":$PATH:" in
    *":$(dirname -- "$target"):"*)
      echo "Run: gloss version"
      ;;
    *)
      local path_directory
      local escaped_directory
      path_directory="$(dirname -- "$target")"
      escaped_directory="${path_directory//\'/\'\\\'\'}"
      echo "$path_directory is not in PATH."
      echo "Add this to your shell configuration:"
      printf "  export PATH='%s':%s\n" "$escaped_directory" '$PATH'
      echo "Then run: gloss version"
      ;;
  esac
  echo "Homebrew users should install with: brew install Architeg/tap/gloss"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
