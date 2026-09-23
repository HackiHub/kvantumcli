package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

// GetIntegrationProvider returns the tenant-visible provider for an integration.
func (c *Client) GetIntegrationProvider(ctx context.Context, integrationID string) (string, error) {
	raw, err := c.Get(ctx, "/integrations/"+url.PathEscape(integrationID), nil)
	if err != nil {
		return "", err
	}
	var payload struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", fmt.Errorf("parse integration response: %w", err)
	}
	if len(bytes.TrimSpace(payload.Data)) == 0 {
		return "", fmt.Errorf("parse integration response: data is required")
	}
	var data struct {
		Provider string `json:"provider"`
	}
	if err := json.Unmarshal(payload.Data, &data); err != nil {
		return "", fmt.Errorf("parse integration response data: %w", err)
	}
	if data.Provider == "" {
		return "", fmt.Errorf("parse integration response: provider is required")
	}
	return data.Provider, nil
}

// ListIntegrationResourcesByID calls GET /integrations/byintegration/{integration}/resources.
func (c *Client) ListIntegrationResourcesByID(ctx context.Context, integrationID string) (json.RawMessage, error) {
	return c.Get(ctx, "/integrations/byintegration/"+url.PathEscape(integrationID)+"/resources", nil)
}

// ListIntegrationBranches calls GET /integrations/{provider}/resources/{resourceId}/branches.
func (c *Client) ListIntegrationBranches(ctx context.Context, provider, resourceID string) (json.RawMessage, error) {
	path := "/integrations/" + url.PathEscape(provider) + "/resources/" + url.PathEscape(resourceID) + "/branches"
	return c.Get(ctx, path, nil)
}

// ListIntegrationBranchesByID calls GET /integrations/byintegration/{integration}/resources/{resourceId}/branches.
func (c *Client) ListIntegrationBranchesByID(ctx context.Context, integrationID, resourceID string) (json.RawMessage, error) {
	path := "/integrations/byintegration/" + url.PathEscape(integrationID) + "/resources/" + url.PathEscape(resourceID) + "/branches"
	return c.Get(ctx, path, nil)
}
