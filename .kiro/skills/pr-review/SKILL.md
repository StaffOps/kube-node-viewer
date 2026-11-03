---
name: pr-review
description: Use when reviewing a pull request to kube-node-viewer, especially from another contributor ("review the PR", "mandaram um PR", "revisa o #N") — checks it against the current main, runs verify and lint on the merged result, checks the spec and CHANGELOG rules, and fixes or reports what is wrong.
---

# Pull request review (kube-node-viewer)

Rules: `AGENTS.md`. If this file disagrees with it, it wins — fix this file.

## 1. Read it

```bash
gh pr view <n> --repo StaffOps/kube-node-viewer \
  --json title,author,body,baseRefName,headRefName,maintainerCanModify,files,statusCheckRollup
gh pr diff <n> --repo StaffOps/kube-node-viewer
```

- Behaviour change → it needs a spec in `specs/` (AGENTS.md table) reviewed
  before code; check requirements are WHEN/THEN/SHALL and each has a test.
- Boundaries, Apache headers on new files, no AWS/Amazon/EKS branding,
  Conventional Commits without AI trailers, README keys/flags updated.
- Nothing from a real cluster committed (exports, kubeconfigs, node names).

## 2. Test the merge, not the branch

CI ran on the PR's base, which may be old. Merge onto today's `main` in a
throwaway worktree:

```bash
git fetch origin && git fetch origin pull/<n>/head:pr-<n>
git worktree add ../knv-pr<n> origin/main && cd ../knv-pr<n>
git merge --no-ff --no-edit pr-<n>
scripts/verify.sh && make lint
```

**CHANGELOG trap**: if a release happened after the branch was cut, git
merges its `[Unreleased]` lines into the released `## [x.y.z]` section with
no conflict and GitHub still says "mergeable". Look at the top of
`CHANGELOG.md` after the merge: new lines must sit under `## [Unreleased]`.

Clean up: `git worktree remove ../knv-pr<n>` and delete `pr-<n>`.

## 3. Fix or report

- The owner decides between asking the author and fixing it ourselves.
- Fixing a branch: merge `main` into it and add a commit on top. Never
  rebase or force-push someone else's branch.
- UI change not run against a cluster → say so; run it only against a
  cluster the owner names, from a scratch directory.
- Comments on GitHub are public: show the text to the owner before posting.

## 4. Hand off

Summarise for the owner: what it does, merge-test result (verify, lint,
CI), findings with file:line, and what you changed. Merging is the owner's.
