package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hackihub/kvantumcli/internal/api"
)

const (
	maxProjectTags          = 20
	maxProjectTagUTF16Units = 64
)

func newProjectCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage projects",
	}
	cmd.AddCommand(newProjectCreateCmd(opts))
	cmd.AddCommand(newProjectListCmd(opts))
	cmd.AddCommand(newProjectGetCmd(opts))
	return cmd
}

func newProjectCreateCmd(opts *rootOptions) *cobra.Command {
	var name, parentID, icon, tagsCSV string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a project",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJSON(func(ctx context.Context) (any, error) {
				tags := splitCSV(tagsCSV)
				if err := validateProjectTags(tags); err != nil {
					return nil, err
				}
				client, _, err := newClient(opts, true)
				if err != nil {
					return nil, err
				}
				req := api.CreateProjectRequest{Name: name}
				if parentID != "" {
					req.ParentID = &parentID
				}
				if icon != "" {
					req.Icon = &icon
				}
				if len(tags) > 0 {
					req.Tags = tags
				}
				return client.CreateProject(ctx, req)
			})
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Project name")
	_ = cmd.MarkFlagRequired("name")
	cmd.Flags().StringVar(&parentID, "parent-id", "", "Parent project UUID")
	cmd.Flags().StringVar(&icon, "icon", "", "Icon identifier")
	cmd.Flags().StringVar(&tagsCSV, "tags", "", "Comma-separated tags")
	return cmd
}

func newProjectListCmd(opts *rootOptions) *cobra.Command {
	var page, limit int
	var search, tagsCSV string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List projects",
		Args:  cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
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
				return client.ListProjects(ctx, page, limit, search, splitCSV(tagsCSV))
			})
		},
	}
	cmd.Flags().IntVar(&page, "page", 1, "Page number")
	cmd.Flags().IntVar(&limit, "limit", 10, "Page size (1-100)")
	cmd.Flags().StringVar(&search, "search", "", "Search query")
	cmd.Flags().StringVar(&tagsCSV, "tags", "", "Comma-separated tags")
	return cmd
}

func newProjectGetCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "get <projectId>",
		Short: "Get project detail with child tree and repos",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJSON(func(ctx context.Context) (any, error) {
				client, _, err := newClient(opts, true)
				if err != nil {
					return nil, err
				}
				return client.GetProject(ctx, args[0])
			})
		},
	}
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func validateProjectTags(tags []string) error {
	if len(tags) > maxProjectTags {
		return fmt.Errorf("invalid --tags: at most %d tags are allowed", maxProjectTags)
	}
	for _, tag := range tags {
		if strings.TrimSpace(tag) == "" {
			return fmt.Errorf("invalid --tags: tags must not be blank")
		}
		if utf16Units(tag) > maxProjectTagUTF16Units {
			return fmt.Errorf("invalid --tags: each tag must be at most %d UTF-16 code units", maxProjectTagUTF16Units)
		}
	}
	return nil
}

func utf16Units(s string) int {
	units := 0
	for _, r := range s {
		units++
		if r > 0xffff {
			units++
		}
	}
	return units
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
