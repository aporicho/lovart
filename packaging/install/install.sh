#!/usr/bin/env bash
set -euo pipefail

REPO="aporicho/lovart"
VERSION="latest"
INSTALL_DIR="${HOME}/.local/bin"
EXTENSION_DIR="${HOME}/.lovart/extension/lovart-connector"
MCP_CLIENTS="auto"
YES=0
FORCE=0
DRY_RUN=0
JSON=0

usage() {
  cat <<'EOF'
Usage: install.sh [options]

Options:
  --repo OWNER/REPO
  --version latest|vX.Y.Z
  --install-dir PATH
  --extension-dir PATH
  --mcp-clients auto|all|none|codex,claude,opencode,openclaw
  --yes
  --force
  --dry-run
  --json
EOF
}

json_escape() {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  value="${value//$'\n'/\\n}"
  value="${value//$'\r'/\\r}"
  value="${value//$'\t'/\\t}"
  printf '"%s"' "$value"
}

emit_json() {
  local ok="$1"
  local message="$2"
  local asset="${3:-}"
  local path="${4:-}"
  printf '{"ok":%s,"message":%s,"data":{"repo":%s,"version":%s,"asset":%s,"install_path":%s,"extension_path":%s,"mcp_clients":%s,"dry_run":%s}}\n' \
    "$ok" \
    "$(json_escape "$message")" \
    "$(json_escape "$REPO")" \
    "$(json_escape "$VERSION")" \
    "$(json_escape "$asset")" \
    "$(json_escape "$path")" \
    "$(json_escape "$EXTENSION_DIR")" \
    "$(json_escape "$MCP_CLIENTS")" \
    "$([ "$DRY_RUN" -eq 1 ] && echo true || echo false)"
}

fail() {
  if [ "$JSON" -eq 1 ]; then
    emit_json false "$1"
  else
    printf 'error: %s\n' "$1" >&2
  fi
  exit 1
}

log() {
  if [ "$JSON" -eq 0 ]; then
    printf '%s\n' "$1"
  fi
}

require_value() {
  local option="$1"
  local value="${2:-}"
  if [ -z "$value" ]; then
    fail "${option} requires a value"
  fi
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --repo) require_value "$1" "${2:-}"; REPO="$2"; shift 2 ;;
    --version) require_value "$1" "${2:-}"; VERSION="$2"; shift 2 ;;
    --install-dir) require_value "$1" "${2:-}"; INSTALL_DIR="$2"; shift 2 ;;
    --extension-dir) require_value "$1" "${2:-}"; EXTENSION_DIR="$2"; shift 2 ;;
    --mcp-clients) require_value "$1" "${2:-}"; MCP_CLIENTS="$2"; shift 2 ;;
    --yes) YES=1; shift ;;
    --force) FORCE=1; shift ;;
    --dry-run) DRY_RUN=1; shift ;;
    --json) JSON=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) fail "unknown option: $1" ;;
  esac
done

OS="$(uname -s)"
ARCH="$(uname -m)"
ASSET=""
case "${OS}:${ARCH}" in
  Darwin:arm64) ASSET="lovart-macos-arm64" ;;
  Linux:x86_64|Linux:amd64) ASSET="lovart-linux-x64" ;;
  *) fail "unsupported platform: ${OS}/${ARCH}" ;;
esac

INSTALL_DIR="${INSTALL_DIR/#\~/$HOME}"
EXTENSION_DIR="${EXTENSION_DIR/#\~/$HOME}"
INSTALL_PATH="${INSTALL_DIR}/lovart"

if [ "$DRY_RUN" -eq 1 ]; then
  if [ "$JSON" -eq 1 ]; then
    emit_json true "dry run" "$ASSET" "$INSTALL_PATH"
  else
    log "Would download ${ASSET} from ${REPO} (${VERSION})"
    log "Would install to ${INSTALL_PATH}"
    log "Would install Lovart Connector extension to ${EXTENSION_DIR}"
    log "Would run: ${INSTALL_PATH} mcp install --clients ${MCP_CLIENTS} --yes"
  fi
  exit 0
fi

command -v unzip >/dev/null 2>&1 || fail "unzip is required to install Lovart Connector extension"

if [ "$YES" -ne 1 ]; then
  printf 'Install Lovart to %s and configure MCP clients "%s"? [y/N] ' "$INSTALL_PATH" "$MCP_CLIENTS"
  read -r answer
  case "$answer" in
    y|Y|yes|YES) ;;
    *) fail "installation cancelled" ;;
  esac
fi

TMP_DIR="$(mktemp -d)"
INSTALL_TMP=""
BACKUP_TMP=""
cleanup() {
  rm -rf "$TMP_DIR"
  if [ -n "$INSTALL_TMP" ]; then
    rm -f "$INSTALL_TMP"
  fi
  if [ -n "$BACKUP_TMP" ]; then
    rm -f "$BACKUP_TMP"
  fi
}
trap cleanup EXIT

release_asset_url() {
  local asset="$1"
  if [ "$VERSION" = "latest" ]; then
    printf 'https://github.com/%s/releases/latest/download/%s\n' "$REPO" "$asset"
  else
    printf 'https://github.com/%s/releases/download/%s/%s\n' "$REPO" "$VERSION" "$asset"
  fi
}

