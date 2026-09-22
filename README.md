# kvantumci

`kvantumci` is an agent-safe command-line client for the KvantumCI Client API.
It produces JSON on standard output so scripts can create projects and
repositories, run verifications, and inspect results without destructive or
administrative API operations.

Licensed under the [Apache License 2.0](LICENSE).

## Install

The first supported release version is `v1.0.0`. The installation commands
below become available only after that release is published; until then there
is no supported install. Always install from an explicit version tag rather
than executing a mutable `main`-branch script.

The release installers support Linux and macOS on AMD64 and ARM64, and Windows
on AMD64 and ARM64. They download a release manifest, its detached signature,
and the platform binary. Installation proceeds only when the manifest signature
and the binary's SHA-256 hash verify.

The installer needs the repository's trusted
[`release-signing-cert.pem`](release-signing-cert.pem), which contains the
RSA-3072 release public key. Its SHA-256 certificate fingerprint is:

```text
7f200aeb7faf7e5158caa354d7a0c72bfcbca09ab024b8d557016b6a10aa197b
```

The versioned installers pin this fingerprint, verify the signed manifest, and
then verify the selected binary's SHA-256 hash. They fail closed if any check
fails and require HTTPS for initial requests and redirects.

The certificate and installer are fetched from the same versioned repository
ref, so initial trust comes from the HackiHub repository and the fingerprint
documented above. The pin detects a mismatched certificate, a substituted
download mirror, or modified release assets; it does not protect against an
attacker able to replace both the repository ref and its installer. Verify the
fingerprint through an independent HackiHub-controlled channel if that threat
is in scope for your environment.

Linux or macOS:

```bash
version=v1.0.0
curl --fail --location --proto '=https' --proto-redir '=https' \
  --output release-signing-cert.pem \
  "https://raw.githubusercontent.com/HackiHub/kvantumcli/$version/release-signing-cert.pem"
curl --fail --location --proto '=https' --proto-redir '=https' \
  --output install.sh \
  "https://raw.githubusercontent.com/HackiHub/kvantumcli/$version/install.sh"
KVANTUMCI_PUBLIC_KEY_FILE="$PWD/release-signing-cert.pem" \
  sh ./install.sh --version "$version"
```

Windows PowerShell:

```powershell
$version = 'v1.0.0'
Invoke-WebRequest "https://raw.githubusercontent.com/HackiHub/kvantumcli/$version/release-signing-cert.pem" -OutFile release-signing-cert.pem
Invoke-WebRequest "https://raw.githubusercontent.com/HackiHub/kvantumcli/$version/install.ps1" -OutFile install.ps1
$env:KVANTUMCI_PUBLIC_KEY_FILE = (Resolve-Path .\release-signing-cert.pem)
.\install.ps1 -Version $version
```

| Setting | POSIX installer | PowerShell installer | Default |
| --- | --- | --- | --- |
| Release version | `--version` or `KVANTUMCI_VERSION` | `-Version` or `KVANTUMCI_VERSION` | `latest` |
| Destination | `--install-dir` or `KVANTUMCI_INSTALL_DIR` | `-InstallDir` or `KVANTUMCI_INSTALL_DIR` | `~/.local/bin` / `%LocalAppData%\\Programs\\KvantumCI` |
| GitHub repository | `KVANTUMCI_REPOSITORY` | `KVANTUMCI_REPOSITORY` | `HackiHub/kvantumcli` |
| Release download root | `KVANTUMCI_DOWNLOAD_BASE_URL` | `KVANTUMCI_DOWNLOAD_BASE_URL` | GitHub release downloads |
| Trusted certificate | `KVANTUMCI_PUBLIC_KEY_FILE` | `KVANTUMCI_PUBLIC_KEY_FILE` | required |
| Update Windows user PATH | n/a | `-NoPathUpdate` disables it | enabled |

`KVANTUMCI_DOWNLOAD_BASE_URL` must be an absolute HTTPS root containing
`<tag>/<filename>` paths and serve the same signed manifest and assets. The
installer resolves `latest` through GitHub once before fetching any assets, so
all downloads use one tag. A mirror therefore does not make `latest` offline:
choose an explicit release version when GitHub cannot be reached. A failed
installation leaves an existing destination binary in place.

## Build

Requires Go 1.26.6.

```bash
make build          # bin/kvantumci
make test
make dist           # dist/kvantumci-<os>-<arch>[.exe]
```

## Configure

Configuration values use this precedence: flags, environment, then the local
config file.

| Setting | Flag | Environment | Config key |
| --- | --- | --- | --- |
| API base URL | `--api-url` | `KVANTUMCI_API_URL` | `apiUrl` |
| Bearer PAT/GAT | `--token` | `KVANTUMCI_TOKEN` | `token` |
| Tenant ID | `--tenant-id` | `KVANTUMCI_TENANT_ID` | `tenantId` |
| Config path | — | `KVANTUMCI_CONFIG` | — |

Config files are located at:

- Linux: `$XDG_CONFIG_HOME/kvantumci/config.json`, or `~/.config/kvantumci/config.json`
- macOS: `~/Library/Application Support/kvantumci/config.json`
- Windows: `%AppData%\\kvantumci\\config.json`

