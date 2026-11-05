# ADR-003: Release in batches, with the version decided by the CHANGELOG headings

> Status: Accepted · Date: 2026-11-04 · Recorded after the fact: the
> decision was made on 2026-11-02 (`5a973d5`), and batching was added on
> 2026-11-03 (`3cb4f3c`).

## Context

- Every user-visible change already needs a `CHANGELOG.md` line under
  `## [Unreleased]`, grouped by Keep a Changelog headings (`Added`,
  `Changed`, `Fixed`, …). The `commit-guard` hook and
  `scripts/changelog-check.py` enforce where those lines go.
- Merges are automatic ([ADR-002](ADR-002-merge-green-prs-from-writers.md)).
  Releasing on every merge produced a version per PR. On 2026-11-02 alone
  that came to four releases (v0.8.0, v0.8.1, v0.8.2, v0.9.0), each one a
  Homebrew upgrade for users.
- The `main` and `v*` tag rulesets do not let `GITHUB_TOKEN` push, so the
  release pushes its commit and tag with `RELEASE_DEPLOY_KEY` (`aa56b4a`).

## Decision

`release.yaml` runs:

- every Monday at 12:00 UTC;
- on demand (`gh workflow run release.yaml`);
- right after the merge of a PR labelled `release` (the automerge job
  starts it).

When it runs, `scripts/release-prepare.py` reads the headings under
`[Unreleased]` and picks the next version:

| Headings under `[Unreleased]` | Next version |
|-------------------------------|--------------|
| `BREAKING` anywhere | major (minor while 0.x) |
| any `Added`, `Changed`, `Removed` or `Deprecated` | minor |
| only `Fixed` or `Security` | patch |
| empty | no release (the run re-tags a released version only if its tag is missing) |

With `--write`, `release-prepare.py` then:

- renames `[Unreleased]` to `[x.y.z] - DATE`;
- updates the compare links;
- marks as **shipped** every spec whose tasks are all done.

The workflow then:

- runs `make vuln licenses`;
- pushes the release commit and the tag;
- builds with GoReleaser, which also publishes the Homebrew cask;
- signs the checksum file and attaches the SBOMs;
- checks the published release with `scripts/release-check.sh --strict`.

## Rationale

1. **One source for the version and the notes.** The headings a
   contributor already writes for users decide the bump. There is no
   second label or commit-type convention to keep in sync with the
   CHANGELOG.
2. **Batches give fewer, readable releases.** A week of merges ships as
   one version with one set of notes, and an urgent fix still ships the
   same day through the `release` label or a manual run.
3. **Released sections never change.** A merge only adds to
   `[Unreleased]`. A branch that predates a release has to merge `main`
   in, and the checks refuse lines added to a released section.

## Consequences

- A wrong heading means a wrong version: a feature filed under `Fixed`
  ships as a patch. PR review has to read the headings.
- Merged changes wait up to a week unless someone triggers a release.
- The release commit is pushed to `main` by a deploy key, which bypasses
  the rulesets. That key is a long-lived credential (`TODO.md`: replace
  it with a GitHub App token).
- A release run edits files (`CHANGELOG.md`, the spec statuses), so a PR
  open across a release has to merge `main` again.

## When this would be wrong

- If users need every fix as soon as it merges (an outage tool), the
  batch delay costs more than the extra releases.
- If several people release independently, a version picked by the
  headings in one shared file becomes a merge-conflict point; tooling
  that keeps release notes per PR would fit better then.

## Alternatives discarded

- **A release on every merge** (`7c393ee`, `5a973d5`). Several versions
  a day, each one a user upgrade; replaced by batches in `3cb4f3c`.
- **Versions from Conventional Commit types** (semantic-release,
  release-please). Commit types are also written for every commit, but
  they describe commits rather than user-visible changes, and they would
  be a second source alongside the CHANGELOG.
- **Manual tags.** GitHub refuses them anyway (`v*` tag ruleset), and they
  rely on a person computing the version.
