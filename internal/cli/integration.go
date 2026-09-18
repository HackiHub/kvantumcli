package cli

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func validateIntegrationProvider(ctx context.Context, client interface {
	GetIntegrationProvider(context.Context, string) (string, error)
}, integrationID, provider string) error {
	if !uuidPattern.MatchString(integrationID) {
		return fmt.Errorf("invalid --integration %q: must be a UUID", integrationID)
	}
	actualProvider, err := client.GetIntegrationProvider(ctx, integrationID)
	if err != nil {
		return err
	}
	if !strings.EqualFold(actualProvider, provider) {
		return fmt.Errorf("integration %s uses provider %q, not %q", integrationID, actualProvider, provider)
	}
	return nil
}

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
	var integrationID string
	cmd := &cobra.Command{
		Use:   "resources <provider>",
		Short: "List provider repos/resources (github, gitlab, jenkins, ...)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJSON(func(ctx context.Context) (any, error) {
				client, _, err := newClient(opts, true)
				if err != nil {
					return nil, err
				}
				if integrationID != "" {
					if err := validateIntegrationProvider(ctx, client, integrationID, args[0]); err != nil {
						return nil, err
					}
					return client.ListIntegrationResourcesByID(ctx, integrationID)
				}
				return client.ListIntegrationResources(ctx, args[0])
			})
		},
	}
	cmd.Flags().StringVar(&integrationID, "integration", "", "Tenant integration UUID to select")
	return cmd
}

func newIntegrationBranchesCmd(opts *rootOptions) *cobra.Command {
	var integrationID string
	cmd := &cobra.Command{
		Use:   "branches <provider> <resourceId>",
		Short: "List branches for a provider resource",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJSON(func(ctx context.Context) (any, error) {
				client, _, err := newClient(opts, true)
				if err != nil {
					return nil, err
				}
				if integrationID != "" {
					if err := validateIntegrationProvider(ctx, client, integrationID, args[0]); err != nil {
						return nil, err
					}
					return client.ListIntegrationBranchesByID(ctx, integrationID, args[1])
				}
				return client.ListIntegrationBranches(ctx, args[0], args[1])
			})
		},
	}
	cmd.Flags().StringVar(&integrationID, "integration", "", "Tenant integration UUID to select")
	return cmd
}
