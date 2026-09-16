package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hackihub/kvantumcli/internal/api"
)

func TestClient_AuthHeadersAndPath(t *testing.T) {
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
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
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
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
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
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
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
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
	if _, ok := body["parentId"]; !ok {
		t.Fatalf("parentId missing: %+v", body)
	}
	if body["parentId"] != nil {
		t.Fatalf("parentId want null, got %#v", body["parentId"])
	}
	if _, ok := body["icon"]; !ok {
		t.Fatalf("icon missing: %+v", body)
	}
	if body["icon"] != nil {
		t.Fatalf("icon want null, got %#v", body["icon"])
	}
}

func TestClient_CreateProjectExplicitParentAndIcon(t *testing.T) {
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"id":"1"}`))
	}))
	defer srv.Close()

	parentID := "11111111-1111-1111-1111-111111111111"
	icon := "folder"
	client := api.New(srv.URL, "pat_abc", "tenant-1")
	_, err := client.CreateProject(context.Background(), api.CreateProjectRequest{
		Name:     "child",
		ParentID: &parentID,
		Icon:     &icon,
	})
	if err != nil {
		t.Fatal(err)
	}
	if body["parentId"] != parentID {
		t.Fatalf("parentId: %#v", body["parentId"])
	}
	if body["icon"] != icon {
		t.Fatalf("icon: %#v", body["icon"])
	}
}

func TestWaitForVerification_Finished(t *testing.T) {
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		i := n.Add(1)
		status := "running"
		if i >= 2 {
			status = "finished"
		}
		_, _ = w.Write([]byte(`{"data":{"id":"v1","status":"` + status + `"}}`))
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
	if string(result.Body) != `{"data":{"id":"v1","status":"finished"}}` {
		t.Fatalf("body: %s", result.Body)
	}
	if n.Load() < 2 {
		t.Fatalf("expected at least 2 polls, got %d", n.Load())
	}
}

func TestWaitForVerification_Timeout(t *testing.T) {
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"id":"v1","status":"running"}}`))
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

func TestWaitForVerification_TerminalError(t *testing.T) {
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"data":{"id":"v1","status":"error"}}`)
	}))
	defer srv.Close()
	result, err := api.New(srv.URL, "token", "tenant").WaitForVerification(context.Background(), "v1", api.WaitOptions{Interval: time.Millisecond, Timeout: time.Second})
	if err != nil || result == nil || result.Status != "error" {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
}

func TestWaitForVerification_RejectsMalformedDetail(t *testing.T) {
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
	for name, body := range map[string]string{
		"missing data":   `{"status":"finished"}`,
		"null data":      `{"data":null}`,
		"array data":     `{"data":[]}`,
		"missing status": `{"data":{"id":"v1"}}`,
		"null status":    `{"data":{"status":null}}`,
		"numeric status": `{"data":{"status":1}}`,
		"empty status":   `{"data":{"status":""}}`,
		"unknown status": `{"data":{"status":"bogus"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			var polls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				polls.Add(1)
				_, _ = io.WriteString(w, body)
			}))
			defer srv.Close()
			_, err := api.New(srv.URL, "token", "tenant").WaitForVerification(context.Background(), "v1", api.WaitOptions{Interval: time.Millisecond, Timeout: time.Second})
			if err == nil || !strings.Contains(err.Error(), "parse verification status") || polls.Load() != 1 {
				t.Fatalf("polls = %d, error = %v", polls.Load(), err)
			}
		})
	}
}

func TestWaitForVerification_RetriesTransientStatus(t *testing.T) {
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
	var polls atomic.Int32
	var delays []time.Duration
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if polls.Add(1) == 1 {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = io.WriteString(w, `{"data":{"status":"finished"}}`)
	}))
	defer srv.Close()
	result, err := api.New(srv.URL, "token", "tenant").WaitForVerification(context.Background(), "v1", api.WaitOptions{Interval: time.Millisecond, Timeout: time.Second, Sleep: func(_ context.Context, d time.Duration) error {
		delays = append(delays, d)
		return nil
	}})
	if err != nil || result.Status != "finished" || polls.Load() != 2 || len(delays) != 1 || delays[0] != 2*time.Second {
		t.Fatalf("result = %#v, polls = %d, delays = %v, error = %v", result, polls.Load(), delays, err)
	}
}

