# Compatibility and upstream pinning

## Product

**gvault** — GPU snapshot registry and lifecycle control plane.

## Active local path

| Field | Value |
|---|---|
| Default provider | `fake` |
| NVIDIA Snapshot release | **v0.3.0** (pinned) |
| Live NVIDIA kube client | Not connected in default demos |
| Real GPU restore | Requires GPU cluster; tests tagged `gpu` are SKIPPED without hardware |

## Pin record (Milestone 3)

| Item | Value |
|---|---|
| Upstream | https://github.com/ai-dynamo/snapshot |
| Tag | `v0.3.0` |
| Verified | 2026-11-17 against published API docs / `api/v1alpha1` + `api/podcontract` |
| API group | `nvidia.com/v1alpha1` |
| Capture CR | `PodSnapshot` with `spec.source.podRef.{name,uid,containers}` |
| Content CR | `PodSnapshotContent` (operator-managed — do not create directly) |
| Restore annotation | `nvidia.com/restore-from` |
| Restore restartPolicy | `Never` (v0.3.0 restore proxy contract) |
| Storage MVP | RWX PVC (S3 not advertised until verified for this pin) |

Dry-run manifests: `POST /api/v1/providers/nvidia/dry-run`.

## Fail-closed preflight (shared engine)

Required exact matches: `gpuProduct`, `driverVersion`, `architecture`, `imageDigest`.
Optional: `gpuCount` when both sides set. Cross-cluster blocked in MVP.
