# What Gvault stores (and what it does not)

Gvault is a **control-plane catalog**, not a checkpoint object store.

## Two stores, two jobs

| Store | Authoritative for | Never holds |
|---|---|---|
| **PostgreSQL** (Gvault catalog) | Searchable metadata, operation state, audit, tags/hold/version | Checkpoint / VRAM / process memory bytes |
| **NVIDIA Snapshot + RWX PVC** (upstream) | Raw GPU checkpoint artifacts on shared storage | Product naming, compatibility policy, retention UX |

```text
Inference Pod (GPU)
        │  capture
        ▼
NVIDIA Snapshot operator  ──writes──►  RWX PVC / provider content
        │                                (artifact bytes)
        │ status (Ready / Failed)
        ▼
Gvault (API / controller) ──writes──►  PostgreSQL catalog
                                        (metadata + ops + audit only)
```

## PostgreSQL — exact information we persist

### `snapshots` (catalog row)

| Field group | Examples | Why |
|---|---|---|
| Identity | `id`, `name`, `cluster_id`, `namespace` | Locate a versioned snapshot |
| Lifecycle | `status`, `created_at`, `ready_at`, `deleted_at`, `resource_version` | Async truth; never READY on accept alone |
| Source workload | `source_pod_name`, `source_pod_uid`, `source_container_name` | Provenance |
| Provider binding | `provider`, `provider_version`, `provider_snapshot_*`, `provider_content_name` | Opaque link to NVIDIA `PodSnapshot` / content |
| Hardware / runtime facts | `gpu_product`, `gpu_count`, `driver_version`, `architecture`, `kernel_version` | Compatibility preflight |
| Workload identity | `image_digest`, model/framework fields, `labels_jsonb` | Match restores to the same image/model |
| Artifact locator (opaque) | `artifact_backend`, `artifact_ref_jsonb`, `size_bytes` | Pointers only — e.g. backend=`nvidia-snapshot`, handle=content name |
| Management | `version_tag`, `retention_policy`, `retention_hold` | Promote / hold / guarded delete |

### `operations`

Async CAPTURE / RESTORE / DELETE attempts: status, actor, request id, idempotency key, target pod/node, error code/message, timestamps.

### `audit_events`

Who did what (`actor`, `action`, `resource_*`, `decision`, `request_id`, sanitized `attributes_jsonb`). **No tokens, secrets, or checkpoint payloads.**

### `snapshot_tags` / `provider_health`

Key-value tags for discovery; last-known provider health projection.

## NVIDIA / cluster — what Gvault does *not* copy

- Checkpoint blob bytes on the RWX volume
- CUDA / CRIU / NRI restore-proxy internals
- Node agent privileged state

Gvault only stores **references** returned by the provider (PodSnapshot name/UID, content name, optional size).

## Artifact reference shape (catalog)

```json
{
  "backend": "nvidia-snapshot",
  "storeId": "v0.3.0",
  "handle": "<provider-content-or-podsnapshot-name>",
  "extra": {}
}
```

If `backend` is anything else in older rows, treat as non-production / test data.

## Sources of truth (ordering)

1. **Vault CRD `spec`** — desired intent (when using operator path).
2. **NVIDIA CRD / Pod status** — provider progress.
3. **PostgreSQL** — searchable projection, refreshed idempotently from provider/reconcile.
4. **RWX / provider content** — bytes.

**Rule:** never mark catalog `READY` only because HTTP returned 202.
