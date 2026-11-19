---
name: Bug report
about: Something is broken in Gvault (API, CLI, controller, or NVIDIA path)
title: "bug: "
labels: ["bug"]
---

## Summary

<!-- One sentence: what went wrong? -->

## Environment

- Gvault version / commit:
- Provider: `nvidia-snapshot`
- Cluster / node (if any):
- NVIDIA Snapshot pin expected: `v0.3.0`
- OS / Go version (if building from source):
- How you run it: API / CLI / Helm / kustomize / local

## Steps to reproduce

1.
2.
3.

## Expected behavior

## Actual behavior

## Logs / evidence

```text
<!-- paste sanitized logs; no tokens, kubeconfigs, or checkpoint payloads -->
```

## Checklist

- [ ] I did **not** claim physical GPU `READY` without a live NVIDIA cluster
- [ ] Secrets / tokens / checkpoint bytes are **not** included
- [ ] Related docs checked: `docs/scope.md`, `docs/storage.md`, `docs/gpu-setup.md`
