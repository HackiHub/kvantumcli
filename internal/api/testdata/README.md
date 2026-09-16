# Sanitized CLI API contract fixtures

These fixtures are derived from the merged API source at commit
`f4c5d782d2e2155b5fd7efd4f64fbc86b24d958f` (API PR #419), specifically the
verification routes and their response schemas in
`src/verifications/verifications.controller.ts`, and the results route and rows
in `src/results/`. Repository-run and verification-detail responses are wrapped
as `{ "statusCode": ..., "data": ... }`. List endpoints, including results,
are flat pagination objects whose `data` field contains the list items.

They contain invented IDs and no credentials, tenant data, repository URLs, or
production findings. The CLI process acceptance test reads these files while a
local deterministic fake server serves the documented endpoint shapes.
