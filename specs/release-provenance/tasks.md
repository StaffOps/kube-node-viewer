# Tasks: release-provenance

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done
> (Kiro's task-list format — the Kiro spec panel tracks these checkboxes).

- [x] T1. `release.yaml`: `deps`, `prepare`, `publish`, `check` jobs; `actions/attest` on the checksum file's subjects; a rerun of `publish` verifies the signed checksum file and the digests, then attests; `v*` tags only
  - _Covers: R1, R3, R6 · Depends on: — · Verified by: actionlint and zizmor (`workflows` job); reading the permissions per job_
- [x] T2. `scripts/release-check.sh`: provenance of the archive for this machine, required with `--strict` from 0.14.0
  - _Covers: R2 · Depends on: — · Verified by: `scripts/release-check.sh --strict v0.13.1` (predates note, exit 0); shellcheck_

  > **Verified 2026-11-04.** `scripts/release-check.sh --strict v0.13.1`
  > printed `note: v0.13.1 predates build provenance (0.14.0)` and exited
  > 0 (cosign stubbed locally; CI has the real one). `gh attestation
  > verify --cert-identity` on an attested GitHub CLI release archive
  > passed only for its exact workflow ref (`…@refs/heads/trunk`) and
  > failed for `refs/heads/main` and a tag ref. Version comparison
  > checked for 0.9.9, 0.13.1 (below), 0.14.0, 0.14.1, 0.100.0, 1.0.0
  > (at or above). shellcheck 0.9.0, 0.10.0 and 0.11.0 clean.
- [x] T3. `release-snapshot` job in `test.yaml` (always reports; builds when a release file changed, on `main` and weekly), `scripts/checks/release-snapshot.sh`, in `automerge.needs`
  - _Covers: R4, R5 · Depends on: — · Verified by: local snapshot plus the script; this PR's `release-snapshot` run_

  > **Verified 2026-11-04.** Local goreleaser 2.19.0 snapshot (no SBOM):
  > `release-snapshot.sh --no-sbom` passed; without `--no-sbom` it failed
  > on the missing SBOM, and with the zip removed on the missing archive.
  > The PR's first CI run (job 126999940137, 10m54s) printed `ok: 4
  > archives and their SBOMs` and the linux binary's version.
- [x] T4. `scripts/checks/tool-versions.py`: the snapshot's cosign, syft and goreleaser pins and the check job's cosign; pins of one tool must match; `--pins-only` in `scripts/verify.sh`
  - _Covers: R4 · Depends on: T3 · Verified by: running it; a changed pin reported as DIFFER_

  > **Verified 2026-11-04.** Full run: every pin `ok`. `--pins-only`
  > printed `pins match`; with the snapshot's goreleaser at 2.18.3, the
  > snapshot's cosign at 3.1.3, or the check job's cosign at 3.1.0, it
  > printed `DIFFER` for that tool and exited 1.
- [x] T5. README "Verifying a release", AGENTS.md (layout, guardrails table), `.github/zizmor.yml` comment, CHANGELOG `Added`, `TODO.md`: remove both items
  - _Covers: R1, R2 · Depends on: T1, T2, T3, T4 · Verified by: review_
- [x] T6. Verify: `scripts/verify.sh`, `make check`, the PR's `release-snapshot` and `workflows` runs. No cluster run: nothing under `cmd/` or `pkg/` changes. R1 can only be checked after the first release: deferred below
  - _Covers: R1, R2, R3, R4, R5, R6 · Depends on: T1, T2, T3, T4, T5 · Verified by: dated note here_

  > **Verified 2026-11-04.** `scripts/verify.sh` (with `tool-versions.py
  > --pins-only`) and `make check` (pre-push) green; shellcheck 0.9.0
  > clean on every script; actionlint and zizmor clean (`artipacked` now
  > reported for release.yaml when the inline exception is removed).
  > PR CI run 41955553006 on f779e43: every job passed;
  > `release-snapshot` (8m47s) verified cosign 3.1.4, built the four
  > archives and their SBOMs and printed the linux binary's version.
  > Not checked here, deferred below: R1 on a published release
  > (`gh attestation verify` on an archive and an SBOM, the `check`
  > job), and R5 on a PR that changes no release file.

## Deferred

> **Corrected 2026-11-05.** The R1 and R5 checks were done (dated notes
> under R1 and R5 in requirements.md) and their rows removed.

| Task | Promotion trigger |
|------|-------------------|
| `release-snapshot` as a required check in the `main` ruleset | the owner changes the ruleset |
| SLSA level 3 builder | a consumer asks for level 3 |
