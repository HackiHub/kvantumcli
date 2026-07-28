package api

import (
	"context"
	"encoding/json"
	"net/url"
)

// RunVerificationRequest is the body for verification run endpoints.
type RunVerificationRequest struct {
	Types []string `json:"types,omitempty"`
}

// RunVerificationForRepo calls POST /verifications/run/repository/{projectRepositoryId}/{tenantId}.
func (c *Client) RunVerificationForRepo(ctx context.Context, projectRepositoryID, tenantID string, req RunVerificationRequest) (json.RawMessage, error) {
	path := "/verifications/run/repository/" + url.PathEscape(projectRepositoryID) + "/" + url.PathEscape(tenantID)
	return c.Post(ctx, path, req)
}

// RunVerificationForProject calls POST /verifications/run/{projectId}/{tenantId}.
func (c *Client) RunVerificationForProject(ctx context.Context, projectID, tenantID string, req RunVerificationRequest) (json.RawMessage, error) {
	path := "/verifications/run/" + url.PathEscape(projectID) + "/" + url.PathEscape(tenantID)
	return c.Post(ctx, path, req)
}

// ListVerifications calls GET /verifications.
func (c *Client) ListVerifications(ctx context.Context) (json.RawMessage, error) {
	return c.Get(ctx, "/verifications", nil)
}

// GetVerification calls GET /verifications/{id}.
func (c *Client) GetVerification(ctx context.Context, verificationID string) (json.RawMessage, error) {
	return c.Get(ctx, "/verifications/"+url.PathEscape(verificationID), nil)
}

// VerificationStatus extracts a top-level "status" field from a verification JSON payload.
func VerificationStatus(raw json.RawMessage) (string, error) {
	var payload struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", err
	}
	return payload.Status, nil
}
