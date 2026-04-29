# MCP Tools

Generated from annotated ops functions. Do not edit by hand.

## groups

### `add_group_user`

Add a user to a Redmine group. Requires --enable-writes.

- Mode: `write`
- Source: `ops.AddGroupUser`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `group_id` | `int` | yes | Group ID. |
| `user_id` | `int` | yes | User ID. |

### `create_group`

Create a new Redmine group. Requires --enable-writes.

- Mode: `write`
- Source: `ops.CreateGroup`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `name` | `string` | yes | Group name. |
| `user_ids` | `[]int` | no | Optional list of user IDs to add as group members. |

### `delete_group`

Delete a Redmine group. Destructive. Requires --enable-writes.

- Mode: `write`
- Source: `ops.DeleteGroup`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `id` | `int` | yes | Group ID to delete. Destructive. |

### `get_group`

Fetch a single Redmine group by ID.

- Mode: `read`
- Source: `ops.GetGroup`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `id` | `int` | yes | Numeric group ID. |
| `includes` | `[]string` | no | Extra sections to include: 'users', 'memberships'. |

### `list_groups`

List Redmine groups.

- Mode: `read`
- Source: `ops.ListGroups`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `limit` | `int` | no | Max results to return. Defaults to 50 when omitted. |
| `offset` | `int` | no | Number of leading results to skip (pagination). |

### `remove_group_user`

Remove a user from a Redmine group. Requires --enable-writes.

- Mode: `write`
- Source: `ops.RemoveGroupUser`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `group_id` | `int` | yes | Group ID. |
| `user_id` | `int` | yes | User ID. |

### `update_group`

Update an existing Redmine group. Requires --enable-writes.

- Mode: `write`
- Source: `ops.UpdateGroup`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `id` | `int` | yes | Group ID to update. |
| `name` | `*string` | no | New group name. |
| `user_ids` | `*[]int` | no | Replacement set of user IDs. Pass an empty list to remove all members. |

## issues

### `add_issue_comment`

Add a journal comment to an existing issue. Requires --enable-writes.

- Mode: `write`
- Source: `ops.AddIssueComment`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `id` | `int` | yes | Issue ID to comment on. |
| `notes` | `string` | yes | Comment body (journal note). |
| `private_notes` | `bool` | no | Mark the note as private. |

### `assign_issue`

Assign an issue to a user. Requires --enable-writes.

- Mode: `write`
- Source: `ops.AssignIssue`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `id` | `int` | yes | Issue ID to assign. |
| `assignee_id` | `int` | yes | User ID to assign. Must be > 0 (use update_issue to unassign). |

### `close_issue`

Close an issue by setting its status to the first closed status. Requires --enable-writes.

- Mode: `write`
- Source: `ops.CloseIssue`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `id` | `int` | yes | Issue ID to close. |
| `notes` | `string` | no | Optional journal note to attach. |

### `create_issue`

Create a new Redmine issue. Requires --enable-writes.

- Mode: `write`
- Source: `ops.CreateIssue`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `project_id` | `int` | yes | Numeric project ID to create the issue in. |
| `subject` | `string` | yes | Issue subject (title). |
| `description` | `string` | no | Issue body (Textile or Markdown depending on the Redmine configuration). |
| `tracker_id` | `int` | no | Tracker ID (Bug, Feature, ...). Use list_trackers to discover. |
| `status_id` | `int` | no | Initial status ID. Use list_statuses to discover. |
| `priority_id` | `int` | no | Priority ID. |
| `assigned_to_id` | `int` | no | User ID of the assignee. |
| `category_id` | `int` | no | Issue category ID. Use list_categories to discover. |
| `fixed_version_id` | `int` | no | Fixed version (milestone) ID. |
| `parent_issue_id` | `int` | no | Parent issue ID for sub-tasks. |
| `estimated_hours` | `float64` | no | Estimated effort in hours. |
| `is_private` | `*bool` | no | Mark the issue as private. |

### `delete_issue`

Delete a Redmine issue. Destructive. Requires --enable-writes.

- Mode: `write`
- Source: `ops.DeleteIssue`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `id` | `int` | yes | Issue ID to delete. |