download_release_asset() {
  local pattern="$1"
  local output="$2"
  local url
  url="$(release_asset_url "$pattern")"

  if command -v curl >/dev/null 2>&1; then
    if curl -fL --retry 3 --retry-delay 1 -o "$output" "$url"; then
      return 0
    fi
    rm -f "$output"
    log "Public download failed for ${pattern}; trying authenticated gh fallback..."
  else
    log "curl is not available; trying authenticated gh fallback..."
  fi

  command -v gh >/dev/null 2>&1 || fail "failed to download ${pattern}; install curl for public releases, or install GitHub CLI and run gh auth login for private forks or API-limited access"
  gh auth status >/dev/null 2>&1 || fail "failed to download ${pattern}; public download failed and gh is not authenticated; run gh auth login for private forks or API-limited access"
  if [ "$VERSION" = "latest" ]; then
    gh release download --repo "$REPO" --pattern "$pattern" -O "$output" || fail "failed to download ${pattern}"
  else
    gh release download "$VERSION" --repo "$REPO" --pattern "$pattern" -O "$output" || fail "failed to download ${pattern}"
  fi
}

BIN_TMP="${TMP_DIR}/lovart"
EXT_TMP="${TMP_DIR}/lovart-connector-extension.zip"
SUMS_TMP="${TMP_DIR}/SHA256SUMS"

log "Downloading ${ASSET}..."
download_release_asset "$ASSET" "$BIN_TMP"
download_release_asset "lovart-connector-extension.zip" "$EXT_TMP"
download_release_asset "SHA256SUMS" "$SUMS_TMP"

verify_release_asset() {
  local asset="$1"
  local path="$2"
  local expected_line expected_hash actual_hash
  expected_line="$(grep "  ${asset}$" "$SUMS_TMP" || true)"
  if [ -z "$expected_line" ]; then
    fail "SHA256SUMS does not contain ${asset}"
  fi
  expected_hash="${expected_line%% *}"
  if command -v sha256sum >/dev/null 2>&1; then
    actual_hash="$(sha256sum "$path" | awk '{print $1}')"
  else
    actual_hash="$(shasum -a 256 "$path" | awk '{print $1}')"
  fi
  [ "$expected_hash" = "$actual_hash" ] || fail "checksum mismatch for ${asset}"
}

verify_release_asset "$ASSET" "$BIN_TMP"
verify_release_asset "lovart-connector-extension.zip" "$EXT_TMP"

mkdir -p "$INSTALL_DIR"
if [ -e "$INSTALL_PATH" ]; then
  if [ "$FORCE" -ne 1 ]; then
    fail "${INSTALL_PATH} already exists; rerun with --force"
  fi
  BACKUP_TMP="${INSTALL_PATH}.bak.tmp.$$"
  cp "$INSTALL_PATH" "$BACKUP_TMP"
  mv -f "$BACKUP_TMP" "${INSTALL_PATH}.bak"
  BACKUP_TMP=""
fi

INSTALL_TMP="${INSTALL_PATH}.tmp.$$"
cp "$BIN_TMP" "$INSTALL_TMP"
chmod +x "$INSTALL_TMP"

"$INSTALL_TMP" --version >/dev/null
"$INSTALL_TMP" self-test >/dev/null

mv -f "$INSTALL_TMP" "$INSTALL_PATH"
INSTALL_TMP=""
"$INSTALL_PATH" --version >/dev/null
"$INSTALL_PATH" self-test >/dev/null

rm -rf "$EXTENSION_DIR"
mkdir -p "$EXTENSION_DIR"
unzip -q "$EXT_TMP" -d "$EXTENSION_DIR"

if [ "$MCP_CLIENTS" != "none" ]; then
  MCP_ARGS=("$INSTALL_PATH" "mcp" "install" "--clients" "$MCP_CLIENTS" "--yes")
  if [ "$FORCE" -eq 1 ]; then
    MCP_ARGS+=("--force")
  fi
  MCP_OUTPUT="$("${MCP_ARGS[@]}")" || fail "MCP client configuration command failed"
  if ! printf '%s' "$MCP_OUTPUT" | grep -q '"ok":true'; then
    fail "MCP client configuration failed: ${MCP_OUTPUT}"
  fi
fi

if ! command -v lovart >/dev/null 2>&1 && [[ ":$PATH:" != *":${INSTALL_DIR}:"* ]]; then
  log "Note: ${INSTALL_DIR} is not on PATH. Add it to your shell profile or use ${INSTALL_PATH} directly."
fi

if [ "$JSON" -eq 1 ]; then
  emit_json true "installed" "$ASSET" "$INSTALL_PATH"
else
  log "Installed Lovart at ${INSTALL_PATH}"
  log "Installed Lovart Connector extension at ${EXTENSION_DIR}"
  log "Chrome setup: open chrome://extensions, enable Developer mode, then Load unpacked ${EXTENSION_DIR}"
  log "Run: ${INSTALL_PATH} --version"
fi
