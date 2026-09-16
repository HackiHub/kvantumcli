package api

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"
	"unicode/utf8"
)

// MaxSuccessBodyBytes bounds a successful API response, including large BOMs.
const MaxSuccessBodyBytes int64 = 64 << 20

const maxErrorBodyBytes int64 = 1 << 20

// ErrResponseTooLarge identifies a successful response exceeding the client limit.
var ErrResponseTooLarge = errors.New("API response exceeds 64 MiB limit")

// RequestFailureKind classifies a failed HTTP request without exposing its raw error.
type RequestFailureKind string

const (
	RequestFailureCanceled  RequestFailureKind = "canceled"
	RequestFailureTimeout   RequestFailureKind = "timeout"
	RequestFailureTransient RequestFailureKind = "transient"
	RequestFailureTLS       RequestFailureKind = "tls"
	RequestFailureRedirect  RequestFailureKind = "redirect"
	RequestFailurePermanent RequestFailureKind = "permanent"
)

// RequestError contains a safe printable classification of a transport failure.
type RequestError struct {
	Kind  RequestFailureKind
	cause error
}

func (e *RequestError) Error() string {
	if e.Kind == RequestFailureCanceled {
		return "API request failed: context canceled"
	}
	if e.Kind == RequestFailureTimeout {
		return "API request failed: context deadline exceeded"
	}
	return "API request failed: " + string(e.Kind)
}
func (e *RequestError) Unwrap() error { return e.cause }

// IsTransientRequestError reports whether a failed request may be safely retried by a GET caller.
func IsTransientRequestError(err error) bool {
	var requestErr *RequestError
	return errors.As(err, &requestErr) && (requestErr.Kind == RequestFailureTransient || requestErr.Kind == RequestFailureTimeout)
}

// Client talks to the KvantumCI API.
type Client struct {
	BaseURL  string
	Token    string
	TenantID string
	HTTP     *http.Client
}

// New creates a Client with a default HTTP timeout.
func New(baseURL, token, tenantID string) *Client {
	return &Client{
		BaseURL:  strings.TrimRight(baseURL, "/"),
		Token:    token,
		TenantID: tenantID,
		HTTP:     &http.Client{Timeout: 60 * time.Second},
	}
}

// APIError is returned for non-2xx responses.
type APIError struct {
	StatusCode int
	Body       string
	RetryAfter time.Duration
}

func (e *APIError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("API error: HTTP %d", e.StatusCode)
	}
	return fmt.Sprintf("API error: HTTP %d: %s", e.StatusCode, e.Body)
}

// Do executes an HTTP request. When auth is true, Bearer and x-tenant-id are set.
func (c *Client) Do(ctx context.Context, method, path string, query url.Values, body any, auth bool) (json.RawMessage, error) {
	u, err := c.resolveURL(path, query)
	if err != nil {
		return nil, err
	}

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if auth {
		if c.Token != "" {
			req.Header.Set("Authorization", "Bearer "+c.Token)
		}
		if c.TenantID != "" {
			req.Header.Set("x-tenant-id", c.TenantID)
		}
	}

	httpClient := *c.HTTP
	previousRedirect := httpClient.CheckRedirect
	redirectRejected := false
	httpClient.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if err := validateURL(next.URL); err != nil {
			redirectRejected = true
			return err
		}
		if auth && len(via) > 0 && !sameOrigin(via[0].URL, next.URL) {
			redirectRejected = true
			return fmt.Errorf("refusing authenticated cross-origin redirect")
		}
		if previousRedirect != nil {
			err := previousRedirect(next, via)
			if err != nil {
				redirectRejected = true
			}
			return err
		}
		if len(via) >= 10 {
			redirectRejected = true
			return fmt.Errorf("too many redirects")
		}
		return nil
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, classifyRequestError(err, ctx.Err(), redirectRejected)
	}
	defer resp.Body.Close()

	success := resp.StatusCode >= 200 && resp.StatusCode < 300
	limit := maxErrorBodyBytes
	if success {
		limit = MaxSuccessBodyBytes
	}
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		if !success {
			return nil, &APIError{StatusCode: resp.StatusCode, Body: "upstream error response could not be read", RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"))}
		}
		return nil, classifyRequestError(err, ctx.Err(), false)
	}

	if !success {
		body := safeErrorBody(respBody, c.Token)
		if int64(len(respBody)) > maxErrorBodyBytes {
			body = "upstream error response exceeded size limit"
		}
		return nil, &APIError{StatusCode: resp.StatusCode, Body: body, RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"))}
	}
	if int64(len(respBody)) > MaxSuccessBodyBytes {
		return nil, ErrResponseTooLarge
	}

	if len(respBody) == 0 {
		return json.RawMessage("null"), nil
	}
	if !json.Valid(respBody) {
		// Wrap non-JSON success bodies so callers can still print something useful.
		wrapped, _ := json.Marshal(map[string]string{"raw": string(respBody)})
		return wrapped, nil
	}
	return json.RawMessage(respBody), nil
}

func classifyRequestError(err, contextErr error, redirectRejected bool) error {
	if contextErr != nil {
		if errors.Is(contextErr, context.Canceled) {
			return &RequestError{Kind: RequestFailureCanceled, cause: context.Canceled}
		}
		return &RequestError{Kind: RequestFailureTimeout, cause: context.DeadlineExceeded}
	}
	if redirectRejected {
		return &RequestError{Kind: RequestFailureRedirect}
	}
	if errors.Is(err, context.Canceled) {
		return &RequestError{Kind: RequestFailureCanceled, cause: context.Canceled}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &RequestError{Kind: RequestFailureTimeout, cause: context.DeadlineExceeded}
	}
	var certInvalid x509.CertificateInvalidError
	var certUnknown x509.UnknownAuthorityError
	var certHost x509.HostnameError
	var certVerify *tls.CertificateVerificationError
	var tlsHeader tls.RecordHeaderError
	if errors.As(err, &certInvalid) || errors.As(err, &certUnknown) || errors.As(err, &certHost) || errors.As(err, &certVerify) || errors.As(err, &tlsHeader) {
		return &RequestError{Kind: RequestFailureTLS}
	}
	var netErr net.Error
	if errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary()) {
		return &RequestError{Kind: RequestFailureTransient}
	}
	if errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return &RequestError{Kind: RequestFailureTransient}
	}
	return &RequestError{Kind: RequestFailurePermanent}
}

func parseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	var delay time.Duration
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds >= 0 {
		if seconds > 60 {
			seconds = 60
		}
		delay = time.Duration(seconds) * time.Second
	} else if date, err := http.ParseTime(value); err == nil {
		delay = time.Until(date)
	}
	if delay <= 0 {
		return 0
	}
	if delay > time.Minute {
		return time.Minute
	}
	return delay
}

// Get performs an authenticated GET.
func (c *Client) Get(ctx context.Context, path string, query url.Values) (json.RawMessage, error) {
	return c.Do(ctx, http.MethodGet, path, query, nil, true)
}

// GetPublic performs a GET without auth headers.
func (c *Client) GetPublic(ctx context.Context, path string, query url.Values) (json.RawMessage, error) {
	return c.Do(ctx, http.MethodGet, path, query, nil, false)
}

// Post performs an authenticated POST with a JSON body.
func (c *Client) Post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return c.Do(ctx, http.MethodPost, path, nil, body, true)
}

func (c *Client) resolveURL(path string, query url.Values) (string, error) {
	if c.BaseURL == "" {
		return "", fmt.Errorf("api base URL is empty")
	}
	base, err := url.Parse(c.BaseURL)
	if err != nil {
		return "", fmt.Errorf("invalid base URL: %w", err)
	}
	if err := validateURL(base); err != nil {
		return "", err
	}
	if base.RawQuery != "" {
		return "", fmt.Errorf("API base URL must not contain a query")
	}
	rel, err := url.Parse(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	if rel.IsAbs() || rel.Host != "" || rel.Fragment != "" || strings.HasPrefix(path, "//") {
		return "", fmt.Errorf("invalid API endpoint path")
	}
	for _, segment := range strings.Split(rel.Path, "/") {
		if segment == "." || segment == ".." {
			return "", fmt.Errorf("invalid API endpoint path")
		}
	}
	basePrefix := strings.TrimSuffix(base.EscapedPath(), "/")
	escapedPath := basePrefix + "/" + strings.TrimPrefix(rel.EscapedPath(), "/")
	decodedPath, err := url.PathUnescape(escapedPath)
	if err != nil {
		return "", fmt.Errorf("invalid API endpoint path: %w", err)
	}
	u := *base
	u.Path = decodedPath
	u.RawPath = escapedPath
	u.RawQuery = rel.RawQuery
	if query != nil {
		u.RawQuery = query.Encode()
	}
	return u.String(), nil
}

func validateURL(u *url.URL) error {
	if u == nil || u.Host == "" || u.User != nil || u.Fragment != "" || (u.Scheme != "https" && (u.Scheme != "http" || os.Getenv("KVANTUMCI_ALLOW_HTTP") != "true")) {
		return fmt.Errorf("API URL must be absolute HTTPS (set KVANTUMCI_ALLOW_HTTP=true for development HTTP)")
	}
	return nil
}

func sameOrigin(a, b *url.URL) bool {
	return strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Host, b.Host)
}

func redact(s, token string) string {
	s = strings.ToValidUTF8(s, "�")
	if token != "" {
		s = strings.ReplaceAll(s, token, "[redacted]")
	}
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == 127 {
			return ' '
		}
		return r
	}, s)
	if utf8.RuneCountInString(s) > 256 {
		s = string([]rune(s)[:256])
	}
	return s
}

func safeErrorBody(body []byte, token string) string {
	if token != "" && utf8.RuneCountInString(token) <= 3 {
		return "upstream returned an error; details suppressed to protect credentials"
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(body, &fields) != nil {
		return "upstream returned a non-JSON error response"
	}
	result := make(map[string]any)
	for _, key := range []string{"message", "error", "code", "statusCode", "verificationId", "outcome", "scanDispatch", "bomDispatch", "nextStep"} {
		var value any
		if json.Unmarshal(fields[key], &value) == nil {
			switch v := value.(type) {
			case string:
				result[key] = redact(v, token)
			case []any:
				if key != "message" {
					break
				}
				messages := make([]string, 0, 8)
				for _, item := range v {
					if len(messages) == 8 {
						break
					}
					if s, ok := item.(string); ok {
						messages = append(messages, redact(s, token))
					}
				}
				if len(messages) > 0 {
					result[key] = messages
				}
			case float64:
				result[key] = v
			}
		}
	}
	var dispatch map[string]json.RawMessage
	if json.Unmarshal(fields["data"], &dispatch) == nil {
		selected := make(map[string]any)
		for _, key := range []string{"verificationId", "outcome", "scanDispatch", "bomDispatch"} {
			var value any
			if json.Unmarshal(dispatch[key], &value) == nil {
				if s, ok := value.(string); ok {
					selected[key] = redact(s, token)
				}
			}
		}
		if len(selected) > 0 {
			result["data"] = selected
		}
	}
	if len(result) == 0 {
		return "upstream returned no safe error details"
	}
	encoded, _ := json.Marshal(result)
	if len(encoded) > 2048 {
		return "upstream error details exceeded display limit"
	}
	return string(encoded)
}
