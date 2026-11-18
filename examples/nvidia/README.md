# NVIDIA Snapshot path (Phase 1)

Product path for Gvault. Pin: **NVIDIA Snapshot v0.3.0**.

## Prerequisites

1. GPU cluster with NVIDIA GPU Operator
2. NVIDIA Snapshot **v0.3.0** installed
3. RWX PVC available to checkpoint agents
4. `KUBECONFIG` (or in-cluster SA) reachable from `gvault-api`

See [docs/gpu-setup.md](../../docs/gpu-setup.md).

## Configure

```bash
cp .env.example .env
# PROVIDER=nvidia-snapshot
# ENABLE_NVIDIA_PROVIDER=true
make docker-up
make build
make run-api
```

Without a live kube client, `/readyz` and capture stay **fail-closed** (prerequisites unmet — not PASSED).

## CLI

```bash
./bin/gvaultctl providers
./bin/gvaultctl providers nvidia-dry-run inference snap1 llama-vllm-0 main

# Live cluster only:
./bin/gvaultctl snapshot create llama8b gpu-cluster-a inference llama-vllm-0 main
./bin/gvaultctl operation wait <operationId>
./bin/gvaultctl snapshot check <snapshotId> gpu-cluster-a
./bin/gvaultctl snapshot restore <snapshotId> gpu-cluster-a inference llama-restored-1 gpu-node-b
```

## What gets stored

Catalog metadata + opaque refs in Postgres. Bytes on NVIDIA RWX. Details: [docs/storage.md](../../docs/storage.md).

## Stop line

AMD and other vendors are **out of scope** until Phase 1 exit criteria in [docs/scope.md](../../docs/scope.md).
