package cli

import (
	"context"

	"github.com/spf13/cobra"
)

func newHealthCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Check API connectivity",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJSON(func(ctx context.Context) (any, error) {
				client, _, err := newClient(opts, false)
				if err != nil {
					return nil, err
				}
				return client.Health(ctx)
			})
		},
	}
}

func newWhoamiCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Current user, tenants, and permissions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJSON(func(ctx context.Context) (any, error) {
				client, _, err := newClient(opts, true)
				if err != nil {
					return nil, err
				}
				return client.WhoAmI(ctx)
			})
		},
	}
}
