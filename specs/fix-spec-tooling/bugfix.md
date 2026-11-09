# Bugfix: spec-tooling

> Status: shipped (v0.15.0, 2026-11-09) · Reported: 2026-11-05 · Reviewed: 2026-11-05 (spec-refuter; 1 HIGH, 4 MEDIUM, 5 LOW, applied), 2026-11-05 (spec-refuter, code; 1 HIGH, 2 MEDIUM, 2 LOW, applied)

Two items of `TODO.md` (Follow-ups): "Bugfix specs are never marked shipped"
(found while writing [ADR-003](../decisions/ADR-003-release-from-changelog-headings.md))
and "spec-lint checks the decision sections only for `### D<n>` headings"
(found by the retroactive review of `aks-pricing` and `node-details`).

## Current behavior

1. `scripts/release-prepare.py --write` marks a spec shipped only when its
   `requirements.md` contains the text "Status: active" and every task is
   `[x]`. A bugfix spec has `bugfix.md`, not `requirements.md`, so it stays
   **active** after its release. Reproduction: the v0.14.0 release commit
   (5cedfd6) shipped `search-regex`, `search-negation`, `group-focus` and
   `release-provenance`, while the six bugfix specs it also shipped
   (`fix-config-errors`, `fix-hook-tests-git-env`,
   `fix-aws-pricing-region`, `fix-config-case`, `fix-export-search`,
   `fix-azure-price-product`; every task `[x]`, each added after v0.13.1
   and an ancestor of v0.14.0) are still **active** in their files and in
   `specs/README.md`.
2. The same function tests for that text anywhere in the file and
   replaces its first occurrence anywhere. A spec whose prose or a dated
   note quotes it (this file does) would be rewritten on the next release,
   and a draft spec whose prose quotes it would be shipped.
3. `scripts/spec-lint.sh` counts decisions with `^### D[0-9]+`. A design
   written with `## D1. …` (`aks-pricing`, `node-details`) counts zero
   decisions, so the "When this would be wrong" and "Reversal action"
   check never applies to it. Reproduction: delete both markers from
   `specs/node-details/design.md`; `bash scripts/spec-lint.sh` still prints
   `spec-lint: ok`.

## Expected behavior

**R1** WHEN `release-prepare.py --write` releases a version and a spec —
a feature spec through `requirements.md`, a bugfix spec (`specs/fix-*/`)
through `bugfix.md` — has the header line `> Status: active …` and every
task in its `tasks.md` is `[x]` THEN the system SHALL replace that header
line's status with `shipped (v<version>, <date>)`, reading the status only
from the file's first line that starts with `> Status:`, before its first
`## ` heading, and changing only
that line (in that file, and in its `design.md` and `tasks.md` when theirs
says active), and mark its `specs/README.md` entry **shipped** in
v<version>.

- [x] a bugfix spec with every task done is shipped, files and index
- [x] a bugfix spec with an open task (`[ ]` or `[-]`) stays active
- [x] a draft spec whose prose quotes "Status: active", even at the start
  of a later `> ` line, stays draft
- [x] a shipped spec whose note quotes "Status: active" is unchanged

**R2** WHEN a `design.md` names a decision with a heading of level two or
three, `D<n>` followed by `.`, `:`, a space or the end of the line
(`## D1. …`, `### D1: …`, `### D1`) THEN `spec-lint.sh` SHALL count it,
and SHALL fail when the design has fewer "When this would be wrong" or
"Reversal action" markers than decisions.

- [x] `## D1.` without the markers fails
- [x] `### D1:` without the markers still fails
- [x] both forms with the markers pass

> **Corrected 2026-11-05.** The totals rule here ("fewer markers than
> decisions") is replaced by a per-decision rule: each decision needs both
> markers in its own section
> ([fix-spec-lint-markers](../fix-spec-lint-markers/bugfix.md) R1). The
> heading rule above is unchanged; the three "without markers" cases now
> look for the per-decision message.

**R3** WHEN a spec's `specs/README.md` entry names a status
(`— **active**`) that differs from the status on the spec's own
`> Status:` line THEN `spec-lint.sh` SHALL fail, and `release-prepare.py`
SHALL warn on stderr when it ships a spec whose entry it could not mark.

- [x] an index entry saying draft for an active spec fails
- [x] a matching entry passes

The regression tests are the `release prepare:` `spec lint: decision` and `spec lint: index`
cases of `scripts/checks/test-checks.sh`, run by the `guardrails` job.

## Unchanged behavior

Each is a case of `scripts/checks/test-checks.sh`:

- A feature spec with every task done still ships: files and index line.
- No `[Unreleased]` entries: nothing is written, nothing is marked.
- The CHANGELOG text and compare links `--write` writes are byte-identical
  to today's for the same input.
- A heading that is not a decision (`## Decisions`) is not counted.
- An existing `### D1:` design with both markers still passes.

Also unchanged, not separately tested: the status values (draft, active,
shipped, superseded); the review gate on active and shipped specs (all six
bugfix specs already carry a `Reviewed:` entry); `spec-trace.py`, which
already reads active and shipped bugfix specs.

## Root cause

- `scripts/release-prepare.py` `ship_specs`: line 73 reads only
  `requirements.md` and tests the text anywhere; line 79 replaces its
  first occurrence anywhere.
- `scripts/spec-lint.sh` line 74 greps `^### D[0-9]+` only.

## Fix

Bugfix specs released before this merges are marked by hand the same way,
each with a dated note: today the six shipped in v0.14.0 (2026-11-05).
At merge time, check that no other `fix-*` spec with every task done sits
in a released tag. The tool handles every later release.

Not done: the marker check stays a count over the whole design, not per
decision (a design with two decisions and both markers under one passes);
`TODO.md` lists it.
