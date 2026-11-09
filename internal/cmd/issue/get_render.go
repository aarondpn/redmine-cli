package issue

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

// issueDetailRows returns the detail rows of an issue. Optional rows only
// appear when Redmine sent the field (or the matching include was requested). Everything here is
// omitted when empty or absent, so older servers and restricted callers get
// a clean output.
func issueDetailRows(issue *models.Issue) []output.KeyValue {
	var pairs []output.KeyValue
	add := func(k, v string) {
		if v != "" {
			pairs = append(pairs, output.KeyValue{Key: k, Value: v})
		}
	}
	addHours := func(k string, own, total *float64, skipZero bool) {
		nonZero := func(p *float64) bool { return p != nil && *p != 0 }
		switch {
		case skipZero && !nonZero(own) && !nonZero(total):
		case own != nil && total != nil && *own != *total:
			add(k, fmt.Sprintf("%s (total %s)", cmdutil.FormatHours(*own), cmdutil.FormatHours(*total)))
		case own != nil:
			add(k, cmdutil.FormatHours(*own))
		case nonZero(total):
			add(k, fmt.Sprintf("total %s", cmdutil.FormatHours(*total)))
		}
	}

	pairs = append(pairs,
		output.KeyValue{Key: "ID", Value: output.StyleID.Render(fmt.Sprintf("#%d", issue.ID))},
		output.KeyValue{Key: "Project", Value: issue.Project.Name},
		output.KeyValue{Key: "Tracker", Value: issue.Tracker.Name},
		output.KeyValue{Key: "Status", Value: output.StatusStyle(issue.Status.Name).Render(issue.Status.Name)},
		output.KeyValue{Key: "Priority", Value: output.PriorityStyle(issue.Priority.Name).Render(issue.Priority.Name)},
		output.KeyValue{Key: "Subject", Value: issue.Subject},
		output.KeyValue{Key: "Author", Value: issue.Author.Name},
		output.KeyValue{Key: "Assignee", Value: assigneeName(issue.AssignedTo)},
	)
	if issue.Category != nil {
		add("Category", issue.Category.Name)
	}
	if issue.IsPrivate {
		add("Private", "yes")
	}
	pairs = append(pairs, output.KeyValue{Key: "Done Ratio", Value: fmt.Sprintf("%d%%", issue.DoneRatio)})
	pairs = append(pairs,
		output.KeyValue{Key: "Created", Value: issue.CreatedOn},
		output.KeyValue{Key: "Updated", Value: issue.UpdatedOn},
	)
	add("Closed", issue.ClosedOn)
	add("Start Date", issue.StartDate)
	add("Due Date", issue.DueDate)
	addHours("Estimated Hours", issue.EstimatedHours, issue.TotalEstimatedHours, false)
	addHours("Spent Hours", issue.SpentHours, issue.TotalSpentHours, true)
	if issue.FixedVersion != nil {
		add("Version", issue.FixedVersion.Name)
	}
	if issue.Parent != nil {
		add("Parent", fmt.Sprintf("#%d", issue.Parent.ID))
	}
	add("Custom Fields", cmdutil.FormatCustomFields(issue.CustomFields))
	if len(issue.Watchers) > 0 {
		names := make([]string, len(issue.Watchers))
		for i, w := range issue.Watchers {
			names[i] = w.Name
		}
		add("Watchers", strings.Join(names, ", "))
	}
	if len(issue.AllowedStatuses) > 0 {
		names := make([]string, len(issue.AllowedStatuses))
		for i, s := range issue.AllowedStatuses {
			names[i] = s.Name
		}
		add("Allowed Statuses", strings.Join(names, ", "))
	}
	add("Description", issue.Description)
	return pairs
}

