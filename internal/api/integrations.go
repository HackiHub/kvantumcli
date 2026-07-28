package api

import (
	"context"
	"encoding/json"
	"net/url"
)

// ListIntegrations calls GET /integrations.
func (c *Client) ListIntegrations(ctx context.Context) (json.RawMessage, error) {
	return c.Get(ctx, "/integrations", nil)
}

// ListIntegrationResources calls GET /integrations/{provider}/resources.
func (c *Client) ListIntegrationResources(ctx context.Context, provider string) (json.RawMessage, error) {
	return c.Get(ctx, "/integrations/"+url.PathEscape(provider)+"/resources", nil)
}

// ListIntegrationBranches calls GET /integrations/{provider}/resources/{resourceId}/branches.
func (c *Client) ListIntegrationBranches(ctx context.Context, provider, resourceID string) (json.RawMessage, error) {
	path := "/integrations/" + url.PathEscape(provider) + "/resources/" + url.PathEscape(resourceID) + "/branches"
	return c.Get(ctx, path, nil)
}
