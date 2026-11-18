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

- **Date:** 2026-11-16 (updated 2026-11-17)
- **Status:** Accepted
- **Decision:** Default `PROVIDER=fake`. NVIDIA adapter is feature-flagged and fail-closed without a live kube client — never reports physical READY.
- **Consequences:** Local UI and CI stay GPU-free.

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
- **Decision:** Wire `internal/kube` into `nvidia-snapshot` only. Default `PROVIDER=fake` never talks to the cluster. NVIDIA without a live client stays fail-closed (prerequisites SKIPPED, not PASSED).
- **Consequences:** FakeProvider CI/UI regressions are impossible from kube wiring; live path unit-tested with `kube.FakeAPI`.

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
