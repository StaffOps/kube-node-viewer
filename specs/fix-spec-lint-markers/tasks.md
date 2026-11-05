# Tasks: fix-spec-lint-markers

> Implements [bugfix](bugfix.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done

- [ ] T1. `spec-lint.sh` checks the two markers inside each decision's own section; the six `spec lint:` cases in `test-checks.sh`; the three fix-spec-tooling R2 cases look for the new message
  - _Covers: R1 · Depends on: — · Verified by: the new cases; `spec-lint: ok` on every design on `main`_
- [ ] T2. Remove the `TODO.md` item
  - _Covers: R1 · Depends on: T1 · Verified by: review_
- [ ] T3. Verify
  - _Covers: R1 · Depends on: T1, T2 · Verified by: `scripts/verify.sh`, `scripts/checks/test-checks.sh`, shellcheck; dated note here_
