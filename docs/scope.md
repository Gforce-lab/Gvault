# Phase scope & stop-limits

This document is the **hard stop line** for the current delivery phase.

## Phase 1 — NVIDIA GPU only (current stop)

**In scope**

1. Gvault control plane (API, catalog, ops, audit, tags/hold/promote, CLI, UI)
2. Provider: **NVIDIA Snapshot pinned to v0.3.0**
3. Live kube client → create/watch/delete `PodSnapshot` + restore Pod contract
4. Fail-closed behavior when the cluster / operator / client is missing (prerequisites SKIPPED, not PASSED)
5. Compatibility preflight for NVIDIA GPU product / driver / arch / image digest
6. Post-restore HTTP probe hook
7. Auth: API key + optional OIDC; namespace allowlist
8. Deploy stubs: Helm, kustomize, CRDs, RBAC

**Explicitly out of scope for Phase 1 (stop here)**

| Stop item | Why we stop |
|---|---|
| **AMD GPU provider** | Next phase after NVIDIA path is proven on hardware |
| **Multi-vendor abstraction beyond interface** | One real provider first; avoid fake/generic GPU stories |
| **Claiming real GPU READY without a cluster** | Never advertise physical success from demos or unit fakes |
| **S3 / object-store export as READY** | Fail-closed until NVIDIA pin documents verified export |
| **Storing checkpoint bytes in Postgres or the API** | Catalog metadata only |
| **Full controller-runtime + envtest in CI** | MemoryStore/manager is interim; not a Phase-1 blocker for NVIDIA API path |
| **Production JWKS OIDC** | HS256 staging verifier is enough for Phase 1 |
| **Billing, multi-tenancy UI, marketplace** | Not this product slice |

## Phase 2 — only after Phase 1 exit criteria

Start Phase 2 **only when** all of the following are true:

1. Capture on real NVIDIA GPU → catalog `READY` with provider binding + opaque artifact ref
2. Compatibility correctly blocks mismatched GPU/driver/image
3. Restore GPU A → GPU B (same product class per pin) + probe passes
4. Guarded delete / hold verified against live provider lifecycle
5. Docs/runbooks match measured results (`docs/gpu-setup.md`, e2e no longer SKIPPED)

Then — and only then — design **AMD** (or other) as a second `CheckpointProvider` implementation.

## AMD (future)

- Not implemented.
- No AMD fields, drivers, or ROCm assumptions in the NVIDIA adapter.
- When started: new package `internal/provider/amd/…`, separate pin, separate compatibility matrix — do not overload the NVIDIA adapter.

## CI without a physical GPU

Tests use the **nvidia-snapshot** provider with an in-memory Kubernetes API (`kube.MemoryAPI` / `nvidia.NewWithMemoryCluster`). That is still the NVIDIA adapter — there is no separate fake product provider.

Physical GPU e2e remains `make test-gpu` and stays SKIPPED until hardware prerequisites exist.

Product `PROVIDER` default is **`nvidia-snapshot`** only.
