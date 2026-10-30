package cmdutil

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestDeprecateFlag(t *testing.T) {
	cmd := &cobra.Command{Use: "get", Run: func(*cobra.Command, []string) {}}
	var old bool
	cmd.Flags().BoolVar(&old, "old", false, "old flag")
	DeprecateFlag(cmd, "old", "--include users", "2027-01-31")

	fl := cmd.Flags().Lookup("old")
	if !fl.Hidden {
		t.Error("deprecated flag should be hidden")
	}
	want := "use --include users instead; will be removed after 2027-01-31"
	if fl.Deprecated != want {
		t.Errorf("Deprecated = %q, want %q", fl.Deprecated, want)
	}

	var stderr bytes.Buffer
	cmd.SetOut(&stderr)
	cmd.SetArgs([]string{"--old"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !old {
		t.Error("deprecated flag should still work")
	}
	if !strings.Contains(stderr.String(), want) {
		t.Errorf("warning = %q, want it to contain %q", stderr.String(), want)
	}
}

func TestDeprecateFlag_InvalidDatePanics(t *testing.T) {
	cmd := &cobra.Command{Use: "get"}
	cmd.Flags().Bool("old", false, "")
	defer func() {
		if recover() == nil {
			t.Error("expected panic for malformed removeAfter")
		}
	}()
	DeprecateFlag(cmd, "old", "--new", "31.01.2027")
}

func TestDeprecatedFlags_WalksSubcommands(t *testing.T) {
	root := &cobra.Command{Use: "redmine"}
	group := &cobra.Command{Use: "groups"}
	get := &cobra.Command{Use: "get"}
	get.Flags().Bool("old", false, "")
	get.Flags().Bool("keep", false, "")
	DeprecateFlag(get, "old", "--new", "2027-01-31")
	group.AddCommand(get)
	root.AddCommand(group)

	got := DeprecatedFlags(root)
	want := []DeprecatedFlag{{Command: "redmine groups get", Name: "old", RemoveAfter: "2027-01-31"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DeprecatedFlags = %+v, want %+v", got, want)
	}
}
