# ADR-005: Never rewrite published history; fix forward, and rotate what leaked

> Status: Accepted · Date: 2026-11-04 · Recorded after the fact: the rules
> below were written into `AGENTS.md`, the hooks and the rulesets between
> 2026-10-30 (`4f4be0e`) and 2026-11-03 (`aa56b4a`).

## Context

- The history was rewritten at least once in the project's first days.
  [ADR-001](ADR-001-import-as-new-repo.md) and
  [`rebrand`](../rebrand/requirements.md) cite the import commit as
  `af96088` on top of `6b6779a`. Neither object exists any more: the
  import is now `f3aceaa`, on top of `11dc2e0`
  (`git log --reverse | head`). Every SHA that a spec, an ADR, an issue
  or a commit message had quoted for the old history stopped pointing
  anywhere.
- The repository is public. A pushed commit can be cloned, forked or
  cached before a rewrite, so a rewrite does not take back what was
  published.
- The release flow tags `main`, and the published binaries carry the
  commit they were built from (`-X main.commit=…`). Rewriting `main`
  would orphan those commits.
- Branches are shared between people and agents. A force-push on a
  branch someone else has checked out silently discards their base.

## Decision

1. **`main` and `v*` tags are never rewritten.** The rulesets forbid
   force-pushes and deletions on `main`, and creating, moving or deleting
   `v*` tags. Only the release deploy key bypasses them, and the release
   workflow only adds commits and tags.
2. **Branches are fixed forward.** Bring `main` into a branch by merging
   it, not by rebasing. Never force-push someone else's branch
   (skill `pr-review`). Force-push your own branch only with the owner's
   explicit approval (`AGENTS.md` Conventions).
3. **Agents ask first, every time.** Before an agent runs a command that
   rewrites or deletes published git state or wipes local work, the
   `guard-destructive` agent hook stops it. Claude Code asks the owner;
   Kiro blocks the command, and the owner runs it themselves. The
   commands it stops are:
   - `push` with `--force`, `--delete`, `--mirror`, `--prune`, or a `+`
     or `:` refspec;
   - `tag -d`;
   - `filter-repo` and `filter-branch`;
   - `reset --hard` and `clean -f`;
   - `branch -D`;
   - discarding `checkout`, `switch` and `restore`;
   - `stash drop` and `stash clear`;
   - `update-ref -d`;
   - `reflog expire` and `reflog delete`.
4. **A leak is answered by rotation and removal, not by a rewrite.**
   - A secret that reached a commit is revoked and rotated, then removed
     in a new commit.
   - Real-environment data (names, endpoints, IPs, accounts) is removed
     in a new commit, and its pattern goes into the private deny list so
     it is caught next time.
   - The guardrails ([ADR-004](ADR-004-three-guardrail-layers.md)) exist
     so that this case stays rare.

## Rationale

1. **A rewrite does not undo a publication.** Once something is in a
   public history, the only fix that holds is to make it worthless
   (rotation) or to stop repeating it (removal plus the deny list).
2. **Stable SHAs are part of the record.** Specs, ADRs, CHANGELOG compare
   links, tags and binaries all cite commits. The one rewrite so far
   broke two citations in shipped documents.
3. **Merging keeps every collaborator's base valid.** It costs merge
   commits in the log, which is cheap next to lost work.

## Consequences

- The log carries merge commits and "fix the previous commit" commits.
  Branch commits are tidied before the first push, not after.
- A mistake in a pushed commit message stays. The `commit-msg` hook and
  the `secrets` job (`content.sh --commits`) catch the usual ones before
  the push or the merge.
- ADR-001 and the `rebrand` spec keep the old SHAs, with dated
  corrections giving the current ones.

## When this would be wrong

- If something that must never be public (a credential that cannot be
  rotated, or personal data under a legal removal request) reaches
  `main`, a rewrite may be required in addition to removal. That is an
  owner decision, made outside this policy:
  - suspend the ruleset;
  - rewrite;
  - record the old-to-new SHA mapping for every cited commit as dated
    corrections;
  - restore the ruleset;
  - ask GitHub Support to purge cached views.

## Alternatives discarded

- **Rewrite to remove any mistake** (filter-repo, force-push). It breaks
  every citation and every collaborator's clone, and the data has already
  been published.
- **Linear history through rebase and force-push on branches.** It gives
  a cleaner log, but it discards others' bases and needs force-pushes
  that the agent guard would have to allow routinely.
- **Rely on the rulesets alone.** They protect only `main` and tags. Work
  branches and local state are covered only by the agent hook and the
  `AGENTS.md` rules.
