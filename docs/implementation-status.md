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

### Milestone 4 — Compatibility / restore contract

- [x] Shared fail-closed engine
- [x] Restore-pod contract constants for v0.3.0
- [x] GPU e2e tagged `gpu` → SKIPPED without hardware

### Milestone 5 — Management

- [x] Tags, promote, hold, guarded delete
- [x] Operations list + overview
- [x] `gvaultctl` (create/check/restore/version)

### Beta packaging

- [x] UI dashboard
- [x] `/api/v1/version`, `/metrics`
- [x] Helm chart stub
- [x] CHANGELOG + release notes

## Verify

```bash
cp .env.example .env
make docker-up
make test && make test-integration && make build
make run-api
curl -s localhost:8080/api/v1/version
```
