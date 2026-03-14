#!/bin/bash
set -euo pipefail

REPO="aarondpn/redmine-cli"
BINARY="redmine"
INSTALL_DIR="/usr/local/bin"

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

info "Downloading ${URL}..."
curl -fsSL -o "${TMPDIR}/${ARCHIVE}" "$URL"

# Extract
tar xzf "${TMPDIR}/${ARCHIVE}" -C "$TMPDIR"
chmod +x "${TMPDIR}/${BINARY}"

# Install
if [ -w "$INSTALL_DIR" ]; then
  mv "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
else
  info "Need sudo to install to ${INSTALL_DIR}"
  sudo mv "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
fi

info "Installed ${BINARY} ${TAG} to ${INSTALL_DIR}/${BINARY}"
