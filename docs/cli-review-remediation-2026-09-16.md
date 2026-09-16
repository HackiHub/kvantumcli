# CLI review remediation ledger

This ledger records the implementation work for the 2026-09-16 CLI review. It
describes source in this checkout. It does not establish that a server contract
is deployed or that a release has been published.

## Contract source

Sanitized API fixtures are based on API source commit `5c32282`:
`src/verifications/verifications.controller.ts` for repository-run responses
and `src/results/results.schema.ts` for result rows. See
[`internal/api/testdata/README.md`](../internal/api/testdata/README.md). This is
source-level fixture provenance, not deployed API validation.

## Findings

| Review items | Status | Changed files | Local checks |
| --- | --- | --- | --- |
| B1, B2 | Implemented. Verification detail reads `data.status`; repository runs preserve the typed response, whose current contract exposes `.data.verificationId`. Empty legacy responses provide no ID. | `internal/api/verifications.go`, `internal/api/wait.go`, `internal/cli/verify.go`, their tests, `internal/api/testdata/*`, `cmd/kvantumci/acceptance_test.go` | Unit and process acceptance cases cover running, finished, error, malformed detail, and run → wait. |
| B3 | Implemented in source, release activation pending. Installers verify a signed six-asset manifest and binary hash against a pinned certificate fingerprint. | `install.sh`, `install.ps1`, `Makefile`, `.github/workflows/release.yml`, `tests/install_sh_test.sh`, `tests/windows_installer_test.go` | The POSIX suite passed locally for valid/latest installation, tampered binary and manifest, missing or duplicate manifest entries, missing certificate, failed asset download, HTTP URL rejection, TERM interruption cleanup and exit `143`, destination preservation, and fail-closed behavior. Windows validation remains pending. |
| B4 | Implemented in source. Both installers validate the configured download base as an absolute HTTPS URL before fetching release assets. | `install.sh`, `install.ps1`, `tests/install_sh_test.sh`, `tests/windows_installer_test.go` | POSIX HTTP URL rejection passed locally. Redirect-downgrade coverage is not recorded in the current installer suite. |
| M1, M2 | Implemented. Installer downloads use HTTPS only, resolve `latest` once, validate repository/version/URLs, stage a verified binary, and fail without replacing an existing binary. | `install.sh`, `install.ps1`, `tests/install_sh_test.sh`, `tests/windows_installer_test.go` | POSIX installer cases listed for B3 passed locally. Windows validation remains pending. |
| M3 | Implemented in source. The PowerShell installer preserves unexpanded user PATH values and registry kind, avoids duplicate entries, and supports `-NoPathUpdate`. | `install.ps1`, `tests/windows_installer_test.go` | Windows installer test suite exists; Windows runner validation remains pending. |
| M4 | Implemented. Config writes use a private same-directory temporary file, reject symlinks, preserve existing data on failure, and use a protected DACL for Windows temporary files. | `internal/config/config.go`, `internal/config/private_temp_unix.go`, `internal/config/private_temp_windows.go`, `internal/config/security_test.go`, `internal/config/security_windows_test.go` | Config security tests cover permissions, symlinks, and failed replacement. |
| M5, M6 | Implemented. Terminal token entry is hidden; prompt input avoids buffered read-ahead. `--token` remains available with an exposure warning. | `internal/cli/login.go`, `internal/cli/login_security_test.go`, `internal/cli/root.go` | Login security and command tests cover prompt/configuration paths. |
| M7, M8, M9 | Implemented. HTTPS is required by default, development HTTP needs `KVANTUMCI_ALLOW_HTTP=true`, auth redirects cannot cross origin, error bodies are bounded and allowlisted, and configure returns selected identity fields only. | `internal/config/config.go`, `internal/api/client.go`, `internal/api/client_security_test.go`, `internal/cli/login.go`, `internal/cli/login_security_test.go` | Client and login security tests cover transport, redirects, redaction, and identity output. |
| M10, m1, m2, m3 | Implemented. Nullable result status is `unknown`; summaries reconcile all count buckets, default to a five-minute timeout, and offer opt-in `--fail-on-findings`. Findings remain tenant-wide unless filtered by `--verification`. | `internal/api/results.go`, `internal/cli/results.go`, `internal/cli/findings.go`, related tests | Results tests cover null status, pagination, timeout, cancellation, default and gated exits, plus tenant-wide and filtered findings queries. |
| M11 | Implemented in source. CI defines formatting, vet, build, unit, race, and cross-platform test jobs; release drafting is separate. | `.github/workflows/ci.yml`, `.github/workflows/release.yml` | Workflow files are present. Required-check and branch-protection configuration are repository settings, not verified here. |
| M12, m8, m9, m10, m11 | Implemented. Added command/process acceptance coverage, output writer failure coverage, fixture provenance, and targeted security/installer tests. | `cmd/kvantumci/acceptance_test.go`, `internal/output/output_test.go`, `internal/api/testdata/*`, test files above | `go test ./...` passes on this macOS checkout; platform-specific installer checks need their matching runners. |
| m4, m5, m6, m7 | Implemented with the credential, transport, and verification changes listed above. | `internal/config/*`, `internal/cli/login.go`, `internal/api/client.go`, `internal/api/{verifications,wait}.go` | Focused security, API, and CLI test files listed above. |
| m12 | Deferred. The finding concerns ignored required-flag registration errors caused only by programmer typos. | No dedicated change. | Existing command construction tests remain the coverage point. |
| m13 | Deferred. The deleted OAuth plan has no established current product intent. | No change. | Requires a product-history decision before documentation is restored. |

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
