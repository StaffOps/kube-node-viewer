# Tasks: fix-spec-tooling

> Implements [bugfix](bugfix.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done

- [x] T1. `ship_specs` ships feature and `fix-*` specs from their header line only; `release prepare:` cases in `test-checks.sh`
  - _Covers: R1 · Depends on: — · Verified by: the new cases; `--write --root` on a `cp -R` copy prints exactly the expected `shipped spec:` lines_
- [x] T2. `spec-lint.sh` counts `## D<n>` and `### D<n>` decisions; `spec lint: decision` cases
  - _Covers: R2 · Depends on: — · Verified by: the new cases; `spec-lint: ok` on every existing design_
- [x] T3. Mark the six bugfix specs shipped in v0.14.0 by hand with a dated note each, and their index entries; dated note in ADR-003; remove both `TODO.md` items; add the per-decision marker item
  - _Covers: R1 · Depends on: T1 · Verified by: `spec-lint`, `spec-trace`; after the marking, `--write` on a scratch copy prints no `shipped spec:` line for the six_
  - [x] the six specs and their index entries say shipped in v0.14.0
- [x] T5. spec-lint checks each index entry's status against the spec's; `release-prepare.py` warns when it cannot mark an entry (code review)
  - _Covers: R3 · Depends on: T1 · Verified by: the `spec lint: index` cases; `spec-lint: ok` on today's index_
- [x] T4. Verify
  - _Covers: R1, R2, R3 · Depends on: T1, T2, T3, T5 · Verified by: `scripts/verify.sh`, `scripts/checks/test-checks.sh`, dated note here_

> **Verified 2026-11-05.** `scripts/checks/test-checks.sh`: 153 passed,
> including the five `spec lint: decision` cases and the seven
> `release prepare:` cases (a done bugfix spec ships, files and index; an
> open one and a draft quoting the status stay; a shipped spec's note is
> unchanged; a feature spec ships; the CHANGELOG as before; nothing marked
> with an empty `[Unreleased]`). The first run caught a draft shipped by a
> later `> Status: active` line, so the status is now read only from the
> first `> Status:` line. The CHANGELOG written by `--write` is
> byte-identical to the one `origin/main`'s script writes for the same
> input. `--write --root` on a `cp -R` copy with a made-up `[Unreleased]`
> entry printed exactly the six `shipped spec:` lines before the hand
> marking and none after it. `spec-lint: ok` on every existing design
> with the new heading rule. shellcheck clean.

> **Verified 2026-11-05 (after the code review).** The index lists this
> spec as **active**; spec-lint now fails when an index entry's status
> differs from the spec's (R3, T5), and `release-prepare.py` warns when it
> cannot mark an entry. The status is read only from the header, before
> the first `## `. `test-checks.sh`: 155 passed. Mutation runs: with
> `origin/main`'s `release-prepare.py` three `release prepare:` cases fail
> (the done bugfix spec, the draft quoting the status, the shipped spec's
> note); with the old `### D` regex the `## D1.` case fails; without the
> index check the `spec lint: index` case fails.
