# Tasks: fix-spec-lint-markers

> Implements [bugfix](bugfix.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done

- [x] T1. `spec-lint.sh` checks the two markers inside each decision's own section; the ten `spec lint:` cases in `test-checks.sh`; the three fix-spec-tooling R2 cases look for the new message
  - _Covers: R1 · Depends on: — · Verified by: the new cases; `spec-lint: ok` on every design on `main`_
- [x] T2. Docs: the marker strings and the per-decision rule in the `specs/README.md` format rules; a dated correction under fix-spec-tooling R2 and its verify note; remove the `TODO.md` item
  - _Covers: R1 · Depends on: T1 · Verified by: review_
- [x] T3. Verify
  - _Covers: R1 · Depends on: T1, T2 · Verified by: `scripts/verify.sh`, `scripts/checks/test-checks.sh`, shellcheck; dated note here_

> **Verified 2026-11-05.** `scripts/checks/test-checks.sh`: 183 passed,
> including the ten new `spec lint:` cases and the three fix-spec-tooling
> R2 cases on the new message. Against `origin/main`'s `spec-lint.sh` the
> same suite fails nine cases (the three adjusted ones and the six that
> must block or name a decision); the four no-false-positive cases pass
> under both. `spec-lint: ok` on every design on `main`, also with
> `mawk 1.3.4` as `awk` (the regexes use no `{n,m}`). The reproduction in
> Current behavior now reports `node-details: D3 (design.md:28)`.
> shellcheck clean on `spec-lint.sh` and `test-checks.sh`.
