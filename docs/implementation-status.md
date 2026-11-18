# Implementation status — Gvault

Last updated: 2026-11-17  
**Release:** [0.1.0-beta.1](release-notes-0.1.0-beta.md)

Project name: **Gvault** · Module: `github.com/gvault/gvault` · Binaries: `gvault-api`, `gvault-controller`, `gvaultctl`

## Milestone checklist (beta complete)

### Milestone 1 — Fake end-to-end API

- [x] Gin API, Postgres, FakeProvider, OpenAPI, Compose, tests

### Milestone 2 — Operator CRDs (GPU-free)

- [x] `VaultSnapshot` / `VaultRestore` CRD YAML (`vault.gvault.dev`)
- [x] RBAC + in-memory reconciler + unit tests
- [x] CRD HTTP bridge + status sync into catalog
- [x] `gvault-controller` local demo

### Milestone 3 — NVIDIA adapter

- [x] Pin **v0.3.0**
- [x] Feature flag + dry-run builders
- [x] Fail-closed without live kube client
- [x] Live kube client (`internal/kube`) + PodSnapshot/Pod create/status/delete path
- [x] Unit tests via in-memory FakeAPI (never claims real GPU READY)

### Milestone 4 — Compatibility / restore contract

- [x] Shared fail-closed engine
- [x] Restore-pod contract constants for v0.3.0
- [x] Post-restore inference/health probe (`FakeProber` / `HTTPProber`)
- [x] GPU e2e tagged `gpu` → SKIPPED without hardware

### Milestone 5 — Management

- [x] Tags, promote, hold, guarded delete
- [x] Operations list + overview + audit list
- [x] Namespace allowlist + optional OIDC Bearer auth
- [x] S3/export fail-closed stub
- [x] `gvaultctl` (create/check/restore/version)

### Beta packaging

- [x] UI dashboard
- [x] `/api/v1/version`, `/metrics`
- [x] Helm chart stub + kustomize base/overlays
- [x] Controller manager (leader-election-shaped) over MemoryStore
- [x] CHANGELOG + release notes

## Verify

```bash
cp .env.example .env
make docker-up
make test && make test-integration && make build
make run-api
curl -s localhost:8080/api/v1/version
```
