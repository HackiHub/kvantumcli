package config_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/hackihub/kvantumcli/internal/config"
)

func TestSaveReplacesPublicFilePrivately(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("KVANTUMCI_CONFIG", path)
	if err := os.WriteFile(path, []byte(`{"apiUrl":"https://api.example","token":"old","tenantId":"tenant"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Save(config.Config{Token: "new"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"token": "new"`) {
		t.Fatalf("unexpected config: %s", data)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("config mode = %o", info.Mode().Perm())
		}
	}
}

func TestSaveRejectsSymlinkWithoutChangingTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.json")
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(target, []byte(`{"token":"unchanged"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	t.Setenv("KVANTUMCI_CONFIG", path)
	if _, err := config.Save(config.Config{Token: "new"}); err == nil {
		t.Fatal("expected symlink rejection")
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"token":"unchanged"}` {
		t.Fatalf("target changed: %s", data)
	}
}

func TestRequireAPIURLSecurityPolicy(t *testing.T) {
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "")
	for _, input := range []string{"http://localhost:3000", "//example.com", "https://user:pass@example.com", "https://example.com?x=1", "https://example.com/#frag"} {
		if err := (config.Config{APIURL: input}).RequireAPIURL(); err == nil {
			t.Errorf("accepted %q", input)
		}
	}
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
	if err := (config.Config{APIURL: "http://localhost:3000/api"}).RequireAPIURL(); err != nil {
		t.Fatal(err)
	}
}
