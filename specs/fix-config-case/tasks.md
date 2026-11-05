# Tasks: fix-config-case

> Implements [bugfix](bugfix.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done

- [x] T1. `caseOf` in `fieldKind.field`; `isBuiltin` on the lower-case form; tests
  - _Covers: R1, R3 · Depends on: — · Verified by: `TestConfigViewBuiltinNamesInAnotherCase`, `TestConfigViewOtherUpperCaseLabelsStillLoad`, the unchanged cases in `configerrors_test.go`_
- [x] T2. `yamlError`: one-line YAML errors; tests
  - _Covers: R2 · Depends on: — · Verified by: `TestConfigYAMLErrorsAreOneLine`_
- [x] T3. `docs/views.md`, CHANGELOG `Fixed`, dated correction of `config-views` R6, `specs/README.md` index
  - _Covers: R1, R2 · Depends on: T1, T2 · Verified by: `TestReadmeConfigViewsDocumentsTheContract` (`another case`, `label:INSTANCE`, `kube-node-viewer/node-age`, `written twice`) and review_
- [x] T4. Verify: `scripts/verify.sh` and `make lint` green; the built binary refuses an upper-case config and loads the owner's
  - _Covers: R1, R2, R3 · Depends on: T1, T2, T3 · Verified by: command output, recorded here as a dated note_

> **Verified 2026-11-04 (before the review).** `scripts/verify.sh` and `make lint` green. Of
> the 13 new cases in `configerrors_test.go`, the 11 for R1 and R2 fail
> without the fix; the `sort: PRICE/M` hint and the R3 labels pass either
> way. The built binary, with a kubeconfig that does not exist so no
> cluster is reached: `columns: [INSTANCE]` stops with `views[0].columns[0]:
> INSTANCE is not a column field; write instance-type, or label:INSTANCE
> for a node label`, a view with `name` twice with `line 3: key "name"
> already set in map` on one line, and the owner's own `config.yaml`
> passes the config read and stops only at the connection. No cluster run:
> the change is in reading the file, before any connection.

> **Review 2026-11-04 (spec-refuter, spec and diff).** Applied: the
> lower-case title `instance`, computed labels without their prefix or in
> another case, and `Label:`/`USAGE:` hints (R1); `YAML: ` for an error
> without a line (R2); CHANGELOG under `Changed` with the trade-off in the
> bugfix; the doc test asserts the new strings; dated note in
> `fix-config-errors`; a TODO item for filter values in another case.

> **Verified 2026-11-04 (after the review).** `scripts/verify.sh` and
> `make lint` green; 21 new cases in `configerrors_test.go`. The built
> binary, with a kubeconfig that does not exist: `columns: [node-age]`
> stops with the computed-label hint, and the owner's own `config.yaml`
> still passes the config read and stops only at the connection.
