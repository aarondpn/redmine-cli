# Seeds the "contract world" the e2e contract tests (e2e/contract_*_test.go)
# diff against. It is piped into `rails runner -` by bootstrap-redmine.sh and
# must therefore run unchanged on every supported Redmine line (Ruby 2.7 /
# Rails 5.2 up to Ruby 3.x / Rails 8): plain syntax only, and capabilities are
# probed (column_names, defined?) rather than inferred from the version.
#
# Why this lives in Ruby: most of what makes Redmine emit its newer JSON keys
# cannot be created over REST (journal edits, reactions, two-factor, auth
# sources, changesets, custom field values on versions/groups/users/
# activities, ...). A contract diff over a payload that never contains such a
# key passes vacuously, so the world has to contain one of everything.
#
# Everything uses save!/create! so a broken fixture aborts the bootstrap
# instead of surfacing later as "Redmine did not return X". The only
# exceptions are the steps listed in `gaps`: each depends on Redmine internals
# that differ between lines, and a failure there is reported (and checked by
# the coverage test in e2e/contract_gates_test.go) rather than hidden.
#
# Idempotent: every object is looked up by a fixed name first. Fixed names are
# what the tests use to find the world again.

admin = User.find_by(login: "admin")
abort("admin user not found") unless admin
User.current = admin
# The seed edits dozens of issues; nothing here should try to send mail.
ActionMailer::Base.perform_deliveries = false

gaps = {}

def seed_cf(klass, name, attrs = {})
  cf = klass.find_or_initialize_by(name: name)
  cf.field_format = "string" if cf.field_format.blank?
  cf.is_required = false
  cf.visible = true if cf.respond_to?(:visible=) && attrs[:visible].nil?
  attrs.each { |k, v| cf.send("#{k}=", v) }
  cf.save!
  cf
end

def set_cf(obj, cf, value)
  obj.custom_field_values = { cf.id.to_s => value }
  obj.save!
end

Setting.rest_api_enabled = "1"
Setting.issue_group_assignment = "1"
# Makes avatar_url (Redmine 5.0+) non-null without any network access: the
# URL is only built, never fetched.
Setting.gravatar_enabled = "1"
Setting.reactions_enabled = "1" if Setting.available_settings.key?("reactions_enabled")

role = Role.givable.order(:id).first
second_role = Role.givable.order(:id).offset(1).first || role
abort("no givable role") unless role

# --- custom fields on every customizable type the API renders ---------------
version_cf = seed_cf(VersionCustomField, "E2E Contract Version CF")
group_cf = seed_cf(GroupCustomField, "E2E Contract Group CF")
user_cf = seed_cf(UserCustomField, "E2E Contract User CF")
entry_cf = seed_cf(TimeEntryCustomField, "E2E Contract Entry CF")
activity_cf = seed_cf(TimeEntryActivityCustomField, "E2E Contract Activity CF")
issue_cf = seed_cf(IssueCustomField, "E2E Contract Issue CF", is_for_all: true, tracker_ids: Tracker.pluck(:id))

# --- project ------------------------------------------------------------------
project = Project.find_or_initialize_by(identifier: "e2e-contract")
project.name = "E2E Contract"
project.is_public = true
project.enabled_module_names = %w[issue_tracking time_tracking wiki repository files news documents]
project.tracker_ids = Tracker.pluck(:id)
project.issue_custom_field_ids = [issue_cf.id]
project.save!

category = IssueCategory.find_or_create_by!(project: project, name: "Contract Category") { |c| c.assigned_to = admin }

# --- users and groups -----------------------------------------------------------
user = User.find_by(login: "e2econtract")
unless user
  user = User.new(login: "e2econtract", firstname: "Contract", lastname: "User", mail: "e2econtract@example.net")
  user.password = user.password_confirmation = "contract-pass-123"
  user.language = "en"
  user.must_change_passwd = false
  user.save!
end
set_cf(user, user_cf, "user cf value")

group = Group.find_by(lastname: "E2E Contract Group")
unless group
  group = Group.new(lastname: "E2E Contract Group")
  group.save!
end
group.users << user unless group.users.include?(user)
set_cf(group, group_cf, "group cf value")

# A direct membership plus a membership inherited through the group gives the
# user a role with "inherited": true next to one without.
direct = Member.find_by(project_id: project.id, user_id: user.id)
unless direct
  direct = Member.new(project: project, principal: user)
  direct.role_ids = [role.id]
  direct.save!
end
unless Member.exists?(project_id: project.id, user_id: group.id)
  m = Member.new(project: project, principal: group)
  m.role_ids = [second_role.id]
  m.save!
