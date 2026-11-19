# Architectural and delivery decisions — Gvault

---

## ADR-000 — Product / repo name Gvault

- **Date:** 2026-11-17 (updated 2026-11-18)
- **Status:** Accepted
- **Decision:** Product and GitHub repo are **Gvault** (`Gforce-lab/Gvault`). Go module remains `github.com/gvault/gvault`. Binaries remain `gvault-api`, `gvault-controller`, `gvaultctl`.
- **Consequences:** README, UI, API `product` field, and docs say **Gvault**. Technical identifiers (module, binaries, DB, CRD DNS) stay lowercase `gvault`.

---

## ADR-001 — Deliver milestones additively on M1

- **Date:** 2026-11-17
- **Status:** Accepted
- **Decision:** Keep Milestone 1 FakeProvider HTTP API intact. Add M2–M5 and UI as additive packages/endpoints.
- **Consequences:** Existing `/api/v1/snapshots` contracts remain; new routes for overview, tags, hold, operations list, nvidia dry-run.

---

## ADR-002 — FakeProvider before live NVIDIA

- **Date:** 2026-11-16 (updated 2026-11-18)
- **Status:** Superseded by ADR-012
- **Decision (historical):** Early milestones used a FakeProvider for GPU-free demos.
- **Superseded by:** NVIDIA-only default; FakeProvider package removed.

---

## ADR-003 — Pin NVIDIA Snapshot v0.3.0

- **Date:** 2026-11-17
- **Status:** Accepted
- **Decision:** Pin `ai-dynamo/snapshot` **v0.3.0** for CRD/podcontract shapes.
- **Consequences:** Recorded in `docs/compatibility.md`; dry-run builders encode that contract.

---

## ADR-004 — In-memory reconciler for M2 GPU-free path

- **Date:** 2026-11-17
- **Status:** Accepted
- **Decision:** Ship CRD YAML + Go types + an in-process reconciler store for idempotent FakeProvider tests without requiring envtest/cluster in CI.
- **Consequences:** `cmd/gvault-controller` demos local reconcile; production controller-runtime manager can replace the store later.

---

## ADR-005 — UI served by gvault-api

- **Date:** 2026-11-17
- **Status:** Accepted
- **Decision:** Static flow dashboard under `web/` served at `/` to teach capture → poll → compat → restore.
- **Consequences:** No React/SPA build step in MVP; clear simulated banner.

---

## ADR-006 — Local Postgres host port 5433

- **Date:** 2026-11-16
- **Status:** Accepted
- **Decision:** Compose maps `5433:5432` because host `5432` was occupied.
- **Consequences:** Defaults and `.env.example` use `localhost:5433`.

---

## ADR-007 — Retention hold blocks delete

- **Date:** 2026-11-17
- **Status:** Accepted
- **Decision:** `DELETE /snapshots/{id}` uses guarded delete (hold + active restore checks).
- **Consequences:** Safer retirement semantics for Milestone 5.

---

## ADR-008 — First beta is 0.1.0-beta.1

- **Date:** 2026-11-17
- **Status:** Accepted
- **Decision:** Ship all plan milestones on the FakeProvider + NVIDIA dry-run surface as **beta**, with explicit limitations (no live GPU READY claims, in-memory CRD bridge, API-key auth).
- **Consequences:** Version embedded in binaries; CHANGELOG + release notes required; post-beta work focuses on live kube client and measured GPU restore.

---

## ADR-009 — Live kube behind NVIDIA flag only

- **Date:** 2026-11-18
- **Status:** Accepted
- **Decision:** Wire `internal/kube` into `nvidia-snapshot` only. NVIDIA without a live client stays fail-closed (prerequisites SKIPPED, not PASSED).
- **Consequences:** CI uses `kube.MemoryAPI` with the NVIDIA adapter; no separate fake provider.

---

## ADR-010 — OIDC + namespace allowlist additive to API key

- **Date:** 2026-11-18
- **Status:** Accepted
- **Decision:** Keep API-key auth. When `OIDC_ISSUER`/`OIDC_AUDIENCE`/`OIDC_CLIENT_SECRET` are set, accept Bearer HS256 JWTs. `ALLOWED_NAMESPACES` gates capture/restore when non-empty.
- **Consequences:** Staging can use OIDC without breaking existing API-key clients; production should replace HS256 with JWKS later.

---

## ADR-011 — S3/export fail-closed until pin verifies

- **Date:** 2026-11-18
- **Status:** Accepted
- **Decision:** Expose `/api/v1/export/s3` but always refuse until the pinned NVIDIA Snapshot release documents a verified export path.
- **Consequences:** No accidental READY export claims; roadmap item stays explicit.

---

## ADR-012 — NVIDIA-only provider (no Fake product path)

- **Date:** 2026-11-18 (updated)
- **Status:** Accepted
- **Decision:** Phase 1 supports **`nvidia-snapshot` only**. The former FakeProvider package is removed. CI uses `nvidia.NewWithMemoryCluster()` (in-memory kube) so tests still exercise the NVIDIA adapter without physical GPUs. **AMD is out of scope** until Phase 1 exit criteria in `docs/scope.md`.
- **Consequences:** `PROVIDER=fake` is rejected by config. Catalog storage rules live in `docs/storage.md`.

---

## ADR-013 — Catalog vs artifact bytes

- **Date:** 2026-11-18
- **Status:** Accepted
- **Decision:** PostgreSQL stores searchable metadata, operations, audit, and opaque artifact locators. Checkpoint bytes remain on NVIDIA Snapshot / RWX. The API never accepts or returns raw checkpoint payloads.
- **Consequences:** Clear operator mental model; safer multi-tenant boundaries; AMD later adds a new provider package without changing this split.

---

## ADR-014 — Restore completion and optional inference verification

- **Date:** 2026-11-18
- **Status:** Proposed
- **Decision:** NVIDIA restore completion requires the agent-owned `nvidia.com/Restored=True` condition and a Running/Ready Pod with the original UID. The HTTP API/CLI may additionally request an `inferenceProbe` using a fixed, bounded OpenAI completions request to that Pod's observed IP. Legacy health checks remain separate and cannot be described as inference evidence.
- **Consequences:** Operations remain RUNNING through application verification. Total restore execution and inference retries have deadlines. Diagnostics and audits omit generated text and response bodies. No upstream Snapshot module dependency is added: the v0.3.0 condition vocabulary is mapped explicitly and unknown conditions remain pending. The pinned vLLM guide's `/generate` protocol is supported separately from OpenAI completions and cannot attest model identity. NVIDIA callers supply a complete `restorePodSpec`; the adapter preserves workload settings and injects the minimum control protocol, rejecting conflicts and mismatched images/GPU counts. Destination compatibility facts are explicit, not copied from the source. Operation provider/simulation markers come from stored snapshot provenance. Authentication to inference endpoints and CRD inference configuration remain separate work. Unit/integration fixtures are simulated; no real GPU success is claimed.
