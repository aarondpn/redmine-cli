#!/usr/bin/env sh
set -eu

base_url="${REDMINE_E2E_BASE_URL:-http://127.0.0.1:3000}"
timeout_seconds="${REDMINE_E2E_TIMEOUT_SECONDS:-300}"

printf 'Waiting for Redmine at %s\n' "$base_url" >&2

start_time="$(date +%s)"

while :; do
  if curl --silent --show-error --fail "$base_url/" >/dev/null 2>&1; then
    printf 'Redmine is ready.\n' >&2
    exit 0
  fi

  now="$(date +%s)"
  if [ $((now - start_time)) -ge "$timeout_seconds" ]; then
    printf 'Timed out waiting for Redmine after %s seconds.\n' "$timeout_seconds" >&2
    exit 1
  fi

  sleep 5
done
