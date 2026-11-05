# Tasks: fix-config-filter-values

> Implements [bugfix](bugfix.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done

- [x] T1. `CapacityValues`, `StatusValues`; `ValueCounts` uses them; tests
  - _Covers: R3 · Depends on: — · Verified by: `TestClosedValueListsHoldEveryConstant`, `TestClosedValueListsCoverEveryBranch`, the existing `ValueCounts` tests_
- [x] T2. Per-item check in `stringList`; `closedValue` with the hints; `readFilters` passes it for `capacity` and `status`; tests
  - _Covers: R1, R2 · Depends on: T1 · Verified by: `TestConfigFilterValuesOfClosedFields`, `TestConfigFilterValuesOfOpenFieldsLoad`_
- [x] T3. `docs/views.md` (the two value lists and the rule), CHANGELOG `Fixed`, dated correction of `config-views` R6, `TODO.md` item removed, `specs/README.md` index
  - _Covers: R1, R2 · Depends on: T2 · Verified by: `TestReadmeConfigViewsDocumentsTheContract` (`is not a capacity value`) and review_
- [x] T4. Verify: `scripts/verify.sh` and `make lint` green; the built binary refuses `values: [spot]` and loads the owner's config
  - _Covers: R1, R2, R3 · Depends on: T1, T2, T3 · Verified by: command output, recorded here as a dated note_

> **Verified 2026-11-05.** `scripts/verify.sh` and `make lint` green. The
> new tests fail without the fix: the 15 cases of
> `TestConfigFilterValuesOfClosedFields` with the old `configview.go`, and
> `TestClosedValueListsHoldEveryConstant` when `Unknown` is dropped from
> `CapacityValues`. The built binary, with a kubeconfig that does not exist
> so no cluster is reached: a config with `exclude: [spot]` on `capacity`
> stops with `views[0].filters[0].exclude[0]: "spot" is not a capacity
> value; write Spot`, and the owner's own `config.yaml` passes the config
> read and stops only at the connection. No cluster run: the change is in
> reading the file, before any connection.
