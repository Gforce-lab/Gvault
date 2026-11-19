# Changelog

## Unreleased — NVIDIA-first Phase 1

- NVIDIA restores now wait for the Snapshot restore condition and Pod readiness, rather than Pod phase alone
- Optional HTTP API/CLI `inferenceProbe` verifies bounded generation directly on the restored Pod IP
- Configurable total restore deadline, retrying application probes, and sanitized inference audit outcomes
- Pinned Snapshot vLLM `/generate` protocol, validated workload Pod specs, and accurate operation provider/simulation markers

- Product default is `PROVIDER=nvidia-snapshot` only; FakeProvider package removed (CI uses in-memory kube + NVIDIA adapter)
- Documented storage model (`docs/storage.md`) and hard stop-limits (`docs/scope.md`); AMD out of scope
- Live Kubernetes client for NVIDIA; probes; audits; OIDC/namespace authz; kustomize; expanded `gvaultctl`
- S3/export remains fail-closed until upstream pin verifies export


## 0.1.0-beta.1 — 2026-11-17

First public **beta** of **Gvault**.

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
- Live NVIDIA path requires kube client + GPU cluster (see `docs/gpu-setup.md`); without them prerequisites stay SKIPPED
- CRD bridge uses MemoryStore + manager loop (full controller-runtime/envtest still later)
- Auth is API-key / optional OIDC HS256 / dev bypass

### Upgrade notes

- Module: `github.com/gvault/gvault`
- Binaries: `gvault-api`, `gvault-controller`, `gvaultctl`
- Local Postgres DB name: `gvault` on host port `5433`
