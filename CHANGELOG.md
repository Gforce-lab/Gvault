# Changelog

## 0.1.0-beta.1 — 2026-11-17

First public **beta** of **gvault**.

### Included (all plan milestones, beta-complete)

- Milestone 1: Gin API, Postgres catalog, FakeProvider, OpenAPI, Compose, tests
- Milestone 2: `VaultSnapshot` / `VaultRestore` CRDs, RBAC, in-memory reconciler, CRD HTTP bridge, status sync to catalog
- Milestone 3: NVIDIA Snapshot **v0.3.0** pin, feature-flagged adapter, dry-run PodSnapshot / restore-pod builders (fail-closed without live kube)
- Milestone 4: Shared fail-closed compatibility engine; GPU e2e tests SKIPPED without hardware
- Milestone 5: Tags, promote, retention hold, guarded delete, operations list, `gvaultctl`
- UI flow dashboard at `/`
- Prometheus text metrics at `/metrics`
- Helm chart stub under `config/helm/gvault`
- Version endpoint `/api/v1/version`

### Explicit beta limitations

- Default provider is FakeProvider (simulated READY — not a physical GPU checkpoint)
- Live NVIDIA kube client not wired; real GPU A→B restore requires a cluster (see `docs/gpu-setup.md`)
- CRD bridge is in-process (not a full controller-runtime manager / leader election yet)
- Auth is API-key / dev bypass (OIDC deferred)

### Upgrade notes

- Module: `github.com/gvault/gvault`
- Binaries: `gvault-api`, `gvault-controller`, `gvaultctl`
- Local Postgres DB name: `gvault` on host port `5433`
