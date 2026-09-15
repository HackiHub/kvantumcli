package cli

import (
	"strings"
	"testing"

	"github.com/hackihub/kvantumcli/internal/api"
)

func severity(value string) *string { return &value }

func TestSummarizeResultsCountsStatusesBySeverity(t *testing.T) {
	var outcomes []api.ResultOutcome
	for _, severityName := range []string{"critical", "high", "medium", "low"} {
		for _, status := range []string{"fail", "pass", "skip"} {
			outcomes = append(outcomes, api.ResultOutcome{
				Status:  status,
				Finding: &api.ResultFinding{Severity: severity(severityName)},
			})
		}
	}
	unknown := "unexpected"
	outcomes = append(outcomes,
		api.ResultOutcome{Status: "fail", Finding: nil},
		api.ResultOutcome{Status: "pass", Finding: &api.ResultFinding{}},
		api.ResultOutcome{Status: "skip", Finding: &api.ResultFinding{Severity: &unknown}},
	)

	got, err := summarizeResults("v1", outcomes)
	if err != nil {
		t.Fatal(err)
	}
	if got.Unit != "ruleOutcomes" || got.TotalRuleOutcomes != 15 {
		t.Fatalf("summary metadata = %+v", got)
	}
	if got.Totals != (statusCounts{Fail: 5, Pass: 5, Skip: 5}) {
		t.Fatalf("totals = %+v", got.Totals)
	}
	for _, severityName := range []string{"critical", "high", "medium", "low", "unclassified"} {
		counts := got.BySeverity[severityName]
		if counts.Fail != 1 || counts.Pass != 1 || counts.Skip != 1 {
			t.Errorf("%s counts = %+v", severityName, counts)
		}
	}
}

func TestSummarizeResultsEmptyHasZeroBuckets(t *testing.T) {
	got, err := summarizeResults("v-empty", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalRuleOutcomes != 0 || got.Totals != (statusCounts{}) || len(got.BySeverity) != 5 {
		t.Fatalf("summary = %+v", got)
	}
	for name, counts := range got.BySeverity {
		if counts != (statusCounts{}) {
			t.Errorf("%s counts = %+v", name, counts)
		}
	}
}

func TestSummarizeResultsRejectsUnknownStatus(t *testing.T) {
	_, err := summarizeResults("v1", []api.ResultOutcome{{Status: "running"}})
	if err == nil || !strings.Contains(err.Error(), "invalid status") {
		t.Fatalf("error = %v", err)
	}
}
