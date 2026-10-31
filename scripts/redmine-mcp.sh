#!/bin/sh
# Launcher for the redmine MCP server, for MCP host configs to use as the
# command (sh redmine-mcp.sh mcp serve). GUI-launched hosts often get a
# minimal PATH, so look in the usual install locations when `redmine` is not
# on PATH.

if command -v redmine >/dev/null 2>&1; then
  exec redmine "$@"
fi

# _REDMINE_MCP_TEST_SYSTEM_DIRS replaces the fixed system directories. It is
# for the tests only and not a supported setting.
for dir in ${_REDMINE_MCP_TEST_SYSTEM_DIRS:-/opt/homebrew/bin /usr/local/bin} "${HOME:-}/.local/bin" "${GOBIN:-}" "${HOME:-}/go/bin"; do
  case "$dir" in
    /*) ;;
    *) continue ;;
  esac
  if [ -f "$dir/redmine" ] && [ -x "$dir/redmine" ]; then
    exec "$dir/redmine" "$@"
  fi
done

# Builtins only: PATH may not contain the directory with cat.
printf '%s\n' \
  "redmine-cli: the 'redmine' binary was not found on PATH or in the usual install locations." \
  "" \
  "Install it with one of:" \
  "  brew install aarondpn/tap/redmine" \
  "  curl -fsSL https://raw.githubusercontent.com/aarondpn/redmine-cli/main/install.sh | bash" \
  "" \
  "Docs: https://redmine-cli.dev/getting-started/installation/" >&2
exit 127
