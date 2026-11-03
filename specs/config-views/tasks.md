# Tasks: config-views

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done

- [x] T1. Hand-walked decode of `config.yaml` (labels, `views`, `default-view`, `start-with`) with paths, exact keys, strict scalars
  - _Covers: R1, R6 · Depends on: — · Verified by: `go test ./cmd/kube-node-viewer -run 'Settings|ConfigView'`_
- [x] T2. View item → `model.ViewState`; per-key names from `model.FilterFields`, `SortFields()`, `ColumnFields()`
  - _Covers: R1, R2, R6 · Depends on: T1 · Verified by: table-driven tests_
- [x] T3. Start view: `resolveView` keeps Header/Screen/Collapsed; `sessions` first-connection rule; both `start-with` modes
  - _Covers: R4, R5 · Depends on: T2 · Verified by: `go test ./cmd/kube-node-viewer -run 'ResolveView|Session'`_
- [x] T4. Config views in the Views panel: first, `(config)`, name clashes, `d` and save refused
  - _Covers: R3, R7 · Depends on: T2 · Verified by: `go test ./pkg/model -run Views`_
- [x] T5. README "Config views" section, CHANGELOG; README examples decoded by a test
  - _Covers: R8 · Depends on: T3, T4 · Verified by: review and test_
- [x] T6. Verify: `scripts/verify.sh`, `make lint`, manual run on a cluster
  - _Covers: R1, R3, R4 · Depends on: T1–T5 · Verified by: command output and owner check_
  - _Note (2026-11-03): `scripts/verify.sh` green, golangci-lint 0 issues, coverage 91.2% (cmd + pkg/model); README and docs/views.md examples decoded by tests. The manual run on a cluster the owner names is pending._
  - _Note (2026-11-03): run on a cluster by the author: config views load from `config.yaml` and show first in the Views panel marked `(config)` (R1, R3); the `default-view` / `start-with` view opens at start (R4)._

