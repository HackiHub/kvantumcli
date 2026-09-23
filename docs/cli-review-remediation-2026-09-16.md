# CLI review remediation ledger

This ledger records the remediation work for the 2026-09-16 CLI review. It
describes the current working tree. It does not establish that a server contract
is deployed, that a release has been published, or that native Windows and Linux
interactive validation has passed.

## Contract source

Sanitized API fixtures are based on merged API commit
`f4c5d782d2e2155b5fd7efd4f64fbc86b24d958f` (API PR #419). Repository-run and
verification-detail endpoints are wrapped responses; list endpoints, including
results, are flat pagination objects. See
[`internal/api/testdata/README.md`](../internal/api/testdata/README.md). This is
source-level fixture provenance, not deployed API validation.

## Findings

The IDs below follow the additional PR #5 review and its follow-up plan.

| Review items | Status | Changed files | Local checks |
| --- | --- | --- | --- |
| B1, M1, M2, m2, m3, m4, m5, m10, m12 | Implemented in the working tree. Installers validate HTTPS URLs and redirects, snapshot and pin the certificate, require RSA-3072, retain default TLS negotiation, stage verified binaries, and add trust and redirect cases. Release builds add deterministic path/build metadata controls. | `install.sh`, `install.ps1`, `Makefile`, `.github/workflows/release.yml`, installer tests | macOS tests and the Windows compile check passed. Native Windows installer validation remains pending. Production pin/bootstrap provisioning is still required. |
| B2 | Implemented in the working tree. Repository runs preserve the typed response and verification detail reads `data.status`. Empty `201` responses print `null` and provide no ID. Documentation now requires the deployed #419 typed contract before chaining run to wait. | `internal/api/verifications.go`, `internal/api/wait.go`, `internal/cli/verify.go`, `internal/api/testdata/*`, `cmd/kvantumci/acceptance_test.go`, user docs | On macOS with Go 1.26.5, the full Go test suite passed, including typed run → wait and empty-201/no-follow-up acceptance coverage. |
| M3, M4, m6 | Implemented in the working tree. Successful API bodies are capped at 64 MiB; error details are bounded and sanitized; credentials are redacted before truncation; transport errors have safe retry classification. | `internal/api/client.go`, `internal/api/client_limits_test.go`, `internal/api/client_security_test.go` | On macOS with Go 1.26.5, the full Go test suite passed, covering response limits, sanitization, and retry classification. |
| M5 | Implemented in the working tree. `verify wait` retries only safe transient GET failures with bounded cancellation-aware backoff and optional bounded `Retry-After`; it preserves the last failure on an overall timeout. | `internal/api/wait.go`, `internal/api/client_test.go` | On macOS with Go 1.26.5, the full Go test suite passed, covering transient recovery, fatal HTTP failures, cancellation/backoff, and deadline context. |
| M6, M7, M8, m1, m8 | Implemented in the working tree. Prompt handling has explicit secret fields, prompts missing fields on interactive partial configuration, accepts final-EOF input, refuses zero-progress reads, and restores terminal state on cancellation. A Windows NOWAIT console implementation and native test path have been added. | `internal/cli/login.go`, `internal/cli/root.go`, prompt helpers and tests | A real macOS process PTY regression passed independently: hidden token entry, SIGINT cancellation, echo restoration, and unchanged config. Windows and Linux compile checks passed; native Windows console and Linux PTY execution remain pending. |
| m7 | Implemented in the working tree. Summary timeout errors name the verification and effective timeout, emit no partial JSON, and the opted-in findings gate exits 3 after complete JSON. | `internal/cli/results.go`, `internal/cli/results_test.go`, `cmd/kvantumci/acceptance_test.go` | On macOS with Go 1.26.5, the full Go test suite passed, including summary timeout and process exit-3 coverage. |
| m9 | Acceptance and fixture work is present in the working tree: synchronized poll counting, typed and empty-201 process paths, exit-code coverage, and merged API fixture provenance. | `cmd/kvantumci/acceptance_test.go`, `internal/api/testdata/*` | On macOS with Go 1.26.5, the full Go test suite passed. |
| m11 | Documented. A mirror serves versioned release assets, while `latest` still asks GitHub to resolve the release tag; an explicit version is required for an offline mirror. | `README.md` | README guarded workflow was exercised independently with real `jq` and a mocked CLI: typed ID succeeds; null/numeric IDs, run failure, and wait failure stop; each case sends one run. |

## Deliberate refinements

- Non-JSON upstream error bodies produce a static explanation; arbitrary proxy
  text is never echoed. When a credential has three or fewer characters, error
  detail is suppressed rather than risking incomplete redaction.
- Only verification polling GETs retry, and only after 408, 429, 500, 502, 503,
  504, or classified transient transport failures. POST dispatches, permanent
  client errors, malformed responses, TLS failures, and redirect-policy failures
  do not retry.
- Final code and security review found no further source changes to request.
  The native-platform and deployment limitations below remain open validation
  work.

## Validation record

On macOS arm64 with Go 1.26.5, `gofmt -l` produced no output and `git diff
--check` passed. The following also passed:

```text
go vet ./...
go build ./...
sh tests/install_sh_test.sh
go test ./... -count=1
go test -race ./... -count=1
```

Both test commands passed all five packages. These cross-compilation checks also
passed without executing tests on the target operating systems:

```text
GOOS=windows GOARCH=amd64 go test ./internal/cli ./tests -run '^$' -exec=true
GOOS=linux GOARCH=amd64 go test ./internal/cli -run '^$' -exec=true
```

Isolated baseline comparisons covered the sensitive regressions. Before this
change, the POSIX installer rejected an A-signed manifest after its certificate
path changed from A to B with `release manifest signature is invalid`; the
current implementation accepts it from the pinned A snapshot. The old
PowerShell `Assert-HttpsUrl` rejected a signed redirect query with `Invalid
HTTPS download URL`; the current implementation accepts it under local pwsh.
The full Windows fixture still needs native execution. The old polling case
stopped at poll 1 with HTTP 502; the current 502 → `finished` case passes. The
old PTY Ctrl-C case remained blocked for more than one second with ECHO disabled;
the current case passes 20 repeats.

Two clean working paths built all six release assets with the same local
toolchain. Their SHA-256 hashes matched asset by asset. This records a
same-toolchain repeat-build check, not reproducibility across arbitrary Go
versions or source provenance.

Native Windows installer and console validation, Linux PTY execution, and
deployed API-contract validation remain pending.

## Release prerequisites

No signed release, release tag, production signing certificate, or certificate
fingerprint is available at the time of this ledger. Release activation remains
blocked until maintainers:

1. Provision a production RSA-3072 signing certificate and protected release
   environment secrets.
2. Replace `PROVISION_PRODUCTION_CERT_SHA256` in both versioned installers with
   the independently checked SHA-256 fingerprint of the certificate DER bytes.
3. Create a mainline tag that contains those exact installer pins, then build,
   sign, verify, and review the draft release assets.
4. Run the Windows installer suite on a Windows runner and record its result.
5. Publish a trusted, immutable bootstrap location and the independent
   certificate-distribution and fingerprint-verification instructions.

Do not present a version placeholder or a `main`-branch pipe-to-shell command as
an installation path before these requirements are complete.
