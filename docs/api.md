# HTTP API

Base path: `/api/v1`. Full contract: [openapi/openapi.yaml](../openapi/openapi.yaml).  
End-to-end process map + CLI: [README.md](../README.md#processes-end-to-end).

## Auth

| Mode | Config |
|---|---|
| Dev bypass | `DEV_AUTH_BYPASS=true` (demo `.env.example`) |
| API key | `DEV_AUTH_BYPASS=false` + `API_KEY`; header `X-API-Key` |
| OIDC Bearer | `OIDC_ISSUER` + `OIDC_AUDIENCE` + `OIDC_CLIENT_SECRET`; header `Authorization: Bearer` |
| Namespace allowlist | `ALLOWED_NAMESPACES=ns1,ns2` (empty = allow all) |

Public without auth: `/healthz`, `/readyz`, `/api/v1/version`, `/`, `/assets/*`.

## Async semantics

Mutations return `202` only after the operation row is committed. Poll `GET /api/v1/operations/{id}` or use `gvaultctl operation wait`.

| Entity | States |
|---|---|
| Snapshot | `PENDING → CAPTURING → READY` (or `FAILED` / `DELETING` / `DELETED`) |
| Restore / delete op | `PENDING → RUNNING → SUCCEEDED` (or `FAILED`) |

All FakeProvider outcomes are **simulated**.

## Endpoint groups

| Group | Paths |
|---|---|
| Health | `/healthz`, `/readyz` |
| Meta | `/api/v1/version`, `/overview`, `/metrics` |
| Snapshots | `/api/v1/snapshots`, `/{id}`, `/compatibility`, `/restore`, `/tags`, `/promote`, `/hold` |
| Ops / audit | `/api/v1/operations`, `/operations/{id}`, `/audits` |
| Providers | `/api/v1/providers`, `/providers/nvidia/dry-run` |
| Export | `/api/v1/export/s3` (fail-closed) |
| CRD bridge | `/api/v1/crds/vaultsnapshots…`, `/crds/vaultrestores` |

## CLI

```bash
make build
./bin/gvaultctl help
```

See README **CLI (`gvaultctl`)** for the full command matrix and env vars.
