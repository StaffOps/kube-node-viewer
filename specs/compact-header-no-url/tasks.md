# Tasks: compact-header-no-url

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done
> (Kiro's task-list format — the Kiro spec panel tracks these checkboxes).

- [x] T1. Compact box without the server URL in `pkg/model/header.go`; tests `TestCompactHeaderHasNoServerURL`, `TestFullHeaderKeepsServerURL` (new `header_internal_test.go`); `TestHeaderPanelAndOneLineFallback` in `panels_internal_test.go` asserts the compact header has no URL; golden scene `header-compact`
  - _Covers: R1, R2 · Depends on: — · Verified by: `go test ./pkg/model/`, the nine existing golden screens unchanged_
- [x] T2. Docs: README sentence above the compact image; `scripts/screenshots.sh`, then `git checkout` every `.static` file whose screen did not change, keeping `header-compact.png` and `demo.gif` (its `ctrl+e` frame loses the URL); CHANGELOG `[Unreleased]` `Changed`; dated note under interactive-tui R16; `TODO.md` item on ARN-style context names
  - _Covers: R1 · Depends on: T1 · Verified by: review; `git status .static` lists only those two files_
- [x] T3. Verify: `scripts/verify.sh` and `make lint` green; run on a local kind cluster: `ctrl+e` both ways, about 20 rows (automatic compact), 12 rows (one line); note what was checked, never the cluster's name
  - _Covers: R1, R2 · Depends on: T1, T2 · Verified by: command output and the cluster run, recorded here as a dated note_

  > **Verified 2026-11-05.** `go test ./pkg/model/` green; the new tests
  > and `TestHeaderPanelAndOneLineFallback` fail on `origin/main`'s
  > `header.go`, and `TestGoldenScreens` fails there on the new
  > `header-compact` screen; the nine existing golden screens are
  > byte-identical. `scripts/screenshots.sh` changed `header-compact.png`,
  > `demo.gif` and `node-details.png`; the last only by its wall-clock
  > creation time, so it was restored. On a local one-node kind cluster
  > (own kubeconfig and state, deleted afterwards), driven through a pty
  > with pricing off: at 40 rows the full panel has `Cluster:` and the URL;
  > `ctrl+e` gives the compact box with `<context> · …` and no `https://`;
  > `ctrl+e` again restores the panel; with a fresh state at 20 rows the
  > box is compact with no URL; at 12 rows the one-line header shows the
  > context and no URL.

> A code change with open tasks here cannot merge, and so cannot release:
> `scripts/spec-tasks-check.py` fails CI and the `pr-guard` hook blocks
> `gh pr create` (use `--draft` until the tasks are done).
