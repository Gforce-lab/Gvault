# Architecture — Gvault

```text
UI / gvaultctl / curl
        ↓
Gin API (gvault-api) → auth + namespace policy
        ↓
 SnapshotService → PostgreSQL catalog (metadata, ops, audit ONLY)
        ↓
 CheckpointProvider
   └── nvidia-snapshot   ← Phase 1 product provider (pin v0.3.0)
         └── kube client → PodSnapshot / restore Pod
               └── RWX PVC holds checkpoint BYTES (not in Postgres)

Out of scope (Phase 1 stop): AMD GPU provider, S3 READY export, multi-vendor marketplace
```

## Storage split

See [storage.md](storage.md).

- **Postgres:** searchable projection — identity, hardware facts, opaque artifact refs, operations, audit.
- **NVIDIA + RWX:** authoritative checkpoint bytes.
- **Never** stream or store checkpoint payloads through the HTTP API.

## Phase stop-limits

See [scope.md](scope.md). Ship and harden NVIDIA end-to-end before any AMD work.
