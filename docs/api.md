# HTTP API (Milestone 1)

Base path: `/api/v1`. Full contract: [openapi/openapi.yaml](../openapi/openapi.yaml).

## Auth

- Production-like local: set `DEV_AUTH_BYPASS=false` and send `X-API-Key`.
- Demo default in `.env.example`: `DEV_AUTH_BYPASS=true` (localhost only).

## Async semantics

Mutations return `202` only after the operation row is committed. Poll `GET /api/v1/operations/{id}`.

Snapshot statuses: `PENDING → CAPTURING → READY` (or `FAILED`).
Restore operation statuses: `PENDING → RUNNING → SUCCEEDED` (or `FAILED`).

All FakeProvider outcomes are simulated.
