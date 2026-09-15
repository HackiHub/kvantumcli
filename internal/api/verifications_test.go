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

func TestLatestFinishedVerificationEmpty(t *testing.T) {
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
		"null data":        `{"data":null,"total":0}`,
		"non-array data":   `{"data":{"id":"v1"},"total":1}`,
		"non-object entry": `{"data":["v1"],"total":1}`,
	} {
		t.Run(name, func(t *testing.T) {
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
