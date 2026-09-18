package cli

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestHistoryListPaginationAndValidation(t *testing.T) {
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
	requests := make([]string, 0, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.String())
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()
	opts := &rootOptions{apiURL: srv.URL, token: "token", tenantID: "tenant"}
	for _, tc := range []struct {
		cmd  *cobra.Command
		args []string
	}{
		{newVerifyListCmd(opts), []string{"--page", "2", "--limit", "300"}},
		{newResultsListCmd(opts), []string{"--verification", "v1", "--page", "3", "--limit", "50"}},
	} {
		tc.cmd.SetArgs(tc.args)
		if err := tc.cmd.Execute(); err != nil {
			t.Fatal(err)
		}
	}
	if len(requests) != 2 || !strings.Contains(requests[0], "page=2") || !strings.Contains(requests[0], "limit=300") || !strings.Contains(requests[1], "page=3") || !strings.Contains(requests[1], "limit=50") || !strings.Contains(requests[1], "verificationId=v1") {
		t.Fatalf("requests = %v", requests)
	}
	for _, tc := range []struct {
		cmd  *cobra.Command
		args []string
	}{
		{newVerifyListCmd(opts), []string{"--limit", "501"}},
		{newResultsListCmd(opts), []string{"--page", "0"}},
	} {
		tc.cmd.SetArgs(tc.args)
		if err := tc.cmd.Execute(); err == nil || !strings.Contains(err.Error(), "invalid --") {
			t.Fatalf("error = %v", err)
		}
	}
	if len(requests) != 2 {
		t.Fatalf("invalid pagination made requests: %v", requests)
	}
}

func TestIntegrationSelectionAndProjectTagLimits(t *testing.T) {
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
	const integrationID = "11111111-1111-4111-8111-111111111111"
	var requests []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.EscapedPath())
		switch r.URL.Path {
		case "/integrations/" + integrationID:
			_, _ = w.Write([]byte(`{"data":{"provider":"github"}}`))
		default:
			_, _ = w.Write([]byte(`{"data":[]}`))
		}
	}))
	defer srv.Close()
	opts := &rootOptions{apiURL: srv.URL, token: "token", tenantID: "tenant"}
	cmd := newIntegrationBranchesCmd(opts)
	cmd.SetArgs([]string{"github", "repo/one", "--integration", integrationID})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 2 || requests[1] != "/integrations/byintegration/"+integrationID+"/resources/repo%2Fone/branches" {
		t.Fatalf("requests = %v", requests)
	}
	cmd = newIntegrationResourcesCmd(opts)
	cmd.SetArgs([]string{"gitlab", "--integration", integrationID})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "uses provider") {
		t.Fatalf("error = %v", err)
	}
	if err := validateProjectTags([]string{strings.Repeat("a", 64), strings.Repeat("😀", 32)}); err != nil {
		t.Fatal(err)
	}
	if err := validateProjectTags([]string{strings.Repeat("😀", 33)}); err == nil {
		t.Fatal("expected UTF-16 validation error")
	}
	if err := validateProjectTags(make([]string, 21)); err == nil {
		t.Fatal("expected tag count validation error")
	}
}
