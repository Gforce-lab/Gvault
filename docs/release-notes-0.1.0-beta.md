# Gvault 0.1.0-beta.1

**Channel:** beta  
**Date:** 2026-11-17

## Goal of this beta

Ship a coherent, demoable product that covers every milestone in the engineering plan on the **GPU-free path**, with an honest NVIDIA integration surface for a future live cluster.

## What you can prove today

1. Capture → async operation → READY (simulated)
2. Compatibility reject / allow with structured codes
3. Restore operation (simulated)
4. Retention hold blocks delete
5. CRD reconcile path via `/api/v1/crds/...`
6. Metrics scrape at `/metrics`
7. UI + CLI teach the same loop

## What you cannot claim yet

- Physical GPU restore latency improvements
- Production multi-tenant auth
- Full controller-runtime operator in-cluster

## Try it

```bash
cp .env.example .env
make docker-up && make build && make run-api
open http://localhost:8080
curl -s localhost:8080/api/v1/version
```
