---
name: spec-new
description: Use when starting any new feature, behavior change, or bugfix in kube-node-viewer, or when the user asks to "write a spec", "plan a feature", "criar spec". Drafts specs/<feature>/ (requirements → design → tasks) from the templates, asking the questions that turn a vague idea into testable WHEN/THEN/SHALL requirements.
---

# New spec (kube-node-viewer)

Rules: `AGENTS.md` and `specs/README.md`. Templates: `specs/.templates/`.
If this file disagrees with them, they win — fix this file.

## 1. Classify

| Request | Output |
|---------|--------|
| Feature / behavior change | `specs/<kebab-name>/requirements.md`, `design.md`, `tasks.md` |
| Bug | `specs/fix-<slug>/bugfix.md` + `tasks.md` |
| One decision only | `specs/decisions/ADR-NNN-<imperative-title>.md` (next free number) + index row in `specs/README.md` |
| Trivial (typo, bump, comment) | no spec — say so and stop |

## 2. Ask before writing (one round, only what you cannot infer)

Read the relevant code first (`pkg/model` for UI, `pkg/client` for cluster
data, `cmd/*/flag.go` for flags). Then ask only the gaps:

- Who uses it and what do they see today? Ask for one concrete example:
  command, cluster shape, expected screen.
- Edge cases for this tool: empty cluster, thousands of nodes, node without
  the label, pricing disabled/unavailable, Karpenter `nodeclaims` absent,
  terminal narrower than the table.
- Does it add a flag? Then: name, default, config-file key (flag > config file;
  an env var only if there is a standard one, like `KUBECONFIG`), and whether `-h` text changes.
- What must not change (invariants: requests not usage, existing flags,
  `eks-node-viewer/*` label compatibility).

Mark anything unanswered `[TO CONFIRM]`; never invent numbers (`unmeasured`).

## 3. Write

1. `requirements.md` — examples first, then `**R<n>** WHEN … THEN the system
   SHALL …` each with checkable acceptance boxes, counter-examples, out of scope.
2. `design.md` — current state with file:line, target state, decisions in the
   D-format (choice, justification, trade-offs, when wrong, reversal action),
   invariants, failure modes, test plan.
3. `tasks.md` — checkbox list (`- [ ] T<n>. …` + `_Covers · Depends on ·
   Verified by_` line); each task < 1 day.
4. Add the spec to "Reading order" in `specs/README.md` as **draft**.
5. Run `scripts/spec-lint.sh`.

## 4. Hand off

Request the independent review (agent `spec-refuter`) and list open
`[TO CONFIRM]` items for the owner. Do not implement until the owner moves
the spec to **active**.
