# Tasks: fix-config-errors

> Implements [bugfix](bugfix.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done

- [x] T1. Label keys and resource names checked as qualified names; padded filter values and label patterns refused
  - _Covers: R1, R2 · Depends on: — · Verified by: `TestConfigViewNamesAreKubernetesNames` and the unchanged cases in `TestConfigViewErrorsKeepTheirText`_
- [x] T2. `label: zone` hint; values shown as `{k: v}`, `[a, b]`, whole numbers; "quote it" only for scalars; sibling-field hints
  - _Covers: R3, R4, R5 · Depends on: — · Verified by: `TestConfigViewErrorHints`, `TestDescribeItem`; full-message cases_
- [x] T3. CHANGELOG `Fixed`; `docs/views.md` errors list; dated correction of `config-views` R6; `TestReadmeConfigViewsDocumentsTheContract` asserts the whitespace rule; remove the `TODO.md` item
  - _Covers: R1, R2 · Depends on: T1, T2 · Verified by: the doc test and review_
- [x] T4. Verify: `scripts/verify.sh` and `make lint` green; the binary refuses a padded config and loads the owner's
  - _Covers: R1, R2, R3, R4, R5 · Depends on: T1, T2, T3 · Verified by: command output, recorded here as a dated note_

> **Verified 2026-11-04.** `scripts/verify.sh` and `make lint` green; 47
> cases in `configerrors_test.go` (35 of them fail without the fix, the
> unchanged ones pass either way). The built binary, with a kubeconfig that
> does not exist so no cluster is reached: a config with `- label: zone`
> stops with `views[0].columns[0]: write label:zone with no space after the
> colon`, one with `sort: price/m` with the `sort: price` hint, and the
> owner's own `config.yaml` passes the config read and stops only at the
> connection. No cluster run: the change is in reading the file, before any
> connection.

> **Review 2026-11-04 (spec-refuter, the diff).** Applied: the `- label: X`
> hint only when `label:X` would be valid; sibling hints for upper-case
> titles (`sort: PRICE/M`); huge numbers keep an exponent; the doc test
> ignores line wrapping; docs and CHANGELOG no longer say every upper-case
> name is refused (only one with a `/`). Valid names pinned by
> `TestConfigViewValidNamesStillLoad` (`hugepages-2Mi`,
> `ephemeral-storage`, `MyLabel`, `example.com/MyLabel`, `<none>`).
> The flags and the legacy file do not go through these checks (the new
> functions are called only from the config.yaml readers); their trimming
> stays covered by `view_test.go` (group-by, extra labels with spaces),
> `flag_test.go` (resources) and `settings_test.go` ("both, comma
> separated with spaces" for `--labels-allow`). 49 cases in
> `configerrors_test.go` now.
