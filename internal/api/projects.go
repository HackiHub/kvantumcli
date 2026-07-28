package api

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
)

// CreateProjectRequest is the body for POST /projects.
type CreateProjectRequest struct {
	Name     string   `json:"name"`
	ParentID *string  `json:"parentId,omitempty"`
	Icon     *string  `json:"icon,omitempty"`
	Tags     []string `json:"tags,omitempty"`
}

// CreateProject calls POST /projects.
func (c *Client) CreateProject(ctx context.Context, req CreateProjectRequest) (json.RawMessage, error) {
	return c.Post(ctx, "/projects", req)
}

// ListProjects calls GET /projects.
func (c *Client) ListProjects(ctx context.Context, page, limit int, search string, tags []string) (json.RawMessage, error) {
	q := url.Values{}
	if page > 0 {
		q.Set("page", strconv.Itoa(page))
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if search != "" {
		q.Set("search", search)
	}
	for _, t := range tags {
		if t != "" {
			q.Add("tags", t)
		}
	}
	return c.Get(ctx, "/projects", q)
}

// GetProject calls GET /projects/{id}.
func (c *Client) GetProject(ctx context.Context, projectID string) (json.RawMessage, error) {
	return c.Get(ctx, "/projects/"+url.PathEscape(projectID), nil)
}
