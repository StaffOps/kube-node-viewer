# Bugfix: spec-lint-markers

> Status: active · Reported: 2026-11-05 · Reviewed: 2026-11-05 (spec-refuter; 1 HIGH, 4 MEDIUM, 5 LOW, applied)

`TODO.md` (Follow-ups): "spec-lint counts the 'When this would be wrong'
and 'Reversal action' markers over the whole design, not per decision",
found by the code review of [fix-spec-tooling](../fix-spec-tooling/bugfix.md).

## Current behavior

`scripts/spec-lint.sh` counts the decision headings of a `design.md`, then
counts the `**When this would be wrong**` and `**Reversal action**` markers
anywhere in the file, and fails only when either count is below the number
of decisions. A decision can therefore ship with no failure case or
reversal of its own, as long as another section carries extra markers.

Reproduction, on `main` (`specs/node-details/design.md` has one decision,
D1, with one of each marker):

```
printf '\n## D2. A\n\n**When this would be wrong**: x. **Reversal action**: y.\n**When this would be wrong**: x. **Reversal action**: y.\n\n## D3. B\n\nNo markers.\n' >> specs/node-details/design.md
bash scripts/spec-lint.sh   # prints "spec-lint: ok"; D3 has no marker
git checkout specs/node-details/design.md
```

Every existing design already has both markers inside each of its own
decision sections: checked 2026-11-05 over the 16 designs (63 decisions)
on `main` with the section rule of R1, so no design needs a correction.

## Expected behavior

**R1** WHEN a `design.md` has a decision heading (a level-2 or level-3 ATX
heading `D<n>` followed by `.`, `:`, a space or the end of the line) THEN
spec-lint SHALL require that decision's own section to contain at least
one `**When this would be wrong**` marker and at least one
`**Reversal action**` marker (those exact strings), and SHALL report each
missing one as `<spec>: D<n> (design.md:<line>) has no 'When this would be
wrong'` or `… has no 'Reversal action'`, `<line>` being the heading's line.

A decision's section runs from its heading to the next decision heading
of any level, or the next other heading of the same or a higher level
(fewer `#`), or the end of the file; deeper headings (`####`) stay inside
it. A marker counts only inside its own decision's section, including in a
dated note (`> **Corrected …**`) there; a note placed under another
decision covers that one, so a misplaced note has to move. Fenced code
blocks are skipped: a fence opens on a line starting, after at most three
spaces, with three backticks or three tildes, and closes only on a fence
line of the same character; markers and headings inside a fence neither
count nor start or end a section. A fence inside a quote (`> ```…`) or a
list item indented by four or more spaces is not treated as a fence.
Setext headings (underlined) are not seen, as today.

Regression tests, `scripts/checks/test-checks.sh`, each with
`# fix-spec-lint-markers R1` above it:

- `spec lint: markers under one decision only` (blocked, naming `D2`)
- `spec lint: markers in each decision` (passes)
- `spec lint: a ## decision ends at a ### decision` (blocked, naming `D1`)
- `spec lint: markers in a correction note` (passes)
- `spec lint: only the reversal is missing` (blocked, `D1 … has no 'Reversal action'`)
- `spec lint: marker in a code block does not count` (blocked)
- `spec lint: marker in a ~~~ block does not count` (blocked)
- `spec lint: heading inside a fence ends nothing` (passes)
- `spec lint: #### heading stays inside its decision` (passes)
- `spec lint: ### decisions end at the next ## heading` (blocked)

## Unchanged behavior

- Which headings count as decisions: `## D1.`, `### D1:`, a bare
  `### D1`; `## Decisions` and `### Data flow` do not (fix-spec-tooling
  R2). Covered by its cases `spec lint: decision …`, whose three "without
  markers" cases now look for the per-decision message.
- A design with no decision heading passes. Covered by every
  `spec lint:` case on the demo spec, whose design has no decision.
- ADRs (`specs/decisions/`) are not checked for these markers, as today.
  _Test: manual — read from `spec-lint.sh`: the marker check runs only on
  `specs/*/design.md`._
- Every other spec-lint rule and message.
- `spec-lint: ok` on every design on `main`.

## Root cause

`scripts/spec-lint.sh:91-95`: `grep -c` over the whole `design.md` for the
decisions and for each marker, compared as totals.

## Not done

- The markers' content is not checked (an empty `**Reversal action**:`
  still counts).
