package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

// Every command that uploads files with --attach must also take
// --attach-description, registered through cmdutil.AddAttachFlags.
func TestAttachCommandsTakeAttachDescription(t *testing.T) {
	root := NewRootCmd("test")
	var found int
	walk(root, func(c *cobra.Command) {
		if c.Flags().Lookup("attach") == nil {
			return
		}
		found++
		if c.Flags().Lookup("attach-description") == nil {
			t.Errorf("%s has --attach but no --attach-description (use cmdutil.AddAttachFlags)", c.CommandPath())
		}
	})
	if found == 0 {
		t.Fatal("no command with --attach found")
	}
}