### `get_issue`

Fetch a single Redmine issue by ID.

- Mode: `read`
- Source: `ops.GetIssue`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `id` | `int` | yes | Numeric issue ID. |
| `includes` | `[]string` | no | Extra sections to include: journals, attachments, relations, children, watchers. |

### `list_issues`

List Redmine issues matching the given filters.

- Mode: `read`
- Source: `ops.ListIssues`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `project_id` | `string` | no | Project identifier or numeric ID to filter by. |
| `tracker_id` | `int` | no | Tracker ID (use list_trackers to discover). |
| `status_id` | `string` | no | Status filter: 'open', 'closed', '*', or a numeric status ID. |
| `assigned_to_id` | `string` | no | Assignee: numeric user ID or 'me'. |
| `fixed_version_id` | `int` | no | Fixed version (milestone) ID. |
| `sort` | `string` | no | Sort expression, e.g. 'updated_on:desc'. |
| `includes` | `[]string` | no | Extra fields to include: attachments, relations, children, watchers. |
| `limit` | `int` | no | Max results to return. Defaults to 50 when omitted. |
| `offset` | `int` | no | Number of leading results to skip (pagination). |

### `reopen_issue`

Reopen a closed issue by setting its status to the first open status. Requires --enable-writes.

- Mode: `write`
- Source: `ops.ReopenIssue`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `id` | `int` | yes | Issue ID to reopen. |
| `notes` | `string` | no | Optional journal note to attach. |

### `update_issue`

Update fields on an existing Redmine issue. Requires --enable-writes.

- Mode: `write`
- Source: `ops.UpdateIssue`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `id` | `int` | yes | Issue ID to update. |
| `subject` | `*string` | no | New subject (title). |
| `description` | `*string` | no | New description body. |
| `tracker_id` | `*int` | no | New tracker ID. |
| `status_id` | `*int` | no | New status ID. |
| `priority_id` | `*int` | no | New priority ID. |
| `assigned_to_id` | `*int` | no | Positive user ID to assign the issue to. |
| `category_id` | `*int` | no | Issue category ID. |
| `fixed_version_id` | `*int` | no | Fixed version ID. |
| `parent_issue_id` | `*int` | no | Parent issue ID for sub-tasks. Set to 0 to remove the parent. |
| `done_ratio` | `*int` | no | Completion percentage (0-100). |
| `estimated_hours` | `*float64` | no | Estimated effort in hours. |
| `due_date` | `*string` | no | Due date (YYYY-MM-DD). |
| `notes` | `*string` | no | Journal note to attach to the update. |
| `is_private` | `*bool` | no | Toggle issue privacy. |

## memberships

### `create_membership`

Add a user to a project with the given roles. Requires --enable-writes.

- Mode: `write`
- Source: `ops.CreateMembership`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `project_id` | `string` | yes | Project identifier or numeric ID. |
| `user_id` | `int` | yes | Numeric user ID to add as a member. |
| `role_ids` | `[]int` | yes | One or more role IDs to grant. |

### `delete_membership`

Remove a project membership. Destructive. Requires --enable-writes.

- Mode: `write`
- Source: `ops.DeleteMembership`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `id` | `int` | yes | Numeric membership ID to delete. Destructive. |

### `get_membership`

Fetch a single project membership by ID.

- Mode: `read`
- Source: `ops.GetMembership`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `id` | `int` | yes | Numeric membership ID. |

### `list_memberships`

List memberships for a project.

- Mode: `read`
- Source: `ops.ListMemberships`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `project_id` | `string` | yes | Project identifier or numeric ID. |
| `limit` | `int` | no | Max results to return. Defaults to 50 when omitted. |
| `offset` | `int` | no | Number of leading results to skip. |

### `update_membership`

Replace the roles on a project membership. Requires --enable-writes.

- Mode: `write`
- Source: `ops.UpdateMembership`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `id` | `int` | yes | Numeric membership ID. |
| `role_ids` | `[]int` | yes | Replacement set of role IDs. |

## meta

### `create_version`

Create a project version (milestone). Requires --enable-writes.

