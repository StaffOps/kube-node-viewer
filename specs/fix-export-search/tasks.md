# Tasks: fix-export-search

> Implements [bugfix](bugfix.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done

- [x] T1. `exportView` in `pkg/model/export.go`; tests in `pkg/model` (export) and `pkg/state` (raw state file)
  - _Covers: R1, R2, R3 · Depends on: — · Verified by: `TestExportWritesTheSearch` and `TestExportOmitsAnEmptySearch` (`pkg/model`), `TestStateNeverSavesTheSearch` (`pkg/state`)_
- [x] T2. README export section, CHANGELOG `Fixed`, `TODO.md` item removed, dated notes in `specs/search-negation/requirements.md`, `specs/search-regex/requirements.md` (R4's ticked `view.search` box, lines 68-70) and `specs/search-regex/design.md` (line 17: the export dropped it; stale `export.go:125`), `specs/README.md` index
  - _Covers: R1, R2, R3 · Depends on: T1 · Verified by: review_
- [x] T3. Verify: `scripts/verify.sh` and `make lint` green; a run on a local kind cluster exports a searched view and the JSON holds `view.search`
  - _Covers: R1, R2, R3 · Depends on: T1, T2 · Verified by: command output, recorded here as a dated note_

> **Verified 2026-11-05.** `scripts/verify.sh` (apart from these open
> tasks before ticking) and `make lint` green (0 issues). Without the fix,
> `TestExportWritesTheSearch` fails for all six texts and
> `TestExportOmitsAnEmptySearch` passes (the empty case is today's
> document). On a local kind cluster of one node, with its own kubeconfig
> and state directory, the built binary driven through a pseudo terminal
> (`-disable-pricing`), `e` after each search: `/knv` → `view.search`
> `"knv"`, 1 node; `/!control` → `"!control"`, 0 nodes shown of 1 (the
> node's name holds `control`); no search → no `search` key, the view
> keys `sort, resources, columns, showNodes` as before; `/!zzz` then `I`
> (a view change, so the state file is written) → the export holds
> `"!zzz"` and the state file has no `search` key. The cluster was
> deleted afterwards.
