package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
)

// ErrNoFinishedVerification indicates that a repository has no finished run.
var ErrNoFinishedVerification = errors.New("no finished verification found for repository")

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

// LatestFinishedVerification returns the newest finished verification for a repository.
func (c *Client) LatestFinishedVerification(ctx context.Context, projectRepositoryID string) (json.RawMessage, error) {
	q := url.Values{}
	q.Set("projectRepositoryId", projectRepositoryID)
	q.Set("status", "finished")
	q.Set("page", "1")
	q.Set("limit", "1")
	raw, err := c.Get(ctx, "/verifications", q)
	if err != nil {
		return nil, err
	}
	var shape struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &shape); err != nil {
		return nil, fmt.Errorf("parse verifications response: %w", err)
	}
	data := bytes.TrimSpace(shape.Data)
	if len(data) == 0 || string(data) == "null" || data[0] != '[' {
		return nil, fmt.Errorf("parse verifications response: data must be an array")
	}
	var items []json.RawMessage
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("parse verifications response: %w", err)
	}
	if len(items) == 0 {
		return nil, ErrNoFinishedVerification
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(items[0], &object); err != nil || object == nil {
		return nil, fmt.Errorf("parse verifications response: first data item must be an object")
	}
	return items[0], nil
}

// GetVerification calls GET /verifications/{id}.
func (c *Client) GetVerification(ctx context.Context, verificationID string) (json.RawMessage, error) {
	return c.Get(ctx, "/verifications/"+url.PathEscape(verificationID), nil)
}

// VerificationStatus extracts data.status from a verification detail response.
func VerificationStatus(raw json.RawMessage) (string, error) {
	var payload struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", fmt.Errorf("invalid verification response: %w", err)
	}
	data := bytes.TrimSpace(payload.Data)
	if len(data) == 0 || data[0] != '{' {
		return "", fmt.Errorf("verification response data must be an object")
	}
	var detail struct {
		Status json.RawMessage `json:"status"`
	}
	if err := json.Unmarshal(data, &detail); err != nil {
		return "", fmt.Errorf("invalid verification data: %w", err)
	}
	statusRaw := bytes.TrimSpace(detail.Status)
	if len(statusRaw) == 0 || statusRaw[0] != '"' {
		return "", fmt.Errorf("verification response data.status must be a non-empty string")
	}
	var status string
	if err := json.Unmarshal(statusRaw, &status); err != nil || status == "" {
		return "", fmt.Errorf("verification response data.status must be a non-empty string")
	}
	switch status {
	case "pending", "running", "finished", "error":
	default:
		return "", fmt.Errorf("verification response data.status has unknown value %q", status)
	}
	return status, nil
}
