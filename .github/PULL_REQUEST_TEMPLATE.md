<!--
Thank you for contributing to Gvault!
Please fill every section that applies. Incomplete PRs may be asked to revise before review.
-->

## Summary

<!-- 1–3 bullets: what changed and why (user/impact first, not file lists). -->

-

## Type of change

- [ ] Feature
- [ ] Bug fix
- [ ] Documentation
- [ ] Tests only
- [ ] CI / chore / refactor
- [ ] Security hardening

## Related

- Issue / discussion: <!-- #N or link -->
- ADR / decision updated: <!-- docs/decisions.md ADR-XXX or N/A -->
- Milestone: <!-- M1–M5 / post-beta / N/A -->

## Checklist (required)

- [ ] I read [CONTRIBUTING.md](../CONTRIBUTING.md) and [docs/decisions.md](../docs/decisions.md)
- [ ] Phase 1 stays **NVIDIA-only**; no AMD / multi-vendor product claims ([docs/scope.md](../docs/scope.md))
- [ ] Default / docs path is `PROVIDER=nvidia-snapshot` (CI harness is not a product provider)
- [ ] No claim of real GPU `READY` / restore success without live cluster evidence
- [ ] NVIDIA paths remain fail-closed without a live kube client (prerequisites SKIPPED, not PASSED)
- [ ] Catalog changes respect [docs/storage.md](../docs/storage.md) (metadata/refs only — no checkpoint bytes)
- [ ] No secrets, tokens, kubeconfigs, or checkpoint payloads committed
- [ ] Audit / logs / PR text do not dump credentials or raw artifact contents
- [ ] Docs / CHANGELOG / OpenAPI updated when user-facing behavior changes
- [ ] Material design choices recorded in `docs/decisions.md` when applicable

## Test plan

- [ ] `make fmt` (or `go fmt ./...`)
- [ ] `go test ./...`
- [ ] `go test ./tests/integration/... -tags=integration` (Postgres on `localhost:5433` or `DATABASE_URL`)
- [ ] `make build`
- [ ] UI / API smoke (if applicable): `curl -s localhost:8080/api/v1/version`
- [ ] GPU e2e (`-tags=gpu`) left SKIPPED unless hardware prerequisites are documented
- [ ] No product docs present a non-NVIDIA harness as a GPU provider

### Evidence

<!-- Paste command output summaries, screenshots of UI, or “N/A”. -->

```text

```

## Risk & rollback

- Risk level: <!-- low / medium / high -->
- Rollback: <!-- revert commit / feature flag / config only -->
- Feature flags / env vars: <!-- e.g. ENABLE_NVIDIA_PROVIDER, ALLOWED_NAMESPACES, OIDC_* -->

## Reviewer notes

<!-- Anything tricky, intentional omissions, or follow-ups. -->
