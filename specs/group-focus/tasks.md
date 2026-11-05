# Tasks: group-focus

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done
> (Kiro's task-list format — the Kiro spec panel tracks these checkboxes).

- [x] T1. Focus state and node selection: `focus.go` (steps, levels, `inFocus`), `render` on Overview draws the focused nodes from the display view, `exportDoc` with the `focus` key, both empty messages
  - _Covers: R1, R6, R8 · Depends on: — · Verified by: `go test ./pkg/model -run Focus`_
- [x] T2. Keys: `enter` on a heading focuses (sub-groups, ancestors no-op, after a group-by change), `space` folds except on headings in focus, `esc` clears the search then pops one step with the cursor back on its heading and page (Overview only), `n`/`d` off; `SetView` clears; `resolveCursor` follow
  - _Covers: R1, R3, R4, R5, R6, R7, R8 · Depends on: T1 · Verified by: `go test ./pkg/model -run 'Focus|Cursor|Fold'`_
- [x] T3. Header `Focus:` line, key line and help panel rows
  - _Covers: R2, R7 · Depends on: T2 · Verified by: `go test ./pkg/model -run Focus`_
- [x] T4. Spec corrections (interactive-tui R17 and its acceptance line, node-details R1); README keys table, Group by section and export format; CHANGELOG `[Unreleased]`; `view.go` Collapsed comment; help row in `panels.go`; `TestCursorMovesAndFoldsAGroup`, `TestEnterOnAGroupStillFolds` and the demo steps in `screenshots_test.go` move to `space`; `.static/demo.gif` regenerated if the screenshot tools are installed, else a TODO.md item
  - _Covers: R5 · Depends on: T3 · Verified by: review_
- [x] T5. Verify: `scripts/verify.sh` and `make lint` green; run on a cluster: note what was checked, never the cluster's name
  - _Covers: R1, R2, R3, R4, R5, R8 · Depends on: T4 · Verified by: command output and the cluster run, recorded here as a dated note_

## Deferred

| Task | Promotion trigger |
|------|-------------------|
| Keep the focus across runs | The owner wants to start in the same group every time |
| Folding inside a focus with `n` | Focused groups with many sub-groups |

> **Note 2026-11-04.** T1–T4 done: `scripts/verify.sh` and `make lint`
> green; `pkg/model` coverage 94.6%, `focus.go` 83–100% per function. Every
> part checked by breaking it on purpose (thirteen mutations: selection,
> space guard, page follow, pending move, field levels, esc order, export,
> folds kept, Recommendations, esc on Recommendations, SetView, n, ancestor
> enter), each failing at least one test. `esc: back` moved to the front of
> the key line (R2 correction). `.static/demo.gif` is not regenerated
> (no ffmpeg); a TODO.md item tracks it. T5 (cluster run) is open.

> **Verified 2026-11-04.** Ran the binary (`--group-by karpenter.sh/nodepool
> --node-sort name=asc`) in a real terminal on a local kind cluster: three
> workers labelled into two nodepools, and the control plane in `<none>`.
> Checked: `n` folds to the nodepool headings; `enter` on `<none>` and on
> `gpu` lists only that group's node, with `Focus:` in the View line,
> `1 of 4 shown` and the pod count of the group, `(filtered: 1 of 4 nodes)`
> in the summary, and `esc: back` first in the key line; `enter` on a node
> inside the focus opens its details and `esc` closes them; `esc` then
> returns to the folded list with the cursor on the group; `space` folds
> `gpu` (`▸`), `enter` lists its node anyway, `esc` brings `▸ gpu` back and
> `space` unfolds it.
