package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/hackihub/kvantumcli/internal/api"
)

func newRepoCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo",
		Short: "Manage project repositories",
	}
	cmd.AddCommand(newRepoAddCmd(opts))
	cmd.AddCommand(newRepoListCmd(opts))
	cmd.AddCommand(newRepoGetCmd(opts))
	return cmd
}

func newRepoAddCmd(opts *rootOptions) *cobra.Command {
	var projectID, integrationID, name, branch, resourceID, resourceURL, repositoryURL string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a repository to a project",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJSON(func(ctx context.Context) (any, error) {
				client, _, err := newClient(opts, true)
				if err != nil {
					return nil, err
				}
				req := api.AddRepositoryRequest{
					Name:                 name,
					ProjectID:            projectID,
					TenantIntegrationsID: integrationID,
					BranchName:           strPtr(branch),
					ResourceID:           strPtr(resourceID),
					ResourceURL:          strPtr(resourceURL),
					RepositoryURL:        strPtr(repositoryURL),
				}
				return client.AddRepository(ctx, req)
			})
		},
	}
	cmd.Flags().StringVar(&projectID, "project", "", "Project UUID")
	cmd.Flags().StringVar(&integrationID, "integration", "", "Tenant integration UUID")
	cmd.Flags().StringVar(&name, "name", "", "Repository name")
	_ = cmd.MarkFlagRequired("project")
	_ = cmd.MarkFlagRequired("integration")
	_ = cmd.MarkFlagRequired("name")
	cmd.Flags().StringVar(&branch, "branch", "", "Branch name")
	cmd.Flags().StringVar(&resourceID, "resource-id", "", "Provider resource ID")
	cmd.Flags().StringVar(&resourceURL, "resource-url", "", "Provider resource URL")
	cmd.Flags().StringVar(&repositoryURL, "repository-url", "", "Repository URL")
	return cmd
}

func newRepoListCmd(opts *rootOptions) *cobra.Command {
	var projectID string
	var page, limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List repositories in a project",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJSON(func(ctx context.Context) (any, error) {
				client, _, err := newClient(opts, true)
				if err != nil {
					return nil, err
				}
				return client.ListRepositories(ctx, projectID, page, limit)
			})
		},
	}
	cmd.Flags().StringVar(&projectID, "project", "", "Project UUID")
	_ = cmd.MarkFlagRequired("project")
	cmd.Flags().IntVar(&page, "page", 1, "Page number")
	cmd.Flags().IntVar(&limit, "limit", 10, "Page size")
	return cmd
}

func newRepoGetCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "get <repoId>",
		Short: "Get repository detail",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJSON(func(ctx context.Context) (any, error) {
				client, _, err := newClient(opts, true)
				if err != nil {
					return nil, err
				}
				return client.GetRepository(ctx, args[0])
			})
		},
	}
}
