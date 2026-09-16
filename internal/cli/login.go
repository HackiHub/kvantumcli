package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/spf13/cobra"

	"github.com/hackihub/kvantumcli/internal/api"
	"github.com/hackihub/kvantumcli/internal/config"
	"github.com/hackihub/kvantumcli/internal/output"
)

func newConfigureCmd(opts *rootOptions) *cobra.Command {
	return newConfigurationCmd(opts, "configure", false)
}

func newLoginCmd(opts *rootOptions) *cobra.Command {
	return newConfigurationCmd(opts, "login", true)
}

func newConfigurationCmd(opts *rootOptions, use string, legacy bool) *cobra.Command {
	var (
		apiURL   string
		token    string
		tenantID string
		noVerify bool
	)

	cmd := &cobra.Command{
		Use:   use,
		Short: "Save PAT/GAT credentials to the local config file",
		Long: `Configure the CLI with an API URL, Personal/Group Access Token, and tenant ID.

The API does not issue credentials. Create a PAT or GAT in the web UI (or via
an already-authenticated session), then run configure to store it locally.

Re-running configure updates only the values you provide; other config fields are kept.

Examples:
	kvantumci configure   # interactive prompts; existing config used as defaults
	KVANTUMCI_TOKEN=<secret> kvantumci configure --api-url https://api.example.com --tenant-id <uuid>

The --token flag remains available, but may expose the token in shell history and process listings.`,
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
			// Bare interactive configure still offers the saved defaults. Partial
			// flags only suppress prompts when all other fields are already known.
			flagsProvided := cmd.Flags().Changed("api-url") || cmd.Flags().Changed("token") || cmd.Flags().Changed("tenant-id") || cmd.InheritedFlags().Changed("api-url") || cmd.InheritedFlags().Changed("token") || cmd.InheritedFlags().Changed("tenant-id")
			promptAll := interactive && !flagsProvided

			apiURLVal, err := resolveLoginField(cmd.Context(), "API URL", "api-url", providedAPI, existing.APIURL, promptAll || interactive && providedAPI == "" && existing.APIURL == "", false)
			if err != nil {
				return err
			}
			tokenVal, err := resolveLoginField(cmd.Context(), "PAT/GAT token", "token", providedToken, existing.Token, promptAll || interactive && providedToken == "" && existing.Token == "", true)
			if err != nil {
				return err
			}
			tenantVal, err := resolveLoginField(cmd.Context(), "Tenant ID", "tenant-id", providedTenant, existing.TenantID, promptAll || interactive && providedTenant == "" && existing.TenantID == "", false)
			if err != nil {
				return err
			}

			// Save merges non-empty fields onto the existing file (reconfiguration preserves omitted keys).
			update := config.Config{
				APIURL:   apiURLVal,
				Token:    tokenVal,
				TenantID: tenantVal,
			}
			if err := update.RequireAPIURL(); err != nil {
				return err
			}
			if err := update.RequireAuth(); err != nil {
				return err
			}

			var whoami any
			if !noVerify {
				client := api.New(update.APIURL, update.Token, update.TenantID)
				raw, err := client.WhoAmI(cmd.Context())
				if err != nil {
					return fail("credentials rejected by API (whoami failed): %v", err)
				}
				whoami, err = safeWhoami(raw)
				if err != nil {
					return err
				}
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
	if legacy {
		cmd.Hidden = true
		cmd.Deprecated = "use 'kvantumci configure' instead"
	}

	cmd.Flags().StringVar(&apiURL, "api-url", "", "API base URL")
	cmd.Flags().StringVar(&token, "token", "", "Bearer PAT or GAT (warning: shell history and process listings may expose it; prefer KVANTUMCI_TOKEN or interactive entry)")
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
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// resolveLoginField picks an explicit value, prompts when requested, or falls back to the file default.
func resolveLoginField(ctx context.Context, label, flag, provided, fileDefault string, promptMissing, secret bool) (string, error) {
	if provided != "" {
		return provided, nil
	}
	if promptMissing {
		return promptLoginField(ctx, label, fileDefault, secret)
	}
	if fileDefault != "" {
		return fileDefault, nil
	}
	return "", fail("%s is required (pass --%s or run interactively in a terminal)", label, flag)
}

func promptLoginField(ctx context.Context, label, fileDefault string, secret bool) (string, error) {
	line, err := readTerminalPrompt(ctx, os.Stdin, secret, func() {
		switch {
		case !secret && fileDefault != "":
			fmt.Fprintf(os.Stderr, "%s [%s]: ", label, fileDefault)
		case secret && fileDefault != "":
			fmt.Fprintf(os.Stderr, "%s [stored; leave empty to keep]: ", label)
		default:
			fmt.Fprintf(os.Stderr, "%s: ", label)
		}
	})
	if secret {
		fmt.Fprintln(os.Stderr)
	}
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

// Read a byte at a time so a terminal secret read cannot lose input to a
// buffered reader's read-ahead.
func readPromptLine(r io.Reader) (string, error) {
	var b strings.Builder
	var one [1]byte
	for {
		n, err := r.Read(one[:])
		if n == 1 {
			if one[0] == '\n' {
				return b.String(), nil
			}
			if b.Len() >= 4096 {
				return "", fmt.Errorf("input too long")
			}
			b.WriteByte(one[0])
		}
		if err != nil {
			if b.Len() > 0 && err == io.EOF {
				return b.String(), nil
			}
			return "", err
		}
		if n == 0 {
			return "", io.ErrNoProgress
		}
	}
}

func safeWhoami(raw json.RawMessage) (map[string]any, error) {
	var envelope struct {
		Data struct {
			ID       string  `json:"id"`
			Email    *string `json:"email"`
			Username *string `json:"username"`
			FullName *string `json:"fullName"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || envelope.Data.ID == "" {
		return nil, fmt.Errorf("whoami response missing identity")
	}
	identity := map[string]any{"id": envelope.Data.ID}
	if envelope.Data.Email != nil {
		identity["email"] = *envelope.Data.Email
	}
	if envelope.Data.Username != nil {
		identity["username"] = *envelope.Data.Username
	}
	if envelope.Data.FullName != nil {
		identity["fullName"] = *envelope.Data.FullName
	}
	return identity, nil
}