end

# Account details Redmine only renders when they are non-null.
user.update_column(:twofa_scheme, "totp") if User.column_names.include?("twofa_scheme")
user.update_column(:passwd_changed_on, Time.now.utc) if User.column_names.include?("passwd_changed_on")
if User.column_names.include?("auth_source_id")
  source = AuthSourceLdap.find_by(name: "E2E Contract LDAP")
  source ||= AuthSourceLdap.create!(
    name: "E2E Contract LDAP", host: "ldap.invalid", port: 389, base_dn: "dc=example,dc=net",
    attr_login: "uid", attr_firstname: "givenName", attr_lastname: "sn", attr_mail: "mail"
  )
  user.update_column(:auth_source_id, source.id)
end

# Builtin principals as project members (membership keys group_anonymous /
# group_non_member). Not creatable on every line.
begin
  [GroupAnonymous, GroupNonMember].map { |k| k.respond_to?(:instance) ? k.instance : k.load_instance }.each do |builtin|
    next if Member.exists?(project_id: project.id, user_id: builtin.id)
    m = Member.new(project: project, principal: builtin)
    m.role_ids = [role.id]
    m.save!
  end
rescue StandardError => e
  gaps["builtin_group_members"] = e.message
end

# --- versions -------------------------------------------------------------------
version = Version.find_or_initialize_by(project_id: project.id, name: "E2E Contract v1")
version.description = "contract version"
version.due_date = Date.today + 30
version.sharing = "none"
version.save!
set_cf(version, version_cf, "version cf value")

# --- issues -----------------------------------------------------------------------
open_status = IssueStatus.where(is_closed: false).order(:position).first
closed_status = IssueStatus.where(is_closed: true).order(:position).first
tracker = project.trackers.first
priority = IssuePriority.active.first || IssuePriority.first

def seed_issue(project, tracker, status, priority, author, subject, attrs = {})
  issue = Issue.find_by(project_id: project.id, subject: subject)
  return issue if issue
  issue = Issue.new(project: project, tracker: tracker, status: status, priority: priority,
                    author: author, subject: subject)
  attrs.each { |k, v| issue.send("#{k}=", v) }
  issue.save!
  # Redmine 5.1+ bumps lock_version on create; later save!/update! calls on
  # the in-memory object would raise StaleObjectError without a reload.
  issue.reload
end

parent = seed_issue(project, tracker, open_status, priority, admin, "Contract parent",
                    description: "parent description", estimated_hours: 8, category: category,
                    fixed_version: version, assigned_to: user, start_date: Date.today,
                    due_date: Date.today + 7, done_ratio: 30)
child = seed_issue(project, tracker, open_status, priority, admin, "Contract child",
                   parent_issue_id: parent.id, estimated_hours: 4, fixed_version: version)
grandchild = seed_issue(project, tracker, open_status, priority, admin, "Contract grandchild",
                        parent_issue_id: child.id, estimated_hours: 2)
leaf = seed_issue(project, tracker, open_status, priority, admin, "Contract leaf")
private_issue = seed_issue(project, tracker, open_status, priority, admin, "Contract private", is_private: true)
closed = seed_issue(project, tracker, open_status, priority, admin, "Contract closed",
                    start_date: Date.today - 3, due_date: Date.today - 1)
parent.reload

# The custom field value goes through a separate save so it also works for
# issues that already exist.
set_cf(parent, issue_cf, "contract issue cf")

# Journals: a note, a private note, a status change, and an edited note.
if closed.journals.empty?
  closed.init_journal(admin, "closing this one")
  closed.status = closed_status
  closed.save!
end
# Notes are written as Journal rows directly: saving an issue that has no
# attribute changes does not reliably create a note-only journal on every line.
if parent.journals.where.not(notes: [nil, ""]).empty?
  Journal.create!(journalized: parent, user: admin, notes: "first note")
  Journal.create!(journalized: parent, user: admin, notes: "a private note", private_notes: true)
  parent.init_journal(admin, "")
  parent.done_ratio = 40
  parent.save!
end
if Journal.column_names.include?("updated_by_id")
  edited = parent.journals.where.not(notes: [nil, ""]).order(:id).first
  edited.notes = "first note (edited)"
  edited.updated_by = admin
  edited.save!
end

# The Reaction model exists from Redmine 6.1 on; which release renders it in
# the API is recorded in the gate table (e2e/contract_gates_test.go).
if defined?(Reaction)
  Reaction.find_or_create_by!(reactable: parent, user: admin)
  note = parent.journals.where.not(notes: [nil, ""]).order(:id).first
  Reaction.find_or_create_by!(reactable: note, user: admin)
