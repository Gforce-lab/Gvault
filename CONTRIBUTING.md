# Contributing

1. Read [docs/scope.md](docs/scope.md), [docs/storage.md](docs/storage.md), and [docs/decisions.md](docs/decisions.md).
2. Phase 1 is **NVIDIA-only** — do not add AMD or other GPU vendors yet.
3. Do not reintroduce a FakeProvider; tests must use the NVIDIA adapter (+ in-memory kube if needed).
4. Do not claim real GPU restore success without hardware evidence.
5. Run `make fmt test build` before opening a PR.
6. Record material design choices in `docs/decisions.md`.
7. Open PRs against a feature branch (not direct `main`). Use [.github/PULL_REQUEST_TEMPLATE.md](.github/PULL_REQUEST_TEMPLATE.md).

## Opening an issue

Use **New issue** on GitHub and pick a template (see [#15](https://github.com/Gforce-lab/Gvault/issues/15)):

| Template | When to use |
|---|---|
| Bug report | Something is broken |
| Feature request | New capability (check Phase 1 fit) |
| Documentation | Docs / README / OpenAPI fixes |
| NVIDIA / GPU setup | Real cluster / pin / capture-restore blockers |
| Question | Architecture or how-to |
| Security (non-sensitive) | Process/docs only — private vulns go to [SECURITY.md](SECURITY.md) |

Blank issues are disabled. Do not paste tokens, kubeconfigs, or checkpoint payloads.

## Opening a pull request

```bash
git checkout -b feat/short-description
# ... commit ...
git push -u origin HEAD
# Then open a PR from the GitHub UI, or:
gh pr create --fill
```

Use the PR template sections as written. Maintainers use them to verify NVIDIA fail-closed behavior and test evidence.
