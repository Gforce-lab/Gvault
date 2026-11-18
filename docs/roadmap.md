# Roadmap

## Phase 1 — NVIDIA only (current stop)

See [scope.md](scope.md) and [storage.md](storage.md).

- [x] Control plane API + Postgres catalog (metadata/refs only)
- [x] NVIDIA Snapshot **v0.3.0** adapter + dry-run + live kube wiring
- [x] Compatibility, restore probe hook, authz, CLI, UI
- [ ] Measured GPU A → B restore on real hardware (promote e2e out of SKIPPED)
- [ ] Production JWKS OIDC
- [ ] controller-runtime + envtest replacing MemoryStore

**Do not start AMD until Phase 1 exit criteria pass.**

## Phase 2 — after NVIDIA proven

1. AMD GPU provider (separate package + pin + compatibility matrix)
2. Verified object-store export if/when upstream pins support it
3. Additional vendors only behind the same `CheckpointProvider` interface

## Explicit non-goals (still)

- Storing checkpoint bytes in Postgres or HTTP
- Advertising READY without provider evidence
