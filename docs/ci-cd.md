# CI/CD — Gvault

## What runs on every MR

Workflow: [`.github/workflows/ci.yml`](../.github/workflows/ci.yml)

**Triggers:** `pull_request` (opened / synchronize / reopened / ready_for_review), `push` to `main`/`master`/`develop`, `merge_group` (merge queue), and `workflow_dispatch`.

| Step | Purpose |
|---|---|
| `go mod tidy` diff | modules stay consistent |
| `gofmt -l` | formatting gate |
| `go vet ./...` | static checks |
| `go test ./... -race` | unit + package tests (auth, nvidia, probe, router, …) |
| `go test ./tests/integration/... -tags=integration` | Postgres-backed API flows (capture/restore, MR gates) |
| `go test ./tests/e2e/... -tags=gpu` | compiles GPU package (SKIPPED without hardware) |
| Build `gvault-api` / `gvault-controller` / `gvaultctl` | binaries must link |
| `gvaultctl version --local` | CLI smoke |
| Docker build (no push) | image still builds |

Protect `main` with required checks **`test`** and (optionally) **`docker-build`**.

## Local parity (same as MR)

```bash
make test-mr
```

Or step-by-step:

```bash
make vet
make test
make test-integration
make test-gpu
make build
```

## Workflows

| Workflow | Trigger | What it does |
|---|---|---|
| `ci.yml` | PR / push / merge_group | gates above |
| `release.yml` | tag `v*` | artifacts + GHCR |
| `dependabot-auto-merge.yml` | Dependabot PRs | auto-squash when green |
| `dependabot.yml` | weekly | Go + Actions updates |

### Dependabot auto-merge

1. Enable **Settings → General → Pull Requests → Allow auto-merge**.
2. Protect `main` with required status check `test`.
3. Dependabot PRs labeled `dependencies` auto-merge when CI is green.

## Opening an MR

```bash
git checkout -b feat/your-change
# ... commit ...
git push -u origin HEAD
# open PR against main — CI runs automatically
```

Use [.github/PULL_REQUEST_TEMPLATE.md](../.github/PULL_REQUEST_TEMPLATE.md).

## Release

```bash
git tag v0.1.0-beta.1
git push origin v0.1.0-beta.1
```