func TestWaitForVerification_RetriesTransportFailure(t *testing.T) {
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
	client := api.New("http://example.test", "token", "tenant")
	var polls int
	client.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		polls++
		if polls == 1 {
			return nil, &net.DNSError{Err: "temporary failure", IsTemporary: true}
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"data":{"status":"finished"}}`)), Header: make(http.Header)}, nil
	})
	result, err := client.WaitForVerification(context.Background(), "v1", api.WaitOptions{Interval: time.Millisecond, Timeout: time.Second, Sleep: func(context.Context, time.Duration) error { return nil }})
	if err != nil || result.Status != "finished" || polls != 2 {
		t.Fatalf("result = %#v, polls = %d, error = %v", result, polls, err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestWaitForVerification_RejectsPermanentStatus(t *testing.T) {
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
	for _, status := range []int{400, 401, 403, 404, 422} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var polls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { polls.Add(1); w.WriteHeader(status) }))
			defer srv.Close()
			_, err := api.New(srv.URL, "token", "tenant").WaitForVerification(context.Background(), "v1", api.WaitOptions{Interval: time.Millisecond, Timeout: time.Second, Sleep: func(context.Context, time.Duration) error { t.Fatal("unexpected retry"); return nil }})
			var apiErr *api.APIError
			if !errors.As(err, &apiErr) || apiErr.StatusCode != status || polls.Load() != 1 {
				t.Fatalf("polls = %d, error = %v", polls.Load(), err)
			}
		})
	}
}

func TestWaitForVerification_DeadlinePreservesLastFailure(t *testing.T) {
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
	defer srv.Close()
	_, err := api.New(srv.URL, "token", "tenant").WaitForVerification(ctx, "v1", api.WaitOptions{Interval: time.Millisecond, Timeout: 10 * time.Millisecond, Sleep: func(ctx context.Context, _ time.Duration) error { <-ctx.Done(); return ctx.Err() }})
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "HTTP 503") {
		t.Fatalf("error = %v", err)
	}
}

func TestWaitForVerification_CancellationDuringBackoff(t *testing.T) {
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var polls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		polls.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	_, err := api.New(srv.URL, "token", "tenant").WaitForVerification(ctx, "v1", api.WaitOptions{Interval: time.Millisecond, Timeout: time.Second, Sleep: func(ctx context.Context, _ time.Duration) error {
		cancel()
		return ctx.Err()
	}})
	if !errors.Is(err, context.Canceled) || polls.Load() != 1 {
		t.Fatalf("polls = %d, error = %v", polls.Load(), err)
	}
}

func TestWaitForVerification_BackoffResetsAfterSuccess(t *testing.T) {
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
	var polls atomic.Int32
	var delays []time.Duration
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch polls.Add(1) {
		case 1, 2, 4:
			w.WriteHeader(http.StatusServiceUnavailable)
		case 3:
			_, _ = io.WriteString(w, `{"data":{"status":"running"}}`)
		default:
			_, _ = io.WriteString(w, `{"data":{"status":"finished"}}`)
		}
	}))
	defer srv.Close()
	result, err := api.New(srv.URL, "token", "tenant").WaitForVerification(context.Background(), "v1", api.WaitOptions{Interval: 10 * time.Millisecond, Timeout: time.Second, Sleep: func(_ context.Context, d time.Duration) error {
		delays = append(delays, d)
		return nil
	}})
	want := []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 10 * time.Millisecond, 10 * time.Millisecond}
	if err != nil || result.Status != "finished" || !slices.Equal(delays, want) {
		t.Fatalf("result = %#v, delays = %v, error = %v", result, delays, err)
	}
}

func TestWaitForVerification_Cancel(t *testing.T) {
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
	ctx, cancel := context.WithCancel(context.Background())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"data":{"status":"running"}}`)
		cancel()
	}))
	defer srv.Close()
	_, err := api.New(srv.URL, "token", "tenant").WaitForVerification(ctx, "v1", api.WaitOptions{Interval: time.Millisecond, Timeout: time.Second})
	if !errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "timed out") {
		t.Fatalf("error = %v", err)
	}
}
