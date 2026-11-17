# gvault

**Version:** `0.1.0-beta.1` · **Channel:** beta  
See [CHANGELOG.md](CHANGELOG.md) and [docs/release-notes-0.1.0-beta.md](docs/release-notes-0.1.0-beta.md).

**gvault** is a Kubernetes-native registry and lifecycle control plane for reusable GPU execution snapshots.

It sits **above** checkpoint engines (starting with NVIDIA Snapshot) and owns the platform problems those engines do not: discovery, versioning, compatibility preflight, async operations, retention safety, audit, and controlled restore orchestration.

> Default local path uses **FakeProvider**. `READY` / `SUCCEEDED` are **simulated** so you can learn and demo the product without a GPU. They are not physical CUDA checkpoints.

---

## Why this product matters

### The expensive problem

Modern LLM / inference Pods often spend a long time before they can serve traffic:

- pulling and loading model weights
- initializing CUDA contexts
- warming kernels / CUDA graphs
- reaching a stable, request-ready state

On GPU clusters, that cold-start cost shows up as:

- slower autoscaling
- wasted GPU idle time while replicas “boot”
- brittle manual runbooks for checkpoint/restore
- hard-to-answer questions: *which snapshot is safe to restore on this node?*

Checkpoint systems (for example NVIDIA Snapshot) can capture and restore an initialized GPU process. That is necessary — but not enough for a platform team.

### The missing control plane

Operators still need answers like:

- Which snapshot matches this model revision, image digest, driver, and GPU product?
- Is destination node B eligible *before* we create a restore Pod?
- Who requested capture/restore, and what failed?
- Can we retain, promote, or safely delete a version without orphaning artifacts?
- How do we measure cold start vs restored start honestly?

**gvault’s job is that control plane.**  
The external provider owns checkpoint mechanics. gvault owns **inventory, policy, compatibility, orchestration, and observability**.

That separation is the product:

| Layer | Owns | Does not own |
|---|---|---|
| **gvault** | Catalog, compatibility preflight, async ops, retention/hold, audit, restore intent | CUDA serialization, CRIU, privileged node agents |
| **NVIDIA Snapshot (pinned v0.3.0)** | `PodSnapshot` / restore-pod contract, RWX artifact bytes | Global naming, multi-provider policy, product registry UX |

### Why it is a strong project for work

This is a good implementation target for platform / MLOps / Kubernetes work because it forces real engineering skills that show up in production systems:

1. **Clear system boundary** — integrate with an upstream, don’t reinvent CUDA checkpointing  
2. **Async APIs done correctly** — durable operations, `202`, idempotency, status polling  
3. **Fail-closed safety** — reject incompatible restores with structured reasons  
4. **Provider interface** — FakeProvider for CI; NVIDIA adapter behind a pin + feature flag  
5. **Kubernetes-native direction** — CRDs (`VaultSnapshot` / `VaultRestore`) + reconciler path  
6. **Honest demos** — simulated path labeled clearly; real GPU path documented and SKIPPED without hardware  

It is **not** a generic CRUD app. It is a credible slice of an inference platform: warm capacity, faster scale-out, and safer restore governance.

### What success looks like (product sense)

A platform engineer can:

1. Capture an initialized workload once  
2. Find the right snapshot by model / runtime / GPU identity  
3. Preflight destination compatibility  
4. Restore on a compatible node  
5. Track status, failures, and retention without touching raw checkpoint files  

Locally, you practice that full loop with FakeProvider + UI. On a real GPU cluster, the same contracts map onto NVIDIA Snapshot.

---

## Quick start (understand the flow in minutes)

```bash
cp .env.example .env
make docker-up          # Postgres on localhost:5433
make test && make build
make run-api            # API + UI
```

Open **http://localhost:8080** and walk:

**Capture → poll operation → compatibility → restore**

CLI:

```bash
./bin/gvaultctl overview
./bin/gvaultctl snapshot list
./bin/gvaultctl snapshot create llama8b-demo dev-gpu-cluster inference llama-vllm-0 main
```

More detail: [examples/fake-provider/README.md](examples/fake-provider/README.md)

---

## Architecture (short)

```text
UI / gvaultctl / curl
        ↓
   gvault-api (Gin)
        ↓
 SnapshotService  →  PostgreSQL catalog (metadata + operations + audit)
        ↓
 CheckpointProvider
   ├── fake            (default, simulated, CI-safe)
   └── nvidia-snapshot (pinned v0.3.0, fail-closed without live cluster)
```

Checkpoint **bytes** never pass through the HTTP API. gvault stores opaque provider references only.

---

## Status

See [docs/implementation-status.md](docs/implementation-status.md) and [docs/decisions.md](docs/decisions.md).

| Area | Status |
|---|---|
| FakeProvider end-to-end API + UI | Implemented |
| Compatibility engine (fail-closed) | Implemented |
| Vault CRDs + local reconciler demo | Implemented |
| NVIDIA v0.3.0 pin + dry-run contracts | Implemented (live kube path gated) |
| Real GPU A→B restore | Requires cluster; tests SKIPPED without hardware |

---

## CI/CD

GitHub Actions runs on every PR/push: unit + race tests, Postgres integration, binary build, and Docker image build (no push). Tag `v*` cuts a release + GHCR image.

Details: [docs/ci-cd.md](docs/ci-cd.md)

## Docs

- [Why / architecture](docs/architecture.md)
- [Compatibility + NVIDIA pin](docs/compatibility.md)
- [GPU setup](docs/gpu-setup.md)
- [CI/CD](docs/ci-cd.md)
- [Full engineering plan](docs/GPU_Snapshot_Vault_Project_Plan.md)

---

## License

Apache-2.0
