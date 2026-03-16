package project

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
)

func newCmdDelete(f *cmdutil.Factory) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:     "delete <identifier>",
		Aliases: []string{"rm"},
		Short:   "Delete a project",
		Long:    "Delete a Redmine project and all its data.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer("")
			identifier := args[0]

			if !force {
				fmt.Fprintf(cmd.ErrOrStderr(), "Are you sure you want to delete project %q? [y/N]: ", identifier)
				reader := bufio.NewReader(f.IOStreams.In)
				answer, _ := reader.ReadString('\n')
				answer = strings.TrimSpace(strings.ToLower(answer))
				if answer != "y" && answer != "yes" {
					printer.Warning("Deletion cancelled")
					return nil
				}
			}

			err = client.Projects.Delete(context.Background(), identifier)
			if err != nil {
				return err
			}

			printer.Success(fmt.Sprintf("Project %q deleted", identifier))
			return nil
		},
	}

	cmdutil.AddForceFlag(cmd, &force)

	return cmd
}
