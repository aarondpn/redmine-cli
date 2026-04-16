#!/usr/bin/env sh
set -eu

compose_file="${REDMINE_E2E_COMPOSE_FILE:-e2e/compose.yaml}"
container_service="${REDMINE_E2E_REDMINE_SERVICE:-redmine}"
login="${REDMINE_E2E_USERNAME:-admin}"

docker compose -f "$compose_file" exec -T "$container_service" \
  bundle exec rails runner \
  "u = User.find_by(login: \"$login\"); abort(\"user not found: $login\") unless u; puts u.api_key" \
  2>/dev/null | tail -n 1
