# CLI Operations Reference

Limited, agent-safe scope. No DELETE endpoints, credential/token writes, or
administrative writes are available.

Authenticated Client API calls send both of these headers:

```text
Authorization: Bearer <PAT-or-GAT>
x-tenant-id: <tenant-id>
```

`health` is public and does not send either header. Configure an absolute HTTPS
API URL. `KVANTUMCI_ALLOW_HTTP=true` permits HTTP only for local development at
runtime; it is not written to configuration and does not relax TLS verification.

## Diagnostics

| CLI command | Description | API |
| --- | --- | --- |
| `configure [--api-url] [--token] [--tenant-id] [--no-verify]` | Store or update PAT/GAT and tenant in local config | `GET /users/me` unless `--no-verify` |
| `health` | Check API connectivity | `GET /health` |
| `whoami` | Read the selected current-user identity fields | `GET /users/me` |

The API does not issue credentials. Create a PAT or GAT in the web UI, then
run `configure`. Interactive token entry is hidden. For scripts, prefer
`KVANTUMCI_TOKEN` to `--token`, because command-line values can appear in shell
history and process listings. Configuration merges values supplied by flags or
environment over the existing file. `login` is a hidden deprecated alias for
`configure`. In a terminal, partial flags still prompt for missing required
values; non-interactive calls with missing values fail promptly. Interrupting a
prompt restores the terminal before exit.

## Projects

| CLI command | Description | API |
| --- | --- | --- |
| `project create --name <name> [--parent-id <uuid>] [--icon <str>] [--tags a,b]` | Create a project | `POST /projects` |
| `project list [--page 1] [--limit 10] [--search <q>] [--tags a,b]` | List projects | `GET /projects` |
| `project get <projectId>` | Get project detail with child tree and repositories | `GET /projects/{id}` |

Create body: `{ name, parentId?, icon?, tags? }`. Permission: `project:create`.

## Repositories

| CLI command | Description | API |
| --- | --- | --- |
| `repo add --project <uuid> --integration <uuid> --name <name> [--branch <name>] [--resource-id <id>] [--resource-url <url>] [--repository-url <url>]` | Add repository to a project | `POST /projects/repository` |
| `repo list --project <uuid> [--page 1] [--limit 10]` | List project repositories | `GET /projects/repository/list?projectId=<uuid>` |
| `repo get <repoId>` | Get repository detail | `GET /projects/repository/{id}` |

Add body: `{ name, projectId, tenantIntegrationsId, branchName?, resourceId?, resourceUrl?, repositoryUrl?, metadata? }`.
Permission: `project:repository:create`.

## Integrations

| CLI command | Description | API |
| --- | --- | --- |
| `integration list` | List tenant integrations | `GET /integrations` |
| `integration resources <provider>` | List provider repositories/resources | `GET /integrations/{provider}/resources` |
| `integration branches <provider> <resourceId>` | List branches for a resource | `GET /integrations/{provider}/resources/{resourceId}/branches` |

Providers: `github`, `gitlab`, `jenkins`, `nexus`, `jfrog`, `azure_repos`, and
`aws`. Permission: `integrations:read`.

## Verifications

| CLI command | Description | API |
| --- | --- | --- |
| `verify run --repo <repoId> [--types sbom,cbom,aibom,mlbom]` | Run one repository verification | `POST /verifications/run/repository/{projectRepositoryId}/{tenantId}` |
| `verify run --project <projectId> --all-repos [--types ...] --confirm` | Run all project repositories after explicit confirmation | `POST /verifications/run/{projectId}/{tenantId}` |
| `verify list` | List verification runs | `GET /verifications` |
| `verify latest --repo <repoId>` | Get the newest finished verification for a repository | `GET /verifications?projectRepositoryId=<repoId>&status=finished&page=1&limit=1` |
| `verify get <verificationId>` | Get one verification detail | `GET /verifications/{id}` |
| `verify wait <verificationId> [--interval 5s] [--timeout 30m]` | Poll until `finished` or `error` | `GET /verifications/{id}` |

Run body: `{ types?: ["sbom" | "cbom" | "aibom" | "mlbom"] }`.
Repository-run → wait requires the deployed typed repository-run contract from
API PR #419. Its merged source commit is `f4c5d782`, but deployment is not
verified here. The typed response exposes the ID at `.data.verificationId`; use
that exact, nonempty string with `verify wait`. An empty `201 Created` prints
`null`, exits successfully for the accepted request, and provides no usable ID.
Stop the workflow in that case. Do not retry the POST or use `verify latest` to
correlate a newly submitted run because it can select a previous completed
verification.

