# Tasks: search-negation

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done

- [ ] T1. `negate` and `pairs` in `newSearchMatcher`; `Node.contains` matches label pairs and negates
  - _Covers: R1, R2, R3, R4 · Depends on: — · Verified by: matcher and `Select` table tests in `search_internal_test.go` and `search_test.go` (counter-examples and edge rows of the design's test plan); hidden-label test; search-regex selection and matcher tests unchanged; benchmark pair row within 3× of the substring row_
- [ ] T2. Help line; README keys table, Search section and export paragraph; CHANGELOG `Added`; dated note in search-regex requirements; remove the `TODO.md` item
  - _Covers: R1, R5 · Depends on: T1 · Verified by: help-line test; footer/header test for `!a(b`; review_
- [ ] T3. Verify: `scripts/verify.sh`, `make lint`, and a run on a local throwaway multi-node kind cluster: `!x` on a label value, `key=value`, `key=` and `!key=` on labelled nodes
  - _Covers: R1, R2, R3, R4, R5 · Depends on: T1, T2 · Verified by: dated note here (what was checked, not which cluster)_
