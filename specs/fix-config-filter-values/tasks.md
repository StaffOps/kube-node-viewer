# Tasks: fix-config-filter-values

> Implements [bugfix](bugfix.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done

- [ ] T1. `CapacityValues`, `StatusValues` in `pkg/model`; the check in `readFilters`; tests
  - _Covers: R1, R2 · Depends on: — · Verified by: `TestConfigFilterValuesOfClosedFields`, `TestConfigFilterValuesOfOpenFieldsLoad`, `TestClosedValueListsHoldEveryConstant`_
- [ ] T2. `docs/views.md`, CHANGELOG `Fixed`, `TODO.md` item removed, `specs/README.md` index
  - _Covers: R1, R2 · Depends on: T1 · Verified by: `TestReadmeConfigViewsDocumentsTheContract` (the new rule) and review_
- [ ] T3. Verify: `scripts/verify.sh` and `make lint` green; the built binary refuses `values: [spot]` and loads the owner's config
  - _Covers: R1, R2 · Depends on: T1, T2 · Verified by: command output, recorded here as a dated note_
