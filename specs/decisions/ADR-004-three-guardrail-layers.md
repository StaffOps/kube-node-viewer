# ADR-004: Check the same rules in three layers: git hooks, agent hooks and CI

> Status: Accepted · Date: 2026-11-04 · Recorded after the fact: the
> layers were added between 2026-10-30 (`4f4be0e`, agent hooks) and
> 2026-11-03 (`7e0025f`, git hooks; `aaf0bab` and `e84ed1f`, content and
> secrets checks).

## Context

- This repository is public. Anything pushed can be copied before anyone
  notices it, so a secret or a real cluster name has to be caught before
  the push, not after.
- Changes come from people and from two coding agents, Claude Code and
  Kiro. Agents act fast and read refusals as instructions: a check that
  says why it refused gets the right fix. A plain non-zero exit tends to
  get retried or worked around.
- Each layer misses something:
  - git hooks are per clone, are installed only by `make hooks` (or the
    session-context hook), and `--no-verify` skips them;
  - agent hooks see only agent sessions;
  - CI sees only what was pushed.

## Decision

The rules live once, as scripts, and three layers run them:

| Layer | Where | What it is for |
|-------|-------|----------------|
| Git hooks | `.githooks/` (`pre-commit`, `commit-msg`, `pre-push`), installed with `core.hooksPath` | everyone, at commit and push, before anything leaves the machine |
| Agent hooks | `scripts/agent-hooks/` (`session-context`, `guard-destructive`, `commit-guard`, `commit-reminder`, `pr-guard`), wired in `.claude/settings.json` and the two Kiro files | the agent, before it acts, with the reason in words |
| CI | `test.yaml` jobs and `release.yaml` | the gate: nothing merges or releases without it |

The shared scripts are:

- `scripts/checks/staged.sh` for the pre-commit checks;
- `scripts/checks/content.sh` for real-environment data, in modes for
  every file, the staged diff, a commit message and a commit range;
- gitleaks for secrets.

Each rule's row in the `AGENTS.md` Guardrails table names the layers that
run it and how to fix a refusal. The `guardrails` CI job tests the hooks
and checks themselves (`scripts/checks/test-checks.sh`, run once plain
and once with `GIT_DIR` set the way a hook sets it).

## Rationale

1. **Early, explained, and enforced are different jobs.** The agent hook
   explains before the command runs. The git hook stops the commit
   before the push. CI enforces what the first two could not, because
   they were skipped or not installed.
2. **One script per rule.** Each layer calls the same script with a
   different mode, so the layers cannot drift apart in what they accept.
3. **Leaks cost more than duplicate checks.** Running the same content
   check up to three times is cheap. A cluster endpoint or a key in a
   public history is permanent
   ([ADR-005](ADR-005-never-rewrite-published-history.md)).

## Consequences

- Adding a rule touches up to three wirings: the git hook, the agent
  hooks (`.claude/settings.json` and both Kiro files), and a CI job. The
  `AGENTS.md` table is the checklist.
- The hook scripts are code that can break the repository they run in.
  On 2026-11-04 a push from a worktree ran the hook tests with the hook's
  `GIT_DIR` and they wrote into the real repository
  (`specs/fix-hook-tests-git-env/`). CI now runs those tests under the
  same environment and checks that the repository is unchanged.
- The private deny list (`.git/info/denylist`, and the `KNV_DENYLIST`
  secret in CI) is never committed, so a clone without it catches only
  the generic patterns until CI runs.
- The Kiro hooks follow the Kiro docs but have not yet been confirmed in
  a real session (`TODO.md`).

## When this would be wrong

- If the agents stop being a main source of changes, the agent-hook
  layer is upkeep for little gain; git hooks and CI would be enough.
- If GitHub's server-side push protection came to cover the custom
  content rules (deny list, cluster identifiers), the pre-push part could
  move there.

## Alternatives discarded

- **CI only.** It catches everything, but only after the push, which is
  too late for a secret or a real name in a public repository.
- **Git hooks only.** They can be skipped with `--no-verify` and are not
  installed by a fresh clone. They also give an agent an exit code, not a
  reason it can act on.
- **The `pre-commit` framework.** It adds a Python tool and a hook
  manager to every clone to run what are mostly shell scripts. Agent
  hooks and CI would still need wiring of their own.
- **Separate implementations per layer.** The same rule would accept
  different things in each place; that drift is what the shared scripts
  prevent.
