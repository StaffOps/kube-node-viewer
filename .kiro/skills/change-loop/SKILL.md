---
name: change-loop
description: Use for implementing any non-trivial change in kube-node-viewer (flags, UI, pricing, client/informers, build/release, specs). Runs the AGENTS.md loop — spec first, implement task by task, verify, adversarial review, docs, commit — with the reviewer brief and the standing constraints.
---

# Change loop (kube-node-viewer)

Rules: `AGENTS.md` and `specs/README.md`. This skill is the procedure; if they
disagree, they win — fix this file.

## 1. Spec gate
- Find the spec in `specs/`. None and the change needs one → skill `spec-new`, stop.
- Spec is **draft** or has `[TO CONFIRM]` → ask the owner; do not implement.
- Spec wrong while implementing → dated correction in place
  (`> **Corrected YYYY-MM-DD (T<n>).** …`), never a silent edit.

## 2. Implement one task at a time
- Mark the task `[-]` in `tasks.md`; keep the diff to that task.
- Boundaries: Kubernetes I/O in `pkg/client`, state + rendering in
  `pkg/model`, prices behind `pricing.Provider` (`pkg/aws` only for AWS).
- New file → Apache 2.0 header (`hack/boilerplate.go` format). Changed
  upstream file → keep its Apache header; add a modification note.
- Tests next to the code (`pkg/model/*_test.go` style, table-driven).

## 3. Verify
- `scripts/verify.sh` must pass (gofmt, vet, tests, spec lint).
- UI change → `scripts/build.sh` and run `bin/kube-node-viewer` against a
  cluster the owner named; otherwise state "not run against a cluster".
- Tick the task `[x]` only when its "Verified by" line is satisfied.

## 4. Independent review
Launch the `spec-refuter` agent (never review your own change). Brief:

```
Refute <change> in <abs repo path> (branch <b>; `git diff <base>...HEAD` +
uncommitted `git diff`). Spec: specs/<feature>/. Check: requirements each
covered by code + test; invariants in design.md; Apache headers/NOTICE intact;
no AWS branding; label-prefix compatibility; flag precedence
(flag > config file; `KUBECONFIG` env only for `-kubeconfig`). Read-only; no commits; no cluster access unless
granted. Every claim needs file:line or command output.
Report BLOCKING/HIGH/MEDIUM/LOW with a concrete fix.
```

Fix → re-review the fixes (same agent) until no BLOCKING/HIGH remains.

## 5. Docs and commit
- `CHANGELOG.md` → `## [Unreleased]` (Added / Changed / Fixed / Removed).
- `README.md` usage block when flags or keys change (paste real `-h` output).
- Spec status → **shipped** when every task is `[x]` (or moved to Deferred).
- Commit per validated task, Conventional Commits, no AI trailers. Push only
  when the owner asks.
