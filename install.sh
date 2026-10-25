#!/bin/bash
set -euo pipefail

REPO="aarondpn/redmine-cli"
BINARY="redmine"
INSTALL_DIR="${REDMINE_INSTALL_DIR:-$HOME/.local/bin}"
# Release tag to install (e.g. v2.14.1). Defaults to the latest release.
VERSION="${REDMINE_VERSION:-}"

info() { printf "\033[1;34m==>\033[0m %s\n" "$*"; }
warn() { printf "\033[1;33mwarning:\033[0m %s\n" "$*" >&2; }
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

if [ -n "$VERSION" ]; then
  # Pinned release: accept "2.14.1" as well as "v2.14.1".
  case "$VERSION" in
    v*) TAG="$VERSION" ;;
    *)  TAG="v${VERSION}" ;;
  esac
  info "Requested version: ${TAG}"
else
  # Get latest release tag
  info "Fetching latest release..."
  TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)
  [ -z "$TAG" ] && error "Could not determine latest release"
  info "Latest version: ${TAG}"
fi

# The tag ends up in URLs and file names; only allow release-tag characters.
# Match the whole string (grep would accept any single matching line of a
# multi-line value).
if ! [[ "$TAG" =~ ^v[0-9A-Za-z][0-9A-Za-z.+-]*$ ]]; then
  error "Invalid release tag: ${TAG}"
fi

# Download
ARCHIVE="redmine-cli-${OS}-${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${TAG}/${ARCHIVE}"
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${TAG}/redmine-cli_${TAG#v}_checksums.txt"

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

# Verify build provenance. The checksums come from the same release as the
# archive, so only the attestation proves the archive was built by this
# repository's release workflow for this tag.
attestation_notice() {
  warn "$1; skipping attestation verification."
  printf "To verify this release yourself:\n\n  curl -fsSLO %s\n  gh attestation verify %s --repo %s --signer-workflow %s/.github/workflows/release.yml --source-ref refs/tags/%s\n\n" \
    "$URL" "$ARCHIVE" "$REPO" "$REPO" "$TAG" >&2
}

if ! command -v gh >/dev/null 2>&1; then
  attestation_notice "GitHub CLI (gh) not found (https://cli.github.com)"
elif ! gh attestation --help >/dev/null 2>&1; then
  attestation_notice "Installed GitHub CLI does not support 'gh attestation' (needs gh 2.49+)"
elif [ -z "${GH_TOKEN:-}" ] && [ -z "${GITHUB_TOKEN:-}" ] && ! gh auth status >/dev/null 2>&1; then
  attestation_notice "GitHub CLI is not logged in (run 'gh auth login')"
else
  info "Verifying attestation..."
  if ! gh attestation verify "${TMPDIR}/${ARCHIVE}" \
      --repo "$REPO" \
      --signer-workflow "${REPO}/.github/workflows/release.yml" \
      --source-ref "refs/tags/${TAG}" >/dev/null; then
    error "Attestation verification failed for ${ARCHIVE}; refusing to install"
  fi
  info "Attestation verified."
fi

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
