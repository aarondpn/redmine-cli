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

# The result is captured rather than piped straight into tail: a pipeline
# reports the exit status of its last command, so `rails runner ... | tail`
# would report success even when the seeding aborted. With set -e, a failing
# command substitution stops the script here instead.
bootstrap_output=$(docker compose -f "$compose_file" exec -T \
  -e REDMINE_E2E_PASSWORD="$admin_password" \
  "$container_service" \
  bundle exec rails runner '
    Setting.rest_api_enabled = "1"
    # Lets the assignee e2e suite assign issues to groups. Off by default in
    # Redmine and not exposed over REST, so the bootstrap has to set it.
    Setting.issue_group_assignment = "1"
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

    # Seed a free-text issue custom field alongside the list-format one.
    # The list field can only carry its fixed possible_values, so it cannot
    # exercise values containing spaces (issue #155); a string field can.
    notes_field_name = "E2E Notes"
    seeded_notes_field = IssueCustomField.find_or_initialize_by(name: notes_field_name)
    seeded_notes_field.field_format = "string" if seeded_notes_field.field_format.blank?
    seeded_notes_field.is_required = false
    seeded_notes_field.is_filter = false
    seeded_notes_field.searchable = false
    seeded_notes_field.is_for_all = true
    seeded_notes_field.tracker_ids = Tracker.pluck(:id)
    seeded_notes_field.save!

    # Seed a role-restricted time entry custom field. Redmine 7.0 (#44152)
    # started returning "roles" for non-issue custom fields, and this fixture
    # is what makes that observable over the REST API. visible=false is what
    # gives the role restriction meaning: a visible field is visible to every
    # role and reports no roles at all.
    #
    # save! rather than a rescue-and-degrade: role_ids and visible live on the
    # base CustomField class in every supported line (verified on 4.2, 6.1 and
    # 7.0), so a failure here is a real regression. Degrading quietly would
    # surface later as "Redmine did not return roles", blaming the server for
    # a broken fixture.
    time_entry_cf_name = "E2E Billing Code"
    seeded_time_entry_cf = TimeEntryCustomField.find_or_initialize_by(name: time_entry_cf_name)
    seeded_time_entry_cf.field_format = "string" if seeded_time_entry_cf.field_format.blank?
    seeded_time_entry_cf.is_required = false
    seeded_time_entry_cf.visible = false
    restricted_role_ids = Role.givable.pluck(:id).first(1)
    abort("no givable roles to restrict the time entry custom field to") if restricted_role_ids.empty?
    seeded_time_entry_cf.role_ids = restricted_role_ids
    seeded_time_entry_cf.save!

    # Fixtures for the --custom-field input tests (custom_field_input_test.go):
    # a multi-value issue field, a name shared by an issue and a time entry
    # field (type-scoped name resolution), and one writable field per other
    # resource type. None is required: a required time entry field would make
    # every other test that logs time fail. All are visible, so they carry no
    # roles and stay valid under the Redmine 7.0 roles gate.
    def seed_cf(klass, name, format, values, multiple, extra = {})
      cf = klass.find_or_initialize_by(name: name)
      cf.field_format = format if cf.field_format.blank?
      cf.possible_values = values if values && cf.possible_values.blank?
      cf.multiple = multiple
      cf.is_required = false
      cf.visible = true
      extra.each { |k, v| cf.send("#{k}=", v) }
      cf.save!
      cf
    end
    all_trackers = { is_for_all: true, tracker_ids: Tracker.pluck(:id), is_filter: false }
    tags_cf = seed_cf(IssueCustomField, "E2E Tags", "list", %w[Alpha Beta Gamma], true, all_trackers)
    shared_issue_cf = seed_cf(IssueCustomField, "E2E Shared Code", "string", nil, false, all_trackers)
    # Not for all projects: --issue-custom-field has to enable it per project.
    scoped_issue_cf = seed_cf(IssueCustomField, "E2E Project Scoped", "string", nil, false,
                              all_trackers.merge(is_for_all: false))
    shared_entry_cf = seed_cf(TimeEntryCustomField, "E2E Shared Code", "string", nil, false)
    entry_tags_cf = seed_cf(TimeEntryCustomField, "E2E Entry Tags", "list", %w[Billable Onsite Overtime], true)
    project_labels_cf = seed_cf(ProjectCustomField, "E2E Project Labels", "list", %w[red green blue], true)
    version_owner_cf = seed_cf(VersionCustomField, "E2E Version Owner", "string", nil, false)
    user_skills_cf = seed_cf(UserCustomField, "E2E User Skills", "list", %w[Go Ruby SQL], true, { editable: true })
    group_code_cf = seed_cf(GroupCustomField, "E2E Group Code", "string", nil, false)

    puts({
      rest_api_enabled: Setting.rest_api_enabled?,
      issue_group_assignment: Setting.issue_group_assignment?,
      admin_api_key_present: admin.api_key.to_s != "",
      admin_password_set: ENV["REDMINE_E2E_PASSWORD"].to_s != "",
      trackers: Tracker.count,
      statuses: IssueStatus.count,
      seeded_query_id: seeded_query.id,
      seeded_query_name: seeded_query.name,
      seeded_custom_field_id: seeded_custom_field.id,
      seeded_custom_field_name: seeded_custom_field.name,
      seeded_notes_custom_field_id: seeded_notes_field.id,
      seeded_time_entry_custom_field_id: seeded_time_entry_cf.id,
      seeded_time_entry_custom_field_roles: seeded_time_entry_cf.roles.count,
      seeded_multi_issue_custom_field_id: tags_cf.id,
      seeded_shared_issue_custom_field_id: shared_issue_cf.id,
      seeded_shared_time_entry_custom_field_id: shared_entry_cf.id,
      seeded_entry_tags_custom_field_id: entry_tags_cf.id,
      seeded_project_labels_custom_field_id: project_labels_cf.id,
      seeded_version_owner_custom_field_id: version_owner_cf.id,
      seeded_user_skills_custom_field_id: user_skills_cf.id,
      seeded_group_code_custom_field_id: group_code_cf.id,
      seeded_project_scoped_issue_custom_field_id: scoped_issue_cf.id
    }.inspect)
  ' 2>/dev/null)

printf '%s\n' "$bootstrap_output" | tail -n 1

# Seed the "contract world" the contract tests diff against (see
# e2e/seed-contract.rb for why it cannot be created over REST). It is piped in
# over stdin and its output captured for the same reason as above: a failing
# seed must abort the bootstrap, and the fixture gaps it reports (steps that
# depend on Redmine internals) must be visible rather than swallowed.
contract_err=$(mktemp)
if ! contract_output=$(docker compose -f "$compose_file" exec -T "$container_service" \
  bundle exec rails runner - <"$(dirname "$0")/seed-contract.rb" 2>"$contract_err"); then
  echo "contract seed failed:" >&2
  grep -v -E '^(DEPRECATION|W, \[|I, \[)' "$contract_err" | tail -n 40 >&2
  printf '%s\n' "$contract_output" | tail -n 20 >&2
  rm -f "$contract_err"
  exit 1
fi
rm -f "$contract_err"

printf '%s\n' "$contract_output" | tail -n 1
