package api_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hackihub/kvantumcli/internal/api"
)

func TestClientRejectsPlainHTTPBeforeRequest(t *testing.T) {
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "")
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer srv.Close()
	_, err := api.New(srv.URL, "secret", "tenant").WhoAmI(context.Background())
	if err == nil || called {
		t.Fatalf("err=%v called=%v", err, called)
	}
}

func TestClientRetainsBasePathAndEscapedID(t *testing.T) {
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.EscapedPath(); got != "/api/v/abc%2Fdef" {
			t.Errorf("path=%q", got)
		}
		io.WriteString(w, `{}`)
	}))
	defer srv.Close()
	if _, err := api.New(srv.URL+"/api", "secret", "tenant").Get(context.Background(), "/v/abc%2Fdef", nil); err != nil {
		t.Fatal(err)
	}
}

func TestClientRefusesCredentialCrossOriginRedirect(t *testing.T) {
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
	called := false
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer destination.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL, http.StatusFound)
	}))
	defer source.Close()
	_, err := api.New(source.URL, "secret", "tenant").WhoAmI(context.Background())
	if err == nil || called {
		t.Fatalf("err=%v called=%v", err, called)
	}
}

func TestClientErrorBodyIsBoundedAndRedacted(t *testing.T) {
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"message":"token-secret\n`+strings.Repeat("x", 1000)+`","private":"do-not-print"}`)
	}))
	defer srv.Close()
	_, err := api.New(srv.URL, "token-secret", "tenant").WhoAmI(context.Background())
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err=%v", err)
	}
	if strings.Contains(apiErr.Error(), "token-secret") || strings.Contains(apiErr.Error(), "do-not-print") || len(apiErr.Error()) > 400 {
		t.Fatalf("unsafe error: %q", apiErr.Error())
	}
}

func TestClientPreservesSafeDispatchErrorFields(t *testing.T) {
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		io.WriteString(w, `{"message":"dispatch uncertain","verificationId":"v1","outcome":"unknown","nextStep":"inspect_verification_before_retry","internalSecret":"hidden"}`)
	}))
	defer srv.Close()
	_, err := api.New(srv.URL, "secret", "tenant").WhoAmI(context.Background())
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err=%v", err)
	}
	if !strings.Contains(apiErr.Body, "inspect_verification_before_retry") || strings.Contains(apiErr.Body, "hidden") {
		t.Fatalf("body=%q", apiErr.Body)
	}
}

func TestClientDoesNotTruncateLargeSuccessfulJSON(t *testing.T) {
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
	large := strings.Repeat("x", 1<<20)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"data":"`+large+`"}`)
	}))
	defer srv.Close()
	raw, err := api.New(srv.URL, "secret", "tenant").WhoAmI(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), large) {
		t.Fatalf("large JSON success was truncated (len=%d)", len(raw))
	}
}

func TestClientBoundsLargeErrorBody(t *testing.T) {
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		io.WriteString(w, `{"message":"`+strings.Repeat("x", 2<<20)+`"}`)
	}))
	defer srv.Close()
	_, err := api.New(srv.URL, "secret", "tenant").WhoAmI(context.Background())
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusBadGateway || len(apiErr.Error()) > 400 {
		t.Fatalf("unbounded or missing API error: %v", err)
	}
}
