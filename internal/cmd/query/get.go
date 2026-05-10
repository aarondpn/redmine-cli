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

			id, err := resolver.ResolveQuery(ctx, client, args[0], project)
			if err != nil {
				return err
			}

			printer := f.Printer(format)
			stop := printer.Spinner("Fetching saved query...")
			queries, _, err := client.Queries.List(ctx, 0, 0)
			stop()
			if err != nil {
				return err
			}

			for _, q := range queries {
				if q.ID != id {
					continue
				}
				if printer.Format() == output.FormatJSON {
					printer.JSON(q)
					return nil
				}

				details := []output.KeyValue{
					{Key: "ID", Value: fmt.Sprintf("%d", q.ID)},
					{Key: "Name", Value: q.Name},
					{Key: "Visibility", Value: queryVisibility(q)},
					{Key: "Scope", Value: queryScope(q)},
				}
				printer.Detail(details)
				return nil
			}

			return fmt.Errorf("saved query %d not found", id)
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project identifier or numeric ID (used to disambiguate query names)")
	cmdutil.AddOutputFlag(cmd, &format)
	return cmd
}
