package cli

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hackihub/kvantumcli/internal/api"
)

func severity(value string) *string  { return &value }
func statusPtr(value string) *string { return &value }

func TestSummarizeResultsCountsStatusesBySeverity(t *testing.T) {
	var outcomes []api.ResultOutcome
	for _, severityName := range []string{"critical", "high", "medium", "low"} {
		for _, status := range []string{"fail", "pass", "skip"} {
			outcomes = append(outcomes, api.ResultOutcome{
				Status:  statusPtr(status),
				Finding: &api.ResultFinding{Severity: severity(severityName)},
			})
		}
	}
	unknown := "unexpected"
	outcomes = append(outcomes,
		api.ResultOutcome{Status: statusPtr("fail"), Finding: nil},
		api.ResultOutcome{Status: statusPtr("pass"), Finding: &api.ResultFinding{}},
		api.ResultOutcome{Status: statusPtr("skip"), Finding: &api.ResultFinding{Severity: &unknown}},
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

func TestResultsSummaryExitPolicyAndTimeout(t *testing.T) {
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/results" || r.URL.Query().Get("verificationId") != "v1" {
			t.Errorf("request = %s", r.URL.String())
		}
		_, _ = w.Write([]byte(`{"data":[{"status":"fail","finding":{"severity":"high"}},{"status":null,"finding":null}],"total":2,"page":1,"limit":100,"lastPage":1,"hasNext":false}`))
	}))
	defer srv.Close()
	opts := &rootOptions{apiURL: srv.URL, token: "token", tenantID: "tenant"}
	for _, tc := range []struct {
		name      string
		args      []string
		wantError bool
	}{
		{"default", []string{"--verification", "v1"}, false},
		{"gated", []string{"--verification", "v1", "--fail-on-findings"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newResultsSummaryCmd(opts)
			cmd.SetArgs(tc.args)
			previous := os.Stdout
			file, err := os.CreateTemp(t.TempDir(), "summary-*.json")
			if err != nil {
				t.Fatal(err)
			}
			os.Stdout = file
			defer func() { os.Stdout = previous; _ = file.Close() }()
			err = cmd.Execute()
			if (err != nil) != tc.wantError {
				t.Fatalf("error = %v", err)
			}
			if tc.wantError && !errors.Is(err, ErrFindingsRejected) {
				t.Fatalf("gate error = %v", err)
			}
			if _, err := file.Seek(0, 0); err != nil {
				t.Fatal(err)
			}
			var got resultsSummary
			if err := json.NewDecoder(file).Decode(&got); err != nil {
				t.Fatal(err)
			}
			if got.Totals.Fail != 1 || got.Totals.Unknown != 1 {
				t.Fatalf("totals = %+v", got.Totals)
			}
		})
	}
	cmd := newResultsSummaryCmd(opts)
	cmd.SetArgs([]string{"--verification", "v1", "--timeout", "0s"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "must be positive") {
		t.Fatalf("timeout error = %v", err)
	}
}

func TestResultsSummaryRespectsCanceledContext(t *testing.T) {
	opts := &rootOptions{apiURL: "https://example.test", token: "token", tenantID: "tenant"}
	cmd := newResultsSummaryCmd(opts)
	cmd.SetArgs([]string{"--verification", "v1"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cmd.SetContext(ctx)
	start := time.Now()
	if err := cmd.Execute(); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("cancellation was slow")
	}
}

func TestResultsSummaryTimeoutDoesNotPrintPartialOutput(t *testing.T) {
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()
	cmd := newResultsSummaryCmd(&rootOptions{apiURL: srv.URL, token: "token", tenantID: "tenant"})
	cmd.SetArgs([]string{"--verification", "v1", "--timeout", "20ms"})
	previous := os.Stdout
	file, err := os.CreateTemp(t.TempDir(), "summary-timeout-*.json")
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = file
	defer func() { os.Stdout = previous; _ = file.Close() }()
	if err := cmd.Execute(); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v", err)
	} else if !strings.Contains(err.Error(), "verification v1") || !strings.Contains(err.Error(), "20ms") || !strings.Contains(err.Error(), "--timeout") {
		t.Fatalf("missing timeout guidance: %v", err)
	}
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 0 {
		t.Fatalf("printed %d bytes on timeout", info.Size())
	}
}

