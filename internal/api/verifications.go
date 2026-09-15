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
	var payload struct {
		Data  []json.RawMessage `json:"data"`
		Total int               `json:"total"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("parse verifications response: %w", err)
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
	if len(payload.Data) == 0 {
		return nil, ErrNoFinishedVerification
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(payload.Data[0], &object); err != nil || object == nil {
		return nil, fmt.Errorf("parse verifications response: first data item must be an object")
	}
	return payload.Data[0], nil
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
