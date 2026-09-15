package api_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hackihub/kvantumcli/internal/api"
)

func TestListResultsFilteredUsesResultStatusQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":null,"total":0,"page":1,"limit":100,"lastPage":0,"hasNext":false}`))
	}))
	defer srv.Close()

	client := api.New(srv.URL, "token", "tenant")
	if _, err := client.ListResultsPage(context.Background(), api.ResultsListOptions{}); err == nil {
		t.Fatal("expected malformed data error")
	}
}

func TestListAllResultsRejectsPrematureFinalPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"status":"pass"}],"total":2,"page":1,"limit":100,"lastPage":2,"hasNext":false}`))
	}))
	defer srv.Close()

	client := api.New(srv.URL, "token", "tenant")
	if _, err := client.ListAllResults(context.Background(), "v1"); err == nil {
		t.Fatal("expected premature final page error")
	}
}

func TestListResultsPageRejectsMissingPaginationMetadata(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[],"total":0}`))
	}))
	defer srv.Close()

	client := api.New(srv.URL, "token", "tenant")
	if _, err := client.ListResultsPage(context.Background(), api.ResultsListOptions{}); err == nil {
		t.Fatal("expected missing pagination metadata error")
	}
}
