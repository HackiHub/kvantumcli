# Plan: Browser OAuth login for `kvantumci login`

Goal: `kvantumci login` opens the product/Authentik login in a browser, user signs in normally, browser redirects to a localhost callback served by the CLI, CLI stores credentials in the local config file.

This document is the checklist for a later implementation run. Current `login` (PAT/GAT paste) stays as a fallback.

---

## Summary

| Layer | Change required? | What |
|-------|------------------|------|
| Authentik | **Yes** | Allow CLI redirect URI; preferably a dedicated public client + PKCE |
| Product API (backend) | **Optional** | Public discovery of OAuth URLs / client id; optional PAT mint after JWT |
| CLI | **Yes** | Auth-code + PKCE flow, localhost server, config save |

The API already accepts `Authorization: Bearer <jwt>`. No new “CLI login” endpoint is strictly required for the JWT path.

---

## Target user flow

```
kvantumci login [--api-url ...] [--tenant-id ...]
        │
        ├─ start localhost HTTP server (fixed port, e.g. 8765)
        ├─ open browser → Authentik authorize URL (PKCE)
        ├─ user logs in (password / SSO)
        ├─ redirect → http://127.0.0.1:8765/callback?code=...&state=...
        ├─ CLI exchanges code → access_token (JWT) at Authentik token URL
        ├─ optional: call GET /users/me; pick / confirm tenant
        ├─ optional (later): POST /tokens/pat with JWT → store pat_… instead
        └─ write/update config.json (apiUrl, token, tenantId)
```

Re-login: same flow; `config.Save` merges and updates values only.

---

## 1. Authentik (required)

### 1.1 Application / provider

- [ ] Create or reuse an OAuth2/OIDC application suitable for a **native/public CLI** client.
- [ ] Client type: **public** (no client secret shipped in the binary).
- [ ] Enable **PKCE** (S256). Required for public clients.
- [ ] Note `client_id` for the CLI.

Prefer a **separate** Authentik app from Swagger/web (`AUTHENTIK_REDIRECT_URI=http://localhost:3000/api/oauth2-redirect.html`) so web and CLI redirect rules do not collide. Sharing one app is possible if both redirect URIs are allowlisted on that app.

### 1.2 Redirect URI

- [ ] Allowlist exactly (recommend fixed port):

  `http://127.0.0.1:8765/callback`

- [ ] Optionally also allow `http://localhost:8765/callback` (some OS/browsers differ). Prefer **one** canonical host (`127.0.0.1`) in the CLI to avoid mismatch errors.

**Not enough:** only changing backend `AUTHENTIK_REDIRECT_URI` to the Swagger HTML page. That URI never delivers the `code` to the CLI process.

### 1.3 Scopes / claims

- [ ] Confirm scopes the API expects on the JWT (e.g. `openid`, `profile`, `email`, plus any custom scopes).
- [ ] Confirm the JWT is accepted by the API’s JWKS validation the same way the web app’s token is.

### 1.4 Values to hand to the CLI team

| Value | Example / source |
|-------|------------------|
| Authorization URL | Authentik `/application/o/authorize/` (or `AUTHENTIK_AUTHORIZATION_URI`) |
| Token URL | Authentik `/application/o/token/` (or `AUTHENTIK_TOKEN_URI`) |
| Client ID | Authentik application |
| Redirect URI | `http://127.0.0.1:8765/callback` |
| Scopes | e.g. `openid profile email` |

---

## 2. Product API / backend (optional but useful)

### 2.1 No new endpoint (minimum path)

- [ ] Document that CLI uses Authentik directly with the values above.
- [ ] Bake or configure those values in the CLI (`--authentik-*` flags / env / config).
- [ ] After login, CLI stores JWT and uses existing Client API + `x-tenant-id`.

### 2.2 Public OAuth discovery endpoint (recommended)

Avoid hardcoding Authentik URLs per environment.

- [ ] Add something like `GET /cli/oauth-config` or `GET /.well-known/kvantumci-cli` (public, no auth) returning JSON, e.g.:

```json
{
  "authorizationUri": "https://authentik.example/application/o/authorize/",
  "tokenUri": "https://authentik.example/application/o/token/",
  "clientId": "kvantumci-cli",
  "redirectUri": "http://127.0.0.1:8765/callback",
  "scopes": ["openid", "profile", "email"]
}
```

- [ ] CLI: `login` with `--api-url` fetches this, then starts the browser flow.
- [ ] Keep redirect URI in sync with Authentik allowlist (config-driven on both sides).

### 2.3 Long-lived CLI tokens (optional follow-up)

JWTs expire. For agent/CI-friendly longevity:

- [ ] After OAuth, with the JWT, call existing `POST /tokens/pat` (if allowed for that user) and store `pat_…` in config instead of the JWT.
- [ ] Or add a dedicated “create CLI PAT” endpoint with fixed scopes matching [CLI_OPERATIONS.md](CLI_OPERATIONS.md) minimum permissions.
- [ ] Decide whether browser-login CLI is allowed to create PATs (product/security call). Note: current agent-safe CLI scope excluded token **writes**; browser login minting a PAT is an explicit policy exception.

### 2.4 Tenant selection

