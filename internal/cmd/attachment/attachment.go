// Package attachment implements the `redmine attachments` command group, which
// surfaces the Redmine REST Attachments API so users (and agents) can inspect
// attachment metadata and download the raw bytes without dropping down to
// `redmine api` plus a manual `curl` with a hand-extracted API key.
package attachment

import (
	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
)

// NewCmdAttachments creates the parent attachments command.
func NewCmdAttachments(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "attachments",
		Aliases: []string{"attachment", "a"},
		Short:   "Inspect and download Redmine attachments",
		Long: "Inspect attachment metadata and download attachment files.\n\n" +
			"Discover attachment IDs with `redmine issues get <id> --attachments`, then\n" +
			"fetch metadata with `redmine attachments get <id>` or download the file with\n" +
			"`redmine attachments download <id>`. Downloads reuse the active profile's\n" +
			"server and API key, so there is no need to extract the key or shell out to curl.",
	}

	cmd.AddCommand(newCmdGet(f))
	cmd.AddCommand(newCmdDownload(f))

	return cmd
}
