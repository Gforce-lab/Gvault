# gvault

[![ci](https://github.com/Gforce-lab/gvault/actions/workflows/ci.yml/badge.svg)](https://github.com/Gforce-lab/gvault/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Version](https://img.shields.io/badge/version-0.1.0--beta.1-orange.svg)](VERSION)

**gvault** is a Kubernetes-native registry and lifecycle control plane for reusable GPU execution snapshots.

It sits **above** checkpoint engines (starting with [NVIDIA Snapshot](https://github.com/ai-dynamo/snapshot)) and owns the platform problems those engines do not: discovery, versioning, compatibility preflight, async operations, retention safety, audit, and controlled restore orchestration.

> **Beta (`0.1.0-beta.1`)** — default local path uses **FakeProvider**. `READY` / `SUCCEEDED` are **simulated** so you can demo without a GPU. They are not physical CUDA checkpoints.

**Repo:** https://github.com/Gforce-lab/gvault

---

## Why gvault exists

LLM / inference Pods often spend a long time before they can serve traffic (weights, CUDA init, kernel warmup). Checkpoint systems can capture that warmed state — but platform teams still need:

- which snapshot matches this model, image digest, driver, and GPU product?
- is node B eligible *before* creating a restore Pod?
- who requested capture/restore, and what failed?
- how do we retain, promote, or safely delete versions?

**gvault is that control plane.** The provider owns checkpoint mechanics; gvault owns inventory, policy, compatibility, orchestration, and observability.

| Layer | Owns | Does not own |
|---|---|---|
| **gvault** | Catalog, compatibility preflight, async ops, retention/hold, audit, restore intent | CUDA serialization, CRIU, privileged node agents |
| **NVIDIA Snapshot (v0.3.0 pin)** | `PodSnapshot` / restore-pod contract, RWX artifact bytes | Global naming, multi-provider policy, product UX |

---

## Quick start

```bash
git clone https://github.com/Gforce-lab/gvault.git
cd gvault
cp .env.example .env
make docker-up          # Postgres on localhost:5433
make test && make build
make run-api            # API + UI → http://localhost:8080
```

Walk the flow in the UI: **Capture → poll operation → compatibility → restore**

```bash
./bin/gvaultctl overview
./bin/gvaultctl snapshot list
./bin/gvaultctl snapshot create llama8b-demo dev-gpu-cluster inference llama-vllm-0 main
```

More: [examples/fake-provider/README.md](examples/fake-provider/README.md)

---

## Architecture

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

## Status (0.1.0-beta.1)

| Area | Status |
|---|---|
| FakeProvider end-to-end API + UI | Done |
| Compatibility engine (fail-closed) | Done |
| Vault CRDs + local reconciler demo | Done |
| NVIDIA v0.3.0 pin + dry-run contracts | Done (live kube path gated) |
| Real GPU A→B restore | Needs cluster; tests SKIPPED without hardware |

See [docs/implementation-status.md](docs/implementation-status.md), [CHANGELOG.md](CHANGELOG.md), and [docs/release-notes-0.1.0-beta.md](docs/release-notes-0.1.0-beta.md).

---

## CI/CD & Dependabot

| Workflow | Trigger | Purpose |
|---|---|---|
| `ci.yml` | push / PR | tidy, vet, race tests, Postgres integration, binaries, Docker build |
| `release.yml` | tag `v*` | release artifacts + GHCR image |
| `dependabot-auto-merge.yml` | Dependabot PRs | enable auto-merge (squash) after CI is green |
| `dependabot.yml` | weekly | Go modules + GitHub Actions updates |

Details: [docs/ci-cd.md](docs/ci-cd.md)

Dependabot PRs are labeled `dependencies` and set to **auto-merge** once required checks pass. The repository must have **Allow auto-merge** enabled (Settings → General → Pull Requests).

---

## Docs

- [Architecture](docs/architecture.md)
- [Compatibility + NVIDIA pin](docs/compatibility.md)
- [GPU setup](docs/gpu-setup.md)
- [CI/CD](docs/ci-cd.md)
- [Engineering plan](docs/GPU_Snapshot_Vault_Project_Plan.md)

---

## License

Apache-2.0 — see [LICENSE](LICENSE).
