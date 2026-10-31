#!/bin/bash
set -euo pipefail

REPO="aarondpn/redmine-cli"
BINARY="redmine"
INSTALL_DIR="${REDMINE_INSTALL_DIR:-$HOME/.local/bin}"
# Release tag to install (e.g. v2.14.1). Defaults to the latest release. Not
# REDMINE_VERSION: the official Redmine Docker image sets that to its Redmine
# release.
VERSION="${REDMINE_CLI_VERSION:-}"
# Set to 1 to skip the attestation check, e.g. behind a proxy that blocks
# Sigstore. Not recommended: only the checksums are verified then.
case "${REDMINE_CLI_SKIP_ATTESTATION:-}" in
  1|[Tt][Rr][Uu][Ee]) SKIP_ATTESTATION=1 ;;
  *)                  SKIP_ATTESTATION="" ;;
esac

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
  TAG=$(curl -fsSL --retry 3 "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)
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
curl -fsSL --retry 3 -o "${TMPDIR}/${ARCHIVE}" "$URL"
curl -fsSL --retry 3 -o "${TMPDIR}/checksums.txt" "$CHECKSUMS_URL"

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
manual_verification_hint() {
  printf "To verify this release yourself:\n\n  curl -fsSLO %s\n  gh attestation verify %s --repo %s --signer-workflow %s/.github/workflows/release.yml --source-ref refs/tags/%s\n\n" \
    "$URL" "$ARCHIVE" "$REPO" "$REPO" "$TAG" >&2
}

attestation_notice() {
  warn "$1; skipping attestation verification."
  manual_verification_hint
}

# gh 2.49 added 'gh attestation', but --signer-workflow and --source-ref came
# later. Without them a valid release would fail verification, so treat such a
# gh like a missing one instead of reporting a security incident.
gh_verify_supports_flags() {
  local help
  help=$(gh attestation verify --help 2>&1) || return 1
  grep -Eq -- '^[[:space:]]*--signer-workflow[[:space:]]' <<<"$help" &&
    grep -Eq -- '^[[:space:]]*--source-ref[[:space:]]' <<<"$help"
}

if [ -n "$SKIP_ATTESTATION" ]; then
  warn "REDMINE_CLI_SKIP_ATTESTATION is set; skipping attestation verification (not recommended)."
  warn "The archive was only checked against the release checksums, which come from the same place as the archive; a tampered release would NOT be detected."
  manual_verification_hint
elif ! command -v gh >/dev/null 2>&1; then
  attestation_notice "GitHub CLI (gh) not found (https://cli.github.com)"
elif ! gh attestation --help >/dev/null 2>&1; then
  attestation_notice "Installed GitHub CLI does not support 'gh attestation' (needs gh 2.49+)"
elif [ -z "${GH_TOKEN:-}" ] && [ -z "${GITHUB_TOKEN:-}" ] && ! gh auth status >/dev/null 2>&1; then
  attestation_notice "GitHub CLI is not logged in (run 'gh auth login')"
elif ! gh_verify_supports_flags; then
  attestation_notice "Installed GitHub CLI's 'gh attestation verify' lacks --signer-workflow or --source-ref (update gh)"
else
  info "Verifying attestation..."
  if ! gh attestation verify "${TMPDIR}/${ARCHIVE}" \
      --repo "$REPO" \
      --signer-workflow "${REPO}/.github/workflows/release.yml" \
      --source-ref "refs/tags/${TAG}" >/dev/null; then
    printf "If gh could not reach GitHub or Sigstore (e.g. a proxy blocks them, or gh is logged in to\nGitHub Enterprise Server only), re-run with REDMINE_CLI_SKIP_ATTESTATION=1 to install with\nchecksum verification only (not recommended). If the attestation itself is invalid, do not\ninstall; report it as a security issue. Releases older than v2.11.2 have no attestations;\ninstall those with REDMINE_CLI_SKIP_ATTESTATION=1.\n" >&2
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
REDMINE_CMD="$BINARY"
case ":$PATH:" in
  *":${INSTALL_DIR}:"*) ;;
  *) REDMINE_CMD="${INSTALL_DIR}/${BINARY}"
     printf "\n\033[1;33mwarning:\033[0m %s is not in your PATH.\n" "$INSTALL_DIR"
     printf "Add this to your shell profile:\n\n  export PATH=\"%s:\$PATH\"\n\n" "$INSTALL_DIR" ;;
esac

info "Next: run '${REDMINE_CMD} auth login' to connect to your Redmine server"
info "Docs: https://redmine-cli.dev/getting-started/configuration/"
