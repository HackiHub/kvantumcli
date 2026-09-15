package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

// ResultsListOptions controls filters and pagination for GET /results.
type ResultsListOptions struct {
	VerificationID string
	ResultStatus   string
	Page           int
	Limit          int
}

// ResultOutcome is the subset of a result needed for aggregate summaries.
type ResultOutcome struct {
	Status  string         `json:"status"`
	Finding *ResultFinding `json:"finding"`
}

// ResultFinding is the finding metadata attached to a rule-check outcome.
type ResultFinding struct {
	Severity *string `json:"severity"`
}

// ResultsPage is a page returned by GET /results.
type ResultsPage struct {
	Data     []ResultOutcome `json:"data"`
	Total    int             `json:"total"`
	Page     int             `json:"page"`
	Limit    int             `json:"limit"`
	LastPage int             `json:"lastPage"`
	HasNext  bool            `json:"hasNext"`
}

// ListResults calls GET /results.
func (c *Client) ListResults(ctx context.Context, verificationID string) (json.RawMessage, error) {
	return c.ListResultsFiltered(ctx, ResultsListOptions{VerificationID: verificationID})
}

// ListResultsFiltered calls GET /results with supported result filters and pagination.
func (c *Client) ListResultsFiltered(ctx context.Context, opts ResultsListOptions) (json.RawMessage, error) {
	q := url.Values{}
	if opts.VerificationID != "" {
		q.Set("verificationId", opts.VerificationID)
	}
	if opts.ResultStatus != "" {
		q.Set("resultStatus", opts.ResultStatus)
	}
	if opts.Page > 0 {
		q.Set("page", strconv.Itoa(opts.Page))
	}
	if opts.Limit > 0 {
		q.Set("limit", strconv.Itoa(opts.Limit))
	}
	return c.Get(ctx, "/results", q)
}

// ListResultsPage returns and validates one paginated GET /results response.
func (c *Client) ListResultsPage(ctx context.Context, opts ResultsListOptions) (ResultsPage, error) {
	raw, err := c.ListResultsFiltered(ctx, opts)
	if err != nil {
		return ResultsPage{}, err
	}
	var payload struct {
		Data     json.RawMessage `json:"data"`
		Total    *int            `json:"total"`
		Page     *int            `json:"page"`
		Limit    *int            `json:"limit"`
		LastPage *int            `json:"lastPage"`
		HasNext  *bool           `json:"hasNext"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ResultsPage{}, fmt.Errorf("parse results response: %w", err)
	}
	data := bytes.TrimSpace(payload.Data)
	if len(data) == 0 || string(data) == "null" || data[0] != '[' {
		return ResultsPage{}, fmt.Errorf("parse results response: data must be an array")
	}
	if payload.Total == nil || payload.Page == nil || payload.Limit == nil || payload.LastPage == nil || payload.HasNext == nil {
		return ResultsPage{}, fmt.Errorf("parse results response: total, page, limit, lastPage, and hasNext are required")
	}
	var outcomes []ResultOutcome
	if err := json.Unmarshal(data, &outcomes); err != nil {
		return ResultsPage{}, fmt.Errorf("parse results response data: %w", err)
	}
	page := ResultsPage{
		Data:     outcomes,
		Total:    *payload.Total,
		Page:     *payload.Page,
		Limit:    *payload.Limit,
		LastPage: *payload.LastPage,
		HasNext:  *payload.HasNext,
	}
	if page.Total < 0 || page.Page < 1 || page.Limit < 1 || page.Limit > 100 || page.LastPage < 0 {
		return ResultsPage{}, fmt.Errorf("invalid results pagination metadata")
	}
	if page.Total > 0 && page.LastPage < page.Page {
		return ResultsPage{}, fmt.Errorf("invalid results pagination: page %d exceeds lastPage %d", page.Page, page.LastPage)
	}
	if len(page.Data) > page.Limit {
		return ResultsPage{}, fmt.Errorf("invalid results pagination: page contains %d rows with limit %d", len(page.Data), page.Limit)
	}
	return page, nil
}

// ListAllResults fetches every result page for a verification.
func (c *Client) ListAllResults(ctx context.Context, verificationID string) ([]ResultOutcome, error) {
	const (
		pageSize = 100
		maxPages = 10000
	)
	var outcomes []ResultOutcome
	expectedTotal := -1
	expectedLastPage := -1
	for requestedPage := 1; requestedPage <= maxPages; requestedPage++ {
		page, err := c.ListResultsPage(ctx, ResultsListOptions{
			VerificationID: verificationID,
			Page:           requestedPage,
			Limit:          pageSize,
		})
		if err != nil {
			return nil, err
		}
		if page.Page != requestedPage {
			return nil, fmt.Errorf("invalid results pagination: requested page %d, response reported page %d", requestedPage, page.Page)
		}
		if page.Limit != pageSize {
			return nil, fmt.Errorf("invalid results pagination: requested limit %d, response reported limit %d", pageSize, page.Limit)
		}
		if expectedTotal < 0 {
			expectedTotal = page.Total
			expectedLastPage = page.LastPage
		} else if page.Total != expectedTotal || page.LastPage != expectedLastPage {
			return nil, fmt.Errorf("invalid results pagination: total or lastPage changed between pages")
		}
		outcomes = append(outcomes, page.Data...)
		if len(outcomes) > page.Total {
			return nil, fmt.Errorf("invalid results pagination: received %d rows but total is %d", len(outcomes), page.Total)
		}
		if !page.HasNext {
			if page.LastPage > requestedPage {
				return nil, fmt.Errorf("invalid results pagination: page %d hasNext is false before lastPage %d", requestedPage, page.LastPage)
			}
			if len(outcomes) != page.Total {
				return nil, fmt.Errorf("invalid results pagination: received %d rows but total is %d", len(outcomes), page.Total)
			}
			return outcomes, nil
		}
		if len(page.Data) == 0 {
			return nil, fmt.Errorf("invalid results pagination: page %d is empty but hasNext is true", requestedPage)
		}
		if requestedPage >= page.LastPage {
			return nil, fmt.Errorf("invalid results pagination: page %d hasNext is true at lastPage %d", requestedPage, page.LastPage)
		}
		if len(outcomes) >= page.Total {
			return nil, fmt.Errorf("invalid results pagination: hasNext is true after receiving total %d rows", page.Total)
		}
	}
	return nil, fmt.Errorf("invalid results pagination: exceeded %d pages", maxPages)
}

// GetResult calls GET /results/{id}.
func (c *Client) GetResult(ctx context.Context, resultID string) (json.RawMessage, error) {
	return c.Get(ctx, "/results/"+url.PathEscape(resultID), nil)
}