func TestSummaryFetchErrorOnlyLabelsOverallDeadline(t *testing.T) {
	requestErr := summaryFetchError(context.DeadlineExceeded, nil, "v1", 5*time.Minute)
	if strings.Contains(requestErr.Error(), "after 5m") {
		t.Fatalf("request timeout mislabelled as overall timeout: %v", requestErr)
	}
	overallErr := summaryFetchError(context.DeadlineExceeded, context.DeadlineExceeded, "v1", 5*time.Minute)
	if !errors.Is(overallErr, context.DeadlineExceeded) || !strings.Contains(overallErr.Error(), "after 5m") {
		t.Fatalf("overall timeout = %v", overallErr)
	}
	apiErr := &api.APIError{StatusCode: http.StatusServiceUnavailable, Body: "upstream unavailable"}
	overallErr = summaryFetchError(apiErr, context.DeadlineExceeded, "v1", 5*time.Minute)
	if !errors.Is(overallErr, context.DeadlineExceeded) || !strings.Contains(overallErr.Error(), "HTTP 503") {
		t.Fatalf("overall timeout with fetch error = %v", overallErr)
	}
}

func TestFindingsListTenantScopeAndFilters(t *testing.T) {
	t.Setenv("KVANTUMCI_ALLOW_HTTP", "true")
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.RawQuery)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()
	opts := &rootOptions{apiURL: srv.URL, token: "token", tenantID: "tenant"}
	for _, args := range [][]string{{}, {"--verification", "v1", "--status", "fail"}} {
		cmd := newFindingsListCmd(opts)
		cmd.SetArgs(args)
		previous := os.Stdout
		file, err := os.CreateTemp(t.TempDir(), "findings-*.json")
		if err != nil {
			t.Fatal(err)
		}
		os.Stdout = file
		err = cmd.Execute()
		os.Stdout = previous
		_ = file.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(queries) != 2 {
		t.Fatalf("queries = %v", queries)
	}
	if strings.Contains(queries[0], "verificationId") || strings.Contains(queries[0], "resultStatus") {
		t.Fatalf("tenant-wide query = %q", queries[0])
	}
	if !strings.Contains(queries[1], "verificationId=v1") || !strings.Contains(queries[1], "resultStatus=fail") {
		t.Fatalf("filtered query = %q", queries[1])
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
	_, err := summarizeResults("v1", []api.ResultOutcome{{Status: statusPtr("running")}})
	if err == nil || !strings.Contains(err.Error(), "invalid status") {
		t.Fatalf("error = %v", err)
	}
}

func TestSummarizeResultsCountsNullStatusAndReconciles(t *testing.T) {
	outcomes := []api.ResultOutcome{
		{Status: statusPtr("fail"), Finding: &api.ResultFinding{Severity: severity("high")}},
		{Status: statusPtr("pass"), Finding: &api.ResultFinding{Severity: severity("high")}},
		{Status: statusPtr("skip"), Finding: &api.ResultFinding{Severity: severity("low")}},
		{Status: nil, Finding: &api.ResultFinding{Severity: severity("high")}},
		{Status: nil},
	}
	got, err := summarizeResults("v1", outcomes)
	if err != nil {
		t.Fatal(err)
	}
	if got.Totals != (statusCounts{Fail: 1, Pass: 1, Skip: 1, Unknown: 2}) || got.TotalRuleOutcomes != 5 {
		t.Fatalf("totals = %+v; total outcomes = %d", got.Totals, got.TotalRuleOutcomes)
	}
	if got.BySeverity["high"] != (statusCounts{Fail: 1, Pass: 1, Unknown: 1}) || got.BySeverity["unclassified"].Unknown != 1 {
		t.Fatalf("by severity = %+v", got.BySeverity)
	}
}
