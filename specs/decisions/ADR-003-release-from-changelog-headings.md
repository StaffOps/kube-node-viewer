# ADR-003: Release in batches, with the version decided by the CHANGELOG headings

> Status: Accepted · Date: 2026-11-04 · Recorded after the fact. The
> decision was made in three steps on 2026-11-03 (UTC):
>
> - 01:41, PR #13 (`7c393ee`): release when a merged CHANGELOG names an
>   untagged version;
> - 02:36, PR #18 (`5a973d5`): release on every merge;
> - 04:17, PR #27 (`3cb4f3c`): release in batches.
>
> Reviewed: 2026-11-04 (spec-refuter; 3 HIGH, 7 MEDIUM, 7 LOW, applied)

## Context

- Every user-visible change already needs a `CHANGELOG.md` line under
  `## [Unreleased]`, grouped by Keep a Changelog headings (`### Added`,
  `### Fixed`, …). The `commit-guard` hook and
  `scripts/changelog-check.py` enforce where those lines go.
- Before automation, releases were tagged by hand. A person computed the
  version and tagged v0.8.0, v0.8.1 and v0.8.2 on 2026-11-02, between
  12:27 and 13:55 (-03:00).
- Merges are automatic ([ADR-002](ADR-002-merge-green-prs-from-writers.md)).
  Releasing on every merge (PR #18) produced one version per PR: v0.10.0,
  v0.11.0 and v0.12.0 shipped in about 75 minutes (02:49–04:04 UTC on
  2026-11-03), each one a Homebrew upgrade for users.
- The rulesets on `main` and `v*` tags do not let `GITHUB_TOKEN` push, so
  the release pushes its commit and tag with `RELEASE_DEPLOY_KEY`
  (`aa56b4a`), which bypasses both rulesets.

## Decision

`release.yaml` runs:

- every Monday at 12:00 UTC;
- on demand (`gh workflow run release.yaml`);
- right after the merge of a PR labelled `release`, started by the
  automerge job;
- when any tag is pushed (`on: push: tags: '*'`), the workflow's own tag
  included; that run stops when the release already exists. Today a
  non-`v*` tag pushed by any writer also starts a run. PR #43 narrows the
  trigger to `v*`.

`scripts/release-prepare.py` (`next_version`) picks the version from the
body of `[Unreleased]`:

| `[Unreleased]` body | Next version |
|---------------------|--------------|
| contains the text `BREAKING` anywhere (case-sensitive) | major (minor while 0.x) |
| a heading `### Added`, `### Changed`, `### Removed` or `### Deprecated` (any case) | minor |
| anything else: only `Fixed` or `Security`, misspelt headings, no headings | patch |
| empty | no new version (the run re-tags the newest released version only if its tag is missing) |

With `--write` it also:

- renames `[Unreleased]` to `[x.y.z] - DATE`;
- updates the compare links;
- marks as **shipped** every feature spec (`requirements.md`) that is
  `Status: active` with all tasks done. Bugfix specs (`bugfix.md`) are
  not handled and stay **active**.

> **Corrected 2026-11-05.** Bugfix specs are handled now: `--write` also
> ships a `fix-*` spec from its `bugfix.md`, and reads the status only from
> a spec's first `> Status:` line
> ([fix-spec-tooling](../fix-spec-tooling/bugfix.md)). The consequence
> "Bugfix specs have to be marked shipped by hand" below no longer holds;
> the six shipped in v0.14.0 were marked by hand once.

The workflow then:

- runs `make vuln licenses`;
- pushes the release commit and the tag;
- builds with GoReleaser, which also publishes the Homebrew cask;
- signs the checksum file and attaches the SBOMs;
- checks the published release with `scripts/release-check.sh --strict`.

> **Corrected 2026-11-04.** With PR #43
> ([release-provenance](../release-provenance/design.md)) the trigger is
> `on: push: tags: ['v*']`, so a non-`v*` tag starts nothing; and the
> steps above run in four jobs: `deps` (`make vuln licenses`, read
> only), `prepare` (release commit and tag), `publish` (GoReleaser,
> signature, SBOMs, SLSA build provenance) and `check`.

## Rationale

1. **One source for the version and the notes.** The headings a
   contributor already writes for users decide the bump. There is no
   label or commit-type convention to keep in sync with the CHANGELOG.
2. **Batches give fewer, readable releases.** A week of merges ships as
   one version with one set of notes. An urgent fix still ships the same
   day, through the `release` label or a manual run.
3. **Released sections never change.** A merge only adds to
   `[Unreleased]`. A branch that predates a release has to merge `main`
   in, and the checks refuse lines added to a released section.

## Consequences

- A wrong heading means a wrong version. A feature filed under `Fixed`,
  or under a misspelt heading such as `### Add`, ships as a patch. PR
  review has to read the headings.
- The word `BREAKING` anywhere in `[Unreleased]`, even in a sentence,
  raises the version; `breaking` in lowercase does not.
- Merged changes wait up to a week unless someone triggers a release.
- Bugfix specs have to be marked shipped by hand (`TODO.md`).
- A deploy key bypasses both rulesets to push the release commit and
  tag. It is a long-lived credential; `TODO.md` proposes a GitHub App
  token instead.
- Admins can also create, move or delete `v*` tags; the tag ruleset gives
  the admin role bypass mode `always`. This is intended: the owner can fix
  a release or a tag by hand when the workflow cannot.
- A release run edits files (`CHANGELOG.md`, spec statuses), so a PR left
  open across a release has to merge `main` again.

## When this would be wrong

- If users need every fix as soon as it merges (an outage tool), the
  batch delay costs more than the extra releases.
- If several people release independently, a version picked from one
  shared file becomes a merge-conflict point, and notes kept per PR would
  fit better.

## Reversal

Drop the `schedule` trigger and start `release.yaml` from the automerge
job after every merge, as PR #18 did. Moving off CHANGELOG-driven
versions means replacing `scripts/release-prepare.py`.

## Alternatives discarded

- **Manual tags.** This was the practice before automation
  (v0.8.0–v0.8.2): a person computes the version and tags it. The `v*`
  tag ruleset now refuses that, except for the deploy key and, on purpose,
  admins.
- **A release on every merge** (PR #18, `5a973d5`). It meant one version
  per PR, three in about 75 minutes, and batches replaced it in PR #27.
- **Versions from Conventional Commit types** (semantic-release,
  release-please). Commit types describe commits rather than
  user-visible changes, and would be a second source next to the
  CHANGELOG.
