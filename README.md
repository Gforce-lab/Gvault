# Gvault

[![ci](https://github.com/Gforce-lab/Gvault/actions/workflows/ci.yml/badge.svg)](https://github.com/Gforce-lab/Gvault/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Version](https://img.shields.io/badge/version-0.1.0--beta.1-orange.svg)](VERSION)

**Gvault** is a Kubernetes-native registry and lifecycle control plane for reusable GPU execution snapshots.

It sits **above** checkpoint engines (starting with [NVIDIA Snapshot](https://github.com/ai-dynamo/snapshot)) and owns the platform problems those engines do not: discovery, versioning, compatibility preflight, async operations, retention safety, audit, and controlled restore orchestration.

> **Beta (`0.1.0-beta.1`)** — default local path uses **FakeProvider**. `READY` / `SUCCEEDED` are **simulated** so you can demo without a GPU. They are not physical CUDA checkpoints.

**Repo:** https://github.com/Gforce-lab/Gvault

---

## Table of contents

1. [Why Gvault exists](#why-gvault-exists)
2. [Architecture](#architecture)
3. [Prerequisites](#prerequisites)
4. [Install & run (every step)](#install--run-every-step)
5. [Processes (end-to-end)](#processes-end-to-end)
6. [CLI (`gvaultctl`)](#cli-gvaultctl)
7. [HTTP API map](#http-api-map)
8. [Configuration](#configuration)
9. [Kubernetes deploy](#kubernetes-deploy)
10. [CI/CD & Dependabot](#cicd--dependabot)
11. [Status](#status-010-beta1)
12. [Contributing](#contributing)
13. [Docs](#docs)
14. [License](#license)

---

## Why Gvault exists

LLM / inference Pods often spend a long time before they can serve traffic (weights, CUDA init, kernel warmup). Checkpoint systems can capture that warmed state — but platform teams still need:

- which snapshot matches this model, image digest, driver, and GPU product?
- is node B eligible *before* creating a restore Pod?
- who requested capture/restore, and what failed?
- how do we retain, promote, or safely delete versions?

**Gvault is that control plane.** The provider owns checkpoint mechanics; Gvault owns inventory, policy, compatibility, orchestration, and observability.

| Layer | Owns | Does not own |
|---|---|---|
| **Gvault** | Catalog, compatibility preflight, async ops, retention/hold, audit, restore intent | CUDA serialization, CRIU, privileged node agents |
| **NVIDIA Snapshot (v0.3.0 pin)** | `PodSnapshot` / restore-pod contract, RWX artifact bytes | Global naming, multi-provider policy, product UX |

---

## Architecture

```text
UI / gvaultctl / curl
        ↓
   gvault-api (Gin)  — auth (API key / OIDC) + namespace allowlist
        ↓
 SnapshotService  →  PostgreSQL catalog (metadata + operations + audit)
        ↓
 CheckpointProvider
   ├── fake            (default, simulated, CI-safe)
   └── nvidia-snapshot (pinned v0.3.0; live kube client; fail-closed without cluster)

Optional:
  gvault-controller  — MemoryStore manager + leader-election-shaped reconcile (GPU-free demo)
  CRD HTTP bridge    — /api/v1/crds/... for VaultSnapshot / VaultRestore
```

Checkpoint **bytes** never pass through the HTTP API. Gvault stores opaque provider references only.

**Binaries**

| Binary | Role |
|---|---|
| `gvault-api` | HTTP API, UI, metrics, Fake/NVIDIA providers |
| `gvault-controller` | Local CRD reconcile demo / manager loop |
| `gvaultctl` | CLI client for every major API process |

---

## Prerequisites

| Requirement | Notes |
|---|---|
| Go **1.25+** | `go version` |
| Docker | for local Postgres (`make docker-up`) |
| `make` | optional but used throughout |
| kubectl (optional) | Helm / kustomize deploy |
| GPU cluster (optional) | only for live NVIDIA path; not required for FakeProvider |

---

## Install & run (every step)

### 1. Clone

```bash
git clone https://github.com/Gforce-lab/Gvault.git
cd Gvault
```

### 2. Configure environment

```bash
cp .env.example .env
# Edit if needed. Demo defaults:
#   PROVIDER=fake
#   DEV_AUTH_BYPASS=true
#   DATABASE_URL=postgres://vault:vault@localhost:5433/gvault?sslmode=disable
```

### 3. Start Postgres

Host port **5433** maps to container `5432` (ADR-006).

```bash
make docker-up
# stop later: make docker-down
```

### 4. Test & build

```bash
make fmt
make test
make test-integration   # needs Postgres
make build              # writes bin/gvault-api, bin/gvault-controller, bin/gvaultctl
```

### 5. Run the API (+ UI)

```bash
make run-api
# → http://localhost:8080        UI flow dashboard
# → http://localhost:8080/api/v1
# → http://localhost:8080/metrics
```

### 6. (Optional) Run controller demo

```bash
make run-controller
# in-memory FakeProvider reconcile — not a real GPU path
```

---

## Processes (end-to-end)

Each process below is available via **UI**, **`gvaultctl`**, or **curl**. FakeProvider results are **simulated**.

### Process A — Health & readiness

1. Liveness: process is up.
2. Readiness: DB reachable **and** active provider prerequisites pass (Fake always; NVIDIA needs live kube).

```bash
./bin/gvaultctl health
./bin/gvaultctl ready
curl -s localhost:8080/healthz
curl -s localhost:8080/readyz
```

### Process B — Version & overview

```bash
./bin/gvaultctl version
./bin/gvaultctl version --local    # CLI binary version only
./bin/gvaultctl overview
curl -s localhost:8080/api/v1/version
curl -s localhost:8080/api/v1/overview
```

### Process C — Capture (async)

1. Client submits capture intent → API returns **202** with `snapshotId` + `operationId`.
2. Operation row is durable in Postgres **before** 202.
3. Background poller drives provider: `PENDING → CAPTURING → READY|FAILED`.
4. Catalog updates idempotently from provider status (never mark READY only because HTTP was accepted).

```bash
./bin/gvaultctl snapshot create llama8b-demo dev-gpu-cluster inference llama-vllm-0 main
# note snapshotId + operationId from JSON

./bin/gvaultctl operation wait <operationId>
# or
./bin/gvaultctl snapshot wait <snapshotId>
```

curl equivalent:

```bash
curl -s -X POST localhost:8080/api/v1/snapshots \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: capture-demo-1' \
  -d '{
    "name": "llama8b-demo",
    "clusterId": "dev-gpu-cluster",
    "namespace": "inference",
    "source": {"podName": "llama-vllm-0", "containerName": "main"},
    "provider": "fake"
  }'
```

### Process D — List / get snapshots

```bash
./bin/gvaultctl snapshot list
GVAULT_NAMESPACE=inference ./bin/gvaultctl snapshot list
./bin/gvaultctl snapshot get <snapshotId>
```

### Process E — Compatibility preflight (fail-closed)

Evaluates target GPU product, driver, arch, image digest, and cluster/namespace rules **before** restore.

```bash
# Defaults match FakeProvider discovered metadata unless you override env:
# GVAULT_GPU / GVAULT_DRIVER / GVAULT_ARCH / GVAULT_DIGEST
./bin/gvaultctl snapshot check <snapshotId> dev-gpu-cluster
./bin/gvaultctl snapshot check <snapshotId> dev-gpu-cluster fake-node-b
```

### Process F — Restore (async) + optional probe

1. Compatibility must pass (or Fake demo fills matching defaults).
2. Restore operation: `PENDING → RUNNING → SUCCEEDED|FAILED`.
3. Optional `probeEndpoint` / `GVAULT_PROBE_ENDPOINT` runs post-restore HTTP health check.
4. FakeProvider always exercises `FakeProber` internally.

```bash
./bin/gvaultctl snapshot restore <snapshotId> dev-gpu-cluster inference llama-restored-1 fake-node-b
./bin/gvaultctl operation wait <operationId>

GVAULT_PROBE_ENDPOINT=http://127.0.0.1:9/health \
  ./bin/gvaultctl snapshot restore <snapshotId> dev-gpu-cluster inference llama-restored-2
```

### Process G — Tags, promote, retention hold

```bash
./bin/gvaultctl snapshot tags <snapshotId>
./bin/gvaultctl snapshot tags <snapshotId> set env=staging model=llama-8b
./bin/gvaultctl snapshot promote <snapshotId> v1.2.0
./bin/gvaultctl snapshot hold <snapshotId> true    # blocks delete
./bin/gvaultctl snapshot hold <snapshotId> false
```

### Process H — Guarded delete

Refuses when retention hold is set or an active restore is running. Submits async DELETE operation.

```bash
./bin/gvaultctl snapshot delete <snapshotId>
./bin/gvaultctl operation wait <operationId>
```

### Process I — Operations & audits

```bash
./bin/gvaultctl operation list
./bin/gvaultctl operation get <operationId>
./bin/gvaultctl wait <operationId>           # alias of operation wait
./bin/gvaultctl audits
./bin/gvaultctl audits 100
```

### Process J — Providers & NVIDIA dry-run

Lists active provider. Dry-run returns pinned v0.3.0 PodSnapshot / restore-pod manifests **without** touching a cluster.

```bash
./bin/gvaultctl providers
./bin/gvaultctl providers nvidia-dry-run inference snap-demo llama-vllm-0 main uid-optional
```

### Process K — S3 / export (fail-closed)

Endpoint exists for roadmap completeness; always refuses until the pinned NVIDIA release verifies export (ADR-011).

```bash
./bin/gvaultctl export s3 <snapshotId> my-bucket path/to/object
# → PROVIDER_ERROR (expected)
```

### Process L — CRD bridge (GPU-free)

In-memory VaultSnapshot reconcile via FakeProvider (not controller-runtime/envtest yet).

```bash
./bin/gvaultctl crd create demo inference llama-vllm-0 main
./bin/gvaultctl crd list
./bin/gvaultctl crd get inference demo
./bin/gvaultctl crd reconcile inference demo
```

### Process M — Auth modes

| Mode | How |
|---|---|
| Dev bypass | `DEV_AUTH_BYPASS=true` (demo `.env`) |
| API key | `DEV_AUTH_BYPASS=false` + `API_KEY=...`; CLI: `GVAULT_API_KEY=...` |
| OIDC Bearer | set `OIDC_ISSUER` / `OIDC_AUDIENCE` / `OIDC_CLIENT_SECRET`; CLI: `GVAULT_BEARER_TOKEN=...` |
| Namespace allowlist | `ALLOWED_NAMESPACES=inference,platform` |

```bash
export GVAULT_API_KEY=dev-local-key
./bin/gvaultctl overview
```

### Process N — UI flow

Open http://localhost:8080 and walk **Capture → poll operation → compatibility → restore**. Banner states FakeProvider is simulated.

### Process O — Live NVIDIA path (optional hardware)

1. Install NVIDIA Snapshot **v0.3.0** + GPU Operator + RWX PVC (see [docs/gpu-setup.md](docs/gpu-setup.md)).
2. Set `PROVIDER=nvidia-snapshot`, `ENABLE_NVIDIA_PROVIDER=true`, and kube access (`KUBECONFIG` or in-cluster).
3. Without a live client, prerequisites stay **SKIPPED** — never advertised as PASSED.
4. Real GPU e2e: `make test-gpu` (SKIPPED without hardware).

---

## CLI (`gvaultctl`)

Build with `make build` (or `make gvaultctl`).

```bash
./bin/gvaultctl help
```

### Command reference

| Command | Process |
|---|---|
| `version [--local]` | B — API or CLI version |
| `health` / `ready` | A — probes |
| `overview` | B — catalog summary |
| `providers` | J |
| `providers nvidia-dry-run …` | J |
| `audits [limit]` | I |
| `export s3 …` | K (fail-closed) |
| `wait <operationId>` | I |
| `snapshot list\|get\|create\|wait\|check\|restore\|tags\|promote\|hold\|delete` | C–H |
| `operation list\|get\|wait` | I |
| `crd list\|get\|create\|reconcile` | L |

### Environment variables (CLI)

| Variable | Purpose |
|---|---|
| `GVAULT_API` | Base URL (default `http://localhost:8080`) |
| `GVAULT_API_KEY` | `X-API-Key` |
| `GVAULT_BEARER_TOKEN` | `Authorization: Bearer` |
| `GVAULT_PROVIDER` | Provider on create/crd (default `fake`) |
| `GVAULT_NAMESPACE` | Filter `snapshot list` |
| `GVAULT_CLUSTER` | Cluster for CRD create |
| `GVAULT_GPU` / `GVAULT_DRIVER` / `GVAULT_ARCH` / `GVAULT_DIGEST` | Compatibility / restore facts |
| `GVAULT_PROBE_ENDPOINT` | Post-restore probe URL |
| `GVAULT_IDEMPOTENCY_KEY` | Override mutation idempotency key |
| `GVAULT_WAIT_TIMEOUT` / `GVAULT_WAIT_POLL` | Wait loops (default `30s` / `200ms`) |

### Full FakeProvider demo script

```bash
make docker-up && make build && make run-api &
sleep 2

./bin/gvaultctl health
./bin/gvaultctl overview

OUT=$(./bin/gvaultctl snapshot create llama8b-demo dev-gpu-cluster inference llama-vllm-0 main)
echo "$OUT"
# Extract IDs from JSON, then:
# ./bin/gvaultctl operation wait "$OP_ID"
# ./bin/gvaultctl snapshot check "$SNAP_ID" dev-gpu-cluster
# ./bin/gvaultctl snapshot restore "$SNAP_ID" dev-gpu-cluster inference llama-restored-1 fake-node-b
# ./bin/gvaultctl snapshot tags "$SNAP_ID" set demo=true
# ./bin/gvaultctl snapshot promote "$SNAP_ID" v0.1.0-demo
# ./bin/gvaultctl audits
```

Also: [examples/fake-provider/README.md](examples/fake-provider/README.md)

---

## HTTP API map

Base path `/api/v1`. Contract: [openapi/openapi.yaml](openapi/openapi.yaml) · notes: [docs/api.md](docs/api.md)

| Method | Path | Process |
|---|---|---|
| GET | `/healthz` | A |
| GET | `/readyz` | A |
| GET | `/api/v1/version` | B |
| GET | `/api/v1/overview` | B |
| GET | `/metrics` | Observability |
| POST/GET | `/api/v1/snapshots` | C / D |
| GET/DELETE | `/api/v1/snapshots/{id}` | D / H |
| POST | `/api/v1/snapshots/{id}/compatibility` | E |
| POST | `/api/v1/snapshots/{id}/restore` | F |
| GET/PUT | `/api/v1/snapshots/{id}/tags` | G |
| POST | `/api/v1/snapshots/{id}/promote` | G |
| POST | `/api/v1/snapshots/{id}/hold` | G |
| GET | `/api/v1/operations` · `/operations/{id}` | I |
| GET | `/api/v1/audits` | I |
| GET | `/api/v1/providers` | J |
| POST | `/api/v1/providers/nvidia/dry-run` | J |
| POST | `/api/v1/export/s3` | K |
| * | `/api/v1/crds/...` | L |

Mutations return **202** only after the operation row is committed. Poll operations until terminal status.

---

## Configuration

See [.env.example](.env.example).

| Variable | Default | Meaning |
|---|---|---|
| `HTTP_ADDR` | `:8080` | Listen address |
| `DATABASE_URL` | `…localhost:5433/gvault…` | Postgres |
| `PROVIDER` | `fake` | `fake` \| `nvidia-snapshot` |
| `ENABLE_NVIDIA_PROVIDER` | `false` | Required for NVIDIA provider |
| `DEV_AUTH_BYPASS` | `false` in prod | Skip auth (demo only) |
| `API_KEY` | — | Required when bypass off and OIDC unset |
| `OIDC_ISSUER` / `OIDC_AUDIENCE` / `OIDC_CLIENT_SECRET` | — | Optional Bearer JWT (HS256 staging) |
| `ALLOWED_NAMESPACES` | empty | Comma allowlist; empty = all |
| `ENABLE_S3_EXPORT` | `false` | Still fail-closed until pin verifies |
| `LEADER_ELECTION` | `false` | Controller manager shape |
| `PRODUCT_NAME` | `Gvault` | Product label |
| `UI_DIR` | `web` | Static UI |

---

## Kubernetes deploy

```bash
# CRDs
make install-crds

# Kustomize (FakeProvider overlay)
make kustomize-build
# NVIDIA overlay (still needs live cluster + flags)
make kustomize-build-nvidia

# Helm stub
helm install gvault ./config/helm/gvault
```

RBAC: [config/rbac/role.yaml](config/rbac/role.yaml)

---

## CI/CD & Dependabot

| Workflow | Trigger | Purpose |
|---|---|---|
| `ci.yml` | push / PR | tidy, vet, race tests, Postgres integration, binaries, Docker build |
| `release.yml` | tag `v*` | release artifacts + GHCR image |
| `dependabot-auto-merge.yml` | Dependabot PRs | enable auto-merge (squash) after CI green |
| `dependabot.yml` | weekly | Go modules + GitHub Actions updates |

Details: [docs/ci-cd.md](docs/ci-cd.md)

Enable **Allow auto-merge** under repo Settings → General → Pull Requests.

Open PRs from feature branches using [.github/PULL_REQUEST_TEMPLATE.md](.github/PULL_REQUEST_TEMPLATE.md) (see [CONTRIBUTING.md](CONTRIBUTING.md)).

---

## Status (0.1.0-beta.1)

| Area | Status |
|---|---|
| FakeProvider end-to-end API + UI + CLI | Done |
| Compatibility engine (fail-closed) | Done |
| Vault CRDs + local reconciler / manager | Done |
| NVIDIA v0.3.0 pin + dry-run + live kube wiring | Done (cluster optional; fail-closed) |
| OIDC + namespace allowlist | Done (HS256 staging; JWKS later) |
| Restore inference probe | Done (FakeProber / HTTPProber) |
| S3/export | Fail-closed stub |
| Real GPU A→B restore | Needs cluster; `make test-gpu` SKIPPED without hardware |

See [docs/implementation-status.md](docs/implementation-status.md), [CHANGELOG.md](CHANGELOG.md), [docs/roadmap.md](docs/roadmap.md).

---

## Contributing

1. Read the [engineering plan](docs/GPU_Snapshot_Vault_Project_Plan.md) and [decisions](docs/decisions.md).
2. Prefer small, testable, additive changes.
3. Never claim real GPU restore success without hardware evidence.
4. Run `make fmt test build` before opening a PR.
5. Use the [PR template](.github/PULL_REQUEST_TEMPLATE.md).

---

## Docs

- [Architecture](docs/architecture.md)
- [API](docs/api.md)
- [Compatibility + NVIDIA pin](docs/compatibility.md)
- [GPU setup](docs/gpu-setup.md)
- [Security](docs/security.md)
- [CI/CD](docs/ci-cd.md)
- [Engineering plan](docs/GPU_Snapshot_Vault_Project_Plan.md)
- [Decisions (ADRs)](docs/decisions.md)

---

## License

Apache-2.0 — see [LICENSE](LICENSE).
