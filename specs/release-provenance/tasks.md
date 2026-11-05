# Tasks: release-provenance

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done
> (Kiro's task-list format — the Kiro spec panel tracks these checkboxes).

- [x] T1. `release.yaml`: `prepare`, `publish`, `check` jobs; `actions/attest` on the checksum file's subjects; a rerun of `publish` attests the published files
  - _Covers: R1, R3 · Depends on: — · Verified by: actionlint and zizmor (`workflows` job); reading the permissions per job_
- [x] T2. `scripts/release-check.sh`: provenance of the archive for this machine, required with `--strict` from 0.14.0
  - _Covers: R2 · Depends on: — · Verified by: `scripts/release-check.sh --strict v0.13.1` (predates note, exit 0); shellcheck_
- [x] T3. `release-snapshot` job in `test.yaml` (always reports; builds when a release file changed, on `main` and weekly), `scripts/checks/release-snapshot.sh`, in `automerge.needs`
  - _Covers: R4, R5 · Depends on: — · Verified by: local snapshot plus the script; this PR's `release-snapshot` run_
- [x] T4. `scripts/checks/tool-versions.py`: the snapshot's goreleaser and syft pins, and pins of one tool must match
  - _Covers: R4 · Depends on: T3 · Verified by: running it; a changed pin reported as DIFFER_
- [x] T5. README "Verifying a release", AGENTS.md (layout, guardrails table), `.github/zizmor.yml` comment, CHANGELOG `Added`, `TODO.md`: remove both items
  - _Covers: R1, R2 · Depends on: T1, T2, T3, T4 · Verified by: review_
- [ ] T6. Verify: `scripts/verify.sh`, `make check`, the PR's `release-snapshot` and `workflows` runs. No cluster run: nothing under `cmd/` or `pkg/` changes. R1 is checked on the first release after the merge (the `check` job)
  - _Covers: R1, R2, R3, R4, R5 · Depends on: T1, T2, T3, T4, T5 · Verified by: dated note here_

## Deferred

| Task | Promotion trigger |
|------|-------------------|
| `release-snapshot` as a required check in the `main` ruleset | the owner changes the ruleset |
| SLSA level 3 builder | a consumer asks for level 3 |
