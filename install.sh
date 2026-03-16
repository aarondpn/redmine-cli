#!/bin/bash
set -euo pipefail

REPO="aarondpn/redmine-cli"
BINARY="redmine"
INSTALL_DIR="${REDMINE_INSTALL_DIR:-$HOME/.local/bin}"

info() { printf "\033[1;34m==>\033[0m %s\n" "$*"; }
error() { printf "\033[1;31merror:\033[0m %s\n" "$*" >&2; exit 1; }

# Detect OS
OS="$(uname -s)"
case "$OS" in
  Linux*)  OS=linux ;;
  Darwin*) OS=darwin ;;
  *)       error "Unsupported OS: $OS. Download manually from https://github.com/$REPO/releases" ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64)  ARCH=amd64 ;;
  aarch64|arm64)  ARCH=arm64 ;;
  *)              error "Unsupported architecture: $ARCH" ;;
esac

info "Detected platform: ${OS}/${ARCH}"

# Get latest release tag
info "Fetching latest release..."
TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)
[ -z "$TAG" ] && error "Could not determine latest release"
info "Latest version: ${TAG}"

# Download
ARCHIVE="redmine-${OS}-${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${TAG}/${ARCHIVE}"
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${TAG}/checksums.txt"

info "Downloading ${URL}..."
curl -fsSL -o "${TMPDIR}/${ARCHIVE}" "$URL"
curl -fsSL -o "${TMPDIR}/checksums.txt" "$CHECKSUMS_URL"

# Verify checksum
info "Verifying checksum..."
EXPECTED=$(grep "  ${ARCHIVE}$" "${TMPDIR}/checksums.txt" | cut -d' ' -f1)
[ -z "$EXPECTED" ] && error "No checksum found for ${ARCHIVE}"

if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL=$(sha256sum "${TMPDIR}/${ARCHIVE}" | cut -d' ' -f1)
elif command -v shasum >/dev/null 2>&1; then
  ACTUAL=$(shasum -a 256 "${TMPDIR}/${ARCHIVE}" | cut -d' ' -f1)
else
  error "Neither sha256sum nor shasum found — cannot verify checksum"
fi

[ "$ACTUAL" != "$EXPECTED" ] && error "Checksum mismatch: expected ${EXPECTED}, got ${ACTUAL}"
info "Checksum verified."

# Extract
tar xzf "${TMPDIR}/${ARCHIVE}" -C "$TMPDIR"
chmod +x "${TMPDIR}/${BINARY}"

# Install
mkdir -p "$INSTALL_DIR"
mv "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"

info "Installed ${BINARY} ${TAG} to ${INSTALL_DIR}/${BINARY}"

# Check if INSTALL_DIR is in PATH
case ":$PATH:" in
  *":${INSTALL_DIR}:"*) ;;
  *) printf "\n\033[1;33mwarning:\033[0m %s is not in your PATH.\n" "$INSTALL_DIR"
     printf "Add this to your shell profile:\n\n  export PATH=\"%s:\$PATH\"\n\n" "$INSTALL_DIR" ;;
esac
