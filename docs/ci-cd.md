# CI/CD — gvault

## Workflows

| Workflow | Trigger | What it does |
|---|---|---|
| [`.github/workflows/ci.yml`](../.github/workflows/ci.yml) | push / PR to `main`, `master`, `develop` | tidy check, vet, unit + race, integration (Postgres service), build binaries, Docker build (no push) |
| [`.github/workflows/release.yml`](../.github/workflows/release.yml) | tag `v*` | cross-compile artifacts, GitHub Release, push image to `ghcr.io/<org>/gvault/gvault-api` |
| [`.github/workflows/dependabot-auto-merge.yml`](../.github/workflows/dependabot-auto-merge.yml) | Dependabot PRs | `gh pr merge --auto --squash` after checks |
| [`.github/dependabot.yml`](../.github/dependabot.yml) | weekly (Monday) | Go modules + Actions updates (grouped) |

### Dependabot auto-merge

1. Enable **Settings → General → Pull Requests → Allow auto-merge** on `Gforce-lab/gvault`.
2. (Recommended) Protect `main` with required status check `test` from `ci.yml`.
3. Dependabot opens PRs labeled `dependencies`; the auto-merge workflow queues a squash merge when CI is green.

## Local parity

```bash
make test
make test-integration
make build
docker build -t gvault/gvault-api:local .
```

## Opening an MR / PR (manual)

1. Create the empty `gvault` repo under your org (GitHub/GitLab).
2. From this project:

```bash
git remote add origin git@github.com:<ORG>/gvault.git   # or your GitLab URL
git push -u origin main
# or feature branch:
git checkout -b feature/beta-0.1.0
git push -u origin HEAD
```

3. Open the MR/PR in the UI against `main`.
4. CI must be green before merge.

## Release

```bash
git tag v0.1.0-beta.1
git push origin v0.1.0-beta.1
```

That runs the release workflow (artifacts + GHCR image when on GitHub).
