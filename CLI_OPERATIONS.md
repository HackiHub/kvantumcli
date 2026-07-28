# CLI Operations Reference

Limited, agent-safe scope. **No DELETE endpoints.** **No credential/token/admin writes.**

**Auth header (all calls):** `x-tenant-id: <tenant-id>`

---

## Diagnostics

| CLI Command | Description | API |
|-------------|-------------|-----|
| `login [--api-url] [--token] [--tenant-id] [--no-verify]` | Store/update PAT/GAT + tenant in local config (not an API login) | `GET /users/me` (verify unless `--no-verify`) |
| `health` | Check API connectivity | `GET /health` |
| `whoami` | Current user, tenants, permissions | `GET /users/me` |

**Login note:** The API does not issue credentials. Create a PAT/GAT in the UI, then `login` writes them to the config file. Re-login updates provided fields only.

---

## Projects

| CLI Command | Description | API |
|-------------|-------------|-----|
| `project create --name <name> [--parent-id <uuid>] [--icon <str>] [--tags a,b]` | Create a project | `POST /projects` |
| `project list [--page 1] [--limit 10] [--search <q>] [--tags a,b]` | List projects | `GET /projects` |
| `project get <projectId>` | Project detail with child tree and repos | `GET /projects/{id}` |

**Create body:** `{ name, parentId?, icon?, tags? }` — permission: `project:create`

---

## Repositories

| CLI Command | Description | API |
|-------------|-------------|-----|
| `repo add --project <uuid> --integration <uuid> --name <name> [--branch <name>] [--resource-id <id>] [--resource-url <url>] [--repository-url <url>]` | Add repo to project | `POST /projects/repository` |
| `repo list --project <uuid> [--page 1] [--limit 10]` | List repos in project | `GET /projects/repository/list?projectId=<uuid>` |
| `repo get <repoId>` | Repo detail | `GET /projects/repository/{id}` |

**Add body:** `{ name, projectId, tenantIntegrationsId, branchName?, resourceId?, resourceUrl?, repositoryUrl?, metadata? }` — permission: `project:repository:create`

---

## Integrations (read-only)

| CLI Command | Description | API |
|-------------|-------------|-----|
| `integration list` | List tenant integrations (resolve `tenantIntegrationsId`) | `GET /integrations` |
| `integration resources <provider>` | List provider repos/resources | `GET /integrations/{provider}/resources` |
| `integration branches <provider> <resourceId>` | List branches for a resource | `GET /integrations/{provider}/resources/{resourceId}/branches` |

**Providers:** `github`, `gitlab`, `jenkins`, `nexus`, `jfrog`, `azure_repos`, `aws` — permission: `integrations:read`

---

## Verifications

| CLI Command | Description | API |
|-------------|-------------|-----|
| `verify run --repo <repoId> [--types sbom,cbom,aibom,mlbom]` | Run verification for one repo (default) | `POST /verifications/run/repository/{projectRepositoryId}/{tenantId}` |
| `verify run --project <projectId> --all-repos [--types ...] [--confirm]` | Run verification for all repos in project (opt-in) | `POST /verifications/run/{projectId}/{tenantId}` |
| `verify list` | List verification runs | `GET /verifications` |
| `verify get <verificationId>` | Single run: status, timing, metadata | `GET /verifications/{id}` |
| `verify wait <verificationId> [--interval 5s] [--timeout 30m]` | Poll until `finished` or `error` (CLI-side) | `GET /verifications/{id}` (repeated) |

**Run body:** `{ types?: ["sbom" \| "cbom" \| "aibom" \| "mlbom"] }` — permission: `verification:run` / `verification:read`

**Status values:** `pending` → `running` → `finished` \| `error`

---

## SBOMs / BOMs (read-only)

| CLI Command | Description | API |
|-------------|-------------|-----|
| `sbom get --verification <uuid> --repo <repoId>` | All BOM types for a verification (preferred) | `GET /bill-of-materials/by-verification?verificationId=<uuid>&projectRepositoryId=<uuid>` |
| `sbom get <bomId>` | Single BOM record with components | `GET /bill-of-materials/{id}` |
| `sbom list --verification <uuid> --repo <repoId> --type <sbom\|cbom\|aibom\|mlbom> [--page 1] [--limit 10]` | Paginated BOM list | `GET /bill-of-materials?verificationId=<uuid>&projectRepositoryId=<uuid>&type=<type>` |

**Optional reads (not v1):**

| CLI Command | Description | API |
|-------------|-------------|-----|
| `sbom diff --verification <uuid> --repo <repoId> --type <type>` | Component diff vs previous run | `GET /bill-of-materials/diff?verificationId=<uuid>&projectRepositoryId=<uuid>&type=<type>` |
| `sbom history --repo <repoId> --type <type> [--limit 50]` | BOM history with diffs | `GET /bill-of-materials/history?projectRepositoryId=<uuid>&type=<type>&limit=<n>` |
| `sbom merge --project <projectId>` | Merged SBOM across project repos | `GET /bill-of-materials/merge-sbom/{projectId}` |

Permission: `billofmaterials:read`

---

## Results (read-only)

| CLI Command | Description | API |
|-------------|-------------|-----|
| `results list [--verification <uuid>]` | Rule-check results for a run | `GET /results` |
| `results get <resultId>` | Single result with evidence | `GET /results/{id}` |

**Result status:** `pass` \| `fail` \| `skip` — permission: `results:read`

---

## Typical flow

```
project create → integration list → repo add → verify run --repo <id> → verify wait <id> → sbom get → results list
```

---

## Excluded from CLI

| Category | Examples |
|----------|----------|
| DELETE | `/projects/{id}`, `/projects/repository/{id}`, `/verifications/{id}`, `/bill-of-materials/{id}` |
| Credentials / tokens | `POST /tokens/*`, `POST/PUT /integrations` |
| Admin / policy writes | rules, labels, roles, settings, SSO, billing, tenants |
| Internal / worker | `PATCH /verifications/results/{verificationId}` |
| BOM generation (separate from verify) | `POST /bill-of-materials/generate/*` |
| Updates | `PUT /projects/{id}`, `POST /projects/repository/{id}` |

---

## Minimum token permissions

`project:read`, `project:create`, `project:repository:read`, `project:repository:create`, `integrations:read`, `verification:run`, `verification:read`, `billofmaterials:read`, `results:read`
