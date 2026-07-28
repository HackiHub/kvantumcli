package cli

import (
	"context"

	"github.com/spf13/cobra"
)

func newIntegrationCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "integration",
		Short: "List tenant integrations and provider resources (read-only)",
	}
	cmd.AddCommand(newIntegrationListCmd(opts))
	cmd.AddCommand(newIntegrationResourcesCmd(opts))
	cmd.AddCommand(newIntegrationBranchesCmd(opts))
	return cmd
}

func newIntegrationListCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List tenant integrations",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJSON(func(ctx context.Context) (any, error) {
				client, _, err := newClient(opts, true)
				if err != nil {
					return nil, err
				}
				return client.ListIntegrations(ctx)
			})
		},
	}
}

func newIntegrationResourcesCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "resources <provider>",
		Short: "List provider repos/resources (github, gitlab, jenkins, ...)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJSON(func(ctx context.Context) (any, error) {
				client, _, err := newClient(opts, true)
				if err != nil {
					return nil, err
				}
				return client.ListIntegrationResources(ctx, args[0])
			})
		},
	}
}

func newIntegrationBranchesCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "branches <provider> <resourceId>",
		Short: "List branches for a provider resource",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJSON(func(ctx context.Context) (any, error) {
				client, _, err := newClient(opts, true)
				if err != nil {
					return nil, err
				}
				return client.ListIntegrationBranches(ctx, args[0], args[1])
			})
		},
	}
}
