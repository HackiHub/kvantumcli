package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hackihub/kvantumcli/internal/config"
)

func TestRootHelpShowsConfigureAndHidesLogin(t *testing.T) {
	root := newRootCmd(&rootOptions{})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	if err := root.Help(); err != nil {
		t.Fatal(err)
	}
	help := out.String()
	if !strings.Contains(help, "configure") {
		t.Fatalf("configure missing from help:\n%s", help)
	}
	if strings.Contains(help, "\n  login") {
		t.Fatalf("login should be hidden from root help:\n%s", help)
	}
	login, _, err := root.Find([]string{"login"})
	if err != nil || !login.Hidden || login.Deprecated == "" {
		t.Fatalf("legacy login command = %#v, err = %v", login, err)
	}
}

func TestConfigureAndLegacyLoginSaveCompatibleConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("KVANTUMCI_CONFIG", configPath)

	configure := newRootCmd(&rootOptions{})
	configure.SetArgs([]string{"configure", "--api-url", "https://api.example", "--token", "pat-old", "--tenant-id", "tenant-1", "--no-verify"})
	if err := configure.Execute(); err != nil {
		t.Fatalf("configure: %v", err)
	}

	login := newRootCmd(&rootOptions{})
	login.SetArgs([]string{"login", "--token", "pat-new", "--no-verify"})
	if err := login.Execute(); err != nil {
		t.Fatalf("legacy login: %v", err)
	}

	got, err := config.LoadFile()
	if err != nil {
		t.Fatal(err)
	}
	if got.APIURL != "https://api.example" || got.Token != "pat-new" || got.TenantID != "tenant-1" {
		t.Fatalf("saved config = %+v", got)
	}
}

func TestFindingsListRejectsInvalidStatusLocally(t *testing.T) {
	cmd := newFindingsListCmd(&rootOptions{})
	cmd.SetArgs([]string{"--status", "broken"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "must be fail, pass, or skip") {
		t.Fatalf("error = %v", err)
	}
}

func TestFindingsListRejectsInvalidPaginationLocally(t *testing.T) {
	for name, args := range map[string][]string{
		"page":        {"--page", "0"},
		"small limit": {"--limit", "0"},
		"large limit": {"--limit", "101"},
	} {
		cmd := newFindingsListCmd(&rootOptions{})
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "invalid --") {
			t.Errorf("%s error = %v", name, err)
		}
	}
}

func TestProjectListRejectsInvalidPaginationLocally(t *testing.T) {
	for name, args := range map[string][]string{
		"page":        {"--page", "0"},
		"small limit": {"--limit", "0"},
		"large limit": {"--limit", "101"},
	} {
		cmd := newProjectListCmd(&rootOptions{})
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "invalid --") {
			t.Errorf("%s error = %v", name, err)
		}
	}
}

func TestRequiredIdentifierFlags(t *testing.T) {
	for name, cmd := range map[string]interface {
		Execute() error
	}{
		"verify latest":   newVerifyLatestCmd(&rootOptions{}),
		"results summary": newResultsSummaryCmd(&rootOptions{}),
	} {
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "required flag") {
			t.Errorf("%s error = %v", name, err)
		}
	}
}
