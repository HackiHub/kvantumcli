package api_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hackihub/kvantumcli/internal/api"
)

func TestTemplateRegistrationRequests(t *testing.T) {
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		calls = append(calls, r.Method+" "+r.URL.RequestURI()+" "+string(body))
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer srv.Close()

	c := api.New(srv.URL, "token", "tenant")
	ctx := context.Background()
	if _, err := c.CreateTemplateRegistration(ctx, api.CreateTemplateRegistrationRequest{ProjectRepositoryID: "r1", RequestedRef: "v1.0.0", RefType: "TAG"}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ListTemplateRegistrations(ctx, "r1", 2, 5); err != nil {
		t.Fatal(err)
	}
	if _, err := c.RefreshTemplateRegistration(ctx, "id/1"); err != nil {
		t.Fatal(err)
	}
	want := []string{
		`POST /capabilities/registrations {"projectRepositoryId":"r1","requestedRef":"v1.0.0","refType":"TAG"}`,
		`GET /capabilities/registrations?limit=5&page=2&projectRepositoryId=r1 `,
		`POST /capabilities/registrations/id%2F1/refresh `,
	}
	for i, w := range want {
		if i >= len(calls) || calls[i] != w {
			t.Errorf("call %d = %q, want %q", i, calls, w)
		}
	}
}
