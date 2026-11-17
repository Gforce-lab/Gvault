# Security notes (Milestone 1)

- Prefer `DEV_AUTH_BYPASS=false` + `API_KEY` even locally when exposing the port beyond localhost.
- Audit events must not contain secrets, tokens, or checkpoint dumps (attributes are sanitized by construction in M1 handlers).
- No raw artifact download endpoints.
- Real GPU path will require least-privilege RBAC for API and controller service accounts (Milestone 2+).
