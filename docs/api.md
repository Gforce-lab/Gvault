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

READY/SUCCEEDED require NVIDIA provider evidence (live cluster or CI in-memory kube exercising the same adapter).

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

## Restore readiness and inference verification

On the NVIDIA path, `Running` is not restore success: it can mean only that
the restore proxy has started. Gvault waits for the Snapshot v0.3.0
`nvidia.com/Restored=True` condition, Pod phase `Running`, and `Ready=True`.
Failed, incompatible, and partially successful restores fail the operation.
A Pod that exits or is replaced with another UID cannot satisfy the check.

To verify the restored worker can generate, include `inferenceProbe` in the
HTTP restore request:

```json
{
  "targetClusterId": "gpu-cluster-a",
  "namespace": "inference",
  "podName": "restored-vllm",
  "targetNodeName": "gpu-node-b",
  "inferenceProbe": {
    "protocol": "openai",
    "model": "Qwen/Qwen3-0.6B",
    "port": 8000,
    "timeoutSeconds": 60
  },
  "restorePodSpec": {
    "containers": [{
      "name": "main",
      "image": "your-registry/worker@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      "resources": {"limits": {"nvidia.com/gpu": "1"}}
    }]
  }
}
```

The Pod spec above illustrates the field shape. Supply the workload's actual
digest-pinned image, command, model mounts, runtime class, and serving probes.
For the pinned upstream vLLM guide, use the complete
[`examples/nvidia/restore-pod-spec.json`](../examples/nvidia/restore-pod-spec.json)
and select `protocol: "snapshot-vllm"`. It requires the upstream guide's
`vllm-app` ConfigMap and `model-cache` PVC, and its original `main` container.

NVIDIA restores require `restorePodSpec`; the previous minimal generated Pod
could not carry the workload's GPU allocation or model mounts. Gvault preserves
the supplied workload configuration and adds the canonical `snapshot-control`
volume, per-container mount, `SNAPSHOT_CONTROL_DIR`, and `restartPolicy: Never`.
It rejects conflicting settings, mutable/mismatched images, missing source
containers, and incorrect GPU allocations before creating a Pod. It does not
copy the Pod spec into audit events. Set `targetNodeName` rather than `nodeName`
in the Pod spec. Destination `gpuProduct`, `driverVersion`, `architecture`, and
`imageDigest` must be explicitly supplied for NVIDIA preflight; only the fake
harness can default destination facts from the source snapshot.

Once the provider reports readiness, Gvault POSTs to
`http://<restored-pod-ip>:<port>/v1/completions`. The IP comes from the observed
Pod, not a caller URL or Service, so another replica cannot answer the check.
The fixed synthetic prompt is `Say ready.`, with `max_tokens=8`,
`temperature=0`, and `stream=false`. Success requires valid JSON, the exact
requested model name, and non-empty completion text. This is a serving smoke
test, not a model-quality or deterministic-output assertion.

For `protocol: "snapshot-vllm"`, Gvault instead POSTs `{"prompt":"Say ready."}`
to `/generate`, matching NVIDIA Snapshot v0.3.0's vLLM guide. That server caps
generation at eight tokens itself. Success requires a non-empty `text` field.
The protocol does not report the model name, so this check asserts serving
readiness only; `model` is optional and is not verified in the response.
Unknown protocols and response `error` objects fail closed. Probe audit events
include the selected protocol so these two kinds of evidence remain distinct.

The probe retries once per second until its deadline, then the operation fails
with `RESTORE_PROBE_FAILED`. HTTP failures, malformed/oversized responses,
empty text, and the wrong model do not pass. Redirects and environment proxies
are disabled. OpenAI callers must provide a non-blank model name of at most 256
characters. Port defaults to 8000; probe timeout defaults to 60 seconds and is
capped at 300 seconds (zero selects the default for either field).

`RESTORE_TIMEOUT_SECONDS` bounds the entire restore execution, including provider
requests, readiness polling, and verification. Its default is 300 seconds and
its allowed range is 1–3600. Provider/readiness timeout reports `RESTORE_TIMEOUT`.
Operations stay `RUNNING` until verification finishes. A failed verification
does not invalidate the reusable `READY` snapshot. Readiness and Pod identity
are checked again after a successful application probe.

Probe audit events include the check type (`inference` or `http`), outcome,
endpoint, and a sanitized diagnostic. They do not store the prompt, response
body, generated text, or credentials. The restore duration includes verification.
The existing `restore_probe_failed` metric outcome includes inference failures.

For CLI use:

```bash
GVAULT_RESTORE_POD_SPEC=examples/nvidia/restore-pod-spec.json \
GVAULT_INFERENCE_PROTOCOL=snapshot-vllm \
GVAULT_GPU="$GPU_PRODUCT" GVAULT_DRIVER="$DRIVER_VERSION" \
GVAULT_ARCH="$CPU_ARCH" GVAULT_DIGEST="$IMAGE_DIGEST" \
GVAULT_INFERENCE_PORT=8000 \
GVAULT_INFERENCE_TIMEOUT_SECONDS=60 \
  ./bin/gvaultctl snapshot restore SNAPSHOT_ID gpu-cluster-a inference restored-vllm gpu-node-b
```

The optional legacy `probeEndpoint` remains a GET/2xx health check and now
retries within the total restore deadline. It cannot be combined with
`inferenceProbe`. Omitting both checks provider/Pod readiness only and provides
no inference evidence. The in-memory CRD bridge does not expose inference
verification; this option belongs to the HTTP API/CLI and dashboard restore flow.
The dashboard includes the workload Pod spec, explicit destination facts,
inference protocol, model, port, and timeout fields.

Set those destination fact variables to the actual target's values; do not
substitute source facts when the destination is unknown. CLI Pod spec input
is a JSON object containing the Pod `spec` alone, not a complete Pod manifest.
Operation responses report the snapshot's provider and derive `simulated` from
that stored provider, independently of the currently running API provider.

The inference endpoint currently supports plain HTTP without authentication.
Gvault must be able to reach the restored Pod IP through cluster networking and
NetworkPolicy. Workloads and restore Pods must already satisfy the pinned
Snapshot workload contract. Gvault supplies the minimum restore control
protocol but cannot infer model mounts or workload configuration. GPU-tagged
tests remain skipped until real hardware is available. Local integration tests
exercise HTTP submission, the real Kubernetes HTTP client against simulated
Pod observations, database state, inference HTTP requests, idempotent replay,
and sanitized audit evidence for both protocols. These do not prove CUDA restore.
