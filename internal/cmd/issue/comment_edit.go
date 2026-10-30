package issue

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

const journalEditHint = "Editing comments through the REST API needs Redmine 5.0 or later; older servers reject the request. " +
	"Check the journal ID with `redmine issues get <issue> --journals`."

// Redmine 4.2 ignores the API key for this endpoint and answers 403 to the
// anonymous request, so the forbidden hint names both possible causes.
const journalEditForbiddenHint = "Redmine denied this edit: the account may lack the Edit issue notes permission (or Edit own issue notes for its own comments), or the server is older than 5.0, which does not accept comment edits over the API. Don't retry with other credentials; ask the user or a Redmine administrator."

func newCmdCommentEdit(f *cmdutil.Factory) *cobra.Command {
	var (
		message   string
		private   bool
		public    bool
		clearText bool
		force     bool
	)

	cmd := &cobra.Command{
		Use:   "edit <journal-id>",
		Short: "Edit an issue comment (journal)",
		Long: "Edit the text or privacy of an existing issue comment. Requires Redmine 5.0 or later.\n\n" +
			"Comments are identified by their journal ID, not the issue ID. List them with:\n" +
			"  redmine issues get 123 --journals --jq '.journals[] | {id, notes}'\n" +
			"The table view of `issues get --journals` also prints [journal N] in each comment header.\n\n" +
			"Editing the text needs the \"Edit issue notes\" permission (or \"Edit own issue notes\" for your own\n" +
			"comments); --private/--public needs \"Set notes as private\" and is silently ignored without it.\n" +
			"--clear removes the comment's text: Redmine removes the comment from the issue history (some versions delete it entirely, others keep an\n" +
			"empty row the API no longer lists). It asks for confirmation unless --force.",
		Example: `  # Change the text of a comment
  redmine issues comment edit 456 --message "Corrected: deployed to production"

  # Read the new text from a file or standard input
  redmine issues comment edit 456 --message-file notes.textile
  echo "Updated text" | redmine issues comment edit 456 -F -

  # Make a comment private (or public again)
  redmine issues comment edit 456 --private
  redmine issues comment edit 456 --public

  # Remove the text of a comment
  redmine issues comment edit 456 --clear --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil || id <= 0 {
				return fmt.Errorf("invalid journal ID: %s", args[0])
			}

			messageGiven, err := cmdutil.ReadTextFileFlag(cmd, f.IOStreams.In, "message", "message-file", &message)
			if err != nil {
				return err
			}
			if err := cmdutil.RejectEmptyTextFile(cmd, "message-file", message, ""); err != nil {
				return err
			}
			if !messageGiven && !private && !public && !clearText {
				return errors.New("nothing to change: pass --message, --message-file, --private, --public or --clear")
			}
			if clearText && messageGiven {
				return errors.New("--clear cannot be combined with --message or --message-file")
			}
			if messageGiven && message == "" {
				return errors.New("--message must not be empty; use --clear to remove the comment's text")
			}

			input := ops.UpdateJournalInput{ID: id}
			switch {
			case clearText:
				empty := ""
				input.Notes = &empty
			case messageGiven:
				input.Notes = &message
			}
			if private || public {
				input.PrivateNotes = &private
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			printer := f.Printer("")
			if clearText && !force {
				msg := fmt.Sprintf("Clearing the text of journal %d removes the comment from the issue history. Continue?", id)
				if !cmdutil.ConfirmAction(f.IOStreams.In, f.IOStreams.ErrOut, msg) {
					printer.Outcome(false, output.ActionUpdated, "journal", id, "Edit cancelled")
					return nil
				}
			}

			stop := printer.Spinner("Updating comment...")
			_, err = ops.UpdateJournal(cmdutil.Context(cmd), client, input)
			stop()
			if err != nil {
				err = fmt.Errorf("failed to update journal %d: %w", id, err)
				var apiErr *api.APIError
				if errors.As(err, &apiErr) {
					switch {
					case apiErr.IsAuthError() || apiErr.IsNotFound():
						return cmdutil.WithHint(err, journalEditHint)
					case apiErr.IsForbidden():
						return cmdutil.WithHint(err, journalEditForbiddenHint)
					}
				}
				return err
			}

			printer.Action(output.ActionUpdated, "journal", id, fmt.Sprintf("Updated comment (journal %d)", id))
			return nil
		},
	}

	cmd.Flags().StringVarP(&message, "message", "m", "", "New comment text")
	cmdutil.AddTextFileFlag(cmd, "message", "message-file", "F")
	cmd.Flags().BoolVar(&private, "private", false, "Make the comment private")
	cmd.Flags().BoolVar(&public, "public", false, "Make the comment public")
	cmd.Flags().BoolVar(&clearText, "clear", false, "Remove the comment's text (asks for confirmation)")
	cmdutil.AddForceFlag(cmd, &force)
	cmd.MarkFlagsMutuallyExclusive("private", "public")
	cmd.MarkFlagsMutuallyExclusive("clear", "message")
	cmd.MarkFlagsMutuallyExclusive("clear", "message-file")

	return cmd
}
