package cmdutil

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
)

func TestContext(t *testing.T) {
	if ctx := Context(nil); ctx == nil {
		t.Fatal("Context(nil) = nil")
	}
	if ctx := Context(&cobra.Command{}); ctx == nil {
		t.Fatal("Context() of a command run without Execute = nil")
	}

	type key struct{}
	parent := context.WithValue(context.Background(), key{}, "v")
	var got context.Context
	cmd := &cobra.Command{
		Use: "x",
		RunE: func(cmd *cobra.Command, _ []string) error {
			got = Context(cmd)
			return nil
		},
	}
	cmd.SetArgs([]string{})
	if err := cmd.ExecuteContext(parent); err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Value(key{}) != "v" {
		t.Fatal("Context() did not return the context passed to ExecuteContext")
	}
}
