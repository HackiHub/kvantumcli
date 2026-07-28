package api_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hackihub/kvantumci-cli/internal/api"
)

func TestClient_AuthHeadersAndPath(t *testing.T) {
	var gotAuth, gotTenant, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotTenant = r.Header.Get("x-tenant-id")
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	client := api.New(srv.URL, "pat_abc", "tenant-1")
	raw, err := client.WhoAmI(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer pat_abc" {
		t.Fatalf("Authorization: got %q", gotAuth)
	}
	if gotTenant != "tenant-1" {
		t.Fatalf("x-tenant-id: got %q", gotTenant)
	}
	if gotPath != "/users/me" {
		t.Fatalf("path: got %q", gotPath)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil || m["ok"] != true {
		t.Fatalf("body: %s", raw)
	}
}

func TestClient_HealthPublicNoAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" || r.Header.Get("x-tenant-id") != "" {
			t.Errorf("health should not send auth headers")
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	client := api.New(srv.URL, "pat_abc", "tenant-1")
	if _, err := client.Health(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestClient_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"message":"denied"}`)
	}))
	defer srv.Close()

	client := api.New(srv.URL, "pat_abc", "tenant-1")
	_, err := client.WhoAmI(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*api.APIError)
	if !ok {
		t.Fatalf("want *APIError, got %T", err)
	}
	if apiErr.StatusCode != 403 {
		t.Fatalf("status: %d", apiErr.StatusCode)
	}
	if !strings.Contains(apiErr.Body, "denied") {
		t.Fatalf("body: %q", apiErr.Body)
	}
}

func TestClient_PostJSON(t *testing.T) {
	var method, contentType string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		contentType = r.Header.Get("Content-Type")
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"id":"1"}`))
	}))
	defer srv.Close()

	client := api.New(srv.URL, "pat_abc", "tenant-1")
	_, err := client.CreateProject(context.Background(), api.CreateProjectRequest{Name: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPost {
		t.Fatalf("method: %s", method)
	}
	if contentType != "application/json" {
		t.Fatalf("content-type: %s", contentType)
	}
	if body["name"] != "demo" {
		t.Fatalf("body: %+v", body)
	}
}

func TestWaitForVerification_Finished(t *testing.T) {
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		i := n.Add(1)
		status := "running"
		if i >= 2 {
			status = "finished"
		}
		_, _ = w.Write([]byte(`{"id":"v1","status":"` + status + `"}`))
	}))
	defer srv.Close()

	client := api.New(srv.URL, "pat_abc", "tenant-1")
	result, err := client.WaitForVerification(context.Background(), "v1", api.WaitOptions{
		Interval: time.Millisecond,
		Timeout:  time.Second,
		Sleep: func(ctx context.Context, d time.Duration) error {
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "finished" {
		t.Fatalf("status: %s", result.Status)
	}
	if n.Load() < 2 {
		t.Fatalf("expected at least 2 polls, got %d", n.Load())
	}
}

func TestWaitForVerification_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"v1","status":"running"}`))
	}))
	defer srv.Close()

	client := api.New(srv.URL, "pat_abc", "tenant-1")
	_, err := client.WaitForVerification(context.Background(), "v1", api.WaitOptions{
		Interval: time.Millisecond,
		Timeout:  20 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("expected timeout")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("error: %v", err)
	}
}
