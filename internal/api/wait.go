package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// WaitOptions controls verification polling.
type WaitOptions struct {
	Interval time.Duration
	Timeout  time.Duration
	// Sleep is used between polls; defaults to time.Sleep.
	Sleep func(context.Context, time.Duration) error
}

// WaitResult is returned when polling completes.
type WaitResult struct {
	Body   json.RawMessage
	Status string
}

// WaitForVerification polls GET /verifications/{id} until finished, error, or timeout.
func (c *Client) WaitForVerification(ctx context.Context, verificationID string, opts WaitOptions) (*WaitResult, error) {
	if opts.Interval <= 0 {
		return nil, fmt.Errorf("interval must be positive")
	}
	if opts.Timeout <= 0 {
		return nil, fmt.Errorf("timeout must be positive")
	}
	sleep := opts.Sleep
	if sleep == nil {
		sleep = func(ctx context.Context, d time.Duration) error {
			t := time.NewTimer(d)
			defer t.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-t.C:
				return nil
			}
		}
	}

	deadline := time.Now().Add(opts.Timeout)
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	first := true
	consecutiveFailures := 0
	var lastFailure error
	nextDelay := opts.Interval
	for {
		if err := ctx.Err(); err != nil {
			return nil, waitContextError(err, verificationID, opts.Timeout, lastFailure)
		}
		if !first {
			if err := sleep(ctx, nextDelay); err != nil {
				if ctx.Err() != nil {
					return nil, waitContextError(ctx.Err(), verificationID, opts.Timeout, lastFailure)
				}
				return nil, err
			}
		}
		first = false

		raw, err := c.GetVerification(ctx, verificationID)
		if err != nil {
			if ctx.Err() != nil {
				return nil, waitContextError(ctx.Err(), verificationID, opts.Timeout, lastFailure)
			}
			if !retryablePollError(err) {
				return nil, err
			}
			lastFailure = err
			consecutiveFailures++
			nextDelay = retryDelay(opts.Interval, consecutiveFailures)
			var apiErr *APIError
			if errors.As(err, &apiErr) && apiErr.RetryAfter > nextDelay {
				nextDelay = apiErr.RetryAfter
			}
			continue
		}
		consecutiveFailures = 0
		lastFailure = nil
		nextDelay = opts.Interval
		status, err := VerificationStatus(raw)
		if err != nil {
			return nil, fmt.Errorf("parse verification status: %w", err)
		}
		switch status {
		case "finished", "error":
			return &WaitResult{Body: raw, Status: status}, nil
		}
	}
}

func retryablePollError(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case 408, 429, 500, 502, 503, 504:
			return true
		}
		return false
	}
	return IsTransientRequestError(err)
}

func retryDelay(interval time.Duration, failures int) time.Duration {
	const maximum = 30 * time.Second
	if interval >= maximum {
		return interval
	}
	delay := interval
	for i := 1; i < failures && delay < maximum; i++ {
		if delay >= maximum/2 {
			return maximum
		}
		delay *= 2
	}
	return delay
}

func waitContextError(err error, verificationID string, timeout time.Duration, lastFailure error) error {
	if err == context.DeadlineExceeded {
		if lastFailure != nil {
			return fmt.Errorf("timed out waiting for verification %s after %s (last poll error: %v): %w", verificationID, timeout, lastFailure, err)
		}
		return fmt.Errorf("timed out waiting for verification %s after %s: %w", verificationID, timeout, err)
	}
	return fmt.Errorf("waiting for verification %s: %w", verificationID, err)
}
