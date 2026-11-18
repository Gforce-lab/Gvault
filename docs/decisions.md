# Architectural and delivery decisions — Gvault

---

## ADR-000 — Product name Gvault

- **Date:** 2026-11-17
- **Status:** Accepted
- **Decision:** Project name is **Gvault**. Go module is `github.com/gvault/gvault`. Binaries are `gvault-api`, `gvault-controller`, `gvaultctl`.
- **Consequences:** README, UI, and docs use **Gvault**. Go module/imports stay `github.com/gvault/gvault`; binaries stay `gvault-api` / `gvaultctl`; CRD group `vault.gvault.dev`; DB name `gvault`.

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
