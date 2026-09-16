package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

const acceptanceToken = "acceptance-token"
const acceptanceTenant = "tenant-acceptance"

func TestProcessRunWaitAndSummaryUsesReturnedVerificationID(t *testing.T) {
	fixtures := acceptanceFixtures(t)
	var mu sync.Mutex
	var requests []string
	pollCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+acceptanceToken || r.Header.Get("x-tenant-id") != acceptanceTenant {
			http.Error(w, "missing expected authentication", http.StatusUnauthorized)
			return
		}
		mu.Lock()
		requests = append(requests, r.Method+" "+r.URL.RequestURI())
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/verifications/run/repository/repo-acceptance/tenant-acceptance":
			if got := r.Header.Get("Content-Type"); got != "application/json" {
				t.Errorf("run Content-Type = %q", got)
			}
			var body struct {
				Types []string `json:"types"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode run body: %v", err)
			} else if strings.Join(body.Types, ",") != "sbom,cbom" {
				t.Errorf("run types = %v", body.Types)
			}
			fmt.Fprint(w, fixtures["repository-run-response.json"])
		case r.Method == http.MethodGet && r.URL.Path == "/verifications/verification-acceptance-1":
			pollCount++
			if pollCount == 1 {
				fmt.Fprint(w, fixtures["verification-detail-running.json"])
			} else {
				fmt.Fprint(w, fixtures["verification-detail-finished.json"])
			}
		case r.Method == http.MethodGet && r.URL.Path == "/results":
			q := r.URL.Query()
			if q.Get("verificationId") != "verification-acceptance-1" || q.Get("page") != "1" || q.Get("limit") != "100" {
				http.Error(w, "unexpected results query", http.StatusBadRequest)
				return
			}
			fmt.Fprint(w, fixtures["results-page-null-status.json"])
		default:
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))
	defer server.Close()

	run := runCLI(t, server.URL, "verify", "run", "--repo", "repo-acceptance", "--types", "sbom,cbom")
	if run.exitCode != 0 {
		t.Fatalf("verify run exit = %d, stderr = %s", run.exitCode, run.stderr)
	}
	var runResponse struct {
		Data struct {
			VerificationID string `json:"verificationId"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(run.stdout), &runResponse); err != nil {
		t.Fatalf("verify run stdout is not JSON: %v\n%s", err, run.stdout)
	}
	if runResponse.Data.VerificationID != "verification-acceptance-1" {
		t.Fatalf("verify run verification ID = %q", runResponse.Data.VerificationID)
	}

	wait := runCLI(t, server.URL, "verify", "wait", runResponse.Data.VerificationID, "--interval", "1ms", "--timeout", "1s")
	if wait.exitCode != 0 {
		t.Fatalf("verify wait exit = %d, stderr = %s", wait.exitCode, wait.stderr)
	}
	if !strings.Contains(wait.stdout, `"status": "finished"`) || pollCount != 2 {
		t.Fatalf("verify wait stdout = %s; polls = %d, want finished after two polls", wait.stdout, pollCount)
	}

	summary := runCLI(t, server.URL, "results", "summary", "--verification", runResponse.Data.VerificationID)
	if summary.exitCode != 0 {
		t.Fatalf("results summary exit = %d, stderr = %s", summary.exitCode, summary.stderr)
	}
	var output struct {
		VerificationID string `json:"verificationId"`
		Totals         struct {
			Fail, Pass, Skip, Unknown int
		} `json:"totals"`
	}
	if err := json.Unmarshal([]byte(summary.stdout), &output); err != nil {
		t.Fatalf("summary stdout is not JSON: %v\n%s", err, summary.stdout)
	}
	if output.VerificationID != runResponse.Data.VerificationID || output.Totals.Fail != 1 || output.Totals.Pass != 1 || output.Totals.Skip != 0 || output.Totals.Unknown != 1 {
		t.Fatalf("summary = %+v", output)
	}
	mu.Lock()
	defer mu.Unlock()
	if got, want := strings.Join(requests, "; "), "POST /verifications/run/repository/repo-acceptance/tenant-acceptance; GET /verifications/verification-acceptance-1; GET /verifications/verification-acceptance-1; GET /results?limit=100&page=1&verificationId=verification-acceptance-1"; got != want {
		t.Errorf("requests = %s\nwant %s", got, want)
	}
}

func TestProcessWaitTerminalErrorPrintsFinalJSONAndFails(t *testing.T) {
	fixtures := acceptanceFixtures(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+acceptanceToken || r.Header.Get("x-tenant-id") != acceptanceTenant {
			http.Error(w, "missing expected authentication", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodGet || r.URL.Path != "/verifications/verification-acceptance-error" {
			http.Error(w, "unexpected request", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, fixtures["verification-detail-error.json"])
	}))
	defer server.Close()

	result := runCLI(t, server.URL, "verify", "wait", "verification-acceptance-error", "--interval", "1ms", "--timeout", "1s")
	if result.exitCode == 0 {
		t.Fatalf("verify wait unexpectedly succeeded; stdout=%s stderr=%s", result.stdout, result.stderr)
	}
	if !strings.Contains(result.stdout, `"status": "error"`) {
		t.Errorf("stdout = %q, want final error JSON", result.stdout)
	}
	if !strings.Contains(result.stderr, "verification verification-acceptance-error ended with status error") {
		t.Errorf("stderr = %q, want terminal error", result.stderr)
	}
}

type cliResult struct {
	stdout, stderr string
	exitCode       int
}

func runCLI(t *testing.T, apiURL string, args ...string) cliResult {
	t.Helper()
	binary := builtCLI(t)
	commandArgs := append([]string{"--api-url", apiURL, "--token", acceptanceToken, "--tenant-id", acceptanceTenant}, args...)
	cmd := exec.Command(binary, commandArgs...)
	cmd.Env = append(os.Environ(), "KVANTUMCI_ALLOW_HTTP=true", "KVANTUMCI_CONFIG="+filepath.Join(t.TempDir(), "config.json"))
	var result cliResult
	var err error
	result.stdout, err = captureCommandOutput(cmd, &result.stderr)
	if err == nil {
		return result
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		result.exitCode = exitErr.ExitCode()
		return result
	}
	t.Fatalf("run CLI: %v", err)
	return cliResult{}
}

func captureCommandOutput(cmd *exec.Cmd, stderr *string) (string, error) {
	var stderrBuffer strings.Builder
	cmd.Stderr = &stderrBuffer
	stdout, err := cmd.Output()
	*stderr = stderrBuffer.String()
	return string(stdout), err
}

func builtCLI(t *testing.T) string {
	t.Helper()
	name := "kvantumci-acceptance"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	cliBinary := filepath.Join(t.TempDir(), name)
	build := exec.Command("go", "build", "-o", cliBinary, "./cmd/kvantumci")
	build.Dir = moduleRoot(t)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}
	return cliBinary
}

func acceptanceFixtures(t *testing.T) map[string]string {
	t.Helper()
	dir := filepath.Join(moduleRoot(t), "internal", "api", "testdata")
	fixtures := make(map[string]string)
	for _, name := range []string{"repository-run-response.json", "verification-detail-running.json", "verification-detail-finished.json", "verification-detail-error.json", "results-page-null-status.json"} {
		contents, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read fixture %s: %v", name, err)
		}
		fixtures[name] = string(contents)
	}
	return fixtures
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate acceptance test source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
