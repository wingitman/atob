#!/usr/bin/env bash
# install.sh — atob installer for Linux and macOS
#
# Usage:
#   ./install.sh                  install to ~/.local/bin  (no sudo needed)
#   ./install.sh /usr/local/bin   install to a custom directory
#   INSTALL_DIR=/usr/local/bin ./install.sh
#
# One-liner:
#   curl -fsSL https://raw.githubusercontent.com/wingitman/atob/main/install.sh | bash
#
# The script will:
#   1. Check for an existing installation and report its version.
#   2. Build atob from source using the Go toolchain already on PATH,
#      or fall back to downloading a pre-built binary from GitHub Releases.
#   3. Copy the binary to INSTALL_DIR and make it executable.
#   4. Warn if INSTALL_DIR is not in $PATH.
#   5. Print example commands.

set -euo pipefail

REPO="wingitman/atob"
BINARY="atob"
INSTALL_DIR="${1:-${INSTALL_DIR:-$HOME/.local/bin}}"
TMP_DIR="$(mktemp -d)"

# ── colours (suppressed when not a TTY) ────────────────────────────────────────
if [ -t 1 ]; then
  BOLD="\033[1m"; GREEN="\033[0;32m"; YELLOW="\033[0;33m"
  RED="\033[0;31m"; RESET="\033[0m"
else
  BOLD=""; GREEN=""; YELLOW=""; RED=""; RESET=""
fi

info()    { echo -e "${GREEN}›${RESET} $*" >&2; }
warn()    { echo -e "${YELLOW}!${RESET} $*" >&2; }
error()   { echo -e "${RED}✗${RESET} $*" >&2; exit 1; }
success() { echo -e "${GREEN}✓${RESET} $*" >&2; }

cleanup() { rm -rf "$TMP_DIR"; }
trap cleanup EXIT

# ── platform detection ─────────────────────────────────────────────────────────
detect_platform() {
  local os arch
  case "$(uname -s)" in
    Linux)  os="linux"  ;;
    Darwin) os="darwin" ;;
    *)      error "Unsupported OS: $(uname -s). Use install.ps1 on Windows." ;;
  esac
  case "$(uname -m)" in
    x86_64|amd64)   arch="amd64" ;;
    arm64|aarch64)  arch="arm64" ;;
    *)              error "Unsupported architecture: $(uname -m)" ;;
  esac
  echo "${os}-${arch}"
}

# ── existing install check ─────────────────────────────────────────────────────
check_existing() {
  if command -v "$BINARY" &>/dev/null; then
    local existing
    existing="$("$BINARY" --version 2>/dev/null || echo "unknown version")"
    warn "atob is already installed: $existing"
    warn "Continuing will replace it."
    echo ""
  fi
}

# ── build from source ──────────────────────────────────────────────────────────
build_from_source() {
  info "Building atob from source…"

  # Prefer running from the repo root if we are inside it; otherwise clone.
  local src_dir
  src_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" 2>/dev/null && pwd || echo "")"
  if [ -z "$src_dir" ] || [ ! -f "$src_dir/go.mod" ]; then
    info "Cloning repository…"
    git clone --depth=1 "https://github.com/$REPO.git" "$TMP_DIR/src"
    src_dir="$TMP_DIR/src"
  fi

  local version buildtime
  version="$(git -C "$src_dir" describe --tags --always --dirty 2>/dev/null || echo "dev")"
  buildtime="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

  (
    cd "$src_dir"
    go build \
      -ldflags="-s -w -X main.version=$version -X main.buildTime=$buildtime" \
      -o "$TMP_DIR/$BINARY" \
      .
  )
  echo "$TMP_DIR/$BINARY"
}

# ── download pre-built binary ──────────────────────────────────────────────────
download_binary() {
  local platform="$1"
  local tag

  info "Fetching latest release tag…"
  tag="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
        | grep '"tag_name"' | head -1 \
        | sed 's/.*"tag_name": *"\(.*\)".*/\1/')"
  if [ -z "$tag" ]; then
    error "Could not determine latest release. Check https://github.com/$REPO/releases"
  fi

  local filename="${BINARY}-${platform}"
  local url="https://github.com/$REPO/releases/download/$tag/$filename"
  info "Downloading $filename ($tag)…"
  curl -fsSL --progress-bar -o "$TMP_DIR/$BINARY" "$url" \
    || error "Download failed. Verify that release assets exist for $platform at:\n  $url"
  echo "$TMP_DIR/$BINARY"
}

# ── main ───────────────────────────────────────────────────────────────────────
main() {
  echo ""
  echo -e "${BOLD}atob installer${RESET}"
  echo "────────────────────────────────"
  echo ""

  check_existing

  local platform
  platform="$(detect_platform)"
  info "Platform:    $platform"
  info "Install dir: $INSTALL_DIR"
  echo ""

  local binary_path
  if command -v go &>/dev/null; then
    info "Go toolchain found: $(go version)"
    binary_path="$(build_from_source)"
  else
    warn "Go not found — downloading pre-built binary."
    warn "To build from source, install Go: https://go.dev/dl/"
    echo ""
    binary_path="$(download_binary "$platform")"
  fi

  mkdir -p "$INSTALL_DIR"
  cp "$binary_path" "$INSTALL_DIR/$BINARY"
  chmod +x "$INSTALL_DIR/$BINARY"
  success "atob installed → $INSTALL_DIR/$BINARY"
  echo ""

  # ── PATH check ──────────────────────────────────────────────────────────────
  if ! echo "$PATH" | tr ':' '\n' | grep -qx "$INSTALL_DIR"; then
    warn "$INSTALL_DIR is not in your \$PATH."
    warn "Add this line to your shell profile (~/.bashrc, ~/.zshrc, etc.):"
    echo ""
    echo "    export PATH=\"\$HOME/.local/bin:\$PATH\""
    echo ""
  fi

  # ── version confirmation ─────────────────────────────────────────────────────
  if command -v "$BINARY" &>/dev/null; then
    success "$("$BINARY" --version 2>/dev/null || true)"
  fi

  echo ""
  echo -e "${BOLD}Next steps:${RESET}"
  echo ""
  echo "  Interactive TUI (no arguments):"
  echo "    atob"
  echo ""
  echo "  Open TUI with a file pre-loaded:"
  echo "    atob ./myfile.json"
  echo "    atob /usr/bin/ls"
  echo ""
  echo "  CLI usage:"
  echo "    echo 'hello world' | atob base64"
  echo "    atob '{\"a\":1}' yaml"
  echo "    atob list"
  echo ""
  echo "  Config file (created on first launch):"
  echo "    ~/.config/delbysoft/atob.toml"
  echo ""
  echo "  Neovim plugin: https://github.com/wingitman/atob.nvim"
  echo ""
}

main "$@"
