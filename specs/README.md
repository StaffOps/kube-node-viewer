# Specs

Planning artefacts for `kube-node-viewer`. Principles and the change loop live
in [`../AGENTS.md`](../AGENTS.md); what shipped lives in
[`../CHANGELOG.md`](../CHANGELOG.md).

## Reading order

1. [`rebrand/`](rebrand/) — **shipped** in v0.10.0. Rename module, binary, config file and
   docs away from the upstream name.
   [requirements](rebrand/requirements.md) · [design](rebrand/design.md) ·
   [tasks](rebrand/tasks.md)
2. [`interactive-tui/`](interactive-tui/) — **shipped** in v0.8.0. Stage 1
   of the sofka-style rework: no-flag start, popup panels (Filters, Group by,
   Sort, Columns, Context), folding, per-cluster state, change-driven
   redraw. The owner's UX changes are dated notes in its requirements.
   [requirements](interactive-tui/requirements.md) ·
   [design](interactive-tui/design.md) · [tasks](interactive-tui/tasks.md)
3. [`views-and-export/`](views-and-export/) — **shipped** in v0.9.0. Excluding filters,
   named views (`v`), export of the screen to CSV and JSON (`e`).
   [requirements](views-and-export/requirements.md) ·
   [design](views-and-export/design.md) · [tasks](views-and-export/tasks.md)
4. [`recommendations/`](recommendations/) — **shipped** in v0.9.0. A Recommendations
   page next to Overview, reached with `:` or `tab`: idle cost,
   consolidation, rightsizing, newer generation, Graviton, old nodes, Spot
   last; rules, no AI.
   [requirements](recommendations/requirements.md) ·
   [design](recommendations/design.md) · [tasks](recommendations/tasks.md)
5. [`aks-pricing/`](aks-pricing/) — **shipped** in v0.9.0. AKS node prices from the
   public Azure Retail Prices API, per node by provider-ID cloud.
   [requirements](aks-pricing/requirements.md) ·
   [design](aks-pricing/design.md) · [tasks](aks-pricing/tasks.md)
6. [`node-details/`](node-details/) — **shipped** in v0.9.0. `enter` on a node opens
   a popup with its details: resources, conditions, taints, labels.
   [requirements](node-details/requirements.md) ·
   [design](node-details/design.md) · [tasks](node-details/tasks.md)
7. [`label-visibility/`](label-visibility/) — **shipped** in v0.10.0. Allow and deny
   lists of label key patterns in `~/.config/kube-node-viewer/config.yaml`
   decide which node labels the panels offer; `a` shows all.
   [requirements](label-visibility/requirements.md) ·
   [design](label-visibility/design.md) · [tasks](label-visibility/tasks.md)
8. [`sort-invert/`](sort-invert/) — **shipped** in v0.9.1. `I` (shift+i) flips the
   sort between ascending and descending from the main screen.
   [requirements](sort-invert/requirements.md) ·
   [design](sort-invert/design.md) · [tasks](sort-invert/tasks.md)
9. [`decisions/`](decisions/) — decision records (index below).

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
