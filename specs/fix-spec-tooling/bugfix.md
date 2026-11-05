# Bugfix: spec-tooling

> Status: draft · Reported: 2026-11-05 · Reviewed: —

Two items of `TODO.md` (Follow-ups): "Bugfix specs are never marked shipped"
(found while writing [ADR-003](../decisions/ADR-003-release-from-changelog-headings.md))
and "spec-lint checks the decision sections only for `### D<n>` headings"
(found by the retroactive review of `aks-pricing` and `node-details`).

## Current behavior

1. `scripts/release-prepare.py --write` marks a spec shipped only when its
   `requirements.md` says `Status: active` and every task is `[x]`. A bugfix
   spec has `bugfix.md`, not `requirements.md`, so it stays **active** after
   its release. Reproduction: the v0.14.0 release commit (5cedfd6) shipped
   `search-regex`, `search-negation`, `group-focus` and `release-provenance`,
   while the six bugfix specs it also shipped (`fix-config-errors`,
   `fix-hook-tests-git-env`, `fix-aws-pricing-region`, `fix-config-case`,
   `fix-export-search`, `fix-azure-price-product`; every task `[x]`, each
   added after v0.13.1 and an ancestor of v0.14.0) are still **active** in
   their files and in `specs/README.md`.
2. `scripts/spec-lint.sh` counts decisions with `^### D[0-9]+`. A design
   written with `## D1. …` (`aks-pricing`, `node-details`) counts zero
   decisions, so the "When this would be wrong" and "Reversal action"
   check never applies to it. Reproduction: delete both markers from
   `specs/node-details/design.md`; `bash scripts/spec-lint.sh` still prints
   `spec-lint: ok`.

## Expected behavior

**R1** WHEN `release-prepare.py --write` releases a version and a bugfix
spec (`specs/fix-*/`) has `Status: active` in `bugfix.md` and every task in
its `tasks.md` is `[x]` THEN the system SHALL write
`Status: shipped (v<version>, <date>)` in place of `Status: active` in its
`bugfix.md`, and in its `tasks.md` and `design.md` when they carry it, and
mark its `specs/README.md` entry **shipped** in v<version>, as it does for a
feature spec.

- [ ] a bugfix spec with every task done is shipped, files and index
- [ ] a bugfix spec with an open task (`[ ]` or `[-]`) stays active
- [ ] a draft bugfix spec stays draft

_Test: ci — the `guardrails` job runs the `release prepare:` cases of
`scripts/checks/test-checks.sh` (a throwaway repository with a feature and
a bugfix spec)._

**R2** WHEN a `design.md` names a decision with a heading of level two or
three, `D<n>` followed by `.`, `:` or a space (`## D1. …`, `### D1: …`)
THEN `spec-lint.sh` SHALL count it, and SHALL fail when the design has
fewer "When this would be wrong" or "Reversal action" markers than
decisions.

- [ ] `## D1.` without the markers fails
- [ ] `### D1:` without the markers still fails
- [ ] both forms with the markers pass

_Test: ci — the `guardrails` job runs the `spec lint: decision` cases of
`scripts/checks/test-checks.sh`._

## Unchanged behavior

- Feature specs ship exactly as today (`requirements.md`, `design.md`,
  `tasks.md`, index line).
- A spec with an open task, a draft spec, or no `[Unreleased]` entries:
  nothing is marked.
- The status values stay draft, active, shipped, superseded; the review
  gate on active and shipped specs is unchanged, so a shipped bugfix spec
  still needs its `Reviewed:` entry (all six have one).
- The version computed from `[Unreleased]`, the CHANGELOG rewrite and the
  compare links are unchanged.
- `spec-trace.py` already reads bugfix specs that are active or shipped;
  marking them shipped changes nothing there.
- Headings that are not decisions (`## Decisions`, `### Data flow`) are not
  counted.

## Root cause

- `scripts/release-prepare.py` `ship_specs` (lines 72-74) reads only
  `requirements.md`.
- `scripts/spec-lint.sh` line 74 greps `^### D[0-9]+` only.

## Fix

The six bugfix specs shipped in v0.14.0, which is already released: their
status is set to `shipped (v0.14.0, 2026-11-05)` by hand in this change,
each with a dated note saying why, and their index entries likewise. The
tool handles every later one.
