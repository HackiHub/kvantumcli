package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/hackihub/kvantumcli/internal/api"
	"github.com/hackihub/kvantumcli/internal/output"
)

func newResultsCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "results",
		Short: "Read rule-check results",
	}
	cmd.AddCommand(newResultsListCmd(opts))
	cmd.AddCommand(newResultsGetCmd(opts))
	cmd.AddCommand(newResultsSummaryCmd(opts))
	return cmd
}

type statusCounts struct {
	Fail    int `json:"fail"`
	Pass    int `json:"pass"`
	Skip    int `json:"skip"`
	Unknown int `json:"unknown"`
}

type resultsSummary struct {
	VerificationID    string                  `json:"verificationId"`
	Unit              string                  `json:"unit"`
	TotalRuleOutcomes int                     `json:"totalRuleOutcomes"`
	Totals            statusCounts            `json:"totals"`
	BySeverity        map[string]statusCounts `json:"bySeverity"`
}

// ErrFindingsRejected identifies a completed summary that failed its opted-in gate.
var ErrFindingsRejected = errors.New("summary contains failed or unknown rule outcomes")

func summarizeResults(verificationID string, outcomes []api.ResultOutcome) (resultsSummary, error) {
	counts := map[string]statusCounts{}
	for _, severity := range []string{"critical", "high", "medium", "low", "unclassified"} {
		counts[severity] = statusCounts{}
	}
	for i, outcome := range outcomes {
		status := ""
		if outcome.Status != nil {
			status = *outcome.Status
		}
		if outcome.Status != nil && status != "fail" && status != "pass" && status != "skip" {
			return resultsSummary{}, fmt.Errorf("result %d has invalid status %q", i+1, status)
		}
		severity := "unclassified"
		if outcome.Finding != nil && outcome.Finding.Severity != nil {
			candidate := strings.ToLower(strings.TrimSpace(*outcome.Finding.Severity))
			if _, ok := counts[candidate]; ok && candidate != "unclassified" {
				severity = candidate
			}
		}
		entry := counts[severity]
		switch status {
		case "fail":
			entry.Fail++
		case "pass":
			entry.Pass++
		case "skip":
			entry.Skip++
		default:
			entry.Unknown++
		}
		counts[severity] = entry
	}
	totals := statusCounts{}
	for _, entry := range counts {
		totals.Fail += entry.Fail
		totals.Pass += entry.Pass
		totals.Skip += entry.Skip
		totals.Unknown += entry.Unknown
	}
	if totals.Fail+totals.Pass+totals.Skip+totals.Unknown != len(outcomes) {
		return resultsSummary{}, fmt.Errorf("result counts do not reconcile with total rule outcomes")
	}
	return resultsSummary{
		VerificationID:    verificationID,
		Unit:              "ruleOutcomes",
		TotalRuleOutcomes: len(outcomes),
		Totals:            totals,
		BySeverity:        counts,
	}, nil
}

func newResultsSummaryCmd(opts *rootOptions) *cobra.Command {
	var verificationID string
	var timeout time.Duration
	var failOnFindings bool
	cmd := &cobra.Command{
		Use:   "summary",
		Short: "Summarize rule outcomes by severity and status",
		Long:  "Summarize rule outcomes. A null status counts as unknown, indicating an incomplete result. Use --fail-on-findings to exit nonzero for failures or unknown outcomes.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if timeout <= 0 {
				return fmt.Errorf("invalid --timeout %s: must be positive", timeout)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()
			client, _, err := newClient(opts, true)
			if err != nil {
				return err
			}
			outcomes, err := client.ListAllResults(ctx, verificationID)
			if err != nil {
				return summaryFetchError(err, ctx.Err(), verificationID, timeout)
			}
			summary, err := summarizeResults(verificationID, outcomes)
			if err != nil {
				return err
			}
			if err := ctx.Err(); err != nil {
				return summaryFetchError(err, ctx.Err(), verificationID, timeout)
			}
			if err := output.JSON(summary); err != nil {
				return err
			}
			if failOnFindings && (summary.Totals.Fail > 0 || summary.Totals.Unknown > 0) {
				return ErrFindingsRejected
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&verificationID, "verification", "", "Verification UUID")
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "Maximum time to fetch all results (default 5m; must be positive)")
	cmd.Flags().BoolVar(&failOnFindings, "fail-on-findings", false, "Exit nonzero after JSON output if failures or unknown outcomes exist")
	_ = cmd.MarkFlagRequired("verification")
	return cmd
}

func summaryFetchError(err, waitErr error, verificationID string, timeout time.Duration) error {
	if errors.Is(waitErr, context.DeadlineExceeded) {
		return fmt.Errorf("timed out fetching summary for verification %s after %s; increase --timeout if needed (last fetch error: %v): %w", verificationID, timeout, err, waitErr)
	}
	return err
}

func newResultsListCmd(opts *rootOptions) *cobra.Command {
	var verificationID string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List rule-check results",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJSON(func(ctx context.Context) (any, error) {
				client, _, err := newClient(opts, true)
				if err != nil {
					return nil, err
				}
				return client.ListResults(cmd.Context(), verificationID)
			})
		},
	}
	cmd.Flags().StringVar(&verificationID, "verification", "", "Filter by verification UUID")
	return cmd
}

func newResultsGetCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "get <resultId>",
		Short: "Get a single result with evidence",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJSON(func(ctx context.Context) (any, error) {
				client, _, err := newClient(opts, true)
				if err != nil {
					return nil, err
				}
				return client.GetResult(cmd.Context(), args[0])
			})
		},
	}
}
