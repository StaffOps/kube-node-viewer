# Tasks: fix-export-search

> Implements [bugfix](bugfix.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done

- [ ] T1. An export view type that adds `search` to `ViewState`'s JSON; tests
  - _Covers: R1, R2 · Depends on: — · Verified by: `TestExportWritesTheSearch`, `TestExportOmitsAnEmptySearch`, `TestStateNeverSavesTheSearch`_
- [ ] T2. README export section, CHANGELOG `Fixed`, `TODO.md` item removed, dated notes in search-negation and search-regex, `specs/README.md` index
  - _Covers: R1, R2 · Depends on: T1 · Verified by: review_
- [ ] T3. Verify: `scripts/verify.sh` and `make lint` green; a run on a cluster exports a searched view and the JSON holds `view.search`
  - _Covers: R1, R2 · Depends on: T1, T2 · Verified by: command output, recorded here as a dated note_
