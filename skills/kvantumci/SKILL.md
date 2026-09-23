---
name: kvantumci
description: Use the kvantumci CLI to inspect or manage KvantumCI projects and repositories, run verifications, and read BOMs, findings, or results. Use for KvantumCI service operations, not for developing the CLI itself.
---

# Use the KvantumCI CLI

Use the installed `kvantumci` executable for the user's requested KvantumCI operation. Check that it is available on `PATH` with `kvantumci --help`, then use the relevant subcommand's `--help` for flags. Successful output is JSON. If the executable is missing, use the [official release installation instructions](https://github.com/HackiHub/kvantumcli#install) for a published version; do not rely on a source checkout or development build.

## Access and credentials

- For authenticated calls, the CLI needs an API URL, a PAT or GAT, and a tenant ID. It reads flags first, then `KVANTUMCI_API_URL`, `KVANTUMCI_TOKEN`, and `KVANTUMCI_TENANT_ID`, then its local config file. `health` needs only the API URL.
- Use an existing configuration or environment-provided credentials. If setup is needed, have the user create a PAT or GAT in the web UI and run `kvantumci configure` interactively, or supply the token through `KVANTUMCI_TOKEN`. Do not put tokens in chat, command arguments, logs, or committed files. `configure` saves credentials locally and verifies them with `whoami` by default.
- API URLs must use HTTPS. `KVANTUMCI_ALLOW_HTTP=true` is only for a local development API; it does not disable TLS verification.
- Start with `kvantumci whoami` when the tenant or identity matters, especially before writes.

## Find the target before acting

- Projects: `project list`, `project get <projectId>`, `project create --name <name>`.
- Integrations: `integration list`; `integration resources <provider> --integration <integrationId>`; `integration branches <provider> <resourceId> --integration <integrationId>`. Select an explicit integration ID when a provider has more than one integration.
- Repositories: `repo list --project <projectId>`, `repo get <repoId>`, `repo add --project <projectId> --integration <integrationId> --name <name>`. For Git integrations, use the discovered resource ID and URL as `--resource-id` and `--repository-url`, and pass a discovered branch with `--branch`.
- Verifications: `verify list`, `verify get <verificationId>`, `verify latest --repo <repoId>` (latest *finished* run only).
- Findings and results: `findings list --verification <verificationId> --status fail`, `results list --verification <verificationId>`, `results get <resultId>`, `results summary --verification <verificationId>`.
- BOMs: `sbom get --verification <verificationId> --repo <repoId>`, `sbom list --verification <verificationId> --repo <repoId> --type sbom`.

Use IDs returned by the CLI or supplied by the user. Inspect list/detail output before selecting among ambiguous matches. `findings list` without `--verification` reads the whole tenant.

## Run and inspect a verification

1. Run `kvantumci verify run --repo <repoId>` once for the requested repository. Add `--types sbom,cbom,aibom,mlbom` only for types the user wants.
2. The repository-run response must contain a nonempty string at `.data.verificationId`. Extract that exact ID from the JSON. An accepted request may print `null` on an older API; if no usable ID is returned, stop and report that the run was accepted without an ID. Do not repeat the POST or substitute `verify latest`, which may select an earlier run.
3. Run `kvantumci verify wait <verificationId>` to poll for completion. The default interval is `5s` and overall timeout is `30m`. A run ending in `error` exits nonzero.
4. After a finished run, use `kvantumci results summary --verification <verificationId>` for counts. `unit: "ruleOutcomes"` means these are rule outcomes, not distinct bugs. Inspect `results list`, `findings list`, `results get`, or `sbom` commands for details.

For an automated gate, add `--fail-on-findings` to `results summary`: it prints complete JSON and exits `3` when `fail` or `unknown` outcomes exist. Exit `0` without that flag does not mean there are no findings. Exit `1` means an operational or usage failure. A null result status counts as `unknown`. Pass only the ID returned by `verify run` and confirmed by `verify wait` for this run. Until the API update that validates verification IDs against the selected tenant is deployed, an unknown or wrong ID can produce a zero-outcome summary that exits `0`; do not rely on the summary as a gate against that API version.

Run all repositories in a project only when the user explicitly requests that scope: `kvantumci verify run --project <projectId> --all-repos --confirm`. The CLI cannot issue API tokens or perform deletes or administrative writes. If the requested operation is outside the CLI, report that limitation rather than inventing a command.
