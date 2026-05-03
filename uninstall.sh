#!/usr/bin/env bash
# uninstall.sh — remove atob on Linux and macOS
#
# Usage:
#   ./uninstall.sh                 removes from ~/.local/bin
#   ./uninstall.sh /usr/local/bin  removes from a custom directory
#   INSTALL_DIR=/usr/local/bin ./uninstall.sh

set -euo pipefail

BINARY="atob"
INSTALL_DIR="${1:-${INSTALL_DIR:-$HOME/.local/bin}}"
TARGET="$INSTALL_DIR/$BINARY"

if [ -t 1 ]; then
  GREEN="\033[0;32m"; YELLOW="\033[0;33m"; RED="\033[0;31m"; RESET="\033[0m"
else
  GREEN=""; YELLOW=""; RED=""; RESET=""
fi

success() { echo -e "${GREEN}✓${RESET} $*"; }
warn()    { echo -e "${YELLOW}!${RESET} $*"; }
error()   { echo -e "${RED}✗${RESET} $*" >&2; exit 1; }

echo ""
echo "atob uninstaller"
echo "────────────────────────────────"
echo ""

if [ ! -f "$TARGET" ]; then
  warn "atob not found at $TARGET — nothing to remove."
  echo ""
  exit 0
fi

rm -f "$TARGET"
success "Removed $TARGET"
echo ""
