---
name: release
description: Use when cutting a kube-node-viewer release ("release v0.x", "lançar versão", "fechar a versão") or checking one that just ran — preparing the release PR from CHANGELOG [Unreleased], what the merge triggers, how to confirm the binaries and the Homebrew cask, and what to do when it fails.
---

# Release (kube-node-viewer)

Rules: `AGENTS.md` ("Releases and pull requests") and `CONTRIBUTING.md`
("Releasing"). If this file disagrees with them, they win — fix this file.

Releases are automated: merging a PR whose CHANGELOG names a version with no
tag yet makes `.github/workflows/release.yaml` tag the merge commit
`vX.Y.Z`, run GoReleaser (binaries on GitHub) and update the Homebrew cask in
`StaffOps/homebrew-tap`. Agents never push tags.

## 1. Pick the version

- Read `## [Unreleased]` in `CHANGELOG.md`. Empty → nothing to release; stop.
- SemVer, pre-1.0: new behaviour (`Added`, `Changed`) → minor; only `Fixed`
  → patch. Ask the owner when unsure.
- Check open PRs (`gh pr list --repo StaffOps/kube-node-viewer`): ask the
  owner whether any should go in first.

## 2. Release PR (branch `chore/release-vX.Y.Z` from `main`)

1. `CHANGELOG.md`: rename `## [Unreleased]` to `## [X.Y.Z] - YYYY-MM-DD`
   (today) and add an empty `## [Unreleased]` above it. Bottom links:
   `[Unreleased]: …/compare/vX.Y.Z...HEAD` and
   `[X.Y.Z]: …/compare/v<previous>...vX.Y.Z`.
2. Specs whose tasks are all `[x]`: status → **shipped (vX.Y.Z, date)** in
   their headers and in `specs/README.md`.
3. `TODO.md`: drop items this release closes.
4. `scripts/verify.sh` and `make lint`; commit `chore: release vX.Y.Z`;
   push and open the PR only when the owner asks.

## 3. After the owner merges

```bash
gh run list --repo StaffOps/kube-node-viewer --workflow release.yaml --limit 1
gh run watch --repo StaffOps/kube-node-viewer <id> --exit-status
gh release view vX.Y.Z --repo StaffOps/kube-node-viewer
brew update && brew upgrade --cask staffops/tap/kube-node-viewer && kube-node-viewer -version
```

Any later merge to `main` finds the tag and does nothing — expected.

## 4. When it fails

| Symptom | Cause | Action |
|---------|-------|--------|
| Step "Version to release" says already tagged | the CHANGELOG top version was released before | bump the version in a new PR |
| Tag created, GoReleaser failed | build or token error | fix in a PR, then the owner re-runs the workflow on the tag (or pushes the tag again by hand) |
| Release ok, cask not updated | `HOMEBREW_TAP_GITHUB_TOKEN` missing or expired | owner renews the secret; re-run |

Never delete or move a published tag without the owner asking.