- Mode: `write`
- Source: `ops.CreateVersion`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `project_id` | `string` | yes | Project identifier or numeric ID. |
| `name` | `string` | yes | Version name. |
| `status` | `string` | no | Version status: open, locked, or closed. |
| `sharing` | `string` | no | Version sharing: none, descendants, hierarchy, tree, or system. |
| `due_date` | `string` | no | Due date (YYYY-MM-DD). |
| `description` | `string` | no | Version description. |
| `wiki_page_title` | `string` | no | Associated wiki page title. |

### `delete_version`

Delete a version (milestone). Destructive. Requires --enable-writes.

- Mode: `write`
- Source: `ops.DeleteVersion`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `id` | `int` | yes | Numeric version (milestone) ID to delete. Destructive. |

### `get_version`

Fetch a single version (milestone) by ID.

- Mode: `read`
- Source: `ops.GetVersion`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `id` | `int` | yes | Numeric version (milestone) ID. |

### `list_categories`

List issue categories for a project.

- Mode: `read`
- Source: `ops.ListCategories`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `project_id` | `string` | yes | Project identifier or numeric ID. |

### `list_statuses`

List all issue statuses configured in this Redmine instance.

- Mode: `read`
- Source: `ops.ListStatuses`

Parameters: none.

### `list_trackers`

List all trackers (Bug, Feature, ...) configured in this Redmine instance.

- Mode: `read`
- Source: `ops.ListTrackers`

Parameters: none.

### `list_versions`

List versions (milestones) for a project.

- Mode: `read`
- Source: `ops.ListVersions`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `project_id` | `string` | yes | Project identifier or numeric ID. |
| `limit` | `int` | no | Max results to return. Defaults to 50 when omitted. |
| `offset` | `int` | no | Number of leading results to skip. |

### `update_version`

Update an existing version (milestone). Requires --enable-writes.

- Mode: `write`
- Source: `ops.UpdateVersion`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `id` | `int` | yes | Numeric version (milestone) ID. |
| `name` | `*string` | no | New version name. |
| `status` | `*string` | no | New status: open, locked, or closed. |
| `sharing` | `*string` | no | New sharing: none, descendants, hierarchy, tree, or system. |
| `due_date` | `*string` | no | New due date (YYYY-MM-DD). |
| `description` | `*string` | no | New description. |
| `wiki_page_title` | `*string` | no | New associated wiki page title. |

## projects

### `create_project`

Create a new Redmine project. Requires --enable-writes.

- Mode: `write`
- Source: `ops.CreateProject`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `name` | `string` | yes | Human-readable project name. |
| `identifier` | `string` | yes | URL-safe project identifier (slug). |
| `description` | `string` | no | Project description. |
| `is_public` | `*bool` | no | Mark the project as public. |
| `parent_id` | `int` | no | Parent project numeric ID. |
| `inherit_members` | `bool` | no | Inherit members from the parent project. |

### `delete_project`

Delete a Redmine project. Destructive. Requires --enable-writes.

- Mode: `write`
- Source: `ops.DeleteProject`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `identifier` | `string` | yes | Project identifier or numeric ID to delete. Destructive. |

### `get_project`

Fetch a single Redmine project by identifier or ID.

- Mode: `read`
- Source: `ops.GetProject`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `identifier` | `string` | yes | Project identifier (slug) or numeric ID. |
| `includes` | `[]string` | no | Extra sections to include: trackers, issue_categories, enabled_modules. |

### `list_project_members`

List members for a Redmine project.

- Mode: `read`
- Source: `ops.ListProjectMembers`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `identifier` | `string` | yes | Project identifier or numeric ID. |
| `limit` | `int` | no | Max results to return. Defaults to 50 when omitted. |
| `offset` | `int` | no | Number of leading results to skip. |

### `list_projects`

List Redmine projects.

- Mode: `read`
- Source: `ops.ListProjects`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `includes` | `[]string` | no | Extra sections to include: trackers, issue_categories, enabled_modules. |
| `limit` | `int` | no | Max results to return. Defaults to 50 when omitted. |
| `offset` | `int` | no | Number of leading results to skip. |

### `update_project`

