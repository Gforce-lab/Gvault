# Changelog

## Unreleased — milestone hardening

- Live Kubernetes client for `nvidia-snapshot` (`internal/kube`); FakeProvider unchanged
- Post-restore inference/health probes; audit list API; namespace allowlist; OIDC Bearer (HS256 staging)
- Controller manager with leader-election-shaped loop; kustomize base/overlays
- S3/export endpoint remain fail-closed until upstream pin verifies export

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
