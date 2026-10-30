# Specs

Planning artefacts for `kube-node-viewer`. Principles and the change loop live
in [`../AGENTS.md`](../AGENTS.md); what shipped lives in
[`../CHANGELOG.md`](../CHANGELOG.md).

## Reading order

1. [`rebrand/`](rebrand/) — **active**. Rename module, binary, config file and
   docs away from the upstream name.
   [requirements](rebrand/requirements.md) · [design](rebrand/design.md) ·
   [tasks](rebrand/tasks.md)
2. [`decisions/`](decisions/) — decision records (index below).

Status values: **draft** (not reviewed) → **active** (reviewed, being
implemented) → **shipped** (all tasks done; historical, only dated
corrections) → **superseded** (link to the replacement).

## Decision records

| ADR | Decision | Status |
|-----|----------|--------|
| [001](decisions/ADR-001-import-as-new-repo.md) | Import upstream into a new repository instead of a GitHub fork | Accepted |

## Format

Copy from [`.templates/`](.templates/). `scripts/spec-lint.sh` enforces the
mechanical parts.

- **Feature spec** `<feature>/` (kebab-case):
  - `requirements.md` — context, concrete usage examples, requirements as
    `**R<n>** WHEN <condition> THEN the system SHALL <behavior>`, each with
    checkable acceptance criteria, counter-examples, explicit **Out of scope**.
  - `design.md` — current vs target state, components touched, and for each
    significant decision: choice, justification (strongest first),
    trade-offs, **when it would be wrong**, **reversal action**, alternatives.
    Invariants. Pure reference tables are exempt from the decision format.
  - `tasks.md` — Kiro checkbox list: `- [ ] T<n>. <task>` (`[-]` in
    progress, `[x]` done) with a `_Covers: R<n> · Depends on: … · Verified
    by: …_` line under each. Deferred work goes in a table with its
    promotion trigger.
- **Bugfix spec** `fix-<slug>/`: `bugfix.md` (current behavior with
  reproduction, expected, unchanged behavior that must not regress) +
  `tasks.md`; `design.md` only when the fix involves a decision.
- **Decision record** `decisions/ADR-NNN-<imperative-title>.md`: context with
  checkable evidence (file:line, command output, links), decision, rationale,
  consequences, when wrong, alternatives. Add it to the index above.

## Kiro and Claude Code

`.kiro/specs` is a symlink to this directory, so the Kiro spec panel lists
every folder here: `decisions/` shows up as an entry but is not a spec;
templates are hidden in `.templates/`. Claude Code reaches specs through
`AGENTS.md`.

## Rules

- Spec before code for anything in the "spec needed" table in `AGENTS.md`.
- No invented numbers: write `unmeasured` when a figure was not measured.
  Mark guesses `[ASSUMPTION]` and open questions `[TO CONFIRM]`; a spec with
  `[TO CONFIRM]` cannot move to **active**.
- Every spec set gets an independent adversarial review (`spec-refuter`)
  before it moves to **active**; record the review date in the spec header.
- Corrections are recorded in place as a dated note
  (`> **Corrected YYYY-MM-DD.** …`), never applied silently.
- Requirements describe behavior a user can observe, not function names.
