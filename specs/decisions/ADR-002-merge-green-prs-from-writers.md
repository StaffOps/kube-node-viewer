# ADR-002: Merge green PRs from writers, and patch-only Dependabot PRs, without a human approval

> Status: Accepted · Date: 2026-11-04 · Recorded after the fact: the
> decision was made on 2026-11-03 (`5a973d5`, PR #18) and later that day
> for Dependabot (`6002687`, `6c5fe67`). · Reviewed: 2026-11-04
> (spec-refuter; 3 HIGH, 7 MEDIUM, 7 LOW, applied)

## Context

- The repository has one maintainer, who works mostly through coding
  agents (Claude Code, Kiro). A required human approval on every PR
  would be the maintainer approving their own agent's work, after the
  fact, with no second reader.
- Most of what a review is meant to catch is already checked elsewhere:
  - the independent adversarial review of the spec and of the diff
    (`spec-refuter`, `AGENTS.md` change loop);
  - seven CI jobs: `verify`, `lint`, `build`, `secrets`, `guardrails`,
    `deps`, `workflows` (`.github/workflows/test.yaml`);
  - `scripts/spec-tasks-check.py`, which refuses a code change under a
    spec while any task of that spec is unticked. It checks the record:
    whether the verify task's cluster run happened is self-reported.
- The `main` ruleset requires a PR and those seven checks, but no review,
  so GitHub needs no approval to merge.
- Dependabot opens Go module PRs every week. `dependabot/fetch-metadata`
  reports an update type only for the dependencies Dependabot lists, but
  `go get` also raises requirements it does not list and can add modules:
  - PR #2 also raised `sso` 1.38→1.39 and `signin` 1.10→1.12;
  - PR #6 added modules and raised `x/net` 0.57→0.60.

## Decision

The `automerge` job in `test.yaml` approves and merges a PR once all seven
checks pass. All of these must hold:

- the PR is not a draft;
- it does not carry the `hold` label;
- its head is a branch of this repository, not a fork;
- one of:
  - its author has `admin`, `maintain` or `write` permission
    (`repos/…/collaborators/<author>/permission`);
  - it is a Dependabot PR, and all of the following hold:
    - its author and the actor of the run are both `dependabot[bot]`;
    - it changes only `go.mod` and `go.sum`;
    - `scripts/checks/dependabot-patch.sh` passes on the go.mod of the
      merge base and of the head: no module added or removed, every
      changed version a plain-release patch raise, and the rest of go.mod
      equal.

Every other PR waits for a person: other authors, forks, and all other
Dependabot PRs, GitHub Actions updates included.

## Rationale

1. **Review stays where it reads the work.** The review happens on the
   spec and on the diff before the PR exists, and CI repeats every check
   that can be automated. An approval click after that adds a delay but
   no second reader.
2. **The trust boundary is write access.** Write access already lets
   someone push a branch here, and with it change the workflows, so
   automerge gives them nothing new. Forks and outside authors do not get
   it.
3. **Dependabot is judged by what it changes, not by what it says.** The
   job fetches the go.mod of the merge base and of the head through the
   API, and runs the judge script from a checkout of `main`, never from
   the PR's code. No third-party action runs with its write token.

## Consequences

- A broken check is a broken gate: whatever CI does not test can merge
  without anyone reading it. The `guardrails` job tests the hooks and
  checks themselves (`scripts/checks/test-checks.sh`) for that reason.
- The cluster-run rule is only as good as its note: `spec-tasks-check.py`
  sees a ticked box, not the run.
- Admins can bypass the `main` ruleset through a PR (bypass mode
  `pull_request`), so an admin can merge a PR whose checks failed.
  Automerge never does: it `needs` all seven checks and merges with
  `--match-head-commit`.
- `hold` and drafts are the only brakes. An agent that opens a PR not
  ready to merge has to open it as a draft; the `pr-guard` hook lets
  `--draft` through.
- So far the Dependabot path has merged nothing by itself:
  - PR #33 was refused (it adds or removes a module) and merged by hand;
  - PR #32 (Actions) is manual by design.

  A run started by a person, for example re-running a Dependabot PR's
  checks, never automerges it, because the actor must be
  `dependabot[bot]` (the `trust` step in `test.yaml`).
- Minor and major Go updates and every Actions update stay manual, and
  pile up if nobody looks.
- Merging does not release: releases are batched
  ([ADR-003](ADR-003-release-from-changelog-headings.md)).

## When this would be wrong

- When a second regular contributor with write access joins: a review
  by the other one becomes a real second reader, so add a required
  review to the ruleset.
- When a check that matters for users cannot be automated, such as a UI
  change that needs eyes on the screen.

## Reversal

Add "require a pull request review" to the `main` ruleset and delete the
`automerge` job; the seven checks stay as they are.

## Alternatives discarded

- **A required human approval on every PR.** With one maintainer, that is
  self-approval after an independent agent review already happened: more
  waiting, no new reader.
- **GitHub's native auto-merge.** The job could run `gh pr merge --auto`
  after its own trust check. That needs the repository setting "Allow
  auto-merge" and changes little: the job already runs after all seven
  checks, and `--match-head-commit` refuses a head they did not see. The
  trust and Dependabot checks would still be the job's own.
- **Dependabot's `update-type` from `fetch-metadata`.** It misses
  unlisted and added modules (PRs #2 and #6), and it ran a third-party
  action with the write token.
- **Merge minor Dependabot updates too.** Minor releases of Go modules
  change behaviour often enough (the Kubernetes client, the AWS SDK) to
  deserve a look.
