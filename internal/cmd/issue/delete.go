package issue

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
)

// NewCmdDelete creates the issues delete command.
func NewCmdDelete(f *cmdutil.Factory) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:     "delete <id>",
		Aliases: []string{"rm"},
		Short:   "Delete an issue",
		Long:    "Permanently delete an issue. This action cannot be undone.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid issue ID: %s", args[0])
			}

			printer := f.Printer("")

			if !force {
				fmt.Fprintf(os.Stderr, "Are you sure you want to delete issue %s? (y/N): ", fmt.Sprintf("#%d", id))
				reader := bufio.NewReader(os.Stdin)
				answer, _ := reader.ReadString('\n')
				answer = strings.TrimSpace(strings.ToLower(answer))
				if answer != "y" && answer != "yes" {
					printer.Warning("Delete cancelled")
					return nil
				}
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			stop := printer.Spinner("Deleting issue...")
			err = client.Issues.Delete(context.Background(), id)
			stop()
			if err != nil {
				return fmt.Errorf("failed to delete issue %s: %w", fmt.Sprintf("#%d", id), err)
			}

			printer.Success(fmt.Sprintf("Deleted issue %s", fmt.Sprintf("#%d", id)))
			return nil
		},
	}

	cmdutil.AddForceFlag(cmd, &force)

	return cmd
}
