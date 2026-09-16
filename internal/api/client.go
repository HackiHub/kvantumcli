package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

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
	httpClient.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if err := validateURL(next.URL); err != nil {
			return err
		}
		if auth && len(via) > 0 && !sameOrigin(via[0].URL, next.URL) {
			return fmt.Errorf("refusing authenticated cross-origin redirect")
		}
		if previousRedirect != nil {
			return previousRedirect(next, via)
		}
		if len(via) >= 10 {
			return fmt.Errorf("too many redirects")
		}
		return nil
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("request deadline exceeded: %w", context.DeadlineExceeded)
		}
		return nil, fmt.Errorf("request failed: %s", redact(err.Error(), c.Token))
	}
	defer resp.Body.Close()

	reader := io.Reader(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		reader = io.LimitReader(resp.Body, 1<<20)
	}
	respBody, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &APIError{StatusCode: resp.StatusCode, Body: safeErrorBody(respBody, c.Token)}
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
	if token != "" {
		s = strings.ReplaceAll(s, token, "[redacted]")
	}
	if len(s) > 256 {
		s = s[:256]
	}
	return strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return ' '
		}
		return r
	}, s)
}

func safeErrorBody(body []byte, token string) string {
	var fields map[string]json.RawMessage
	if json.Unmarshal(body, &fields) != nil {
		return ""
	}
	result := make(map[string]any)
	for _, key := range []string{"message", "error", "code", "statusCode", "verificationId", "outcome", "scanDispatch", "bomDispatch", "nextStep"} {
		var value any
		if json.Unmarshal(fields[key], &value) == nil {
			switch v := value.(type) {
			case string:
				result[key] = redact(v, token)
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
		return ""
	}
	encoded, _ := json.Marshal(result)
	return string(encoded)
}
