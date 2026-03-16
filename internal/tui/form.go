package tui

import (
	"fmt"

	"github.com/charmbracelet/huh"
)

// IssueFormData holds data collected from the issue creation form.
type IssueFormData struct {
	Subject     string
	Description string
	TrackerID   string
	PriorityID  string
	AssigneeID  string
}

// RunIssueForm runs an interactive form for creating an issue.
func RunIssueForm(trackers, priorities, assignees []SelectItem) (*IssueFormData, error) {
	data := &IssueFormData{}

	trackerOpts := make([]huh.Option[string], len(trackers))
	for i, t := range trackers {
		trackerOpts[i] = huh.NewOption(t.Label, t.Value)
	}

	priorityOpts := make([]huh.Option[string], len(priorities))
	for i, p := range priorities {
		priorityOpts[i] = huh.NewOption(p.Label, p.Value)
	}

	groups := []*huh.Group{
		huh.NewGroup(
			huh.NewInput().
				Title("Subject").
				Value(&data.Subject).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("subject is required")
					}
					return nil
				}),
			huh.NewText().
				Title("Description").
				Value(&data.Description),
		),
	}

	if len(trackerOpts) > 0 {
		groups = append(groups, huh.NewGroup(
			huh.NewSelect[string]().
				Title("Tracker").
				Options(trackerOpts...).
				Value(&data.TrackerID),
		))
	}

	if len(priorityOpts) > 0 {
		groups = append(groups, huh.NewGroup(
			huh.NewSelect[string]().
				Title("Priority").
				Options(priorityOpts...).
				Value(&data.PriorityID),
		))
	}

	if len(assignees) > 0 {
		assigneeOpts := []huh.Option[string]{huh.NewOption("(unassigned)", "")}
		for _, a := range assignees {
			assigneeOpts = append(assigneeOpts, huh.NewOption(a.Label, a.Value))
		}
		groups = append(groups, huh.NewGroup(
			huh.NewSelect[string]().
				Title("Assignee").
				Options(assigneeOpts...).
				Value(&data.AssigneeID),
		))
	}

	err := huh.NewForm(groups...).Run()
	if err != nil {
		return nil, err
	}
	return data, nil
}

// TimeEntryFormData holds data collected from the time entry form.
type TimeEntryFormData struct {
	Hours    string
	Comment  string
	Activity string
	Date     string
}

// RunTimeEntryForm runs an interactive form for logging time.
func RunTimeEntryForm(activities []SelectItem) (*TimeEntryFormData, error) {
	data := &TimeEntryFormData{}

	groups := []*huh.Group{
		huh.NewGroup(
			huh.NewInput().
				Title("Hours").
				Value(&data.Hours).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("hours is required")
					}
					return nil
				}),
			huh.NewInput().
				Title("Date (YYYY-MM-DD, empty for today)").
				Value(&data.Date),
			huh.NewInput().
				Title("Comment").
				Value(&data.Comment),
		),
	}

	if len(activities) > 0 {
		actOpts := make([]huh.Option[string], len(activities))
		for i, a := range activities {
			actOpts[i] = huh.NewOption(a.Label, a.Value)
		}
		groups = append(groups, huh.NewGroup(
			huh.NewSelect[string]().
				Title("Activity").
				Options(actOpts...).
				Value(&data.Activity),
		))
	}

	err := huh.NewForm(groups...).Run()
	if err != nil {
		return nil, err
	}
	return data, nil
}
