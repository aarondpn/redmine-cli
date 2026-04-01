package issue

import (
	"context"
	"testing"

	"github.com/aarondpn/redmine-cli/internal/output"
	"github.com/aarondpn/redmine-cli/internal/testutil"
)

func TestResolveIssueAssigneeFilter_MeBypassesLookup(t *testing.T) {
	f := testutil.NewFactory(t, "https://example.invalid")
	printer := output.NewStdPrinter(f.IOStreams.Out, f.IOStreams.ErrOut, false, true, "")

	got, err := resolveIssueAssigneeFilter(context.Background(), nil, "me", printer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "me" {
		t.Fatalf("got %q, want %q", got, "me")
	}
}

func TestResolveIssueStatusFilter_PreservesSpecialValue(t *testing.T) {
	got, err := resolveIssueStatusFilter(context.Background(), nil, "*")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "*" {
		t.Fatalf("got %q, want %q", got, "*")
	}
}

func TestResolveIssueAssigneeFilter_NumericBypassesLookup(t *testing.T) {
	f := testutil.NewFactory(t, "https://example.invalid")
	printer := output.NewStdPrinter(f.IOStreams.Out, f.IOStreams.ErrOut, false, true, "")

	got, err := resolveIssueAssigneeFilter(context.Background(), nil, "42", printer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "42" {
		t.Fatalf("got %q, want %q", got, "42")
	}
}
