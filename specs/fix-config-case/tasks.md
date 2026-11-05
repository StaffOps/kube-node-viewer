# Tasks: fix-config-case

> Implements [bugfix](bugfix.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done

- [x] T1. `caseOf` in `fieldKind.field`; `isBuiltin` on the lower-case form; tests
  - _Covers: R1, R3 · Depends on: — · Verified by: `TestConfigViewBuiltinNamesInAnotherCase`, `TestConfigViewOtherUpperCaseLabelsStillLoad`, the unchanged cases in `configerrors_test.go`_
- [x] T2. `yamlError`: one-line YAML errors; tests
  - _Covers: R2 · Depends on: — · Verified by: `TestConfigYAMLErrorsAreOneLine`_
- [x] T3. `docs/views.md`, CHANGELOG `Fixed`, dated correction of `config-views` R6, `specs/README.md` index
  - _Covers: R1, R2 · Depends on: T1, T2 · Verified by: the doc test and review_
- [x] T4. Verify: `scripts/verify.sh` and `make lint` green; the built binary refuses an upper-case config and loads the owner's
  - _Covers: R1, R2, R3 · Depends on: T1, T2, T3 · Verified by: command output, recorded here as a dated note_

> **Verified 2026-11-04.** `scripts/verify.sh` and `make lint` green. Of
> the 13 new cases in `configerrors_test.go`, the 11 for R1 and R2 fail
> without the fix; the `sort: PRICE/M` hint and the R3 labels pass either
> way. The built binary, with a kubeconfig that does not exist so no
> cluster is reached: `columns: [INSTANCE]` stops with `views[0].columns[0]:
> INSTANCE is not a column field; write instance-type, or label:INSTANCE
> for a node label`, a view with `name` twice with `line 3: key "name"
> already set in map` on one line, and the owner's own `config.yaml`
> passes the config read and stops only at the connection. No cluster run:
> the change is in reading the file, before any connection.
