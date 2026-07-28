package cli

import (
	"context"

	"github.com/spf13/cobra"
)

func newResultsCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "results",
		Short: "Read rule-check results",
	}
	cmd.AddCommand(newResultsListCmd(opts))
	cmd.AddCommand(newResultsGetCmd(opts))
	return cmd
}

func newResultsListCmd(opts *rootOptions) *cobra.Command {
	var verificationID string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List rule-check results",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJSON(func(ctx context.Context) (any, error) {
				client, _, err := newClient(opts, true)
				if err != nil {
					return nil, err
				}
				return client.ListResults(ctx, verificationID)
			})
		},
	}
	cmd.Flags().StringVar(&verificationID, "verification", "", "Filter by verification UUID")
	return cmd
}

func newResultsGetCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "get <resultId>",
		Short: "Get a single result with evidence",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJSON(func(ctx context.Context) (any, error) {
				client, _, err := newClient(opts, true)
				if err != nil {
					return nil, err
				}
				return client.GetResult(ctx, args[0])
			})
		},
	}
}
