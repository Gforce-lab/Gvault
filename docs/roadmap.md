# Roadmap

## Shipped in 0.1.0-beta.1

Milestones 1–5 on the FakeProvider / dry-run path (see [implementation-status.md](implementation-status.md)).

## After beta

1. Wire live Kubernetes client for NVIDIA Snapshot v0.3.0 on a real GPU cluster
2. Replace in-memory CRD store with controller-runtime manager + leader election
3. OIDC auth and namespace RBAC
4. Measured GPU A → GPU B restore + inference probe (promote out of SKIPPED)
5. Verified S3/export only after upstream pin supports it

Do not advertise real GPU restore until hardware acceptance criteria pass.