Update an existing Redmine project. Requires --enable-writes.

- Mode: `write`
- Source: `ops.UpdateProject`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `identifier` | `string` | yes | Project identifier or numeric ID to update. |
| `name` | `*string` | no | New project name. |
| `description` | `*string` | no | New project description. |
| `is_public` | `*bool` | no | Toggle public visibility. |

## search

### `search`

Search across Redmine issues, wiki pages, news, and more. If no type flag is set, issues and wiki pages are included by default.

- Mode: `read`
- Source: `ops.Search`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `query` | `string` | yes | Full-text search query. |
| `project_id` | `string` | no | Scope search to a single project (identifier or numeric ID). |
| `scope` | `string` | no | One of 'all', 'my_projects', 'subprojects'. |
| `issues` | `bool` | no | Include issues in results. |
| `news` | `bool` | no | Include news in results. |
| `documents` | `bool` | no | Include documents in results. |
| `changesets` | `bool` | no | Include changesets in results. |
| `wiki_pages` | `bool` | no | Include wiki pages in results. |
| `messages` | `bool` | no | Include forum messages in results. |
| `projects` | `bool` | no | Include projects in results. |
| `all_words` | `bool` | no | Require all query words to match. |
| `titles_only` | `bool` | no | Match query against titles only. |
| `open_issues` | `bool` | no | Limit issue results to open issues. |
| `limit` | `int` | no | Max results to return. Defaults to 50 when omitted. |
| `offset` | `int` | no | Number of leading results to skip. |

## time

### `create_time_entry`

Log a new time entry. Requires --enable-writes.

- Mode: `write`
- Source: `ops.CreateTimeEntry`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `issue_id` | `int` | no | Issue to log time against. Either issue_id or project_id is required. |
| `project_id` | `string` | no | Project identifier or numeric ID. Either issue_id or project_id is required. |
| `hours` | `float64` | yes | Hours worked (decimal, e.g. 1.5). |
| `activity_id` | `int` | no | Activity enumeration ID. |
| `spent_on` | `string` | no | Date the work was done (YYYY-MM-DD). Defaults to today. |
| `comments` | `string` | no | Free-text comment. |

### `delete_time_entry`

Delete a time entry. Destructive. Requires --enable-writes.

- Mode: `write`
- Source: `ops.DeleteTimeEntry`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `id` | `int` | yes | Time entry numeric ID. |

### `get_time_entry`

Fetch a single time entry by ID.

- Mode: `read`
- Source: `ops.GetTimeEntry`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `id` | `int` | yes | Time entry numeric ID. |

### `list_time_entries`

List Redmine time entries matching the given filters.

- Mode: `read`
- Source: `ops.ListTimeEntries`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `project_id` | `string` | no | Project identifier or numeric ID to filter by. |
| `user_id` | `string` | no | User numeric ID or 'me'. |
| `issue_id` | `int` | no | Issue numeric ID to filter by. |
| `activity_id` | `int` | no | Activity enumeration ID. |
| `from` | `string` | no | Inclusive start date (YYYY-MM-DD). |
| `to` | `string` | no | Inclusive end date (YYYY-MM-DD). |
| `limit` | `int` | no | Max results to return. Defaults to 50 when omitted. |
| `offset` | `int` | no | Number of leading results to skip. |

### `summary_time_entries`

Summarize time entries grouped by day, project, or activity.

- Mode: `read`
- Source: `ops.SummaryTimeEntries`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `project_id` | `string` | no | Project identifier or numeric ID to filter by. |
| `user_id` | `string` | no | User numeric ID or 'me'. |
| `from` | `string` | no | Inclusive start date (YYYY-MM-DD). |
| `to` | `string` | no | Inclusive end date (YYYY-MM-DD). |
| `group_by` | `string` | no | One of 'day' (default), 'project', 'activity'. |

### `update_time_entry`

Update an existing time entry. Requires --enable-writes.

- Mode: `write`
- Source: `ops.UpdateTimeEntry`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `id` | `int` | yes | Time entry numeric ID. |
| `hours` | `*float64` | no | New hours worked. |
| `activity_id` | `*int` | no | New activity enumeration ID. |
| `spent_on` | `*string` | no | New date (YYYY-MM-DD). |
| `comments` | `*string` | no | New comment body. |

