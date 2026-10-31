#!/usr/bin/env sh
# Creates (or renews) the OAuth applications the oauth e2e tests log in with
# and prints one "<client id> <client secret>" line for each, wide app first.
# Prints nothing on Redmine older than 6.1, which has no OAuth provider.
set -eu

compose_file="${REDMINE_E2E_COMPOSE_FILE:-e2e/compose.yaml}"
container_service="${REDMINE_E2E_REDMINE_SERVICE:-redmine}"

docker compose -f "$compose_file" exec -T "$container_service" \
  bundle exec rails runner '
    exit 0 unless defined?(Doorkeeper::Application)
    wide = Redmine::AccessControl.permissions.map { |p| p.name.to_s } + ["admin"]
    apps = {
      "redmine-cli-e2e" => wide.uniq,
      "redmine-cli-e2e-narrow" => %w[view_issues],
    }
    apps.each do |name, scopes|
      app = Doorkeeper::Application.find_or_initialize_by(name: name)
      app.redirect_uri = "http://127.0.0.1/callback"
      app.scopes = scopes.join(" ")
      app.confidential = true
      app.save!
      # The secret is stored hashed: a fresh one is the only way to read it.
      app.renew_secret
      app.save!
      puts "#{app.uid} #{app.plaintext_secret}"
    end
  ' 2>/dev/null | grep -E '^[A-Za-z0-9_-]+ [A-Za-z0-9_-]+$' || true
