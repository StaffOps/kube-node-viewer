# Design: release-provenance

> Implements [requirements](requirements.md).

> **Corrected 2026-11-04.** After the spec-refuter review: `make vuln
> licenses` moved out of the deploy-key job into a read-only `deps` job
> (D1); a rerun of `publish` verifies the signed checksum file and every
> digest before attesting (D5, reasons rewritten); the snapshot installs
> cosign and its cost is measured (D3); the 0.14.0 constant's dependence
> on the merge date is stated (D4); the tag trigger narrowed to `v*` (D6).

## Current state

- `release.yaml` has one job, `build`, with `contents: write` and
  `id-token: write`. Steps: checkout with `RELEASE_DEPLOY_KEY`, version
  (`scripts/release-prepare.py`), Go, `make vuln licenses`, cosign and
  syft installs, release commit and tag pushed with the deploy key,
  goreleaser (signs the checksum file, SBOM per archive, publishes the
  release and the cask), `scripts/release-check.sh --strict`.
- `on: push: tags: '*'`: any tag push starts a release.
- The tag push starts `release.yaml` again; that run waits on the
  `release` concurrency group and stops when the GitHub release exists,
  or releases the tag when it does not (a retry after a failed publish).
- `test.yaml` never runs goreleaser.

## Target state

```text
release.yaml  (on: schedule, workflow_dispatch, push of v* tags)
  deps     (contents: read)                     checkout newest main (or the tag), make vuln licenses
     │ outputs: sha
  prepare  (contents: write; deploy key)        checkout sha, version, release commit + tag
     │ outputs: release, version
  publish  (contents: write, id-token: write,   checkout the tag, cosign; released already?
            attestations: write; no deploy key)  → verify signature + digests; else syft,
                                                 goreleaser release; attest provenance
     │
  check    (contents: read, attestations: read) scripts/release-check.sh --strict

test.yaml
  release-snapshot (contents: read)  release files changed? → cosign, syft,
                                      goreleaser --snapshot --skip=sign,publish,
                                      scripts/checks/release-snapshot.sh dist
  automerge needs: … + release-snapshot
scripts/verify.sh: tool-versions.py --pins-only (offline)
```

## Components touched

