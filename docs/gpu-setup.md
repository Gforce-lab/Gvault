# GPU setup guide (NVIDIA Snapshot v0.3.0)

This path is **not** satisfied by Kind or the local FakeProvider demo.

## Prerequisites (verify before claiming PASSED)

1. NVIDIA Snapshot Helm chart `v0.3.0` installed (`ghcr.io/ai-dynamo/snapshot/snapshot`).
2. NVIDIA GPU Operator + compatible driver/CUDA on two nodes of the same GPU product.
3. RWX StorageClass mounted by the Snapshot node agent (not by workload Pods).
4. Workload adapted to Snapshot workload contract (`ready-for-snapshot`, control volume, quiesce).
5. gvault with `PROVIDER=nvidia-snapshot` and `ENABLE_NVIDIA_PROVIDER=true` plus a live kube client (future wiring).

If any item is missing: mark GPU tests **SKIPPED**, never **PASSED**.

## Capture / restore (operator-level)

Use upstream docs for the pinned release. gvault dry-run:

```bash
curl -s -X POST localhost:8080/api/v1/providers/nvidia/dry-run \
  -H 'Content-Type: application/json' \
  -d '{"namespace":"inference","name":"llama8b","podName":"llama-vllm-0","podUid":"POD_UID","containerName":"main"}'
```
