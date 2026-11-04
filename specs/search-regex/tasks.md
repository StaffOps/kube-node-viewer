# Tasks: search-regex

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done

- [x] T1. `searchMatcher` in `pkg/model/search.go`; `Select` and `Node.contains` use it
  - _Covers: R1, R2, R3 (matching) · Depends on: — · Verified by: table tests in `pkg/model/search_internal_test.go` and `search_test.go` (dashed name next to a dotted IP); existing search tests unchanged; `BenchmarkSelectSearch` within D3's bound_
- [x] T2. Footer while searching (`(regex)`, invalid note), header ` (as text)`, help line
  - _Covers: R3 (footer and header), R4 (help) · Depends on: T1 · Verified by: key-driven tests_
- [x] T3. README keys table, search and export paragraphs, CHANGELOG `Changed`, TODO item for negation and label keys
  - _Covers: R4 · Depends on: T1 · Verified by: review_
- [x] T4. Verify: `scripts/verify.sh`, `make lint`, and a run on a cluster: `a|b` on two workloads' label values, an anchored name, an invalid pattern while typing
  - _Covers: R1–R4 · Depends on: T1–T3 · Verified by: dated note here (what was checked, not which cluster)_

> **Verified 2026-11-04.** `scripts/verify.sh`, `make lint` and `go test
> ./pkg/model` green. Ran the binary on an EKS cluster (11 nodes, 7 node
> groups) from a scratch directory, driving the keys through a pseudo
> terminal and reading the export (`e`) of each search. Checked: two node
> group names `A` and `B` keep 1 and 2 nodes, and `A|B` keeps exactly their
> union (3); `^<node name>$` keeps that node only; a node name minus its
> last character, as plain text, still keeps it; `a(b` shows the footer
> `type to search (regex) • invalid regex: as text • …` and the header
> `/a(b (as text)`.