| Path | Change |
|------|--------|
| `.github/workflows/release.yaml` | `deps`, `prepare`, `publish`, `check`; `actions/attest` on the checksum file's subjects; `v*` tags only |
| `.github/workflows/test.yaml` | `release-snapshot` job; in `automerge.needs` |
| `scripts/checks/release-snapshot.sh` | new: checks `dist/` (archives in the checksum file, an SBOM per archive, the linux binary's version when it runs here) |
| `scripts/release-check.sh` | provenance of the downloaded archive (`gh attestation verify --cert-identity`), gated on 0.14.0 |
| `scripts/checks/tool-versions.py` | the snapshot's cosign, syft and goreleaser pins and the check job's cosign; pins of one tool must match; `--pins-only` |
| `scripts/verify.sh` | runs `tool-versions.py --pins-only` |
| `.github/zizmor.yml` | `artipacked` file exception for release.yaml dropped; an inline one on the `prepare` checkout |
| `README.md`, `AGENTS.md`, skill `release`, `CHANGELOG.md`, `TODO.md` | docs, the guardrails table, runbook, remove both items |

## Decisions

### D1: Four jobs, provenance in the job that builds

**Choice**: `deps` runs `make vuln licenses` with read-only permissions
and no credentials; `prepare` checks out the commit `deps` checked with
the deploy key and pushes the release commit and tag; `publish` builds,
signs and attests (`id-token: write`); `check` only reads.

**Justification, strongest first**:
1. Provenance must be produced where the files are built: moving the
   files to another job needs an artifact upload, which another job of
   the run could replace before it is attested.
2. `make vuln licenses` runs tools downloaded at run time (`go run
   …@version`). In `deps` they meet neither the deploy key (which
   bypasses the `main` and `v*` rulesets) nor a Sigstore identity.
3. `prepare` checks out `deps`'s commit, not `main` again: if `main`
   moved in between, `git push origin HEAD:main` of the release commit is
   refused and nothing is tagged, so no unchecked commit is released.
4. `check` downloads what users download; it needs no write.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| cosign and syft are installed after the tag is pushed: a failed install leaves a tag without a release | the tag push queues a run that releases a tag with no release (existing retry) |
| goreleaser still runs `go mod download` and builds with `id-token: write` | it is the build being attested; the modules are checked by `go.sum` and by `deps` |
| `deps` runs in every run, also those that find nothing to release (the tag-push run, a Monday with an empty `[Unreleased]`): about 1–2 minutes, unmeasured | weekly at most; a new advisory then fails that run too, as the weekly `test.yaml` run does |
| a merge to `main` between `deps` and `prepare` fails the release run | rerun the workflow; merges are rare at Monday 12:00 UTC |

**When this would be wrong**: if GitHub offers attestation from a job
that did not build (signed artifact hand-off), a separate signing job
with no build steps would be stricter.

**Reversal action**: merge the jobs back into one; no data to migrate.

**Alternatives discarded**: `slsa-github-generator` (level 3) — a
reusable workflow that builds in isolation, but it replaces goreleaser's
build and release steps and adds a third-party workflow with write
access; one job as today — keeps `id-token` and the deploy key next to
downloaded tools; `make vuln licenses` before the credentialed checkout
in `prepare` — the key still enters the same job and runner.

### D2: Attest the checksum file's subjects

**Choice**: `actions/attest` (v4, the action `attest-build-provenance`
now wraps) with `subject-checksums:` the release's checksum file.

**Justification, strongest first**:
1. One list of subjects for the signature, the SBOMs and the provenance:
   whatever goreleaser adds to the checksum file is attested with no
   glob to keep in step.
2. Archives and SBOMs both get provenance; the checksum file itself is
   already signed.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| the checksum file's name is computed in the step (`kube-node-viewer_<version without v>_sha256_checksums.txt`) | the step fails when the file is missing, before anything is attested |

**When this would be wrong**: more than 1024 subjects (the action's
limit); today 8.

**Reversal action**: switch to `subject-path:` globs.

**Alternatives discarded**: `subject-path: dist/*.tar.gz` — misses the zip
and the SBOMs unless every pattern is listed.

### D3: The snapshot job always runs and decides inside

**Choice**: `release-snapshot` runs on every PR, push and scheduled run;
on a PR it first lists the files changed since the merge base and exits
green when neither release file is among them. It installs cosign, syft
and goreleaser at `release.yaml`'s pins.

**Justification, strongest first**:
1. A job skipped with `if:` or a `paths:` filter makes `automerge`
   (which `needs` it) skip too; a job that always reports can be waited
   on.
2. On `main` and weekly it always builds, so a toolchain change in
   `go.mod` that breaks a cross-build shows within a week.
3. cosign before syft: `download-syft` verifies syft's signature only
   when cosign is on `PATH`, and a broken cosign pin shows on the PR
   instead of after the release tag.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| a build takes about 11 minutes (goreleaser 10m14s, job 10m54s on this PR's first run, job 126999940137); `timeout-minutes: 20`, about twice that | only PRs that change release files, `main`, and weekly |
| a runner start for every other PR (≈ 15 s, unmeasured) | the other jobs run in parallel and take minutes |
| a change to `go.mod` alone does not build the snapshot on its PR | `build` job compiles; the weekly snapshot catches cross-builds |

**When this would be wrong**: if the snapshot becomes a required check
in the ruleset, a skip with `if:` would be reported as skipped (which
rulesets accept), and the in-job check could go.

**Reversal action**: replace the change detection with `if:` on the job.

**Alternatives discarded**: `dorny/paths-filter` — a third-party action
for a `git diff` one-liner.

### D4: `release-check.sh` gates provenance on 0.14.0

**Choice**: provenance is required with `--strict` from 0.14.0; earlier
releases print a note and pass.

**Justification, strongest first**:
1. Old releases cannot gain an attestation; `--strict` must still work
   on them for the cask and checksum checks.
2. The workflow runs `--strict` only on the release it just published,
   always ≥ 0.14.0 from now on (`[Unreleased]` already has `Added` and
   `Changed`, so the next version is 0.14.0).

**When this would be wrong**: if 0.14.0 were released without this
change — that is, if this PR merges after the Monday 2026-11-09 12:00 UTC
release, or a release is started on demand before it merges; then the
constant moves to the first attested version.

**Reversal action**: change the constant.

### D5: A rerun of `publish` attests what is published, once verified

**Choice**: `publish` first asks whether the GitHub release of the tag
exists; if so it skips goreleaser, downloads the release's files into
`dist/`, verifies the checksum file's Sigstore bundle (the identity
`release-check.sh` accepts) and checks every listed file against it
(`sha256sum --check --strict`, no `--ignore-missing`), and only then
attests.

**Justification, strongest first**:
1. A second goreleaser build would produce new archives whose digests
   differ from the ones the signed checksum file covers; attesting them
   would describe files no user downloads, and uploading them would
   break the signature.
2. The attested digests are then the signed ones: an asset replaced by
   hand after the first run fails the digest check instead of being
   attested.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| a release goreleaser left half-published (a listed file never uploaded) fails the digest check, and the rerun cannot fix it | such a release is broken anyway; the runbook says to release a patch |
| a rerun never publishes the cask that a failed first run did not push | the runbook says to release a patch once the token is fixed |

**When this would be wrong**: if goreleaser could resume a partial
release (publish only the missing parts) with the files from the first
run; then rerunning it would be better than a patch release.

**Reversal action**: drop the step; a failed attestation then needs a
new patch release.

### D6: Only `v*` tags start a release

**Choice**: `on: push: tags: ['v*']`.

**Justification, strongest first**:
1. The tag ruleset protects only `v*` tags: any writer can push another
   tag (`1.2.3`), which today starts a full release outside the
   CHANGELOG flow, with the deploy key and a Sigstore identity.
2. Every tag the workflow itself pushes is `v<version>`.

**When this would be wrong**: if releases ever use another tag scheme;
then the ruleset and the trigger change together.

**Reversal action**: widen the pattern.

## Invariants

- Nothing is tagged before `make vuln licenses` passes on the commit
  being tagged.
- No step that runs downloaded tools shares a job with `id-token: write`
  or the deploy key, except the goreleaser build being attested.
- Attested digests are always the ones the signed checksum file lists.
- The snapshot never signs, publishes, or sees a secret.

## Failure modes

| Failure | Behavior | How the user notices |
|---------|----------|----------------------|
| attestation API down | `publish` fails after the release is public; `check` does not run | the run fails; `gh run rerun <id> --failed` reruns `publish` (verify, attest) and `check` (D5) |
| goreleaser fails after creating the release (cask push, an upload) | `publish` fails; a rerun attests only if every listed file is there, never pushes the cask | the run fails; fix the cause and release a patch (skill `release`) |
| `check` fails | the published files are wrong | fix in a PR and release a patch; rerunning `check` fails again |
| `main` moves between `deps` and `prepare` | the release commit push is refused, nothing tagged | the run fails; rerun the workflow |
| `gh` older than 2.49 on a user's machine | `release-check.sh` fails on `gh attestation` | error names the command |
| snapshot breaks on an unrelated PR | only when release files change or on `main` | the `release-snapshot` check |

## Test plan

- `goreleaser check` and a local `goreleaser release --snapshot --clean
  --skip=sign,publish,sbom`; `scripts/checks/release-snapshot.sh dist`
  against it (SBOM check skipped locally without syft).
- shellcheck 0.9.0 and 0.10.0 (CI's) and the local one; actionlint and
  zizmor in the `workflows` job.
- This PR's `release-snapshot` run (with cosign and syft).
- `scripts/release-check.sh --strict v0.13.1` (predates note, exit 0).
- `tool-versions.py --pins-only` reports DIFFER for a changed pin.
- The first release after the merge: the `check` job, and the README's
  `gh attestation verify` on an archive and on an SBOM.
