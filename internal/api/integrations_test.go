package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hackihub/kvantumcli/internal/api"
)

func TestIntegrationSpecificDiscoveryEscapesResourceID(t *testing.T) {
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
	const integrationID = "11111111-1111-4111-8111-111111111111"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.EscapedPath(), "/integrations/byintegration/"+integrationID+"/resources/repo%2Fone/branches"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	if _, err := api.New(srv.URL, "token", "tenant").ListIntegrationBranchesByID(context.Background(), integrationID, "repo/one"); err != nil {
		t.Fatal(err)
	}
}

func TestGetIntegrationProviderReadsTenantVisibleData(t *testing.T) {
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"provider":"github"}}`))
	}))
	defer srv.Close()

	provider, err := api.New(srv.URL, "token", "tenant").GetIntegrationProvider(context.Background(), "id")
	if err != nil || provider != "github" {
		t.Fatalf("provider=%q err=%v", provider, err)
	}
}
