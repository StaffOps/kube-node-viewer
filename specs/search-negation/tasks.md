# Tasks: search-negation

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done

- [x] T1. `negate` and `pairs` in `newSearchMatcher`; `Node.contains` matches label pairs and negates
  - _Covers: R1, R2, R3, R4 · Depends on: — · Verified by: matcher and `Select` table tests in `search_internal_test.go` and `search_test.go` (counter-examples and edge rows of the design's test plan); hidden-label test; search-regex selection and matcher tests unchanged; benchmark pair row within 3× of the substring row_
- [x] T2. Help line; README keys table and Search section; CHANGELOG `Added`; dated note in search-regex requirements; remove the `TODO.md` item
  - _Covers: R1, R5 · Depends on: T1 · Verified by: help-line test; footer/header test for `!a(b`; review_
- [x] T3. Verify: `scripts/verify.sh`, `make lint`, and a run on a local throwaway multi-node kind cluster: `!x` on a label value, `key=value`, `key=` and `!key=` on labelled nodes
  - _Covers: R1, R2, R3, R4, R5 · Depends on: T1, T2 · Verified by: dated note here (what was checked, not which cluster)_

> **Measured 2026-11-04 (T1).** `BenchmarkSelectSearch`, 5,000 nodes × 30
> labels, on a laptop busy with other builds: a first `key+"="+value` version
> measured about 2.7× the substring row, at the bound, so the design's
> fallback was taken at once: literal words are matched on the key and the
> value (`matchPair`), and `TestMatchPairSameAsPairText` checks it gives the
> same answer as the built pair. The pair row then allocates no more than
> the substring row (41,000 B, 3 allocs per op) and timed 0.7–3.6× the
> substring row across interleaved runs, a spread due to the load. `!nomatch`
> costs what `nomatch` costs plus the kept nodes' sort.
>
> **Verified 2026-11-04 (T3).** `scripts/verify.sh`, `make lint` (0 issues)
> and `go test ./pkg/model` green. Ran the binary on a throwaway local kind
> cluster (three nodes, made-up `workload` and `example.com/gpu` labels;
> deleted afterwards) from a scratch directory, typing each search through
> a pseudo terminal and reading the export (`e`). Checked: `batch` keeps
> the two `batch*` nodes and `!batch` the third; `! batch` is the same;
> `workload=batch` keeps both `batch*` pairs, `^workload=batch$` one;
> `example.com/gpu=` keeps the node with that key (empty value) and
> `!example.com/gpu=` the other two; the control-plane role key (empty
> value) splits one node from two the same way; `!` and `!!batch` keep
> every node; `zone` keeps none (keys not searched without `=`); `!a(b`
> keeps every node, the footer reads `invalid regex: as text` and the
> header `/!a(b (as text)`. Found on the way: the JSON export has no
> `view.search` (an existing bug, in `TODO.md`).
