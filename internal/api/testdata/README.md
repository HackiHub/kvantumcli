# Sanitized CLI API contract fixtures

These fixtures are derived from the API source at commit `5c32282`, specifically
`src/verifications/verifications.controller.ts` for the repository-run envelope,
and `src/results/results.schema.ts` for result rows. The verification-detail and
list envelopes follow the controller's wrapped response contract.

They contain invented IDs and no credentials, tenant data, repository URLs, or
production findings. The CLI process acceptance test reads these files while a
local deterministic fake server serves the documented endpoint shapes.
