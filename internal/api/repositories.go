package api

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
)

// AddRepositoryRequest is the body for POST /projects/repository.
type AddRepositoryRequest struct {
	Name                 string         `json:"name"`
	ProjectID            string         `json:"projectId"`
	TenantIntegrationsID string         `json:"tenantIntegrationsId"`
	BranchName           *string        `json:"branchName,omitempty"`
	ResourceID           *string        `json:"resourceId,omitempty"`
	ResourceURL          *string        `json:"resourceUrl,omitempty"`
	RepositoryURL        *string        `json:"repositoryUrl,omitempty"`
	Metadata             map[string]any `json:"metadata,omitempty"`
}

// AddRepository calls POST /projects/repository.
func (c *Client) AddRepository(ctx context.Context, req AddRepositoryRequest) (json.RawMessage, error) {
	return c.Post(ctx, "/projects/repository", req)
}

// ListRepositories calls GET /projects/repository/list.
func (c *Client) ListRepositories(ctx context.Context, projectID string, page, limit int) (json.RawMessage, error) {
	q := url.Values{}
	q.Set("projectId", projectID)
	if page > 0 {
		q.Set("page", strconv.Itoa(page))
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	return c.Get(ctx, "/projects/repository/list", q)
}

// GetRepository calls GET /projects/repository/{id}.
func (c *Client) GetRepository(ctx context.Context, repoID string) (json.RawMessage, error) {
	return c.Get(ctx, "/projects/repository/"+url.PathEscape(repoID), nil)
}
