package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hackihub/kvantumcli/internal/api"
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
	Fail int `json:"fail"`
	Pass int `json:"pass"`
	Skip int `json:"skip"`
}

type resultsSummary struct {
	VerificationID    string                  `json:"verificationId"`
	Unit              string                  `json:"unit"`
	TotalRuleOutcomes int                     `json:"totalRuleOutcomes"`
	Totals            statusCounts            `json:"totals"`
	BySeverity        map[string]statusCounts `json:"bySeverity"`
}

func summarizeResults(verificationID string, outcomes []api.ResultOutcome) (resultsSummary, error) {
	counts := map[string]statusCounts{}
	for _, severity := range []string{"critical", "high", "medium", "low", "unclassified"} {
		counts[severity] = statusCounts{}
	}
	for i, outcome := range outcomes {
		status := strings.TrimSpace(outcome.Status)
		if status != "fail" && status != "pass" && status != "skip" {
			return resultsSummary{}, fmt.Errorf("result %d has invalid status %q", i+1, outcome.Status)
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
		}
		counts[severity] = entry
	}
	totals := statusCounts{}
	for _, entry := range counts {
		totals.Fail += entry.Fail
		totals.Pass += entry.Pass
		totals.Skip += entry.Skip
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
	cmd := &cobra.Command{
		Use:   "summary",
		Short: "Summarize rule outcomes by severity and status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJSON(func(ctx context.Context) (any, error) {
				client, _, err := newClient(opts, true)
				if err != nil {
					return nil, err
				}
				outcomes, err := client.ListAllResults(ctx, verificationID)
				if err != nil {
					return nil, err
				}
				return summarizeResults(verificationID, outcomes)
			})
		},
	}
	cmd.Flags().StringVar(&verificationID, "verification", "", "Verification UUID")
	_ = cmd.MarkFlagRequired("verification")
	return cmd
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
				return client.ListResults(ctx, verificationID)
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
				return client.GetResult(ctx, args[0])
			})
		},
	}
}
