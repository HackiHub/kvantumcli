package cli

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/hackihub/kvantumcli/internal/api"
	"github.com/hackihub/kvantumcli/internal/output"
)

func newVerifyCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Run and inspect verifications",
	}
	cmd.AddCommand(newVerifyRunCmd(opts))
	cmd.AddCommand(newVerifyListCmd(opts))
	cmd.AddCommand(newVerifyLatestCmd(opts))
	cmd.AddCommand(newVerifyGetCmd(opts))
	cmd.AddCommand(newVerifyWaitCmd(opts))
	return cmd
}

func newVerifyLatestCmd(opts *rootOptions) *cobra.Command {
	var repoID string
	cmd := &cobra.Command{
		Use:   "latest",
		Short: "Get the latest finished verification for a repository",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJSON(func(ctx context.Context) (any, error) {
				client, _, err := newClient(opts, true)
				if err != nil {
					return nil, err
				}
				return client.LatestFinishedVerification(ctx, repoID)
			})
		},
	}
	cmd.Flags().StringVar(&repoID, "repo", "", "Project repository UUID")
	_ = cmd.MarkFlagRequired("repo")
	return cmd
}

func newVerifyRunCmd(opts *rootOptions) *cobra.Command {
	var repoID, projectID, typesCSV string
	var allRepos, confirm bool
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run verification for a repo (default) or all repos in a project",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJSON(func(ctx context.Context) (any, error) {
				client, cfg, err := newClient(opts, true)
				if err != nil {
					return nil, err
				}

				types := splitCSV(typesCSV)
				req := api.RunVerificationRequest{}
				if len(types) > 0 {
					req.Types = types
				}

				switch {
				case allRepos:
					if projectID == "" {
						return nil, fail("--project is required with --all-repos")
					}
					if repoID != "" {
						return nil, fail("--repo and --all-repos are mutually exclusive")
					}
					if !confirm {
						return nil, fail("--confirm is required when using --all-repos")
					}
					return client.RunVerificationForProject(ctx, projectID, cfg.TenantID, req)
				case repoID != "":
					if projectID != "" {
						return nil, fail("--project is only valid with --all-repos")
					}
					return client.RunVerificationForRepo(ctx, repoID, cfg.TenantID, req)
				default:
					return nil, fail("either --repo <repoId> or --project <projectId> --all-repos --confirm is required")
				}
			})
		},
	}
	cmd.Flags().StringVar(&repoID, "repo", "", "Project repository UUID")
	cmd.Flags().StringVar(&projectID, "project", "", "Project UUID (with --all-repos)")
	cmd.Flags().BoolVar(&allRepos, "all-repos", false, "Run verification for all repos in the project")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Required confirmation for --all-repos")
	cmd.Flags().StringVar(&typesCSV, "types", "", "Comma-separated BOM types: sbom,cbom,aibom,mlbom")
	return cmd
}

func newVerifyListCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List verification runs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJSON(func(ctx context.Context) (any, error) {
				client, _, err := newClient(opts, true)
				if err != nil {
					return nil, err
				}
				return client.ListVerifications(ctx)
			})
		},
	}
}

func newVerifyGetCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "get <verificationId>",
		Short: "Get a single verification run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJSON(func(ctx context.Context) (any, error) {
				client, _, err := newClient(opts, true)
				if err != nil {
					return nil, err
				}
				return client.GetVerification(ctx, args[0])
			})
		},
	}
}

func newVerifyWaitCmd(opts *rootOptions) *cobra.Command {
	var intervalStr, timeoutStr string
	cmd := &cobra.Command{
		Use:   "wait <verificationId>",
		Short: "Poll until verification finished or error",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			interval, err := time.ParseDuration(intervalStr)
			if err != nil {
				return fail("invalid --interval: %v", err)
			}
			timeout, err := time.ParseDuration(timeoutStr)
			if err != nil {
				return fail("invalid --timeout: %v", err)
			}

			client, _, err := newClient(opts, true)
			if err != nil {
				return err
			}

			result, err := client.WaitForVerification(context.Background(), args[0], api.WaitOptions{
				Interval: interval,
				Timeout:  timeout,
			})
			if err != nil {
				return err
			}
			if err := output.JSON(result.Body); err != nil {
				return err
			}
			if result.Status == "error" {
				return fail("verification %s ended with status error", args[0])
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&intervalStr, "interval", "5s", "Poll interval")
	cmd.Flags().StringVar(&timeoutStr, "timeout", "30m", "Overall timeout")
	return cmd
}
