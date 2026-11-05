---
inclusion: always
---

# Spec workflow (kube-node-viewer)

Rules live in `AGENTS.md` at the repo root (loaded automatically); this file
only pins what Kiro would otherwise do differently.

- Specs live in `specs/<feature>/` at the repo root. `.kiro/specs` is a
  symlink to it — create and edit specs under `specs/`, never a separate copy.
- Follow the format in #[[file:specs/README.md]] (WHEN … THEN the system
  SHALL …; decisions with justification, trade-offs, when wrong, reversal;
  tasks with a status table).
- Before implementing a spec, it needs an independent review (agent
  `spec-refuter`).
- Findings outside the task go to `TODO.md`, not into the current diff.
- Project skills live in `.kiro/skills/` (shared with Claude Code through a
  symlink); hook scripts live in `scripts/agent-hooks/` and are wired in
  `.kiro/hooks/kube-node-viewer.json` (Kiro IDE 1.x and CLI V3 both load it).
