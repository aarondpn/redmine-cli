#!/usr/bin/env sh
set -eu

compose_file="${REDMINE_E2E_COMPOSE_FILE:-e2e/compose.yaml}"
container_service="${REDMINE_E2E_REDMINE_SERVICE:-redmine}"
# Optional: when set, the admin password is reset to this value and the
# must_change_passwd flag is cleared. Used by the basic-auth e2e tests.
admin_password="${REDMINE_E2E_PASSWORD:-}"

if docker compose -f "$compose_file" exec -T "$container_service" \
  bundle exec rails runner 'exit((Tracker.count == 0 || IssueStatus.count == 0) ? 0 : 1)' \
  >/dev/null 2>&1; then
  printf 'Loading Redmine default data...\n' >&2
  docker compose -f "$compose_file" exec -T "$container_service" \
    sh -lc 'REDMINE_LANG=${REDMINE_LANG:-en} bundle exec rake redmine:load_default_data' \
    >/dev/null
fi

docker compose -f "$compose_file" exec -T \
  -e REDMINE_E2E_PASSWORD="$admin_password" \
  "$container_service" \
  bundle exec rails runner '
    Setting.rest_api_enabled = "1"
    admin = User.find_by(login: "admin")
    abort("admin user not found") unless admin
    if ENV["REDMINE_E2E_PASSWORD"].to_s != ""
      admin.password = admin.password_confirmation = ENV["REDMINE_E2E_PASSWORD"]
      admin.must_change_passwd = false
      admin.save!
    end

    # Seed a public, global saved query so the queries e2e suite has a stable
    # fixture. Redmine has no REST endpoint for creating queries, so the
    # bootstrap is the only place this can live without re-implementing
    # session auth + CSRF in the test harness.
    query_name = "E2E All Open Issues"
    seeded_query = IssueQuery.find_or_initialize_by(name: query_name)
    if seeded_query.new_record?
      seeded_query.user = admin
      seeded_query.visibility = IssueQuery::VISIBILITY_PUBLIC
      seeded_query.filters = { "status_id" => { operator: "o", values: [""] } }
      seeded_query.column_names = [:tracker, :status, :priority, :subject, :assigned_to, :updated_on]
      seeded_query.save!
    end

    # Seed an issue custom field so the custom-fields e2e suite has a stable
    # fixture. Redmine has no REST endpoint for creating custom field
    # definitions, so the bootstrap is the only place this can live.
    # is_for_all is enforced on every run (not just new records) so the field
    # applies automatically to projects created on the fly by e2e tests.
    custom_field_name = "E2E Severity"
    seeded_custom_field = IssueCustomField.find_or_initialize_by(name: custom_field_name)
    seeded_custom_field.field_format = "list" if seeded_custom_field.field_format.blank?
    seeded_custom_field.possible_values = ["Low", "Medium", "High"] if seeded_custom_field.possible_values.blank?
    seeded_custom_field.is_required = false
    seeded_custom_field.is_filter = true
    seeded_custom_field.searchable = false
    seeded_custom_field.is_for_all = true
    seeded_custom_field.tracker_ids = Tracker.pluck(:id)
    seeded_custom_field.save!

    # Seed a role-restricted time entry custom field. Redmine 7.0 (#44152)
    # started returning "roles" for non-issue custom fields, and this fixture
    # is what makes that observable over the REST API. Older lines may reject
    # role restrictions on non-issue fields, so the save falls back to a plain
    # visible field and the assertion is version-gated on the Go side.
    time_entry_cf_name = "E2E Billing Code"
    seeded_time_entry_cf = TimeEntryCustomField.find_or_initialize_by(name: time_entry_cf_name)
    seeded_time_entry_cf.field_format = "string" if seeded_time_entry_cf.field_format.blank?
    seeded_time_entry_cf.is_required = false
    seeded_time_entry_cf.visible = false
    seeded_time_entry_cf.role_ids = Role.givable.pluck(:id).first(1)
    unless seeded_time_entry_cf.save
      seeded_time_entry_cf.visible = true
      seeded_time_entry_cf.role_ids = []
      seeded_time_entry_cf.save!
    end

    puts({
      rest_api_enabled: Setting.rest_api_enabled?,
      admin_api_key_present: admin.api_key.to_s != "",
      admin_password_set: ENV["REDMINE_E2E_PASSWORD"].to_s != "",
      trackers: Tracker.count,
      statuses: IssueStatus.count,
      seeded_query_id: seeded_query.id,
      seeded_query_name: seeded_query.name,
      seeded_custom_field_id: seeded_custom_field.id,
      seeded_custom_field_name: seeded_custom_field.name,
      seeded_time_entry_custom_field_id: seeded_time_entry_cf.id,
      seeded_time_entry_custom_field_roles: seeded_time_entry_cf.roles.count
    }.inspect)
  ' 2>/dev/null | tail -n 1
