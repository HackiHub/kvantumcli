package api_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hackihub/kvantumcli/internal/api"
)

func TestLatestFinishedVerificationFiltersAndReturnsFirst(t *testing.T) {
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/verifications" {
			t.Errorf("path = %q", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("projectRepositoryId") != "repo / one" || q.Get("status") != "finished" || q.Get("page") != "1" || q.Get("limit") != "1" {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"latest"},{"id":"older"}],"total":2}`))
	}))
	defer srv.Close()

	client := api.New(srv.URL, "token", "tenant")
	raw, err := client.LatestFinishedVerification(context.Background(), "repo / one")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"id":"latest"`) || strings.Contains(string(raw), "older") {
		t.Fatalf("unexpected selected verification: %s", raw)
	}
}

func TestListVerificationsPassesPagination(t *testing.T) {
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/verifications" || r.URL.Query().Get("page") != "3" || r.URL.Query().Get("limit") != "250" {
			t.Errorf("request = %s", r.URL.String())
		}
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	if _, err := api.New(srv.URL, "token", "tenant").ListVerifications(context.Background(), 3, 250); err != nil {
		t.Fatal(err)
	}
}

func TestLatestFinishedVerificationEmpty(t *testing.T) {
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[],"total":0}`))
	}))
	defer srv.Close()

	client := api.New(srv.URL, "token", "tenant")
	_, err := client.LatestFinishedVerification(context.Background(), "repo")
	if !errors.Is(err, api.ErrNoFinishedVerification) {
		t.Fatalf("error = %v", err)
	}
}

func TestLatestFinishedVerificationRejectsMalformedData(t *testing.T) {
	for name, body := range map[string]string{
		"missing data":     `{}`,
		"null data":        `{"data":null,"total":0}`,
		"non-array data":   `{"data":{"id":"v1"},"total":1}`,
		"non-object entry": `{"data":["v1"],"total":1}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(body))
			}))
			defer srv.Close()

			client := api.New(srv.URL, "token", "tenant")
			_, err := client.LatestFinishedVerification(context.Background(), "repo")
			if err == nil || !strings.Contains(err.Error(), "parse verifications response") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestRunVerificationForRepoPassesThroughTypedAndLegacyResponses(t *testing.T) {
	for name, body := range map[string]string{
		"typed":        `{"statusCode":202,"data":{"verificationId":"v1","outcome":"accepted","scanDispatch":{"status":"accepted"},"bomDispatch":{"status":"accepted"}}}`,
		"legacy empty": `{}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodPost || r.URL.EscapedPath() != "/verifications/run/repository/repo%20%2F%20one/tenant" {
					t.Errorf("method/path = %s %s", r.Method, r.URL.EscapedPath())
				}
				_, _ = w.Write([]byte(body))
			}))
			defer srv.Close()
			raw, err := api.New(srv.URL, "token", "tenant").RunVerificationForRepo(context.Background(), "repo / one", "tenant", api.RunVerificationRequest{Types: []string{"sbom"}})
			if err != nil || string(raw) != body || calls != 1 {
				t.Fatalf("raw = %s, calls = %d, error = %v", raw, calls, err)
			}
		})
	}
}

func TestRunVerificationForRepoDoesNotRetryDispatchFailure(t *testing.T) {
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"message":"dispatch failed","verificationId":"v1"}`))
	}))
	defer srv.Close()
	_, err := api.New(srv.URL, "token", "tenant").RunVerificationForRepo(context.Background(), "repo", "tenant", api.RunVerificationRequest{})
	if err == nil || calls != 1 {
		t.Fatalf("calls = %d, error = %v", calls, err)
	}
}
