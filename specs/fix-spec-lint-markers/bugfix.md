# Bugfix: spec-lint-markers

> Status: draft · Reported: 2026-11-05 · Reviewed: —

`TODO.md` (Follow-ups): "spec-lint counts the 'When this would be wrong'
and 'Reversal action' markers over the whole design, not per decision",
found by the code review of [fix-spec-tooling](../fix-spec-tooling/bugfix.md).

## Current behavior

`scripts/spec-lint.sh` counts the decision headings of a `design.md`, then
counts the `**When this would be wrong**` and `**Reversal action**` markers
anywhere in the file, and fails only when either count is below the number
of decisions. A decision can therefore ship with no failure case or
reversal of its own, as long as another section carries extra markers.

Reproduction, in a scratch copy:

```
printf '\n## D1. A\n\n**When this would be wrong**: x. **Reversal action**: y.\n**When this would be wrong**: x. **Reversal action**: y.\n\n## D2. B\n\nNo markers.\n' >> specs/demo/design.md
bash scripts/spec-lint.sh   # prints "spec-lint: ok"
```

Every existing design already has both markers inside each of its own
decision sections (checked 2026-11-05 over the 16 designs on `main` with
the section rule of R1), so no design needs a correction.

## Expected behavior

**R1** WHEN a `design.md` has a decision heading (a level-2 or level-3
heading `D<n>` followed by `.`, `:`, a space or the end of the line) THEN
spec-lint SHALL require that decision's own section to contain at least
one `**When this would be wrong**` marker and at least one
`**Reversal action**` marker, and SHALL report each missing one as
`<spec>: D<n> has no 'When this would be wrong'` or
`<spec>: D<n> has no 'Reversal action'`. A decision's section runs from
its heading to the next heading of the same or a higher level (fewer `#`),
or to the end of the file; deeper headings (`####`) stay inside it.
Markers inside a dated note (`> **Corrected …**`) in the section count;
markers inside a fenced code block do not.

Regression tests, `scripts/checks/test-checks.sh`, each with
`# fix-spec-lint-markers R1` above it:

- `spec lint: markers under one decision only` (blocked, naming `D2`)
- `spec lint: markers in each decision` (passes)
- `spec lint: markers in a correction note` (passes)
- `spec lint: marker in a code block does not count` (blocked)
- `spec lint: #### heading stays inside its decision` (passes)
- `spec lint: ### decisions end at the next ## heading` (blocked when the
  markers sit after that heading)

## Unchanged behavior

- Which headings count as decisions: `## D1.`, `### D1:`, a bare
  `### D1`; `## Decisions` and `### Data flow` do not (fix-spec-tooling
  R2). Its three "without markers" cases keep passing, with the message
  they look for changed to the per-decision one.
- A design with no decision heading passes.
- ADRs (`specs/decisions/`) are not checked for these markers, as today.
- Every other spec-lint rule and message.
- `spec-lint: ok` on every design on `main`.

## Root cause

`scripts/spec-lint.sh:91-95`: `grep -c` over the whole `design.md` for the
decisions and for each marker, compared as totals.

## Not done

- The markers' content is not checked (an empty `**Reversal action**:`
  still counts).
