# Benchmarks

## Beta note

`0.1.0-beta.1` does **not** publish cold-vs-restore GPU timings until measured on hardware.

When a real GPU cluster is available, record:

- same model revision, image digest, GPU product
- `T0` request/scale trigger → `T3` request-ready (health + sample prompt)
- P50/P95 across multiple attempts
- source/destination node, artifact size, failure rate

Never substitute CI in-memory kube timings for measured GPU results.
