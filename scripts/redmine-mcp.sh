#!/bin/sh
# Launcher for the redmine MCP server used by the plugin manifests.
# GUI-launched hosts often get a minimal PATH, so look in the usual install
# locations when `redmine` is not on PATH.

if command -v redmine >/dev/null 2>&1; then
  exec redmine "$@"
fi

for dir in /opt/homebrew/bin /usr/local/bin "${HOME:-}/.local/bin" "${GOBIN:-}" "${HOME:-}/go/bin"; do
  case "$dir" in
    /*) ;;
    *) continue ;;
  esac
  if [ -f "$dir/redmine" ] && [ -x "$dir/redmine" ]; then
    exec "$dir/redmine" "$@"
  fi
done

cat >&2 <<'MSG'
redmine-cli: the 'redmine' binary was not found on PATH or in the usual install locations.

Install it with one of:
  brew install aarondpn/tap/redmine
  curl -fsSL https://raw.githubusercontent.com/aarondpn/redmine-cli/main/install.sh | bash

Docs: https://redmine-cli.dev/getting-started/installation/
MSG
exit 127
