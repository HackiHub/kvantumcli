package cli

import (
	"context"

	"github.com/spf13/cobra"
)

func newSbomCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sbom",
		Short: "Read bill-of-materials records",
	}
	cmd.AddCommand(newSbomGetCmd(opts))
	cmd.AddCommand(newSbomListCmd(opts))
	return cmd
}

func newSbomGetCmd(opts *rootOptions) *cobra.Command {
	var verificationID, repoID string
	cmd := &cobra.Command{
		Use:   "get [bomId]",
		Short: "Get BOMs by verification+repo, or a single BOM by id",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJSON(func(ctx context.Context) (any, error) {
				client, _, err := newClient(opts, true)
				if err != nil {
					return nil, err
				}
				if len(args) == 1 {
					if verificationID != "" || repoID != "" {
						return nil, fail("do not combine <bomId> with --verification/--repo")
					}
					return client.GetBOM(ctx, args[0])
				}
				if verificationID == "" || repoID == "" {
					return nil, fail("either <bomId> or both --verification and --repo are required")
				}
				return client.GetBOMsByVerification(ctx, verificationID, repoID)
			})
		},
	}
	cmd.Flags().StringVar(&verificationID, "verification", "", "Verification UUID")
	cmd.Flags().StringVar(&repoID, "repo", "", "Project repository UUID")
	return cmd
}

func newSbomListCmd(opts *rootOptions) *cobra.Command {
	var verificationID, repoID, bomType string
	var page, limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Paginated BOM list for a verification and repo",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJSON(func(ctx context.Context) (any, error) {
				client, _, err := newClient(opts, true)
				if err != nil {
					return nil, err
				}
				return client.ListBOMs(ctx, verificationID, repoID, bomType, page, limit)
			})
		},
	}
	cmd.Flags().StringVar(&verificationID, "verification", "", "Verification UUID")
	cmd.Flags().StringVar(&repoID, "repo", "", "Project repository UUID")
	cmd.Flags().StringVar(&bomType, "type", "", "BOM type: sbom|cbom|aibom|mlbom")
	_ = cmd.MarkFlagRequired("verification")
	_ = cmd.MarkFlagRequired("repo")
	_ = cmd.MarkFlagRequired("type")
	cmd.Flags().IntVar(&page, "page", 1, "Page number")
	cmd.Flags().IntVar(&limit, "limit", 10, "Page size")
	return cmd
}