The CLI writes configuration through a private temporary file and rejects a
config-file symlink. On Unix the saved file mode is `0600`. On Windows, it
automatically applies a restrictive ACL that grants access only to the current
user and SYSTEM. Keep any custom path selected with `KVANTUMCI_CONFIG` in a
directory that is not writable by untrusted users.

Use interactive configuration when possible. Token entry does not echo in a
terminal. If you provide only some configuration flags, an interactive terminal
prompts for the remaining required values. A non-interactive invocation with
missing required values fails instead of waiting for input. Interrupting a
prompt restores the terminal before the command exits.

```bash
kvantumci configure
```

For automation, inject the secret through the environment rather than passing it
on the command line:

```bash
KVANTUMCI_TOKEN="$TOKEN" kvantumci configure \
  --api-url https://api.example.com \
  --tenant-id 00000000-0000-0000-0000-000000000000
```

`--token` is retained for compatibility, but shell history and process listings
can expose its value. `configure` checks `whoami` before saving unless
`--no-verify` is supplied. It returns only `configPath`, `apiUrl`, `tenantId`,
and a selected identity when verified; it never returns the token. `login` is a
hidden deprecated alias for `configure`.

API URLs must be absolute HTTPS URLs. For a local development API only, set
`KVANTUMCI_ALLOW_HTTP=true`; this is a runtime opt-in and is not saved in the
config file. It does not disable TLS certificate verification.

Every authenticated Client API request sends both headers:

```text
Authorization: Bearer <PAT-or-GAT>
x-tenant-id: <tenant-id>
```

`health` is public and sends neither header.

## Typical flow

```bash
kvantumci configure
kvantumci health
kvantumci whoami
kvantumci project create --name demo
kvantumci integration list
kvantumci repo add --project <uuid> --integration <uuid> --name my-repo
```

To select one of several integrations for the same provider, discover its
resources explicitly and use the returned resource ID when adding the
repository. Git integrations need a branch as well:

```bash
kvantumci integration resources github --integration <integration-uuid>
kvantumci integration branches github <resource-id> --integration <integration-uuid>
kvantumci repo add --project <project-uuid> --integration <integration-uuid> \
  --name my-repo --resource-id <resource-id> \
  --repository-url <repository-url> --branch <branch-name>
```

For Git integrations, use the resource's returned `url` as
`--repository-url`. The API requires the repository URL and branch.

For a repository run, the deployed API must return the typed repository-run
contract from API PR #419: a wrapped response with the verification ID at
`.data.verificationId`. PR #419 merged as `f4c5d782`, but that only verifies
API source; this checkout does not establish deployment to an environment you
use. Extract and validate the returned ID before waiting:

```bash
repo_id='repository-uuid' # Replace with the ID returned by `repo add`.
if ! run_response="$(kvantumci verify run --repo "$repo_id")"; then
  exit 1
fi
if ! verification_id="$(printf '%s' "$run_response" | jq -er \
  '.data.verificationId | select(type == "string" and length > 0)')"; then
  printf '%s\n' 'Repository run returned no usable verification ID; stopping.' >&2
  exit 1
fi
kvantumci verify wait "$verification_id" && \
  kvantumci results summary --verification "$verification_id" --timeout 5m
```

This example requires `jq`. The guards prevent an empty or non-string ID from
being passed to the shell or to `verify wait`.

An empty `201 Created` response prints `null` and exits successfully because
the request was accepted, but it supplies no usable ID. Stop that workflow;
do not invent an ID, retry the POST, or use `verify latest` to correlate it.
`verify latest` finds the newest completed run, which can be an earlier run.
Project-wide runs retain their API response as returned.

Successful API responses are limited to 64 MiB. A response over that limit is
rejected rather than being treated as a truncated success.

`results summary` counts rule outcomes, rather than distinct bugs. A null result
status is counted as `unknown`, which means the result is incomplete. A successful
summary exits zero by default. In a gate, use `--fail-on-findings`; JSON is still
written first, then the command exits 3 when `fail` or `unknown` outcomes exist.
Use the verification ID returned by `verify run` or `verify wait`; do not invent
or substitute an ID. After the API update that validates the ID against the
selected tenant is deployed, an unknown ID fails instead of producing a
zero-outcome summary. Roll out that API update before relying on this as a gate.
Against an older API, validate the ID from the `verify run` or `verify wait`
response before calling `results summary`.

`findings list` is tenant-wide unless `--verification` is supplied:

```bash
kvantumci findings list --status fail
kvantumci findings list --verification <verificationId> --status fail
```

Bulk verification requires explicit confirmation:

```bash
kvantumci verify run --project <uuid> --all-repos --confirm
```

## Commands

See [CLI_OPERATIONS.md](CLI_OPERATIONS.md) for the command reference. DELETE,
credential/token, and administrative writes are excluded.

## Extending

1. Add an API method under `internal/api/`.
2. Add a Cobra command under `internal/cli/`.
3. Register it from `internal/cli/root.go` or its parent resource command.
