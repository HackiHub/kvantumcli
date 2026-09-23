package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/hackihub/kvantumcli/internal/config"
)

func writeTempConfig(t *testing.T, apiURL, token, tenantID string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	data, err := json.Marshal(map[string]string{
		"apiUrl":   apiURL,
		"token":    token,
		"tenantId": tenantID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KVANTUMCI_CONFIG", path)
	return path
}

func clearEnv(t *testing.T) {
	t.Helper()
	t.Setenv("KVANTUMCI_API_URL", "")
	t.Setenv("KVANTUMCI_TOKEN", "")
	t.Setenv("KVANTUMCI_TENANT_ID", "")
	_ = os.Unsetenv("KVANTUMCI_API_URL")
	_ = os.Unsetenv("KVANTUMCI_TOKEN")
	_ = os.Unsetenv("KVANTUMCI_TENANT_ID")
}

func TestLoad_FlagsOverrideEnvAndFile(t *testing.T) {
	writeTempConfig(t, "https://file.example", "file-token", "file-tenant")
	t.Setenv("KVANTUMCI_API_URL", "https://env.example")
	t.Setenv("KVANTUMCI_TOKEN", "env-token")
	t.Setenv("KVANTUMCI_TENANT_ID", "env-tenant")

	cfg, err := config.Load(config.FlagOverrides{
		APIURL:   "https://flag.example",
		Token:    "flag-token",
		TenantID: "flag-tenant",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIURL != "https://flag.example" || cfg.Token != "flag-token" || cfg.TenantID != "flag-tenant" {
		t.Fatalf("flags should win: %+v", cfg)
	}
}

func TestLoad_EnvOverridesFile(t *testing.T) {
	writeTempConfig(t, "https://file.example", "file-token", "file-tenant")
	clearEnv(t)
	t.Setenv("KVANTUMCI_TOKEN", "env-token")

	cfg, err := config.Load(config.FlagOverrides{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIURL != "https://file.example" {
		t.Fatalf("apiUrl from file: got %q", cfg.APIURL)
	}
	if cfg.Token != "env-token" {
		t.Fatalf("token from env: got %q", cfg.Token)
	}
	if cfg.TenantID != "file-tenant" {
		t.Fatalf("tenant from file: got %q", cfg.TenantID)
	}
}

func TestLoad_FileOnly(t *testing.T) {
	writeTempConfig(t, "https://file.example", "file-token", "file-tenant")
	clearEnv(t)

	cfg, err := config.Load(config.FlagOverrides{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIURL != "https://file.example" || cfg.Token != "file-token" || cfg.TenantID != "file-tenant" {
		t.Fatalf("unexpected: %+v", cfg)
	}
}

func TestLoad_AllValuesFromFlagsDoesNotResolveDefaultConfigPath(t *testing.T) {
	clearEnv(t)
	_ = os.Unsetenv("KVANTUMCI_CONFIG")
	_ = os.Unsetenv("XDG_CONFIG_HOME")
	_ = os.Unsetenv("HOME")

	cfg, err := config.Load(config.FlagOverrides{
		APIURL:   "https://flag.example",
		Token:    "flag-token",
		TenantID: "flag-tenant",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIURL != "https://flag.example" || cfg.Token != "flag-token" || cfg.TenantID != "flag-tenant" {
		t.Fatalf("config = %+v", cfg)
	}
}

func TestRequireAuth(t *testing.T) {
	cfg := config.Config{Token: "t", TenantID: "ten"}
	if err := cfg.RequireAuth(); err != nil {
		t.Fatal(err)
	}
	cfg.Token = ""
	if err := cfg.RequireAuth(); err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestSave_CreatesAndMerges(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	t.Setenv("KVANTUMCI_CONFIG", path)
	clearEnv(t)

	written, err := config.Save(config.Config{
		APIURL:   "https://api.example",
		Token:    "pat_old",
		TenantID: "tenant-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if written != path {
		t.Fatalf("path: got %q want %q", written, path)
	}

	// Re-login: only update token
	if _, err := config.Save(config.Config{Token: "pat_new"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadFile()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIURL != "https://api.example" || cfg.TenantID != "tenant-1" {
		t.Fatalf("other fields should be preserved: %+v", cfg)
	}
	if cfg.Token != "pat_new" {
		t.Fatalf("token: got %q", cfg.Token)
	}
}

func TestLoadFile_Missing(t *testing.T) {
	t.Setenv("KVANTUMCI_CONFIG", filepath.Join(t.TempDir(), "missing.json"))
	cfg, err := config.LoadFile()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIURL != "" || cfg.Token != "" || cfg.TenantID != "" {
		t.Fatalf("expected empty: %+v", cfg)
	}
}
