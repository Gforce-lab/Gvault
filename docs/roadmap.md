# Roadmap

## Shipped in 0.1.0-beta.1

Milestones 1–5 on the FakeProvider / dry-run path (see [implementation-status.md](implementation-status.md)).

## After beta (hardening in progress on `feat/milestones-complete`)

1. [x] Live Kubernetes client wiring for NVIDIA Snapshot v0.3.0 (fail-closed without kube; FakeAPI unit path)
2. [x] Controller manager + leader-election-shaped loop (MemoryStore; full controller-runtime/envtest still later)
3. [x] OIDC Bearer JWT (HS256 staging verifier) + namespace allowlist RBAC
4. [x] Inference/health probe after restore (FakeProber + HTTPProber); real GPU e2e remains SKIPPED without hardware
5. [x] S3/export endpoint fail-closed until upstream pin verifies export
6. [ ] Measured GPU A → GPU B restore on real cluster (promote e2e out of SKIPPED)
7. [ ] Replace MemoryStore with controller-runtime + envtest in CI
8. [ ] Production OIDC via JWKS (replace shared-secret verifier)

Do not advertise real GPU restore until hardware acceptance criteria pass.
