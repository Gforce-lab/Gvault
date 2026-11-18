# Security notes

- Prefer `DEV_AUTH_BYPASS=false` + `API_KEY` (and/or OIDC) even locally when exposing the port beyond localhost.
- Optional OIDC: set `OIDC_ISSUER`, `OIDC_AUDIENCE`, and `OIDC_CLIENT_SECRET` for HS256 Bearer JWT verification (staging). Production should use JWKS against the IdP.
- Optional `ALLOWED_NAMESPACES` (comma-separated) restricts capture/restore to listed namespaces.
- Audit events must not contain secrets, tokens, or checkpoint dumps (attributes are sanitized by construction).
- No raw artifact download endpoints. `/api/v1/export/s3` is fail-closed until the pinned NVIDIA release verifies export.
- Catalog stores metadata/refs only — never checkpoint bytes ([storage.md](storage.md)).
- Real NVIDIA path requires least-privilege RBAC for API and controller service accounts (`config/rbac`).
