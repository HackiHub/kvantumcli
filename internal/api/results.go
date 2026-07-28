package api

import (
	"context"
	"encoding/json"
	"net/url"
)

// ListResults calls GET /results.
func (c *Client) ListResults(ctx context.Context, verificationID string) (json.RawMessage, error) {
	q := url.Values{}
	if verificationID != "" {
		q.Set("verificationId", verificationID)
	}
	return c.Get(ctx, "/results", q)
}

// GetResult calls GET /results/{id}.
func (c *Client) GetResult(ctx context.Context, resultID string) (json.RawMessage, error) {
	return c.Get(ctx, "/results/"+url.PathEscape(resultID), nil)
}
