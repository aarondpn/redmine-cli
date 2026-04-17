#!/usr/bin/env sh
set -eu

config_path="${1:-${REDMINE_E2E_CONFIG_PATH:-$(pwd)/.redmine-cli.e2e.yaml}}"
base_url="${REDMINE_E2E_BASE_URL:-http://127.0.0.1:3000}"
api_key="${REDMINE_E2E_API_KEY:-}"
profile_name="${REDMINE_E2E_PROFILE_NAME:-local-e2e}"

if [ -z "$api_key" ]; then
  api_key="$(./e2e/admin-api-key.sh)"
fi

cat >"$config_path" <<EOF
active_profile: $profile_name
profiles:
  $profile_name:
    server: $base_url
    api_key: $api_key
    auth_method: apikey
    output_format: json
EOF

printf 'Wrote e2e config to %s\n' "$config_path" >&2
