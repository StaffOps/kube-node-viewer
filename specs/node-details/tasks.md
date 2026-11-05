# Tasks: node-details

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done

- [x] T1. `panelNode`: rows from the node, scroll, keys, help text
  - _Covers: R1 · Depends on: — · Verified by: key-driven tests_
- [x] T2. README keys, CHANGELOG, screenshot
  - _Covers: — · Depends on: T1 · Verified by: review_

> **Corrected 2026-11-05.** There was no verify task (`scripts/verify.sh`,
> `make lint`, a recorded cluster run), and none of it was recorded. The
> retroactive review ran the tests (passing) and found some R1 criteria
> untested; `TestDetailsAreLiveSortedAndEnterCloses` now covers a live
> value change, labels sorted by key and `enter` closing. The red colour of
> a bad condition is still checked only by reading the code. Dead code for
> a pods row in Resources was removed.
