# Design: release-provenance

> Implements [requirements](requirements.md).

## Current state

- `release.yaml` has one job, `build`, with `contents: write` and
  `id-token: write`. Steps: checkout with `RELEASE_DEPLOY_KEY`, version
  (`scripts/release-prepare.py`), Go, `make vuln licenses`, cosign and
  syft installs, release commit and tag pushed with the deploy key,
  goreleaser (signs the checksum file, SBOM per archive, publishes the
  release and the cask), `scripts/release-check.sh --strict`.
- The tag push starts `release.yaml` again; that run waits on the
  `release` concurrency group and stops when the GitHub release exists,
  or releases the tag when it does not (a retry after a failed publish).
- `test.yaml` never runs goreleaser.

## Target state

```text
release.yaml
  prepare  (contents: write; deploy key)       version, make vuln licenses, commit + tag
     │ outputs: release, version
  publish  (contents: write, id-token: write,   checkout the tag, cosign, syft,
            attestations: write; no deploy key)  goreleaser release, attest provenance
     │
  check    (contents: read, attestations: read) scripts/release-check.sh --strict

test.yaml
  release-snapshot (contents: read)  release files changed? → goreleaser --snapshot
                                      --skip=sign,publish, then
                                      scripts/checks/release-snapshot.sh dist
  automerge needs: … + release-snapshot
```

## Components touched

| Path | Change |
|------|--------|
| `.github/workflows/release.yaml` | split into `prepare`, `publish`, `check`; `actions/attest` on the checksum file's subjects |
| `.github/workflows/test.yaml` | `release-snapshot` job; in `automerge.needs` |
| `scripts/checks/release-snapshot.sh` | new: checks `dist/` (archives in the checksum file, an SBOM per archive, the linux binary's version when it runs here) |
| `scripts/release-check.sh` | provenance of the downloaded archive (`gh attestation verify`), gated on 0.14.0 |
| `scripts/checks/tool-versions.py` | the snapshot's goreleaser and syft pins; pins sharing a tool must match |
| `.github/zizmor.yml` | `artipacked` comment: release.yaml's other checkouts drop credentials |
| `README.md`, `AGENTS.md`, `CHANGELOG.md`, `TODO.md` | docs, the guardrails table, remove both items |

## Decisions

### D1: Three jobs, provenance in the job that builds

**Choice**: the job that builds (`publish`) holds `id-token: write` and
attests; `prepare` keeps the deploy key and the dependency checks;
`check` only reads.

**Justification, strongest first**:
1. Provenance must be produced where the files are built: moving the
   files to another job needs an artifact upload, which another job of
   the run could replace before it is attested.
2. `make vuln licenses` runs tools downloaded at run time (`go run
   …@version`); outside `publish` they can no longer mint a Sigstore
   identity for this repository, and the deploy key never meets
   `id-token`.
3. `check` downloads what users download; it needs no write.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| cosign and syft are installed after the tag is pushed: a failed install leaves a tag without a release | the tag push queues a run that releases a tag with no release (existing retry); `gh run rerun --failed` reruns `publish` with `prepare`'s outputs |
| goreleaser still runs `go mod download` and builds with `id-token: write` | it is the build being attested; the modules are checked by `go.sum` and by `make vuln licenses` in `prepare` |
| two more job start-ups (≈ 20 s each, unmeasured) | weekly at most |

**When this would be wrong**: if GitHub offers attestation from a job
that did not build (signed artifact hand-off), a separate signing job
with no build steps would be stricter.

**Reversal action**: merge the jobs back into one; no data to migrate.

**Alternatives discarded**: `slsa-github-generator` (level 3) — a
reusable workflow that builds in isolation, but it replaces goreleaser's
build and release steps and adds a third-party workflow with write
access; one job as today — keeps `id-token` next to downloaded tools.

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
green when neither release file is among them.

**Justification, strongest first**:
1. A job skipped with `if:` or a `paths:` filter makes `automerge`
   (which `needs` it) skip too; a job that always reports can be waited
   on.
2. On `main` and weekly it always builds, so a toolchain change in
   `go.mod` that breaks a cross-build shows within a week.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| a runner start for every PR (≈ 15 s, unmeasured) | the other jobs run in parallel and take minutes |
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
change; then the constant moves to the first attested version.

**Reversal action**: change the constant.

### D5: A rerun of `publish` attests what is published

**Choice**: `publish` first asks whether the GitHub release of the tag
exists; if so it skips goreleaser and downloads the release's files into
`dist/` before the attest step.

**Justification, strongest first**:
1. goreleaser cannot publish over an existing release, so without this a
   failed attestation could never be redone for that version.
2. The files attested are then exactly the ones users download.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| a release goreleaser left half-published gets attested as it is | `check` then fails on the missing asset, as today |

**When this would be wrong**: if a release were ever edited by hand
before the rerun; the rerun would attest the edited files (the
signature check in `check` still fails on files not in the signed
checksum file).

**Reversal action**: drop the step; a failed attestation then needs a
new patch release.

## Invariants

- Nothing is tagged before `make vuln licenses` passes.
- No step that runs downloaded tools shares a job with `id-token: write`
  except the goreleaser build being attested.
- The snapshot never signs, publishes, or sees a secret.

## Failure modes

| Failure | Behavior | How the user notices |
|---------|----------|----------------------|
| attestation API down, or any step after goreleaser published | `publish` fails after the release is public | the run fails and `check` does not run; `gh run rerun --failed` reruns `publish`, which finds the release, skips goreleaser, downloads the published files into `dist/` and attests them (D5) |
| `gh` older than 2.49 on a user's machine | `release-check.sh` fails on `gh attestation` | error names the command |
| snapshot breaks on an unrelated PR | only when release files change or on `main` | the `release-snapshot` check |

## Test plan

- `goreleaser check` and a local `goreleaser release --snapshot --clean
  --skip=sign,publish,sbom`; `scripts/checks/release-snapshot.sh dist`
  against it (SBOM check skipped locally without syft).
- actionlint and zizmor in the `workflows` job.
- This PR's `release-snapshot` run (with syft).
- `scripts/release-check.sh --strict v0.13.1` (predates note, exit 0).
- The first release after the merge: the `check` job and the README's
  `gh attestation verify` command.