Verification detail status is read from `data.status`. `verify wait` prints the
final JSON once. It retries only transient polling failures for this safe GET:
HTTP 408, 429, 500, 502, 503, and 504, plus transient transport failures and
per-request timeouts while the overall timeout remains active. Retry delay grows
after consecutive failures, resets after a successful poll, and honors a bounded
valid `Retry-After` value. Authentication, other permanent 4xx, malformed
responses, TLS, and redirect-policy failures stop immediately. It exits nonzero
when the terminal status is `error`, or when polling, cancellation, timeout, or
response parsing fails.

Permission: `verification:run` and `verification:read`.

## SBOMs and BOMs

| CLI command | Description | API |
| --- | --- | --- |
| `sbom get --verification <uuid> --repo <repoId>` | Get all BOM types for a verification | `GET /bill-of-materials/by-verification?verificationId=<uuid>&projectRepositoryId=<uuid>` |
| `sbom get <bomId>` | Get one BOM record with components | `GET /bill-of-materials/{id}` |
| `sbom list --verification <uuid> --repo <repoId> --type <sbom\|cbom\|aibom\|mlbom> [--page 1] [--limit 10]` | List BOM records | `GET /bill-of-materials?verificationId=<uuid>&projectRepositoryId=<uuid>&type=<type>` |

Permission: `billofmaterials:read`.

## Findings and results

| CLI command | Description | API |
| --- | --- | --- |
| `findings list [--verification <uuid>] [--status fail\|pass\|skip] [--page 1] [--limit 10]` | List rule-check outcomes across the tenant, or one verification | `GET /results` |
| `results summary --verification <uuid> [--timeout 5m] [--fail-on-findings]` | Fetch every result page and summarize one verification | `GET /results` |
| `results list [--verification <uuid>]` | List raw rule-check results | `GET /results` |
| `results get <resultId>` | Get one result with evidence | `GET /results/{id}` |

`findings list` is tenant-wide when `--verification` is omitted. The optional
`--status` filter accepts `fail`, `pass`, or `skip`. `page` defaults to `1` and
`limit` defaults to `10`, with values from `1` to `100`.

`results summary` has a five-minute timeout by default. `--timeout` accepts a
positive Go duration such as `30s` or `5m`. It returns `verificationId`,
`unit: "ruleOutcomes"`, `totalRuleOutcomes`, top-level totals, and per-severity
counts for `critical`, `high`, `medium`, `low`, and `unclassified`. Every count
has `fail`, `pass`, `skip`, and `unknown` fields. An explicit null status is
`unknown`, indicating incomplete output.

A completed summary exits zero by default, including when it contains failed
outcomes. `--fail-on-findings` prints the complete JSON and then exits 3 if
`fail` or `unknown` is nonzero. Use it for an automated gate. Operational,
usage, cancellation, and timeout failures exit 1. An overall summary timeout
names the verification and effective timeout and suggests increasing
`--timeout`; the command emits no partial JSON.

| Exit | Meaning |
| --- | --- |
| 0 | Command completed; a summary may still contain failed outcomes unless the gate flag was used. |
| 1 | Usage, operational, cancellation, or timeout failure. |
| 3 | `results summary --fail-on-findings` rejected completed output with `fail` or `unknown` outcomes. |

Permission: `results:read`.

## Typical flow

```text
project create → integration list → repo add → verify run --repo <id> → extract .data.verificationId → verify wait <id> → results summary --verification <id>
```

Scan dispatch acceptance is not verification completion. Wait for the returned
verification ID before treating a run as complete.

## Excluded from the CLI

| Category | Examples |
| --- | --- |
| DELETE | `/projects/{id}`, `/projects/repository/{id}`, `/verifications/{id}`, `/bill-of-materials/{id}` |
| Credentials and tokens | `POST /tokens/*`, `POST/PUT /integrations` |
| Admin and policy writes | Rules, labels, roles, settings, SSO, billing, tenants |
| Internal and worker | `PATCH /verifications/results/{verificationId}` |
| BOM generation | `POST /bill-of-materials/generate/*` |
| Updates | `PUT /projects/{id}`, `POST /projects/repository/{id}` |

## Minimum token permissions

`project:read`, `project:create`, `project:repository:read`,
`project:repository:create`, `integrations:read`, `verification:run`,
`verification:read`, `billofmaterials:read`, and `results:read`.
