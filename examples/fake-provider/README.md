# Gvault FakeProvider demo

This path uses **simulated** capture/restore. It does **not** create real GPU checkpoints.

```bash
cp .env.example .env
make docker-up
make build
make run-api
# UI: http://localhost:8080
```

## CLI (preferred)

```bash
./bin/gvaultctl health
./bin/gvaultctl overview

./bin/gvaultctl snapshot create llama8b-fake-v1 dev-gpu-cluster inference llama-vllm-0 main
# copy snapshotId + operationId from output

./bin/gvaultctl operation wait <operationId>
./bin/gvaultctl snapshot get <snapshotId>

./bin/gvaultctl snapshot check <snapshotId> dev-gpu-cluster
./bin/gvaultctl snapshot restore <snapshotId> dev-gpu-cluster inference llama-vllm-restored-1 fake-node-b
./bin/gvaultctl operation wait <restoreOperationId>

./bin/gvaultctl snapshot tags <snapshotId> set env=demo model=llama
./bin/gvaultctl snapshot promote <snapshotId> v0.1.0-demo
./bin/gvaultctl audits
./bin/gvaultctl providers
./bin/gvaultctl providers nvidia-dry-run inference dry-1 llama-vllm-0 main
```

Full process map: root [README.md](../../README.md#processes-end-to-end).

## curl

```bash
# Health
curl -s localhost:8080/healthz

# Capture (202 + durable operation)
curl -s -X POST localhost:8080/api/v1/snapshots \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: capture-demo-1' \
  -d '{
    "name": "llama8b-fake-v1",
    "clusterId": "dev-gpu-cluster",
    "namespace": "inference",
    "source": {"podName": "llama-vllm-0", "containerName": "main"},
    "provider": "fake"
  }'

# Poll operation until SUCCEEDED (still fake)
curl -s localhost:8080/api/v1/operations/<operationId>

# List / get snapshot (status READY = simulated)
curl -s 'localhost:8080/api/v1/snapshots?namespace=inference'

# Compatibility (omit hardware fields to fail closed; pass matching fields after READY)
curl -s -X POST localhost:8080/api/v1/snapshots/<snapshotId>/compatibility \
  -H 'Content-Type: application/json' \
  -d '{
    "targetClusterId": "dev-gpu-cluster",
    "gpuProduct": "NVIDIA-Fake-A100-80GB",
    "driverVersion": "550.00.fake",
    "architecture": "amd64",
    "imageDigest": "sha256:fake-image-digest"
  }'

# Restore (simulated)
curl -s -X POST localhost:8080/api/v1/snapshots/<snapshotId>/restore \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: restore-demo-1' \
  -d '{
    "targetClusterId": "dev-gpu-cluster",
    "namespace": "inference",
    "podName": "llama-vllm-restored-1",
    "targetNodeName": "fake-node-b"
  }'
```
