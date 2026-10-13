package api

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
)

// CreateTemplateRegistrationRequest is the body for POST /capabilities/registrations.
type CreateTemplateRegistrationRequest struct {
	ProjectRepositoryID string `json:"projectRepositoryId"`
	RequestedRef        string `json:"requestedRef"`
	RefType             string `json:"refType"`
}

// CreateTemplateRegistration calls POST /capabilities/registrations.
func (c *Client) CreateTemplateRegistration(ctx context.Context, req CreateTemplateRegistrationRequest) (json.RawMessage, error) {
	return c.Post(ctx, "/capabilities/registrations", req)
}

// ListTemplateRegistrations calls GET /capabilities/registrations.
func (c *Client) ListTemplateRegistrations(ctx context.Context, projectRepositoryID string, page, limit int) (json.RawMessage, error) {
	q := url.Values{}
	if projectRepositoryID != "" {
		q.Set("projectRepositoryId", projectRepositoryID)
	}
	if page > 0 {
		q.Set("page", strconv.Itoa(page))
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	return c.Get(ctx, "/capabilities/registrations", q)
}

// RefreshTemplateRegistration calls POST /capabilities/registrations/{id}/refresh.
func (c *Client) RefreshTemplateRegistration(ctx context.Context, registrationID string) (json.RawMessage, error) {
	return c.Post(ctx, "/capabilities/registrations/"+url.PathEscape(registrationID)+"/refresh", nil)
}
