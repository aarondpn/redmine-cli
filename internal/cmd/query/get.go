package query

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
)

func newCmdQueryGet(f *cmdutil.Factory) *cobra.Command {
	var (
		format  string
		project string
	)

	cmd := &cobra.Command{
		Use:     "get <id-or-name>",
		Aliases: []string{"show", "view"},
		Short:   "Show saved query details",
		Long:    "Show saved query details. Accepts a numeric ID or query name. Use --project to disambiguate per-project queries that share a name with a global query.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			ctx := context.Background()
			printer := f.Printer(format)

			stop := printer.Spinner("Fetching saved query...")
			match, err := resolver.ResolveQuery(ctx, client, args[0], project)
			stop()
			if err != nil {
				return err
			}

			// Numeric input short-circuits the resolver and returns a stub
			// with only the ID populated. Redmine has no /queries/:id.json
			// endpoint, so list once and pick the matching record to fill
			// in name / visibility / scope.
			if match.Name == "" {
				queries, _, err := client.Queries.List(ctx, 0, 0)
				if err != nil {
					return err
				}
				for i := range queries {
					if queries[i].ID == match.ID {
						match = &queries[i]
						break
					}
				}
				if match.Name == "" {
					// `/queries.json` only returns queries the API key can
					// see (public, or private and owned by the current user),
					// so a missing record could mean genuinely deleted or
					// owned by someone else.
					return fmt.Errorf("saved query %d not found or not visible to you", match.ID)
				}
			}

			if printer.Format() == output.FormatJSON {
				printer.JSON(match)
				return nil
			}
			printer.Detail([]output.KeyValue{
				{Key: "ID", Value: fmt.Sprintf("%d", match.ID)},
				{Key: "Name", Value: match.Name},
				{Key: "Visibility", Value: queryVisibility(*match)},
				{Key: "Scope", Value: queryScope(*match)},
			})
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project identifier or numeric ID (used to disambiguate query names)")
	cmdutil.AddOutputFlag(cmd, &format)
	return cmd
}
