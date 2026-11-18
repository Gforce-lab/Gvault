# Gvault

[![ci](https://github.com/Gforce-lab/Gvault/actions/workflows/ci.yml/badge.svg)](https://github.com/Gforce-lab/Gvault/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Version](https://img.shields.io/badge/version-0.1.0--beta.1-orange.svg)](VERSION)

**Gvault** is a Kubernetes-native registry and lifecycle control plane for **NVIDIA GPU** execution snapshots.

It sits **above** [NVIDIA Snapshot](https://github.com/ai-dynamo/snapshot) (pinned **v0.3.0**) and owns discovery, versioning, compatibility preflight, async operations, retention safety, audit, and controlled restore orchestration.

> **Phase 1 stop-line:** NVIDIA GPU only. **AMD and other vendors are out of scope** until Phase 1 exit criteria pass. See [docs/scope.md](docs/scope.md).

**Repo:** https://github.com/Gforce-lab/Gvault

---

## Table of contents

1. [Why Gvault exists](#why-gvault-exists)
2. [What we store](#what-we-store)
3. [Phase stop-limits](#phase-stop-limits)
4. [Architecture](#architecture)
5. [Prerequisites](#prerequisites)
6. [Install & run](#install--run)
7. [Processes (end-to-end)](#processes-end-to-end)
8. [CLI (`gvaultctl`)](#cli-gvaultctl)
9. [HTTP API map](#http-api-map)
10. [Configuration](#configuration)
11. [Kubernetes deploy](#kubernetes-deploy)
12. [CI/CD](#cicd--dependabot)
13. [Contributing](#contributing)
14. [Docs](#docs)
15. [License](#license)

---

## Why Gvault exists

Inference Pods spend a long time before they serve traffic. Checkpoint systems capture warmed GPU state — platform teams still need:

- which snapshot matches this image digest, driver, and **NVIDIA** GPU product?
- is node B eligible *before* creating a restore Pod?
- who requested capture/restore, and what failed?
- how do we retain, promote, or safely delete versions?

| Layer | Owns | Does not own |
|---|---|---|
| **Gvault** | Catalog, compatibility, async ops, retention/hold, audit, restore intent | CUDA serialization, CRIU, node agents |
| **NVIDIA Snapshot v0.3.0** | `PodSnapshot` / restore-pod contract, RWX artifact bytes | Global naming, multi-provider policy, product UX |

---

## What we store

Full detail: **[docs/storage.md](docs/storage.md)**

| Where | What | Not stored |
|---|---|---|
| **PostgreSQL** | Snapshot metadata, hardware/runtime facts, opaque provider refs, operations, audit, tags/hold/version | Checkpoint / VRAM / process bytes |
| **NVIDIA Snapshot + RWX PVC** | Raw GPU checkpoint artifacts | Product policy / catalog UX |

Catalog artifact pointer example:

```json
{
  "backend": "nvidia-snapshot",
  "storeId": "v0.3.0",
  "handle": "<podsnapshot-or-content-name>"
}
```

**Rule:** never mark catalog `READY` only because HTTP returned 202. Provider status is authoritative for progress; Postgres is a searchable projection.

---

## Phase stop-limits

Full detail: **[docs/scope.md](docs/scope.md)**

### In scope now (Phase 1)

- NVIDIA Snapshot **v0.3.0** provider + live kube client
- Fail-closed without cluster (SKIPPED ≠ PASSED)
- Compatibility preflight, restore + optional probe
- Catalog / ops / audit / tags / hold / promote / guarded delete
- CLI, UI, Helm/kustomize stubs

### Stop — do not start yet

| Stop | Resume when |
|---|---|
| **AMD GPU provider** | Phase 1 exit criteria met on real NVIDIA hardware |
| S3/export advertised READY | Upstream pin documents verified export |
| Checkpoint bytes in API/Postgres | Never — by design |
| Multi-vendor marketplace | After ≥1 real provider is proven |

### Phase 1 exit (before AMD)

1. Live NVIDIA capture → catalog READY with provider binding  
2. Compatibility blocks mismatches  
3. Restore A→B + probe measured  
4. Hold/delete verified against provider lifecycle  
5. GPU e2e no longer SKIPPED without evidence  

---

## Architecture

```text
UI / gvaultctl / curl
        ↓
   gvault-api  (Gin · API key / OIDC · namespace allowlist)
        ↓
 SnapshotService → PostgreSQL (metadata + ops + audit)
        ↓
 nvidia-snapshot (v0.3.0) → kube → PodSnapshot / restore Pod
                                → RWX PVC (bytes)
```

| Binary | Role |
|---|---|
| `gvault-api` | HTTP API, UI, metrics, NVIDIA provider |
| `gvault-controller` | Reconcile/manager demo |
| `gvaultctl` | CLI for every major process |

---

## Prerequisites

| Requirement | Notes |
|---|---|
| Go 1.25+ | build |
| Docker | local Postgres (`make docker-up`, host **5433**) |
| kubectl + GPU cluster | **required for live READY** |
| NVIDIA Snapshot **v0.3.0** + GPU Operator + RWX | see [docs/gpu-setup.md](docs/gpu-setup.md) |

Without kube, dry-run and catalog APIs work; capture/restore stay fail-closed.

---

## Install & run

```bash
git clone https://github.com/Gforce-lab/Gvault.git
cd Gvault
cp .env.example .env
# PROVIDER=nvidia-snapshot (default)
# ENABLE_NVIDIA_PROVIDER=true
# set KUBECONFIG for live cluster

make docker-up
make test && make build
make run-api          # http://localhost:8080
./bin/gvaultctl health
./bin/gvaultctl providers nvidia-dry-run inference demo llama-vllm-0 main
```

Live path walkthrough: [examples/nvidia/README.md](examples/nvidia/README.md)

---

## Processes (end-to-end)

### A — Health & readiness

```bash
./bin/gvaultctl health    # process up
./bin/gvaultctl ready     # DB + NVIDIA prerequisites (needs live kube)
```

### B — Version & overview

```bash
./bin/gvaultctl version
./bin/gvaultctl overview  # includes phase=1-nvidia-only, storage note
```

### C — Capture (async, NVIDIA)

1. Submit intent → **202** + durable `operationId`  
2. Provider creates `PodSnapshot` on cluster  
3. Poll until catalog `READY|FAILED`  

```bash
./bin/gvaultctl snapshot create llama8b gpu-cluster-a inference llama-vllm-0 main
./bin/gvaultctl operation wait <operationId>
```

### D — List / get

```bash
./bin/gvaultctl snapshot list
./bin/gvaultctl snapshot get <snapshotId>
```

### E — Compatibility preflight

```bash
GVAULT_GPU=NVIDIA-A100-SXM4-80GB \
GVAULT_DRIVER=550.54.15 \
GVAULT_DIGEST=sha256:your-pinned-digest \
  ./bin/gvaultctl snapshot check <snapshotId> gpu-cluster-a
```

### F — Restore + optional probe

NVIDIA restore waits for Snapshot completion and Pod readiness. Add an
`inferenceProbe` to verify generation directly on the restored worker. See
[restore readiness and inference verification](docs/api.md#restore-readiness-and-inference-verification)
for the request body, CLI options, deadlines, and networking requirements.

```bash
GVAULT_RESTORE_POD_SPEC=examples/nvidia/restore-pod-spec.json \
GVAULT_INFERENCE_PROTOCOL=snapshot-vllm \
GVAULT_GPU="$GPU_PRODUCT" GVAULT_DRIVER="$DRIVER_VERSION" \
GVAULT_ARCH="$CPU_ARCH" GVAULT_DIGEST="$IMAGE_DIGEST" \
  ./bin/gvaultctl snapshot restore <snapshotId> gpu-cluster-a inference llama-restored-1 gpu-node-b
GVAULT_WAIT_TIMEOUT=6m ./bin/gvaultctl operation wait <operationId>
```

### G — Tags / promote / hold

```bash
./bin/gvaultctl snapshot tags <id> set model=llama env=prod
./bin/gvaultctl snapshot promote <id> v1.0.0
./bin/gvaultctl snapshot hold <id> true
```

### H — Guarded delete

```bash
./bin/gvaultctl snapshot delete <id>
```

### I — Operations & audits

```bash
./bin/gvaultctl operation list
./bin/gvaultctl audits 50
```

### J — NVIDIA dry-run (no cluster mutate)

```bash
./bin/gvaultctl providers nvidia-dry-run inference snap1 llama-vllm-0 main
```

### K — S3 export

Fail-closed until pin verifies export (`./bin/gvaultctl export s3 …` → error expected).

### L — Auth

| Mode | Config |
|---|---|
| API key | `API_KEY` + `GVAULT_API_KEY` |
| OIDC Bearer | `OIDC_*` + `GVAULT_BEARER_TOKEN` |
| Namespace allowlist | `ALLOWED_NAMESPACES` |

### M — UI

http://localhost:8080 — NVIDIA capture/compat/restore + dry-run button.

---

## CLI (`gvaultctl`)

```bash
make build && ./bin/gvaultctl help
```

| Command group | Purpose |
|---|---|
| `health` `ready` `version` `overview` | Probes / meta |
| `snapshot create\|wait\|check\|restore\|tags\|promote\|hold\|delete` | Lifecycle |
| `operation list\|get\|wait` | Async status |
| `providers` / `nvidia-dry-run` | Provider + manifests |
| `audits` `export s3` `crd …` | Audit / fail-closed export / CRD bridge |

Env: `GVAULT_API`, `GVAULT_API_KEY`, `GVAULT_BEARER_TOKEN`, `GVAULT_PROVIDER` (default `nvidia-snapshot`), hardware overrides, `GVAULT_PROBE_ENDPOINT`, wait timeouts — see `./bin/gvaultctl help`.

---

## HTTP API map

Contract: [openapi/openapi.yaml](openapi/openapi.yaml) · [docs/api.md](docs/api.md)

| Area | Paths |
|---|---|
| Health | `/healthz`, `/readyz` |
| Meta | `/api/v1/version`, `/overview`, `/metrics` |
| Snapshots | `/api/v1/snapshots…` compatibility, restore, tags, promote, hold |
| Ops / audit | `/operations`, `/audits` |
| NVIDIA | `/providers`, `/providers/nvidia/dry-run` |
| Export | `/export/s3` (fail-closed) |

---

## Configuration

[.env.example](.env.example)

| Variable | Phase 1 default | Meaning |
|---|---|---|
| `PROVIDER` | `nvidia-snapshot` | Product provider |
| `ENABLE_NVIDIA_PROVIDER` | `true` | Required for NVIDIA |
| `ALLOW_TEST_PROVIDER` | unset | CI harness only — not for demos/prod |
| `KUBECONFIG` | optional | Live cluster access |
| `ALLOWED_NAMESPACES` | empty | Allowlist when set |
| `ENABLE_S3_EXPORT` | `false` | Still fail-closed |

---

## Kubernetes deploy

```bash
make install-crds
make kustomize-build-nvidia
helm install gvault ./config/helm/gvault
```

---

## CI/CD & Dependabot

| Workflow | Purpose |
|---|---|
| `ci.yml` | vet, race tests, integration, binaries, image |
| `release.yml` | tag `v*` artifacts + GHCR |
| Dependabot | weekly deps + auto-merge when enabled |

---

## Contributing

1. Read [docs/scope.md](docs/scope.md) and [docs/storage.md](docs/storage.md)  
2. Do not add AMD / fake-as-product paths in Phase 1  
3. Never claim GPU READY without hardware evidence  
4. `make fmt test build` + [PR template](.github/PULL_REQUEST_TEMPLATE.md)  

---

## Docs

- [Scope / stop-limits](docs/scope.md)
- [Storage model](docs/storage.md)
- [Architecture](docs/architecture.md)
- [GPU setup](docs/gpu-setup.md)
- [Compatibility](docs/compatibility.md)
- [API](docs/api.md)
- [Security](docs/security.md)
- [Decisions](docs/decisions.md)
- [Engineering plan](docs/GPU_Snapshot_Vault_Project_Plan.md)

---

## License

Apache-2.0 — see [LICENSE](LICENSE).
