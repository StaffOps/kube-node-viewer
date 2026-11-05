# Tasks: fix-spec-tooling

> Implements [bugfix](bugfix.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done

- [ ] T1. `ship_specs` also ships `fix-*` specs from `bugfix.md`; `release prepare:` cases in `test-checks.sh`
  - _Covers: R1 · Depends on: — · Verified by: the new cases; a `--write` dry run on a scratch copy_
- [ ] T2. `spec-lint.sh` counts `## D<n>` and `### D<n>` decisions; `spec lint: decision` cases
  - _Covers: R2 · Depends on: — · Verified by: the new cases; `spec-lint: ok` on every existing design_
- [ ] T3. Mark the six bugfix specs shipped in v0.14.0 with a dated note each; dated note in ADR-003; remove both `TODO.md` items
  - _Covers: R1 · Depends on: T1 · Verified by: `spec-lint`, `spec-trace`, `verify.sh`_
- [ ] T4. Verify
  - _Covers: R1, R2 · Depends on: T1, T2, T3 · Verified by: `scripts/verify.sh`, `scripts/checks/test-checks.sh`, dated note here_