- [ ] After token is obtained, CLI calls `GET /users/me`.
- [ ] If one tenant → auto-set `tenantId`.
- [ ] If multiple → prompt or require `--tenant-id`.
- [ ] Persist chosen tenant in config.

---

## 3. CLI changes (this repo)

### 3.1 Keep existing PAT login

- [ ] Keep current flag/prompt PAT/GAT flow as `login --token …` or `login --method token`.
- [ ] Add browser flow as default or as `login --method browser` / `login --browser`.

### 3.2 New packages / files (suggested)

```
internal/oauth/          # PKCE, state, authorize URL, token exchange
internal/oauth/callback.go  # localhost server, one-shot code capture
internal/cli/login.go    # branch: browser vs token
internal/config/         # optional: store tokenType (jwt|pat), expiresAt later
```

No new heavy deps required: stdlib `net/http`, `crypto/sha256`, `encoding/base64`, `crypto/rand`. Optional: open browser via OS (`open` / `xdg-open` / `cmd /c start`) without extra libraries.

### 3.3 Browser login algorithm

1. Resolve `api-url` (flag/env/config/prompt).
2. Load OAuth settings (discovery endpoint and/or flags/env):
   - `KVANTUMCI_OAUTH_CLIENT_ID`
   - `KVANTUMCI_OAUTH_AUTH_URL`
   - `KVANTUMCI_OAUTH_TOKEN_URL`
   - `KVANTUMCI_OAUTH_REDIRECT_URI` (default `http://127.0.0.1:8765/callback`)
   - `KVANTUMCI_OAUTH_SCOPES`
3. Generate `state` + PKCE `code_verifier` / `code_challenge` (S256).
4. Bind localhost server on the redirect port; fail clearly if port busy.
5. Open system browser to authorize URL.
6. Wait for callback (timeout, e.g. 5 minutes); validate `state`.
7. POST token URL (`grant_type=authorization_code`, `code`, `redirect_uri`, `client_id`, `code_verifier`).
8. Extract `access_token` (and optionally `refresh_token` if Authentik issues one — store only if we implement refresh later).
9. Verify with `GET /users/me`; resolve tenant.
10. `config.Save` merge (same as today’s re-login behavior).
11. Print JSON summary **without** printing the raw token (same as current login).

### 3.4 Config file

Keep current shape for v1:

```json
{
  "apiUrl": "https://api.example.com",
  "token": "<jwt or pat_…>",
  "tenantId": "<uuid>"
}
```

Optional later fields: `tokenType`, `expiresAt`, `refreshToken` (treat refresh token as secret; 0600 file already).

### 3.5 Tests

- [ ] PKCE challenge generation unit test.
- [ ] Callback server: valid code+state; reject bad state; timeout.
- [ ] Token exchange against `httptest`.
- [ ] Config merge on re-login unchanged.

### 3.6 Docs

- [ ] Update [README.md](README.md) and [CLI_OPERATIONS.md](CLI_OPERATIONS.md) with browser login + Authentik prerequisites.
- [ ] Document fixed port and “port in use” troubleshooting.

---

## 4. Security checklist

- [ ] PKCE S256; no client secret in the binary.
- [ ] Bind callback server to `127.0.0.1` only (not `0.0.0.0`).
- [ ] One-shot listener; shut down after success/failure.
- [ ] Validate `state` (CSRF).
- [ ] Never print access/refresh tokens to stdout.
- [ ] Config file mode `0600`, directory `0700` (already).
- [ ] Clear error if redirect URI / client_id mismatch (Authentik error page).

---

## 5. Suggested implementation order (later run)

1. **Authentik**: public client + PKCE + `http://127.0.0.1:8765/callback` allowlisted; collect URLs + client id.
2. **Manual smoke**: browser hit authorize URL with redirect to a throwaway local listener; confirm code returns.
3. **CLI**: implement `internal/oauth` + `login --browser` against hardcoded/env OAuth settings.
4. **CLI**: wire tenant via `whoami`; save config; keep `--token` path.
5. **Optional backend**: `GET /cli/oauth-config` discovery.
6. **Optional**: exchange JWT → PAT for long-lived CLI use; document policy.

---

## 6. Out of scope (for this flow)

- Device-code grant (API/Authentik may not expose it today).
- Using Swagger’s `oauth2-redirect.html` as the CLI callback.
- Username/password against the product API.
- Changing `AUTHENTIK_REDIRECT_URI` alone as a substitute for a CLI redirect URI.

---

## 7. Decision log (fill in before coding)

| Decision | Choice | Notes |
|----------|--------|-------|
| Default `login` mode | browser / token / ask | |
| Authentik app | shared with web / dedicated CLI | Prefer dedicated |
| Callback port | `8765` or other | Must match Authentik allowlist |
| Store JWT or mint PAT | JWT first / PAT after | JWT simpler; PAT better for CI |
| OAuth settings source | env only / discovery endpoint | Discovery better for multi-env |
| Refresh tokens | yes / no for v1 | Can defer |

---

## References

- Current PAT login: [`internal/cli/login.go`](internal/cli/login.go)
- Config save/merge: [`internal/config/config.go`](internal/config/config.go)
- Backend guidance (prior): API does not issue credentials; Authentik OAuth auth-code is the interactive path; PAT/GAT best for non-interactive CLI.
