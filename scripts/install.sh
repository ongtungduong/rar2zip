#!/usr/bin/env sh
# install.sh — download the correct rar2zip release asset and install it.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/ongtungduong/rar2zip/main/scripts/install.sh | sh
#   VERSION=v0.2.0 sh install.sh          # pin a version
#   INSTALL_DIR=/usr/local/bin sh install.sh

set -eu

REPO="ongtungduong/rar2zip"
INSTALL_DIR="${INSTALL_DIR:-}"
VERSION="${VERSION:-}"

# ── helpers ──────────────────────────────────────────────────────────────────

info()  { printf '\033[1;32m==> %s\033[0m\n' "$*"; }
warn()  { printf '\033[1;33mwarn: %s\033[0m\n' "$*" >&2; }
fatal() { printf '\033[1;31merror: %s\033[0m\n' "$*" >&2; exit 1; }

need() {
  command -v "$1" >/dev/null 2>&1 || fatal "required tool not found: $1"
}

# ── detect OS / arch ─────────────────────────────────────────────────────────

detect_os() {
  case "$(uname -s)" in
    Linux)  echo linux ;;
    Darwin) echo darwin ;;
    *)      fatal "unsupported OS: $(uname -s)" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo amd64 ;;
    arm64|aarch64) echo arm64 ;;
    *) fatal "unsupported arch: $(uname -m)" ;;
  esac
}

# ── resolve install directory ─────────────────────────────────────────────────

resolve_install_dir() {
  if [ -n "$INSTALL_DIR" ]; then
    echo "$INSTALL_DIR"
    return
  fi
  # Prefer ~/.local/bin (no sudo); fall back to /usr/local/bin.
  local_bin="$HOME/.local/bin"
  if [ -d "$local_bin" ] && echo "$PATH" | grep -q "$local_bin"; then
    echo "$local_bin"
  elif [ -w /usr/local/bin ]; then
    echo /usr/local/bin
  else
    # Create ~/.local/bin and remind user to add it to PATH.
    mkdir -p "$local_bin"
    warn "installed to $local_bin — make sure it is on your PATH"
    echo "$local_bin"
  fi
}

# ── fetch latest version tag ──────────────────────────────────────────────────

latest_version() {
  need curl
  url="https://api.github.com/repos/${REPO}/releases/latest"
  curl -fsSL "$url" \
    | grep '"tag_name"' \
    | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/'
}

# ── download + verify + install ───────────────────────────────────────────────

main() {
  need curl
  need tar

  OS="$(detect_os)"
  ARCH="$(detect_arch)"

  if [ -z "$VERSION" ]; then
    info "Fetching latest release..."
    VERSION="$(latest_version)"
  fi

  info "Installing rar2zip ${VERSION} (${OS}/${ARCH})"

  BASE="rar2zip_${OS}_${ARCH}"
  TARBALL="${BASE}.tar.gz"
  RELEASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"

  WORK_DIR="$(mktemp -d)"
  trap 'rm -rf "$WORK_DIR"' EXIT

  info "Downloading ${TARBALL}..."
  curl -fsSL "${RELEASE_URL}/${TARBALL}" -o "${WORK_DIR}/${TARBALL}"

  # Verify checksum if sha256sum / shasum is available.
  if command -v sha256sum >/dev/null 2>&1 || command -v shasum >/dev/null 2>&1; then
    info "Verifying checksum..."
    curl -fsSL "${RELEASE_URL}/checksums.txt" -o "${WORK_DIR}/checksums.txt"
    cd "$WORK_DIR"
    if command -v sha256sum >/dev/null 2>&1; then
      grep "${TARBALL}" checksums.txt | sha256sum -c -
    else
      grep "${TARBALL}" checksums.txt | shasum -a 256 -c -
    fi
    cd - >/dev/null
  else
    warn "sha256sum/shasum not found — skipping checksum verification"
  fi

  info "Extracting..."
  tar -xzf "${WORK_DIR}/${TARBALL}" -C "$WORK_DIR"

  DEST_DIR="$(resolve_install_dir)"
  mkdir -p "$DEST_DIR"

  info "Installing to ${DEST_DIR}/rar2zip"
  cp "${WORK_DIR}/rar2zip" "${DEST_DIR}/rar2zip"
  chmod +x "${DEST_DIR}/rar2zip"

  info "Done! Run: rar2zip --version"
}

main
