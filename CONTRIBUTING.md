# Contributing

1. Read [docs/GPU_Snapshot_Vault_Project_Plan.md](docs/GPU_Snapshot_Vault_Project_Plan.md) and [docs/decisions.md](docs/decisions.md).
2. Prefer small, testable changes aligned to the current milestone.
3. Do not claim real GPU restore success without hardware evidence.
4. Run `make fmt test build` before opening a PR.
5. Record material design choices in `docs/decisions.md`.
6. Open PRs against a feature branch (not direct `main`). GitHub fills [.github/PULL_REQUEST_TEMPLATE.md](.github/PULL_REQUEST_TEMPLATE.md) automatically — complete every required checklist item.

## Opening a pull request

```bash
git checkout -b feat/short-description
# ... commit ...
git push -u origin HEAD
# Then open a PR from the GitHub UI, or:
gh pr create --fill
```

Use the PR template sections as written. Maintainers use them to verify FakeProvider safety, fail-closed NVIDIA behavior, and test evidence.