// writeIssueSections prints the multi-line sections that follow the detail
// block: children tree, relations, changesets and journals.
func writeIssueSections(w io.Writer, issue *models.Issue) {
	if len(issue.Children) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Children:")
		writeChildren(w, issue.Children, 1)
	}
	if len(issue.Relations) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Relations:")
		for _, r := range issue.Relations {
			fmt.Fprintf(w, "  %s\n", output.SanitizeTerminal(describeRelation(issue.ID, r)))
		}
	}
	if len(issue.Changesets) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Changesets:")
		for _, c := range issue.Changesets {
			who := ""
			if c.User != nil && c.User.Name != "" {
				who = " " + c.User.Name
			}
			first, _, _ := strings.Cut(strings.TrimSpace(c.Comments), "\n")
			line := fmt.Sprintf("  %s%s (%s) %s", c.Revision, who, c.CommittedOn, first)
			fmt.Fprintln(w, strings.TrimRight(output.SanitizeTerminal(line), " "))
		}
	}
	writeJournals(w, issue)
}

func writeChildren(w io.Writer, children []models.IssueChild, depth int) {
	indent := strings.Repeat("  ", depth)
	for _, c := range children {
		tracker := ""
		if c.Tracker != nil && c.Tracker.Name != "" {
			tracker = " [" + c.Tracker.Name + "]"
		}
		fmt.Fprintf(w, "%s#%d%s %s\n", indent, c.ID, output.SanitizeTerminal(tracker), output.SanitizeTerminal(c.Subject))
		writeChildren(w, c.Children, depth+1)
	}
}

// relationLabels words a relation type from the point of view of the issue
// being shown, matching Redmine's own labels.
var relationLabels = map[string]string{
	"relates":     "relates to",
	"duplicates":  "duplicates",
	"duplicated":  "duplicated by",
	"blocks":      "blocks",
	"blocked":     "blocked by",
	"precedes":    "precedes",
	"follows":     "follows",
	"copied_to":   "copied to",
	"copied_from": "copied from",
}

var inverseRelation = map[string]string{
	"duplicates":  "duplicated",
	"duplicated":  "duplicates",
	"blocks":      "blocked",
	"blocked":     "blocks",
	"precedes":    "follows",
	"follows":     "precedes",
	"copied_to":   "copied_from",
	"copied_from": "copied_to",
}

func relationLabel(typ string) string {
	if l, ok := relationLabels[typ]; ok {
		return l
	}
	return strings.ReplaceAll(typ, "_", " ")
}

func pluralDays(n int) string {
	if n == 1 || n == -1 {
		return fmt.Sprintf("%d day", n)
	}
	return fmt.Sprintf("%d days", n)
}

// describeRelation words a relation from the point of view of issueID, e.g.
// "blocks #7" or "blocked by #7": the API returns the same record for both
// ends, so the inverse type is used when the issue is the target.
func describeRelation(issueID int, r models.IssueRelation) string {
	typ, other := r.RelationType, r.IssueToID
	if r.IssueToID == issueID && r.IssueID != issueID {
		other = r.IssueID
		if inv, ok := inverseRelation[typ]; ok {
			typ = inv
		}
	}
	s := fmt.Sprintf("%s #%d", relationLabel(typ), other)
	if r.Delay != nil && *r.Delay != 0 {
		s += fmt.Sprintf(" (delay %s)", pluralDays(*r.Delay))
	}
	return s
}

func writeJournals(w io.Writer, issue *models.Issue) {
	names := newIDNames(issue)
	first := true
	for _, j := range issue.Journals {
		if j.Notes == "" && len(j.Details) == 0 {
			continue
		}
		if first {
			fmt.Fprintln(w)
			first = false
		}
		edited := ""
		if j.UpdatedBy != nil {
			edited = " (edited)"
			if j.UpdatedOn != "" && j.UpdatedBy.Name != "" {
				edited = fmt.Sprintf(" (edited %s by %s)", j.UpdatedOn, j.UpdatedBy.Name)
			}
		}
		private := ""
		if j.PrivateNotes {
			private = " (private)"
		}
		fmt.Fprintf(w, "--- %s (%s)%s%s [journal %d] ---\n",
			output.SanitizeTerminal(j.User.Name), output.SanitizeTerminal(j.CreatedOn),
			output.SanitizeTerminal(edited), private, j.ID)
		for _, d := range j.Details {
			fmt.Fprintf(w, "* %s\n", output.SanitizeTerminal(names.describe(d)))
		}
		if j.Notes != "" {
			if len(j.Details) > 0 {
				fmt.Fprintln(w)
			}
			fmt.Fprintln(w, output.SanitizeTerminal(j.Notes))
		}
		fmt.Fprintln(w)
	}
}

