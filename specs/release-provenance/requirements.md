# Requirements: release-provenance

> Status: shipped (v0.14.0, 2026-11-05) · Owner: Carlos Gomes · Reviewed: 2026-11-04 (spec-refuter; 1 BLOCKING, 2 HIGH, 5 MEDIUM, 6 LOW, applied)

## Context

Two `TODO.md` items (Security):

- **Build provenance (SLSA).** A release today carries a Sigstore signature
  of its checksum file and an SPDX SBOM per archive
  (`.goreleaser.yaml` `signs`, `sboms`). The signature proves the files
  came from `release.yaml` of this repository; it does not say which
  commit and workflow run built them. OpenSSF Scorecard gives
  Signed-Releases its top score only with provenance. Also, the one
  release job holds `id-token: write` while it runs `make vuln licenses`,
  which downloads and runs `govulncheck` and `go-licenses`: any code in
  that job can mint a signing identity for this repository.
- **A goreleaser snapshot in CI.** A broken archive, SBOM or checksum
  setting in `.goreleaser.yaml` or `release.yaml` shows today only in the
  Monday release, after the tag is pushed (a tag without a release).

> **Corrected 2026-11-04.** After the spec-refuter review: R1 and the
> usage example name the workflow's certificate identity (main or a `v*`
> tag) instead of `--signer-workflow`, which accepts any ref; R3 also
> keeps `make vuln licenses` away from the deploy key; R4 checks the pins
> on every commit and adds cosign; R6 (only `v*` tags start a release)
> added.

## Usage examples

A user checks a downloaded archive (GitHub CLI 2.49 or later):

```text
$ gh attestation verify kube-node-viewer_Linux_x86_64.tar.gz \
    -R StaffOps/kube-node-viewer \
    --cert-identity https://github.com/StaffOps/kube-node-viewer/.github/workflows/release.yaml@refs/heads/main
Loaded digest sha256:… for file://kube-node-viewer_Linux_x86_64.tar.gz
✓ Verification succeeded!
```

The owner checks a release from a checkout:

```text
$ scripts/release-check.sh --strict v0.14.0
ok: every asset is in kube-node-viewer_0.14.0_sha256_checksums.txt
ok: every archive has an SBOM
ok: signature of kube-node-viewer_0.14.0_sha256_checksums.txt
ok: provenance of kube-node-viewer_Darwin_all.tar.gz
ok: kube-node-viewer_Darwin_all.tar.gz: checksum and "kube-node-viewer version 0.14.0"

$ scripts/release-check.sh --strict v0.13.1     # released before provenance
…
note: v0.13.1 predates build provenance (0.14.0)
```

A PR that changes `.goreleaser.yaml` gets a `release-snapshot` check that
builds every archive and SBOM without publishing; a PR that changes
neither release file shows the same check passing in seconds.

## Requirements

**R1** WHEN `release.yaml` publishes a release THEN every file listed in
its checksum file (each archive and each SBOM) SHALL have a SLSA build
provenance attestation whose certificate names `release.yaml` of this
repository run from `main` or from the release's `v*` tag, which
`gh attestation verify <file> -R StaffOps/kube-node-viewer
--cert-identity https://github.com/StaffOps/kube-node-viewer/.github/workflows/release.yaml@<ref>`
accepts for one of those two refs.

_Test: manual — after the first release with this change, `gh attestation verify` on a downloaded archive and SBOM, and the release's `check` job (`scripts/release-check.sh --strict`)._

- [ ] after the first release with this change, the command above
      succeeds for an archive and for an SBOM downloaded from it

**R2** WHEN `scripts/release-check.sh` checks a release THEN it SHALL
verify the provenance of the archive for this machine; a release at
0.14.0 or later without it SHALL fail with `--strict` and be reported as
a note without; a release before 0.14.0 SHALL be reported as predating
provenance and pass, also with `--strict`. A certificate naming
`release.yaml` run from any other ref SHALL NOT count as provenance.

_Test: manual — `scripts/release-check.sh --strict v0.13.1`; the `--cert-identity` matching checked on an attested GitHub CLI release file (exact ref only)._

- [x] `scripts/release-check.sh --strict v0.13.1` exits 0 and prints the
      "predates" note
- [x] the release workflow's own check (`--strict`) fails when the
      attestation step is skipped (read from the script: the check runs
      for every release at 0.14.0 or later)

**R3** WHEN `release.yaml` runs THEN the job that holds `id-token: write`
(signature and provenance) SHALL NOT run `make vuln licenses` or hold the
deploy key; the job that holds the deploy key SHALL NOT hold `id-token:
write` or run `make vuln licenses`; and `make vuln licenses` SHALL run in
a job with read-only permissions, on the commit that is then tagged.

_Test: manual — read from `release.yaml` (permissions and steps per job); actionlint and zizmor in the `workflows` job._

- [x] read from `release.yaml`: `id-token` appears only in the publish
      job, the deploy key only in the prepare job, `make vuln licenses`
      only in the `deps` job (`contents: read`), whose commit `prepare`
      checks out

**R4** WHEN a pull request changes `.goreleaser.yaml` or
`.github/workflows/release.yaml` THEN CI SHALL build the release with the
cosign, syft and goreleaser versions `release.yaml` pins, in snapshot
mode, without signing or publishing anything, and SHALL fail when the
build fails or when an archive is missing from the checksum file or has
no SBOM. WHEN any commit is verified THEN a tool pinned at two different
versions across the workflows and the Makefile SHALL fail
`scripts/verify.sh`.

_Test: manual — the `release-snapshot` job (`scripts/checks/release-snapshot.sh`); `scripts/checks/tool-versions.py --pins-only` in `scripts/verify.sh`._

- [x] this PR's own `release-snapshot` run builds the four archives and
      their SBOMs
- [x] `scripts/checks/tool-versions.py --pins-only` fails when the
      snapshot's or the check job's pin differs from `release.yaml`'s

**R5** WHEN a pull request changes neither file THEN the
`release-snapshot` check SHALL pass without building, so the automerge
job can wait on it for every PR.

_Test: manual — the `release-snapshot` check on a PR that changes neither file._

- [ ] a PR without release changes shows `release-snapshot` passing with
      the "no release files changed" message

**R6** WHEN a tag that does not start with `v` is pushed THEN the release
workflow SHALL NOT start; a `v*` tag pushed by hand still releases that
tag as before.

_Test: manual — read from `release.yaml` (`on: push: tags: ['v*']`); actionlint._

- [x] read from `release.yaml`: the only tag pattern is `v*`

## Counter-examples

- The snapshot SHALL NOT receive `HOMEBREW_TAP_GITHUB_TOKEN`, a write
  token or `id-token: write`.
- `--strict` SHALL NOT fail for releases published before provenance
  existed.
- The release SHALL NOT be tagged when `make vuln licenses` fails (as
  today).
- A tag such as `1.2.3`, which the `v*` ruleset does not protect, SHALL
  NOT start a release.

## Out of scope

- Making `release-snapshot` a required status check in the `main`
  ruleset (a repository setting, changed only by the owner).
- Provenance for the Homebrew cask (it lives in another repository; its
  sha256 values are checked against the checksum file).
- SLSA level 3 (a reusable, isolated builder such as
  `slsa-github-generator`).
