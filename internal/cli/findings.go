package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hackihub/kvantumcli/internal/api"
)

func newFindingsCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "findings",
		Short: "Inspect rule-check outcomes",
	}
	cmd.AddCommand(newFindingsListCmd(opts))
	return cmd
}

func newFindingsListCmd(opts *rootOptions) *cobra.Command {
	var verificationID, status string
	var page, limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List rule-check outcomes (tenant-wide unless --verification is set)",
		Long:  "List rule-check outcomes across the tenant. Set --verification to limit results to one verification.",
		Args:  cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if status != "" && status != "fail" && status != "pass" && status != "skip" {
				return fmt.Errorf("invalid --status %q: must be fail, pass, or skip", status)
			}
			if page < 1 {
				return fmt.Errorf("invalid --page %d: must be at least 1", page)
			}
			if limit < 1 || limit > 100 {
				return fmt.Errorf("invalid --limit %d: must be between 1 and 100", limit)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJSON(func(ctx context.Context) (any, error) {
				client, _, err := newClient(opts, true)
				if err != nil {
					return nil, err
				}
				return client.ListResultsFiltered(cmd.Context(), api.ResultsListOptions{
					VerificationID: verificationID,
					ResultStatus:   status,
					Page:           page,
					Limit:          limit,
				})
			})
		},
	}
	cmd.Flags().StringVar(&verificationID, "verification", "", "Filter by verification UUID (default: all tenant results)")
	cmd.Flags().StringVar(&status, "status", "", "Filter by result status: fail, pass, or skip")
	cmd.Flags().IntVar(&page, "page", 1, "Results page number")
	cmd.Flags().IntVar(&limit, "limit", 10, "Results per page (1-100)")
	return cmd
}