## users

### `create_user`

Create a new Redmine user. Requires --enable-writes and admin privileges.

- Mode: `write`
- Source: `ops.CreateUser`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `login` | `string` | yes | Unique login name. |
| `password` | `string` | yes | Initial password for the new account. |
| `firstname` | `string` | yes | Given name. |
| `lastname` | `string` | yes | Family name. |
| `mail` | `string` | yes | Email address. |
| `admin` | `bool` | no | Grant admin privileges. |

### `delete_user`

Delete a Redmine user. Destructive. Requires --enable-writes.

- Mode: `write`
- Source: `ops.DeleteUser`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `id` | `int` | yes | Numeric user ID to delete. Destructive. |

### `get_user`

Fetch a single Redmine user by numeric ID.

- Mode: `read`
- Source: `ops.GetUser`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `id` | `int` | yes | Numeric user ID. |

### `list_users`

List Redmine users matching the given filter.

- Mode: `read`
- Source: `ops.ListUsers`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `status` | `string` | no | Filter by status: 'active', 'registered', 'locked', or a numeric code. |
| `name` | `string` | no | Filter by name substring. |
| `group_id` | `int` | no | Filter users that belong to the given group ID. |
| `limit` | `int` | no | Max results to return. Defaults to 50 when omitted. |
| `offset` | `int` | no | Number of leading results to skip. |

### `me`

Return the currently authenticated Redmine user.

- Mode: `read`
- Source: `ops.GetCurrentUser`

Parameters: none.

### `update_user`

Update an existing Redmine user. Requires --enable-writes.

- Mode: `write`
- Source: `ops.UpdateUser`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `id` | `int` | yes | Numeric user ID to update. |
| `firstname` | `*string` | no | New given name. |
| `lastname` | `*string` | no | New family name. |
| `mail` | `*string` | no | New email address. |
| `admin` | `*bool` | no | Toggle admin privileges. |
| `status` | `*int` | no | Numeric status code (1 active, 2 registered, 3 locked). |

## wiki

### `create_wiki_page`

Create (or overwrite) a wiki page. Requires --enable-writes.

- Mode: `write`
- Source: `ops.CreateWikiPage`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `project_id` | `string` | yes | Project identifier or numeric ID. |
| `page` | `string` | yes | Wiki page title (slug) to create or overwrite. |
| `text` | `string` | yes | Page body (Textile or Markdown depending on the Redmine configuration). |
| `title` | `string` | no | Optional display title; may differ from the slug. |
| `comments` | `string` | no | Edit comment. |

### `delete_wiki_page`

Delete a wiki page. Destructive. Requires --enable-writes.

- Mode: `write`
- Source: `ops.DeleteWikiPage`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `project_id` | `string` | yes | Project identifier or numeric ID. |
| `page` | `string` | yes | Wiki page title (slug) to delete. Destructive. |

### `get_wiki_page`

Fetch a single wiki page.

- Mode: `read`
- Source: `ops.GetWikiPage`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `project_id` | `string` | yes | Project identifier or numeric ID. |
| `page` | `string` | yes | Wiki page title (slug). |
| `includes` | `[]string` | no | Extra sections to include, e.g. 'attachments'. |

### `list_wiki_pages`

List wiki pages for a project.

- Mode: `read`
- Source: `ops.ListWikiPages`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `project_id` | `string` | yes | Project identifier or numeric ID. |
| `limit` | `int` | no | Max results to return. Defaults to 50 when omitted. |
| `offset` | `int` | no | Number of leading results to skip. |

### `update_wiki_page`

Update an existing wiki page. Requires --enable-writes.

- Mode: `write`
- Source: `ops.UpdateWikiPage`

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `project_id` | `string` | yes | Project identifier or numeric ID. |
| `page` | `string` | yes | Wiki page title (slug) to update. |
| `text` | `*string` | no | New page body. |
| `title` | `*string` | no | New display title. |
| `comments` | `*string` | no | Edit comment. |

