# Architecture — gvault

```text
UI / gvaultctl / curl
        ↓
Gin API (gvault-api) → SnapshotService → PostgreSQL catalog
        ↓                    ↓
   Static web/         CheckpointProvider
                       ├── FakeProvider (default, simulated)
                       └── NvidiaProvider (v0.3.0 pin, fail-closed without kube)

Vault CRDs (vault.gvault.dev) → in-memory reconciler demo (gvault-controller)
```

- Default demos are FakeProvider-only; READY is simulated.
- NVIDIA dry-run builders encode pinned PodSnapshot / restore-pod contracts.
- Checkpoint bytes never pass through Gin.
