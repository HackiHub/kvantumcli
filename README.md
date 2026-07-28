# kvantumci

Agent-safe CLI for the KvantumCI Client API. Single static Go binary for macOS, Linux, and Windows.

## Build

Requires Go 1.22+.

```bash
make build          # → bin/kvantumci
make test
make dist           # → dist/kvantumci-<os>-<arch>[.exe]
```

## Configure

Precedence: flags > environment > config file.

| Setting | Flag | Env | Config key |
|---------|------|-----|------------|
| API base URL | `--api-url` | `KVANTUMCI_API_URL` | `apiUrl` |
| Bearer PAT/GAT | `--token` | `KVANTUMCI_TOKEN` | `token` |
| Tenant ID | `--tenant-id` | `KVANTUMCI_TENANT_ID` | `tenantId` |
| Config path | — | `KVANTUMCI_CONFIG` | — |

Config file locations:

- Linux: `$XDG_CONFIG_HOME/kvantumci/config.json` (or `~/.config/kvantumci/config.json`)
- macOS: `~/Library/Application Support/kvantumci/config.json`
- Windows: `%AppData%\kvantumci\config.json`

Example config:

```json
{
  "apiUrl": "https://api.example.com",
  "token": "pat_...",
  "tenantId": "00000000-0000-0000-0000-000000000000"
}
```

Auth: `Authorization: Bearer <token>` plus `x-tenant-id` on Client API calls. `health` is public and does not require a token.

## Login

The API never issues credentials (no username/password or device-code flow). Create a PAT or GAT in the web UI, then:

```bash
# Non-interactive
kvantumci login --api-url https://api.example.com --token pat_... --tenant-id <uuid>

# Interactive (prompts; existing config used as defaults)
kvantumci login

# Update only the token (keeps apiUrl and tenantId)
kvantumci login --token pat_new...
```

By default, login calls `whoami` to verify credentials before writing. Use `--no-verify` to skip. The token is never printed in the JSON result.

## Typical flow

```bash
kvantumci login --api-url https://api.example.com --token pat_... --tenant-id <uuid>
kvantumci health
kvantumci whoami
kvantumci project create --name demo
kvantumci integration list
kvantumci repo add --project <uuid> --integration <uuid> --name my-repo
kvantumci verify run --repo <repoId>
kvantumci verify wait <verificationId>
kvantumci sbom get --verification <uuid> --repo <repoId>
kvantumci results list --verification <uuid>
```

Bulk verification requires explicit confirmation:

```bash
kvantumci verify run --project <uuid> --all-repos --confirm
```

## Commands

See [CLI_OPERATIONS.md](CLI_OPERATIONS.md) for the full agent-safe command surface. DELETE, credential writes, and admin endpoints are intentionally excluded.

## Extending

1. Add an API method under `internal/api/`
2. Add a cobra command under `internal/cli/`
3. Register it from `internal/cli/root.go` (or the parent resource command)
