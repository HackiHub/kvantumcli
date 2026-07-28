package api

import (
	"context"
	"encoding/json"
)

// Health calls GET /health (public).
func (c *Client) Health(ctx context.Context) (json.RawMessage, error) {
	return c.GetPublic(ctx, "/health", nil)
}

// WhoAmI calls GET /users/me.
func (c *Client) WhoAmI(ctx context.Context) (json.RawMessage, error) {
	return c.Get(ctx, "/users/me", nil)
}
