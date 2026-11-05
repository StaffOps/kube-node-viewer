# ADR-002: Merge green PRs from writers, and patch-only Dependabot PRs, without a human approval

> Status: Accepted · Date: 2026-11-04 · Recorded after the fact: the
> decision was made on 2026-11-02 (`5a973d5`) and on 2026-11-03 for
> Dependabot (`6002687`, `6c5fe67`).

## Context

- The repository has one maintainer, who works mostly through coding
  agents (Claude Code, Kiro). A required human approval on every PR
  would be the maintainer approving their own agent's work, after the
  fact, with no second reader.
- What a review is meant to catch is already checked elsewhere:
  - the independent adversarial review of the spec and of the diff
    (`spec-refuter`, `AGENTS.md` change loop);
  - seven CI jobs: `verify`, `lint`, `build`, `secrets`, `guardrails`,
    `deps`, `workflows` (`.github/workflows/test.yaml`);
  - the gate that a code change under a spec merges only when every task
    of that spec is done, the cluster run included
    (`scripts/spec-tasks-check.py`).
- The ruleset on `main` requires those checks and a PR. It does not
  require a review, so a merge needs no approval from GitHub's side.
- Dependabot opens Go module PRs every week. `dependabot/fetch-metadata`
  reports an update type only for the dependencies Dependabot lists. But
  `go get` also raises indirect requirements and can add modules: two
  earlier Dependabot PRs carried minor bumps they did not list
  (`6c5fe67`).

## Decision

The `automerge` job in `test.yaml` approves and merges a PR once all seven
checks pass. All of these must hold:

- the PR is not a draft;
- it does not carry the `hold` label;
- its head is a branch of this repository, not a fork;
- one of:
  - its author has `admin`, `maintain` or `write` permission
    (`repos/…/collaborators/<author>/permission`);
  - the PR is from Dependabot (author and actor `dependabot[bot]`), it
    changes only `go.mod` and `go.sum`, and
    `scripts/checks/dependabot-patch.sh` passes on the go.mod of the merge
    base and the head. That script requires that no module is added or
    removed, that every changed version is a plain-release patch raise,
    and that the rest of go.mod is equal.

Every other PR waits for a person: other authors, forks, and all other
Dependabot PRs, GitHub Actions updates included.

## Rationale

1. **Review stays where it reads the work.** The review happens on the
   spec and on the diff before the PR exists, and CI repeats every check
   that can be automated. An approval click after that adds a delay but
   no second reader.
2. **The trust boundary is write access.** Write access is the boundary
   GitHub already uses to let someone push a branch to this repository.
   An author who can push a branch could already change the workflows,
   so automerge gives them nothing extra. Forks and outside authors do
   not get it.
3. **Dependabot is judged by what it changes, not by what it says.** The
   check compares the go.mod of the merge base with the head's, both
   fetched through the API. The job runs the judge script from a checkout
   of `main`, never from the PR's own code. It runs with the job's write
   token, and no third-party action is involved.

## Consequences

- A broken check is a broken gate: whatever CI does not test can merge
  without anyone reading it. The guardrails job tests the hooks and the
  checks themselves (`scripts/checks/test-checks.sh`) for this reason.
- `hold` and draft PRs are the only brakes. An agent that opens a PR not
  ready to merge has to open it as a draft (the `pr-guard` hook lets
  `--draft` through).
- Minor and major Go updates, and every Actions update, stay manual. They
  pile up if nobody looks.
- Merging does not release. Releases are batched separately
  ([ADR-003](ADR-003-release-from-changelog-headings.md)).

## When this would be wrong

- When a second regular contributor with write access joins, a human
  review between the two of them becomes a real second reader. At that
  point add a required review to the ruleset for their PRs.
- When a check that matters for users cannot be automated, such as a UI
  change that needs eyes on the screen.

## Alternatives discarded

- **A required human approval on every PR.** With one maintainer, this
  means self-approval after an independent agent review already
  happened. It adds wait time and no new reader.
- **GitHub's native auto-merge.** Someone has to enable it on each PR,
  and by itself it cannot tell a writer from an outside author or judge a
  Dependabot diff.
- **Dependabot's `update-type` from `fetch-metadata`.** It misses
  indirect and added modules, which happened twice (`6c5fe67`). It also
  ran a third-party action with the write token.
- **Merge minor Dependabot updates too.** Minor releases of Go modules
  change behaviour often enough (Kubernetes client, AWS SDK) to deserve a
  look.
