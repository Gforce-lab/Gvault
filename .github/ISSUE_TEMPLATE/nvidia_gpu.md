---
name: NVIDIA / GPU setup
about: Cluster prerequisites, pin v0.3.0, capture/restore on real GPUs
title: "gpu: "
labels: ["nvidia", "help wanted"]
---

## Goal

- [ ] Capture → catalog `READY`
- [ ] Compatibility preflight
- [ ] Restore A → B + probe
- [ ] Other:

## Cluster facts

- Kubernetes version:
- GPU Operator version:
- NVIDIA Snapshot version (must be **v0.3.0** for Phase 1):
- GPU product(s):
- Driver version:
- RWX storage class:

## What you already tried

1.
2.

## Blocker

## Evidence (sanitized)

```text
<!-- kubectl describe / gvaultctl output; no secrets -->
```

## Reminder

Do not mark physical GPU success until provider evidence exists. CI in-memory kube is **not** hardware proof.
