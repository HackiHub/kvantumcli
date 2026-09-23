package api

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
)

// GetBOMsByVerification calls GET /bill-of-materials/by-verification.
func (c *Client) GetBOMsByVerification(ctx context.Context, verificationID, projectRepositoryID string) (json.RawMessage, error) {
	q := url.Values{}
	q.Set("verificationId", verificationID)
	q.Set("projectRepositoryId", projectRepositoryID)
	return c.Get(ctx, "/bill-of-materials/by-verification", q)
}

// GetBOM calls GET /bill-of-materials/{id}.
func (c *Client) GetBOM(ctx context.Context, bomID string) (json.RawMessage, error) {
	return c.Get(ctx, "/bill-of-materials/"+url.PathEscape(bomID), nil)
}

// ListBOMs calls GET /bill-of-materials.
func (c *Client) ListBOMs(ctx context.Context, verificationID, projectRepositoryID, bomType string, page, limit int) (json.RawMessage, error) {
	q := url.Values{}
	q.Set("verificationId", verificationID)
	q.Set("projectRepositoryId", projectRepositoryID)
	q.Set("type", bomType)
	if page > 0 {
		q.Set("page", strconv.Itoa(page))
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	return c.Get(ctx, "/bill-of-materials", q)
}
