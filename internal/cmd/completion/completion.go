package completion

import (
	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
)

// NewCmdCompletion creates the completion command.
func NewCmdCompletion() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for your shell.

To load completions:

Bash:
  $ source <(redmine completion bash)

Zsh:
  $ source <(redmine completion zsh)

Fish:
  $ redmine completion fish | source

PowerShell:
  PS> redmine completion powershell | Out-String | Invoke-Expression`,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			switch args[0] {
			case "bash":
				// The V2 script quotes candidates with printf %q before
				// handing them to compgen -W. The legacy script passed them
				// unquoted, so a candidate from the server such as
				// "$(cmd)" (a tracker or version name) ran on TAB.
				return cmd.Root().GenBashCompletionV2(out, true)
			case "zsh":
				return cmd.Root().GenZshCompletion(out)
			case "fish":
				return cmd.Root().GenFishCompletion(out, true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletionWithDesc(out)
			}
			return nil
		},
	}
	cmdutil.MarkNoJSONOutput(cmd)
	return cmd
}