// idNames resolves the ids found in journal details to names, using only
// data that came with the issue itself (no extra requests). Unknown ids are
// shown as they are.
type idNames struct {
	byKind map[string]map[string]string
	cfs    map[string]string
}

func newIDNames(issue *models.Issue) *idNames {
	n := &idNames{byKind: map[string]map[string]string{}, cfs: map[string]string{}}
	put := func(kind string, id int, name string) {
		if id == 0 || name == "" {
			return
		}
		if n.byKind[kind] == nil {
			n.byKind[kind] = map[string]string{}
		}
		n.byKind[kind][strconv.Itoa(id)] = name
	}
	put("status_id", issue.Status.ID, issue.Status.Name)
	for _, s := range issue.AllowedStatuses {
		put("status_id", s.ID, s.Name)
	}
	put("priority_id", issue.Priority.ID, issue.Priority.Name)
	put("tracker_id", issue.Tracker.ID, issue.Tracker.Name)
	put("project_id", issue.Project.ID, issue.Project.Name)
	if issue.Category != nil {
		put("category_id", issue.Category.ID, issue.Category.Name)
	}
	if issue.FixedVersion != nil {
		put("fixed_version_id", issue.FixedVersion.ID, issue.FixedVersion.Name)
	}
	users := []models.IDName{issue.Author}
	if issue.AssignedTo != nil {
		users = append(users, *issue.AssignedTo)
	}
	users = append(users, issue.Watchers...)
	for _, j := range issue.Journals {
		users = append(users, j.User)
	}
	for _, u := range users {
		put("assigned_to_id", u.ID, u.Name)
		put("author_id", u.ID, u.Name)
	}
	for _, cf := range issue.CustomFields {
		n.cfs[strconv.Itoa(cf.ID)] = cf.Name
	}
	return n
}

var attrLabels = map[string]string{
	"status_id":        "Status",
	"priority_id":      "Priority",
	"tracker_id":       "Tracker",
	"project_id":       "Project",
	"category_id":      "Category",
	"fixed_version_id": "Version",
	"assigned_to_id":   "Assignee",
	"parent_id":        "Parent",
	"child_id":         "Subtask",
	"author_id":        "Author",
	"done_ratio":       "Done Ratio",
	"start_date":       "Start Date",
	"due_date":         "Due Date",
	"estimated_hours":  "Estimated Hours",
	"is_private":       "Private",
	"subject":          "Subject",
	"description":      "Description",
}

func (n *idNames) value(kind, v string) string {
	if v == "" {
		return "(none)"
	}
	if name, ok := n.byKind[kind][v]; ok {
		return name
	}
	switch kind {
	case "parent_id", "child_id":
		return "#" + v
	case "is_private":
		switch v {
		case "1":
			return "yes"
		case "0":
			return "no"
		}
	}
	return v
}

func (n *idNames) describe(d models.JournalDetail) string {
	switch d.Property {
	case "attachment":
		if d.NewValue != "" {
			return "Attachment added: " + d.NewValue
		}
		return "Attachment removed: " + d.OldValue
	case "relation":
		if d.NewValue != "" {
			return fmt.Sprintf("Relation added: %s #%s", relationLabel(d.Name), d.NewValue)
		}
		return fmt.Sprintf("Relation removed: %s #%s", relationLabel(d.Name), d.OldValue)
	case "cf":
		label := d.Name
		if name, ok := n.cfs[d.Name]; ok {
			label = name
		}
		return fmt.Sprintf("%s: %s -> %s", label, orNone(d.OldValue), orNone(d.NewValue))
	}
	label, ok := attrLabels[d.Name]
	if !ok {
		label = d.Name
	}
	if d.Name == "description" {
		return "Description changed"
	}
	return fmt.Sprintf("%s: %s -> %s", label, n.value(d.Name, d.OldValue), n.value(d.Name, d.NewValue))
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}
