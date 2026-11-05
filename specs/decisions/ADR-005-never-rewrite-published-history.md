# ADR-005: Never rewrite published history; fix forward, and rotate what leaked

> Status: Proposed, until the owner confirms the account of the rewrite
> below. · Date: 2026-11-04 · Recorded after the fact: the rules were
> written into `AGENTS.md`, the hooks and the rulesets between 2026-10-30
> (`4f4be0e`) and 2026-11-03 (`aa56b4a`). · Reviewed: 2026-11-04
> (spec-refuter; 3 HIGH, 7 MEDIUM, 7 LOW, applied)

## Context

- The history was rewritten once, in the project's first days.
  [ADR-001](ADR-001-import-as-new-repo.md) and
  [`rebrand`](../rebrand/requirements.md) cite the import commit as
  `af96088`, on top of `6b6779a`. Neither commit is reachable from any
  branch or tag any more, although GitHub still serves both by SHA
  (`gh api repos/StaffOps/kube-node-viewer/commits/af96088`).
- The current root is `11dc2e0` (`git rev-list --max-parents=0 HEAD`),
  and the import is `f3aceaa` (`git log -1 f3aceaa`).
- What changed in the rewrite:
  - `af96088` and `f3aceaa` have the same tree (`8dc742f9…`) and the same
    author and committer dates; only the parent differs;
  - `6b6779a` was the initial commit created and signed by GitHub, and
    `11dc2e0` has the same tree (`aa449461…`) but is unsigned.
- When and how: 16 commits share the committer date
  2026-10-31T10:53:02-03:00, among them `4f4be0e`, which added the
  `guard-destructive` hook. So the rewrite happened after the guard
  existed, by a command the guard either asked about or did not cover.
  **[TO CONFIRM by the owner: when, why, and with what command.]**
- The repository is public. A pushed commit can be cloned, forked or
  cached before a rewrite, and as the old SHAs show, GitHub keeps serving
  it afterwards. A rewrite does not take back what was published.
- Released binaries record the commit they were built from: GoReleaser's
  default ldflags set `-X main.commit={{.Commit}}`. Rewriting `main`
  would leave those binaries pointing at commits no branch reaches.
- People and agents share branches, so a force-push on a branch someone
  else has checked out silently discards the base they work on.

## Decision

1. **`main` and `v*` tags are not rewritten.** The `main` ruleset forbids
   force-pushes and deletion. The tag ruleset forbids creating, moving or
   deleting `v*` tags. Both let two actors bypass them:
   - the deploy key, bypass mode `always`; the release workflow uses it
     and only adds commits and tags;
   - the admin role: bypass mode `always` on tags, so an admin can still
     move or delete a `v*` tag, and `pull_request` on `main`.
2. **Branches are fixed forward.** Bring `main` in by merging, not
   rebasing, and never force-push someone else's branch (skill
   `pr-review`). Force-push your own branch only with the owner's
   explicit approval (`AGENTS.md` Conventions).
3. **Agents ask first.** The `guard-destructive` agent hook stops these
   commands before they run; Claude Code asks the owner, Kiro blocks and
   the owner runs it:
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
   - Real-environment data is removed in a new commit, and its pattern
     goes into the private deny list.
   - The guardrails ([ADR-004](ADR-004-three-guardrail-layers.md)) exist
     to keep this rare.

## Rationale

1. **A rewrite does not undo a publication.** Once something is in a
   public history, the only fix that holds is to make it worthless
   (rotation) or to stop repeating it (removal plus the deny list).
2. **Stable SHAs are part of the record.** Specs, ADRs, CHANGELOG compare
   links, tags and binaries all cite commits. The rewrite left two
   citations in shipped documents pointing at commits no branch reaches.
3. **Merging keeps every collaborator's base valid.** It costs merge
   commits in the log, which is cheap next to lost work.

## Consequences

- The log carries merge commits and follow-up fix commits. Branch commits
  are tidied before the first push, not after.
- A mistake in a pushed commit message stays. The `commit-msg` hook and
  the `secrets` job (`content.sh --commits`) catch the usual ones before
  the push or the merge.
- The guard has gaps:
  - with `jq` missing it asks in Claude mode but lets everything through
    in Kiro mode (`guard-destructive.sh`, the `jq` check);
  - it does not cover `git rebase`, `git commit --amend`,
    `gh api -X DELETE` or `gh release delete`.

  Local rewrites stay unguarded until they are pushed, and the push
  itself is guarded.
- An admin can still move or delete a `v*` tag or merge past the checks.
  The policy relies on admins following it, and the rulesets do not
  enforce it for them.
- ADR-001 and the `rebrand` spec keep the old SHAs, with dated
  corrections giving the current ones.

## When this would be wrong

- If something that must never be public reaches `main`, such as a
  credential that cannot be rotated or personal data under a legal
  removal request, a rewrite may be needed on top of removal. That is an
  owner decision, made outside this policy, in this order:
  1. suspend the ruleset;
  2. rewrite;
  3. record the old-to-new SHA mapping for every cited commit as dated
     corrections;
  4. restore the ruleset;
  5. ask GitHub Support to purge cached views and unreachable commits.

## Reversal

Allowing rewrites takes three changes:

- drop `non_fast_forward` from the `main` ruleset;
- remove the force-push patterns from `guard-destructive.sh`;
- change the `AGENTS.md` Conventions line.

Rewrites already done cannot be reversed. The old SHAs stay served only
by SHA.

## Alternatives discarded

- **Rewrite to remove any mistake** (filter-repo, force-push). It breaks
  every citation and every collaborator's clone, and the data has already
  been published.
- **Linear history through rebase and force-push on branches.** The log
  is cleaner, but it discards others' bases and needs force-pushes the
  agent guard would have to allow routinely.
- **Rely on the rulesets alone.** They protect only `main` and tags, not
  for admins. Work branches and local state are covered only by the agent
  hook and the `AGENTS.md` rules.
