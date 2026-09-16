package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hackihub/kvantumcli/internal/api"
)

func newResultsTestServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
	return httptest.NewServer(handler)
}

func TestListResultsFilteredUsesResultStatusQuery(t *testing.T) {
	srv := newResultsTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/results" {
			t.Errorf("path = %q", r.URL.Path)
		}
		q := r.URL.Query()
		if got := q.Get("verificationId"); got != "verification / one" {
			t.Errorf("verificationId = %q", got)
		}
		if got := q.Get("resultStatus"); got != "fail" {
			t.Errorf("resultStatus = %q", got)
		}
		if _, exists := q["status"]; exists {
			t.Errorf("legacy status query must not be sent: %s", r.URL.RawQuery)
		}
		if q.Get("page") != "3" || q.Get("limit") != "10" {
			t.Errorf("pagination query = %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	client := api.New(srv.URL, "token", "tenant")
	_, err := client.ListResultsFiltered(context.Background(), api.ResultsListOptions{
		VerificationID: "verification / one",
		ResultStatus:   "fail",
		Page:           3,
		Limit:          10,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestListAllResultsFollowsPagination(t *testing.T) {
	var requests int
	srv := newResultsTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		q := r.URL.Query()
		if q.Get("verificationId") != "v1" || q.Get("limit") != "100" {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		page := q.Get("page")
		switch page {
		case "1":
			_, _ = w.Write([]byte(`{"data":[{"status":"fail","finding":{"severity":"high"}}],"total":2,"page":1,"limit":100,"lastPage":2,"hasNext":true}`))
		case "2":
			_, _ = w.Write([]byte(`{"data":[{"status":"skip","finding":null}],"total":2,"page":2,"limit":100,"lastPage":2,"hasNext":false}`))
		default:
			t.Errorf("unexpected page %q", page)
		}
	}))
	defer srv.Close()

	client := api.New(srv.URL, "token", "tenant")
	got, err := client.ListAllResults(context.Background(), "v1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || requests != 2 {
		t.Fatalf("got %d outcomes from %d requests", len(got), requests)
	}
}

func TestListAllResultsRejectsInconsistentPagination(t *testing.T) {
	srv := newResultsTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"data":[{"status":"pass"}],"total":1,"page":1,"limit":100,"lastPage":1,"hasNext":true}`)
	}))
	defer srv.Close()

	client := api.New(srv.URL, "token", "tenant")
	if _, err := client.ListAllResults(context.Background(), "v1"); err == nil {
		t.Fatal("expected inconsistent pagination error")
	}
}

func TestListAllResultsRejectsPaginationMetadataDrift(t *testing.T) {
	var requests int
	srv := newResultsTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch r.URL.Query().Get("page") {
		case "1":
			_, _ = w.Write([]byte(`{"data":[{"status":"pass"}],"total":2,"page":1,"limit":100,"lastPage":2,"hasNext":true}`))
		case "2":
			_, _ = w.Write([]byte(`{"data":[{"status":"fail"}],"total":3,"page":2,"limit":100,"lastPage":3,"hasNext":true}`))
		default:
			t.Errorf("unexpected page %q", r.URL.Query().Get("page"))
		}
	}))
	defer srv.Close()

	client := api.New(srv.URL, "token", "tenant")
	if _, err := client.ListAllResults(context.Background(), "v1"); err == nil {
		t.Fatal("expected pagination metadata drift error")
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
}

func TestListResultsPageRejectsMalformedData(t *testing.T) {
	srv := newResultsTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":null,"total":0,"page":1,"limit":100,"lastPage":0,"hasNext":false}`))
	}))
	defer srv.Close()

	client := api.New(srv.URL, "token", "tenant")
	if _, err := client.ListResultsPage(context.Background(), api.ResultsListOptions{}); err == nil {
		t.Fatal("expected malformed data error")
	}
}

func TestListAllResultsRejectsPrematureFinalPage(t *testing.T) {
	srv := newResultsTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"status":"pass"}],"total":2,"page":1,"limit":100,"lastPage":2,"hasNext":false}`))
	}))
	defer srv.Close()

	client := api.New(srv.URL, "token", "tenant")
	if _, err := client.ListAllResults(context.Background(), "v1"); err == nil {
		t.Fatal("expected premature final page error")
	}
}

func TestListResultsPageRejectsMissingPaginationMetadata(t *testing.T) {
	srv := newResultsTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[],"total":0}`))
	}))
	defer srv.Close()

	client := api.New(srv.URL, "token", "tenant")
	if _, err := client.ListResultsPage(context.Background(), api.ResultsListOptions{}); err == nil {
		t.Fatal("expected missing pagination metadata error")
	}
}

