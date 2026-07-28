package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &APIError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(respBody))}
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
	rel, err := url.Parse(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	u := base.ResolveReference(rel)
	if query != nil {
		u.RawQuery = query.Encode()
	}
	return u.String(), nil
}
