---
name: release
description: Use when releasing kube-node-viewer or checking a release ("release", "lançar versão", "qual versão vai sair", "o release falhou") — how merged changes become a version (weekly or on demand), how to steer the version number, how to confirm the binaries and the Homebrew cask, and what to do when it fails.
---

# Release (kube-node-viewer)

Rules: `AGENTS.md` ("Releases and pull requests") and `CONTRIBUTING.md`.
If this file disagrees with them, they win — fix this file.

Releases are automatic and batched. No release PR, no tag by hand. Merges
only add to `[Unreleased]`; a release takes everything there:

```
Monday 12:00 UTC (schedule)          ─┐
gh workflow run release.yaml          ├─ release.yaml on main
merge of a PR labelled `release`     ─┘
       ├─ scripts/release-prepare.py: [Unreleased] → [x.y.z] - date,
       │    compare links, finished specs → shipped
       ├─ commit "chore: release vx.y.z" pushed to main, tag vx.y.z
       └─ GoReleaser: binaries on GitHub + cask in StaffOps/homebrew-tap
```

## 1. Steer the version (in the PR, not after)

The CHANGELOG entry decides it:

| `[Unreleased]` holds | Next version |
|----------------------|--------------|
| only `### Fixed` / `### Security` | patch (0.9.1 → 0.9.2) |
| any `### Added` / `### Changed` / `### Removed` / `### Deprecated` | minor (0.9.1 → 0.10.0) |
| the word `BREAKING` | major (minor while 0.x) |
| nothing | no release |

Preview without writing: `python3 scripts/release-prepare.py`.

Several PRs go into one release by default: they merge during the week
and the Monday run releases them together.

Release sooner: `gh workflow run release.yaml --repo StaffOps/kube-node-viewer --ref main`,
or add the `release` label to the PR before it merges.

## 2. Confirm it

```bash
gh run list --repo StaffOps/kube-node-viewer --workflow release.yaml --limit 1
gh run watch --repo StaffOps/kube-node-viewer <id> --exit-status
gh release view vX.Y.Z --repo StaffOps/kube-node-viewer
brew update && brew upgrade --cask staffops/tap/kube-node-viewer && kube-node-viewer -version
```

## 3. When it fails

| Symptom | Cause | Action |
|---------|-------|--------|
| No release after a PR labelled `release` merged | `gh workflow run` failed in the automerge job | run `gh workflow run release.yaml --ref main` |
| "already tagged and [Unreleased] is empty" | nothing new | expected |
| Release commit pushed, tag created, GoReleaser failed | build or token error | fix in a PR; the owner re-runs the workflow on the tag |
| Release ok, cask not updated | `HOMEBREW_TAP_GITHUB_TOKEN` missing or expired | owner renews the secret; re-run |
| Push of the release commit or tag refused (rule violation / "protected") | `RELEASE_DEPLOY_KEY` missing, or its deploy key removed or read-only; GITHUB_TOKEN cannot bypass the rulesets | owner adds a new deploy key with write access and sets the secret; re-run |

Never delete or move a published tag without the owner asking.