func TestResultOutcomeStatusContract(t *testing.T) {
	for _, tc := range []struct {
		name      string
		body      string
		wantNull  bool
		wantError bool
	}{
		{"null", `{"status":null}`, true, false},
		{"pass", `{"status":"pass"}`, false, false},
		{"missing", `{}`, false, true},
		{"invalid", `{"status":"running"}`, false, true},
		{"wrong type", `{"status":42}`, false, true},
		{"null row", `null`, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got api.ResultOutcome
			err := json.Unmarshal([]byte(tc.body), &got)
			if (err != nil) != tc.wantError {
				t.Fatalf("error = %v", err)
			}
			if err == nil && (got.Status == nil) != tc.wantNull {
				t.Fatalf("status = %v", got.Status)
			}
		})
	}
}

func TestListResultsPageRejectsInvalidMetadata(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"negative total", `{"data":[],"total":-1,"page":1,"limit":100,"lastPage":0,"hasNext":false}`},
		{"zero page", `{"data":[],"total":0,"page":0,"limit":100,"lastPage":0,"hasNext":false}`},
		{"zero limit", `{"data":[],"total":0,"page":1,"limit":0,"lastPage":0,"hasNext":false}`},
		{"oversized limit", `{"data":[],"total":0,"page":1,"limit":101,"lastPage":0,"hasNext":false}`},
		{"negative last page", `{"data":[],"total":0,"page":1,"limit":100,"lastPage":-1,"hasNext":false}`},
		{"page beyond last", `{"data":[{"status":"pass"}],"total":1,"page":2,"limit":100,"lastPage":1,"hasNext":false}`},
		{"rows exceed limit", `{"data":[{"status":"pass"},{"status":"skip"}],"total":2,"page":1,"limit":1,"lastPage":1,"hasNext":false}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := newResultsTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(tc.body)) }))
			defer srv.Close()
			_, err := api.New(srv.URL, "token", "tenant").ListResultsPage(context.Background(), api.ResultsListOptions{})
			if err == nil {
				t.Fatal("expected metadata error")
			}
		})
	}
}

func TestListAllResultsStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	srv := newResultsTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cancel()
		_, _ = w.Write([]byte(`{"data":[{"status":"pass"}],"total":2,"page":1,"limit":100,"lastPage":2,"hasNext":true}`))
	}))
	defer srv.Close()
	_, err := api.New(srv.URL, "token", "tenant").ListAllResults(ctx, "v1")
	if err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("error = %v", err)
	}
}

func TestGetResultRequestAndError(t *testing.T) {
	srv := newResultsTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/results/a%2Fb" {
			t.Errorf("path = %q", r.URL.EscapedPath())
		}
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Errorf("auth header = %q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"id":"a/b","evidence":[]}`))
	}))
	defer srv.Close()
	client := api.New(srv.URL, "token", "tenant")
	raw, err := client.GetResult(context.Background(), "a/b")
	if err != nil || !strings.Contains(string(raw), `"evidence":[]`) {
		t.Fatalf("raw = %s, err = %v", raw, err)
	}
	errorServer := newResultsTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"missing"}`))
	}))
	defer errorServer.Close()
	if _, err := api.New(errorServer.URL, "token", "tenant").GetResult(context.Background(), "missing"); err == nil {
		t.Fatal("expected API error")
	}
}

func TestListAllResultsRejectsPageConsistencyErrors(t *testing.T) {
	for _, tc := range []struct{ name, first, second string }{
		{"wrong page", `{"data":[{"status":"pass"}],"total":1,"page":2,"limit":100,"lastPage":2,"hasNext":false}`, ""},
		{"wrong limit", `{"data":[{"status":"pass"}],"total":1,"page":1,"limit":10,"lastPage":1,"hasNext":false}`, ""},
		{"too many rows", `{"data":[{"status":"pass"},{"status":"pass"}],"total":1,"page":1,"limit":100,"lastPage":1,"hasNext":false}`, ""},
		{"early final count", `{"data":[{"status":"pass"}],"total":2,"page":1,"limit":100,"lastPage":1,"hasNext":false}`, ""},
		{"empty next page", `{"data":[],"total":2,"page":1,"limit":100,"lastPage":2,"hasNext":true}`, ""},
		{"next after total", `{"data":[{"status":"pass"}],"total":1,"page":1,"limit":100,"lastPage":2,"hasNext":true}`, ""},
		{"last page drift", `{"data":[{"status":"pass"}],"total":2,"page":1,"limit":100,"lastPage":2,"hasNext":true}`, `{"data":[{"status":"pass"}],"total":2,"page":2,"limit":100,"lastPage":3,"hasNext":true}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := newResultsTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body := tc.first
				if r.URL.Query().Get("page") == "2" {
					body = tc.second
				}
				_, _ = w.Write([]byte(body))
			}))
			defer srv.Close()
			_, err := api.New(srv.URL, "token", "tenant").ListAllResults(context.Background(), "v1")
			if err == nil {
				t.Fatal("expected pagination error")
			}
		})
	}
}
