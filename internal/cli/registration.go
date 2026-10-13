package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/hackihub/kvantumcli/internal/api"
)

func newRegistrationCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "registration",
		Short: "Manage template repository ref registrations",
	}
	cmd.AddCommand(newRegistrationAddCmd(opts))
	cmd.AddCommand(newRegistrationListCmd(opts))
	cmd.AddCommand(newRegistrationRefreshCmd(opts))
	return cmd
}

func newRegistrationAddCmd(opts *rootOptions) *cobra.Command {
	var repoID, ref, refType string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Register a template repository tag or SHA",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJSON(func(ctx context.Context) (any, error) {
				client, _, err := newClient(opts, true)
				if err != nil {
					return nil, err
				}
				return client.CreateTemplateRegistration(ctx, api.CreateTemplateRegistrationRequest{
					ProjectRepositoryID: repoID,
					RequestedRef:        ref,
					RefType:             refType,
				})
			})
		},
	}
	cmd.Flags().StringVar(&repoID, "repo", "", "Template repository UUID")
	cmd.Flags().StringVar(&ref, "ref", "", "Bare tag name or 40/64-character commit SHA")
	cmd.Flags().StringVar(&refType, "type", "", "Ref type: TAG or SHA")
	_ = cmd.MarkFlagRequired("repo")
	_ = cmd.MarkFlagRequired("ref")
	_ = cmd.MarkFlagRequired("type")
	return cmd
}

func newRegistrationListCmd(opts *rootOptions) *cobra.Command {
	var repoID string
	var page, limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List template repository ref registrations",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJSON(func(ctx context.Context) (any, error) {
				client, _, err := newClient(opts, true)
				if err != nil {
					return nil, err
				}
				return client.ListTemplateRegistrations(ctx, repoID, page, limit)
			})
		},
	}
	cmd.Flags().StringVar(&repoID, "repo", "", "Filter by template repository UUID")
	cmd.Flags().IntVar(&page, "page", 1, "Page number")
	cmd.Flags().IntVar(&limit, "limit", 20, "Page size")
	return cmd
}

func newRegistrationRefreshCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "refresh <registrationId>",
		Short: "Request a reconciliation of a TAG registration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJSON(func(ctx context.Context) (any, error) {
				client, _, err := newClient(opts, true)
				if err != nil {
					return nil, err
				}
				return client.RefreshTemplateRegistration(ctx, args[0])
			})
		},
	}
}
