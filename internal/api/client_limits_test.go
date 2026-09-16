package api_test

import (
	"context"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"syscall"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/hackihub/kvantumcli/internal/api"
)

func TestSuccessfulResponseLimit(t *testing.T) {
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
	for _, tc := range []struct {
		name         string
		size         int64
		wantTooLarge bool
	}{
		{"exact", api.MaxSuccessBodyBytes, false},
		{"over", api.MaxSuccessBodyBytes + 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				io.CopyN(w, strings.NewReader(strings.Repeat("x", int(tc.size))), tc.size)
			}))
			defer srv.Close()
			_, err := api.New(srv.URL, "token", "tenant").Get(context.Background(), "/large", nil)
			if errors.Is(err, api.ErrResponseTooLarge) != tc.wantTooLarge {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestErrorArrayAndSafeFallback(t *testing.T) {
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
	for _, tc := range []struct{ name, payload, token, want string }{
		{"array", `{"message":["first","secret",{"private":"hidden"}],"nextStep":"inspect"}`, "secret", "[redacted]"},
		{"html", `<html>secret</html>`, "secret", "non-JSON"},
		{"short credential", `{"message":"x appears here"}`, "x", "suppressed"},
		{"utf8 boundary", `{"message":"` + strings.Repeat("é", 255) + `secret"}`, "secret", "é"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadGateway)
				io.WriteString(w, tc.payload)
			}))
			defer srv.Close()
			_, err := api.New(srv.URL, tc.token, "tenant").Get(context.Background(), "/bad", nil)
			var apiErr *api.APIError
			if !errors.As(err, &apiErr) || !strings.Contains(apiErr.Body, tc.want) || strings.Contains(apiErr.Body, tc.token) || !utf8.ValidString(apiErr.Body) {
				t.Fatalf("unsafe or missing detail: %v", err)
			}
		})
	}
}

type failingTransport struct{ err error }

func (f failingTransport) RoundTrip(*http.Request) (*http.Response, error) { return nil, f.err }

type failingBody struct{}

func (failingBody) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func (failingBody) Close() error             { return nil }

type statusTransport struct{ status int }

func (s statusTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: s.status, Header: make(http.Header), Body: failingBody{}}, nil
}

type readerTransport struct {
	body   io.ReadCloser
	length int64
	status int
}

func (r readerTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: r.status, Header: make(http.Header), Body: r.body, ContentLength: r.length}, nil
}

type trackingBody struct {
	io.Reader
	closed bool
}

func (b *trackingBody) Close() error { b.closed = true; return nil }

func TestLimitIgnoresIncorrectContentLengthAndClosesBody(t *testing.T) {
	body := &trackingBody{Reader: strings.NewReader(strings.Repeat("x", int(api.MaxSuccessBodyBytes+1)))}
	c := api.New("https://example.test", "token", "tenant")
	c.HTTP.Transport = readerTransport{body: body, length: 1, status: http.StatusOK}
	_, err := c.Get(context.Background(), "/v", nil)
	if !errors.Is(err, api.ErrResponseTooLarge) || !body.closed {
		t.Fatalf("err=%v closed=%v", err, body.closed)
	}
}

func TestUnreadableErrorRetainsHTTPStatus(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound} {
		c := api.New("https://example.test", "token", "tenant")
		c.HTTP.Transport = statusTransport{status}
		_, err := c.Get(context.Background(), "/v", nil)
		var apiErr *api.APIError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != status || api.IsTransientRequestError(err) {
			t.Fatalf("status %d: %v", status, err)
		}
	}
}

func TestRequestFailureClassification(t *testing.T) {
	for _, tc := range []struct {
		name      string
		err       error
		kind      api.RequestFailureKind
		transient bool
	}{
		{"timeout", context.DeadlineExceeded, api.RequestFailureTimeout, true},
		{"canceled", context.Canceled, api.RequestFailureCanceled, false},
		{"network", &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}, api.RequestFailureTransient, true},
		{"invalid address", &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("secret invalid address")}, api.RequestFailurePermanent, false},
		{"unknown", errors.New("secret unknown"), api.RequestFailurePermanent, false},
		{"tls", x509.UnknownAuthorityError{}, api.RequestFailureTLS, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := api.New("https://example.test", "token", "tenant")
			c.HTTP.Transport = failingTransport{tc.err}
			_, err := c.Get(context.Background(), "/v", nil)
			var requestErr *api.RequestError
			if !errors.As(err, &requestErr) || requestErr.Kind != tc.kind || api.IsTransientRequestError(err) != tc.transient || strings.Contains(err.Error(), "secret") {
				t.Fatalf("classification: %v", err)
			}
		})
	}
}

func TestRetryAfterBounded(t *testing.T) {
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	_, err := api.New(srv.URL, "token", "tenant").Get(context.Background(), "/v", nil)
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) || apiErr.RetryAfter != time.Minute {
		t.Fatalf("Retry-After: %v", err)
	}
}
