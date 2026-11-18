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
GVAULT_RESTORE_POD_SPEC=examples/nvidia/restore-pod-spec.json \
GVAULT_INFERENCE_PROTOCOL=snapshot-vllm \
GVAULT_GPU="$GPU_PRODUCT" GVAULT_DRIVER="$DRIVER_VERSION" \
GVAULT_ARCH="$CPU_ARCH" GVAULT_DIGEST="$IMAGE_DIGEST" \
  ./bin/gvaultctl snapshot restore <snapshotId> gpu-cluster-a inference llama-restored-1 gpu-node-b
```

The JSON Pod spec is derived from NVIDIA Snapshot **v0.3.0**'s
[`docs/guides/vllm/restore/1-gpu.yaml`](https://github.com/ai-dynamo/snapshot/blob/v0.3.0/docs/guides/vllm/restore/1-gpu.yaml).
Prepare that guide's `vllm-app` ConfigMap, model cache PVC, source workload, and
checkpoint first. Preserve the checkpointed image/configuration. The model
server exposes `/generate`, so select `snapshot-vllm`; ordinary OpenAI servers
use `openai` plus `GVAULT_INFERENCE_MODEL`. Destination fact variables must
describe the actual target node and image. For request fields and validation,
see [restore verification](../../docs/api.md#restore-readiness-and-inference-verification).

## What gets stored

Catalog metadata + opaque refs in Postgres. Bytes on NVIDIA RWX. Details: [docs/storage.md](../../docs/storage.md).

## Stop line

AMD and other vendors are **out of scope** until Phase 1 exit criteria in [docs/scope.md](../../docs/scope.md).
