package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hackihub/kvantumcli/internal/api"
	"github.com/hackihub/kvantumcli/internal/config"
	"github.com/hackihub/kvantumcli/internal/output"
)

func newLoginCmd(opts *rootOptions) *cobra.Command {
	var (
		apiURL   string
		token    string
		tenantID string
		noVerify bool
	)

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Save PAT/GAT credentials to the local config file",
		Long: `Configure the CLI with an API URL, Personal/Group Access Token, and tenant ID.

The API does not issue credentials. Create a PAT or GAT in the web UI (or via
an already-authenticated session), then run login to store it locally.

Re-running login updates only the values you provide; other config fields are kept.

Examples:
  kvantumci login --api-url https://api.example.com --token pat_... --tenant-id <uuid>
  kvantumci login   # interactive prompts; existing config used as defaults`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			existing, err := config.LoadFile()
			if err != nil {
				return err
			}

			// Explicit inputs only (flags + env). File values are merge defaults, not "provided".
			providedAPI := firstNonEmpty(apiURL, opts.apiURL, os.Getenv("KVANTUMCI_API_URL"))
			providedToken := firstNonEmpty(token, opts.token, os.Getenv("KVANTUMCI_TOKEN"))
			providedTenant := firstNonEmpty(tenantID, opts.tenantID, os.Getenv("KVANTUMCI_TENANT_ID"))

			interactive := isInteractive()
			anyExplicit := providedAPI != "" || providedToken != "" || providedTenant != ""
			// Prompt only for a full interactive login (no flags/env). Partial updates use the file.
			promptMissing := interactive && !anyExplicit

			apiURLVal, err := resolveLoginField("API URL", "api-url", providedAPI, existing.APIURL, promptMissing)
			if err != nil {
				return err
			}
			tokenVal, err := resolveLoginField("PAT/GAT token", "token", providedToken, existing.Token, promptMissing)
			if err != nil {
				return err
			}
			tenantVal, err := resolveLoginField("Tenant ID", "tenant-id", providedTenant, existing.TenantID, promptMissing)
			if err != nil {
				return err
			}

			// Save merges non-empty fields onto the existing file (re-login preserves omitted keys).
			update := config.Config{
				APIURL:   apiURLVal,
				Token:    tokenVal,
				TenantID: tenantVal,
			}

			var whoami any
			if !noVerify {
				client := api.New(update.APIURL, update.Token, update.TenantID)
				raw, err := client.WhoAmI(context.Background())
				if err != nil {
					return fail("credentials rejected by API (whoami failed): %v", err)
				}
				whoami = raw
			}

			path, err := config.Save(update)
			if err != nil {
				return err
			}

			result := map[string]any{
				"configPath": path,
				"apiUrl":     update.APIURL,
				"tenantId":   update.TenantID,
			}
			if whoami != nil {
				result["whoami"] = whoami
			}
			return output.JSON(result)
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", "", "API base URL")
	cmd.Flags().StringVar(&token, "token", "", "Bearer PAT or GAT (pat_… / gat_…)")
	cmd.Flags().StringVar(&tenantID, "tenant-id", "", "Tenant ID for x-tenant-id")
	cmd.Flags().BoolVar(&noVerify, "no-verify", false, "Skip whoami check before saving")
	return cmd
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func isInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// resolveLoginField picks an explicit value, prompts when requested, or falls back to the file default.
func resolveLoginField(label, flag, provided, fileDefault string, promptMissing bool) (string, error) {
	if provided != "" {
		return provided, nil
	}
	if promptMissing {
		return promptLoginField(label, fileDefault, strings.Contains(strings.ToLower(label), "token"))
	}
	if fileDefault != "" {
		return fileDefault, nil
	}
	return "", fail("%s is required (pass --%s or run interactively in a terminal)", label, flag)
}

func promptLoginField(label, fileDefault string, secret bool) (string, error) {
	switch {
	case !secret && fileDefault != "":
		fmt.Fprintf(os.Stderr, "%s [%s]: ", label, fileDefault)
	case secret && fileDefault != "":
		fmt.Fprintf(os.Stderr, "%s [stored; leave empty to keep]: ", label)
	default:
		fmt.Fprintf(os.Stderr, "%s: ", label)
	}

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", fail("read %s: %v", label, err)
	}
	line = strings.TrimSpace(line)
	if line == "" {
		if fileDefault != "" {
			return fileDefault, nil
		}
		return "", fail("%s is required", label)
	}
	return line, nil
}
