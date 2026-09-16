package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"github.com/hackihub/kvantumcli/internal/api"
	"github.com/hackihub/kvantumcli/internal/config"
	"github.com/hackihub/kvantumcli/internal/output"
)

type rootOptions struct {
	apiURL   string
	token    string
	tenantID string
}

// Execute runs the root command.
func Execute() {
	opts := &rootOptions{}
	root := newRootCmd(opts)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := root.ExecuteContext(ctx); err != nil {
		output.Error(err)
		os.Exit(1)
	}
}

func newRootCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "kvantumci",
		Short:         "KvantumCI client API CLI (agent-safe)",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.PersistentFlags().StringVar(&opts.apiURL, "api-url", "", "API base URL (or KVANTUMCI_API_URL)")
	cmd.PersistentFlags().StringVar(&opts.token, "token", "", "Bearer PAT/GAT token (warning: visible in shell history/process listings; prefer KVANTUMCI_TOKEN)")
	cmd.PersistentFlags().StringVar(&opts.tenantID, "tenant-id", "", "Tenant ID for x-tenant-id (or KVANTUMCI_TENANT_ID)")

	cmd.AddCommand(newConfigureCmd(opts))
	cmd.AddCommand(newLoginCmd(opts))
	cmd.AddCommand(newHealthCmd(opts))
	cmd.AddCommand(newWhoamiCmd(opts))
	cmd.AddCommand(newProjectCmd(opts))
	cmd.AddCommand(newRepoCmd(opts))
	cmd.AddCommand(newIntegrationCmd(opts))
	cmd.AddCommand(newVerifyCmd(opts))
	cmd.AddCommand(newSbomCmd(opts))
	cmd.AddCommand(newResultsCmd(opts))
	cmd.AddCommand(newFindingsCmd(opts))

	return cmd
}

func loadConfig(opts *rootOptions) (config.Config, error) {
	return config.Load(config.FlagOverrides{
		APIURL:   opts.apiURL,
		Token:    opts.token,
		TenantID: opts.tenantID,
	})
}

func newClient(opts *rootOptions, requireAuth bool) (*api.Client, config.Config, error) {
	cfg, err := loadConfig(opts)
	if err != nil {
		return nil, config.Config{}, err
	}
	if err := cfg.RequireAPIURL(); err != nil {
		return nil, config.Config{}, err
	}
	if requireAuth {
		if err := cfg.RequireAuth(); err != nil {
			return nil, config.Config{}, err
		}
	}
	return api.New(cfg.APIURL, cfg.Token, cfg.TenantID), cfg, nil
}

func runJSON(fn func(ctx context.Context) (any, error)) error {
	ctx := context.Background()
	v, err := fn(ctx)
	if err != nil {
		return err
	}
	return output.JSON(v)
}

func runJSONContext(cmd *cobra.Command, fn func(context.Context) (any, error)) error {
	v, err := fn(cmd.Context())
	if err != nil {
		return err
	}
	return output.JSON(v)
}

func fail(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}
