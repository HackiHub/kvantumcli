package api

import (
	"context"
	"encoding/json"
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
	for {
		if !first {
			if err := sleep(ctx, opts.Interval); err != nil {
				return nil, fmt.Errorf("timed out waiting for verification %s after %s", verificationID, opts.Timeout)
			}
		}
		first = false

		raw, err := c.GetVerification(ctx, verificationID)
		if err != nil {
			if ctx.Err() != nil {
				return nil, fmt.Errorf("timed out waiting for verification %s after %s", verificationID, opts.Timeout)
			}
			return nil, err
		}
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
