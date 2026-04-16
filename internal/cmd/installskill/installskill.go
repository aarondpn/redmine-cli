package installskill

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/output"
)

// NewCmdInstallSkill creates the install-skill command.
func NewCmdInstallSkill(f *cmdutil.Factory) *cobra.Command {
	var global bool

	cmd := &cobra.Command{
		Use:   "install-skill",
		Short: "Install the AI agent skill for redmine-cli",
		Long:  "Installs a skill that teaches AI coding agents (Claude Code, Cursor, etc.) how to use redmine-cli effectively.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := exec.LookPath("npx"); err != nil {
				return fmt.Errorf("npx is required but not found in PATH. Install Node.js first")
			}

			printer := f.Printer("")

			skillArgs := []string{"-y", "skills", "add", "aarondpn/redmine-cli", "--skill", "redmine-cli", "-y"}
			if global {
				skillArgs = append(skillArgs, "-g")
			}

			stop := printer.Spinner("Installing agent skill...")
			out, err := exec.Command("npx", skillArgs...).CombinedOutput()
			stop()
			if err != nil {
				return fmt.Errorf("could not install agent skill: %s\n%s", err, string(out))
			}

			scope := "project"
			if global {
				scope = "globally"
			}
			printer.Action(output.ActionInstalled, "agent_skill", scope,
				fmt.Sprintf("Agent skill installed %s. AI agents will now know how to use redmine-cli.", scope))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&global, "global", "g", false, "Install globally (user-level) instead of project-level")

	return cmd
}