end

parent.add_watcher(admin) unless parent.watched_by?(admin)

# Relations, including a delay, which only "precedes" carries.
unless IssueRelation.exists?(issue_from_id: leaf.id, issue_to_id: closed.id)
  leaf.update!(start_date: Date.today, due_date: Date.today + 1)
  IssueRelation.create!(issue_from: leaf, issue_to: closed, relation_type: "precedes", delay: 2)
end
unless IssueRelation.exists?(issue_from_id: leaf.id, issue_to_id: private_issue.id)
  IssueRelation.create!(issue_from: leaf, issue_to: private_issue, relation_type: "relates")
end

# Attachments on an issue and as a project file.
def seed_attachment(container, filename, author)
  existing = Attachment.find_by(container_id: container.id, container_type: container.class.base_class.name, filename: filename)
  return existing if existing
  a = Attachment.new(file: StringIO.new("contract attachment body"))
  a.filename = filename
  a.content_type = "text/plain"
  a.author = author
  a.container = container
  a.description = "contract attachment"
  a.save!
  a
end
seed_attachment(parent, "contract-issue.txt", admin)
seed_attachment(project, "contract-file.txt", admin)

# This script runs as root (docker exec) while the server runs as the redmine
# user. The attachment directories created above would stay root-owned and the
# server could no longer store uploads in them, failing every later upload
# test with a 500. Hand them back to the owner of the storage root.
if Process.uid == 0
  owner = File.stat(Attachment.storage_path)
  FileUtils.chown_R(owner.uid, owner.gid, Attachment.storage_path)
end

# --- time entries -------------------------------------------------------------------
activity = TimeEntryActivity.shared.active.first || TimeEntryActivity.first
set_cf(activity, activity_cf, "activity cf value") if activity

def seed_time(project, issue, user, activity, hours, comments, cf, cf_value)
  return if TimeEntry.exists?(project_id: project.id, comments: comments)
  te = TimeEntry.new(project: project, issue: issue, user: user, activity: activity,
                     hours: hours, spent_on: Date.today, comments: comments)
  te.custom_field_values = { cf.id.to_s => cf_value }
  te.save!
end
seed_time(project, parent, admin, activity, 1.5, "contract time on parent", entry_cf, "entry cf value")
seed_time(project, child, admin, activity, 2.25, "contract time on child", entry_cf, "entry cf value")
seed_time(project, nil, admin, activity, 0.5, "contract time on project", entry_cf, "entry cf value")

# --- statuses -------------------------------------------------------------------------
if IssueStatus.column_names.include?("description")
  open_status.update!(description: "contract status description") if open_status.description.blank?
end

# --- wiki -----------------------------------------------------------------------------
wiki = project.wiki || Wiki.create!(project: project, start_page: "Wiki")
unless wiki.find_page("Contract Parent")
  page = WikiPage.new(wiki: wiki, title: "Contract Parent")
  page.content = WikiContent.new(text: "parent page", author: admin)
  page.save!
end
unless wiki.find_page("Contract Child")
  page = WikiPage.new(wiki: wiki, title: "Contract Child")
  page.parent_id = wiki.find_page("Contract Parent").id if page.respond_to?(:parent_id=)
  page.content = WikiContent.new(text: "child page", author: admin)
  page.save!
end

# --- changesets -----------------------------------------------------------------------
# Rows are written by hand: the repository path never has to exist because
# nothing in the e2e run fetches from it.
begin
  repo = project.repositories.first
  unless repo
    repo = Repository::Git.new(project: project, url: "/tmp/e2e-contract.git", identifier: "e2e", is_default: true)
    repo.save!
  end
  cs = repo.changesets.find_by(revision: "e2e0000000000000000000000000000000contract")
  cs ||= Changeset.create!(repository: repo, revision: "e2e0000000000000000000000000000000contract",
                           scmid: "e2e0000000000000000000000000000000contract", committer: "Contract Dev",
                           committed_on: Time.now.utc, comments: "refs ##{parent.id} contract commit")
  cs.issues << parent unless cs.issues.include?(parent)
rescue StandardError => e
  gaps["changesets"] = e.message
end

puts({
  project_id: project.id,
  version_id: version.id,
  parent_id: parent.id,
  child_id: child.id,
  grandchild_id: grandchild.id,
  leaf_id: leaf.id,
  closed_id: closed.id,
  user_id: user.id,
  group_id: group.id,
  gaps: gaps
}.inspect)
