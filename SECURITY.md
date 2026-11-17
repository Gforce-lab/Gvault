# Security policy

## Milestone 1 limitations

- Authentication is API-key based, with an optional local `DEV_AUTH_BYPASS`.
- Do **not** enable `DEV_AUTH_BYPASS` outside trusted local development.
- OIDC / per-namespace authorization is deferred.
- FakeProvider does not handle real checkpoint artifacts. When NVIDIA integration lands, treat artifacts as sensitive (may contain model weights, secrets, process memory).

## Reporting

Please open a private security report through the repository maintainer channel listed in `MAINTAINERS.md`. Do not file public issues for vulnerabilities that could expose credentials or checkpoint data.
