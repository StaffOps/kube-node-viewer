# ADR-004: Check the same rules in three layers: git hooks, agent hooks and CI

> Status: Accepted · Date: 2026-11-04 · Recorded after the fact. The layers
> were added in steps:
>
> - agent hooks: 2026-10-30 (`4f4be0e`);
> - git hooks: 2026-11-03 (`7e0025f`);
> - content and secrets checks: 2026-11-03 (`aaf0bab`, `e84ed1f`).
>
> Reviewed: 2026-11-04 (spec-refuter; 3 HIGH, 7 MEDIUM, 7 LOW, applied)

## Context

- The repository is public. A pushed commit can be copied before anyone
  notices, so a secret or a real cluster name has to be caught before the
  push, not after.
- Changes come from people and from two coding agents, Claude Code and
  Kiro. Agents act fast and read refusals as instructions: a check that
  says why it refused gets the right fix, while a bare non-zero exit tends
  to be retried or worked around.
- Each layer misses something:
  - git hooks are per clone, installed only by `make hooks` or the
    session-context hook, and `--no-verify` skips them;
  - agent hooks see only agent sessions;
  - CI sees only what was pushed.

## Decision

The rules live as scripts, mostly one per rule, and three layers run them:

| Layer | Where | What it is for |
|-------|-------|----------------|
| Git hooks | `.githooks/` (`pre-commit`, `commit-msg`, `pre-push`), installed with `core.hooksPath` | everyone, at commit and push, before anything leaves the machine |
| Agent hooks | `scripts/agent-hooks/` (`session-context`, `guard-destructive`, `commit-guard`, `commit-reminder`, `pr-guard`), wired in `.claude/settings.json` and the two Kiro files | the agent, before it acts, with the reason in words |
| CI | `test.yaml` jobs and `release.yaml` | the gate: nothing merges or releases without it |

Shared scripts:

- `scripts/checks/staged.sh` runs the pre-commit checks;
- `scripts/checks/content.sh` checks for real-environment data, in modes
  for every file, the staged diff, a commit message and a commit range;
- gitleaks finds secrets.

The `AGENTS.md` Guardrails table lists, for each rule, the layers that run
it and how to fix a refusal. The `guardrails` CI job tests the hooks and
the checks themselves with `scripts/checks/test-checks.sh`, run once
plainly and once with `GIT_DIR` set the way a hook sets it.

Not every rule runs in every layer, and one rule has two implementations:

- **Released CHANGELOG sections** are checked twice in different code:
  the awk in `staged.sh` at commit, and `scripts/changelog-check.py` in
  `verify.sh` and CI.
- **The Apache header on new files** is checked by `pre-commit` and
  `commit-guard`, not by CI.
- **Destructive git commands** are stopped only by the agent hook
  `guard-destructive`. For pushes to `main` and tags, the rulesets back it
  up.
- **The Conventional Commit subject** is checked only by the `commit-msg`
  git hook; the agent hook only reminds.

## Rationale

1. **Early, explained and enforced are different jobs.** The agent hook
   explains before the command runs, the git hook stops the commit before
   the push, and CI enforces what the first two missed because they were
   skipped or not installed.
2. **Shared scripts keep the layers in step.** Where a rule has one
   script, each layer calls it with a different mode, so the layers
   cannot accept different things. The exceptions above are where drift
   is possible.
3. **Leaks cost more than duplicate checks.** Running the same content
   check up to three times is cheap. A cluster endpoint or a key in a
   public history stays there
   ([ADR-005](ADR-005-never-rewrite-published-history.md)).

## Consequences

- Adding a rule touches up to three wirings: the git hook, the agent
  hooks (`.claude/settings.json` and both Kiro files), and a CI job. The
  `AGENTS.md` table is the checklist.
- The two CHANGELOG implementations can disagree; a change to one needs
  the same change to the other.
- A rule without a CI layer (the Apache header, the commit subject)
  passes when someone commits with `--no-verify` or without the hooks
  installed.
- Admins can bypass the `main` ruleset through a PR (bypass mode
  `pull_request`) and merge without the CI checks passing. CI is the gate
  for everyone else.
- The hook scripts are code that can damage the repository they run in.
  On 2026-11-04 a push from a worktree ran the hook tests with the hook's
  `GIT_DIR`, and they wrote into the real repository
  (`specs/fix-hook-tests-git-env/`). CI now runs those tests under the
  same environment and checks that the repository is unchanged.
- The private deny list (`.git/info/denylist`, and the `KNV_DENYLIST`
  secret in CI) is never committed. A clone without it catches only the
  generic patterns until CI runs.
- The Kiro hooks follow the Kiro docs but have not yet been confirmed in
  a real session (`TODO.md`).

## When this would be wrong

- If agents stop being a main source of changes, the agent-hook layer is
  upkeep for little gain, and git hooks plus CI would do.
- If GitHub's server-side push protection came to cover the custom
  content rules (the deny list, cluster identifiers), the pre-push part
  could move there.

## Reversal

Remove a layer by unwiring it:

- the agent hooks from `.claude/settings.json` and the two Kiro files;
- the git hooks by unsetting `core.hooksPath`.

CI alone stays the gate. The shared scripts need no change.

## Alternatives discarded

- **CI only.** It catches everything, but only after the push, which is
  too late for a secret or a real name in a public repository.
- **Git hooks only.** `--no-verify` skips them and a fresh clone lacks
  them; they give an agent an exit code, not a reason.
- **The `pre-commit` framework.** It adds a Python tool and a hook
  manager to every clone to run what are mostly shell scripts, and agent
  hooks and CI would still need their own wiring.
- **Separate implementations per layer.** The same rule would accept
  different things in each place. The CHANGELOG check shows that cost on
  a small scale.
