package cmdutil

import (
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

var testIncludes = map[string]struct{}{"b": {}, "a": {}}

func TestValidateIncludes(t *testing.T) {
	if err := ValidateIncludes([]string{"a", " b ", ""}, testIncludes); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err := ValidateIncludes([]string{"a", "zzz"}, testIncludes)
	if err == nil {
		t.Fatal("expected error")
	}
	want := `unknown --include value "zzz" (allowed: a, b)`
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestCompleteIncludes(t *testing.T) {
	got, dir := CompleteIncludes(testIncludes)(nil, nil, "")
	if strings.Join(got, ",") != "a,b" {
		t.Errorf("completions = %v", got)
	}
	if dir != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %v", dir)
	}
}

func TestMergeIncludes(t *testing.T) {
	got := MergeIncludes([]string{"memberships", "", "users"}, "users", "memberships", "groups")
	want := []string{"memberships", "users", "groups"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MergeIncludes = %v, want %v", got, want)
	}
	got = MergeIncludes([]string{"users", " memberships"}, " users ")
	want = []string{"users", "memberships"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MergeIncludes(spaced) = %v, want %v", got, want)
	}
	if got := MergeIncludes(nil); got != nil {
		t.Errorf("MergeIncludes(nil) = %v, want nil", got)
	}
}
