# Tasks: content-cloud-ids

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done
> (Kiro's task-list format — the Kiro spec panel tracks these checkboxes).

- [x] T1. The `gce` and `resourcegroup` rule pairs in `content.sh` (header rules and its "Known gaps" line naming the forms left, `git grep` alternation, `match` calls before `match domain`); fixtures renamed to example resource groups; cases in `test-checks.sh`
  - _Covers: R1, R2, R3, R4 · Depends on: — · Verified by: `scripts/checks/test-checks.sh`, `sh scripts/checks/content.sh` on the tree, `content.sh --staged` before each commit_
- [x] T2. Docs: the `AGENTS.md` guardrails row (`AGENTS.md:215`) names "GCE provider-ID projects, Azure resource groups" and the examples `gce://example-project/`, `resourceGroups/example-rg`; `specs/README.md` index; `TODO.md`: reword the item to the forms left (`--resource-group`, `"resourceGroup"`, `resource_group =`, `%2F` and `\/` escaped ids, project numbers), add tightening `gke_ok`. No CHANGELOG entry (repository tooling, as for earlier guard changes)
  - _Covers: R1, R2 · Depends on: T1 · Verified by: review_
- [x] T3. Verify: `scripts/verify.sh`, `scripts/checks/test-checks.sh` and `make lint` green; no cluster run (repository tooling only), stated in a dated note
  - _Covers: R1, R2, R3, R4 · Depends on: T1, T2 · Verified by: command output, recorded here as a dated note_

  > **Verified 2026-11-10.** `scripts/checks/test-checks.sh`: 220 passed, 0
  > failed, the new cases included (R1 to R3, commit messages and full-tree
  > cases for both rules); `sh scripts/checks/content.sh` on the whole tree: clean, with the 17
  > fixture lines renamed to example resource groups; `scripts/verify.sh`
  > green; `make lint` 0 issues; `shellcheck` clean on both scripts;
  > `go test ./pkg/azure/ ./pkg/client/ ./pkg/model/` green after the renames.
  > No cluster run: repository tooling only.
  >
  > After the code review, on 2026-11-10: 13 of its 27 mutants survived; the
  > cases added for them (both rules in a message and in the full tree, a real
  > or near-example domain before an example project, the project id bounds,
  > a name starting with a digit or `_`) make `test-checks.sh` fail on each
  > (re-run below).

> **Corrected 2026-11-10 (gaps).** T4 to T6 implement R5 to R10 (design D2).

- [x] T4. `content.sh`: narrow `gke_ok`, `gce_ok`, `aks_ok`, `resourcegroup_ok`; the `resourcegroup` alternatives and leading characters; the `gcp` pair, also in the full-tree `git grep` alternation; `LC_ALL=C` in `match()`; the full-tree prefilter; header rules and known gaps. Cases in `test-checks.sh` for each acceptance criterion, a full-tree case per R6/R7 form
  - _Covers: R5, R6, R7, R8, R9, R10 · Depends on: T1 · Verified by: `scripts/checks/test-checks.sh`, `sh scripts/checks/content.sh` on the tree_
- [x] T5. Docs: the `AGENTS.md` guardrails row names the new forms and `gcp`; `TODO.md` drops the two items done and keeps the forms left (the requirements' out-of-scope note), adds the subscription id outside `subscriptions/`
  - _Covers: R5, R6, R7, R8 · Depends on: T4 · Verified by: review_
- [x] T6. Verify: `scripts/verify.sh`, `scripts/checks/test-checks.sh`, `shellcheck` and `make lint` green, with the result as a dated note; no cluster run (repository tooling only)
  - _Covers: R5, R6, R7, R8, R9, R10 · Depends on: T4, T5 · Verified by: command output, recorded here as a dated note_

  > **Verified 2026-11-10 (gaps).** `scripts/checks/test-checks.sh`: 308
  > passed, 0 failed on macOS (BSD grep and sed); 299 passed, 0 failed, 4
  > skipped (gofmt, gitleaks, jq missing) under dash with GNU grep 3.8 and sed
  > 4.9 (Debian bookworm, no network). `sh scripts/checks/content.sh` on the
  > whole tree: clean on both, with no new `content-check: allow` line;
  > `shellcheck` clean on `scripts/checks/*.sh`; `scripts/verify.sh` green;
  > `make lint` 0 issues. Two spec-refuter reviews (spec, then diff with 69
  > mutants) applied. No cluster run: repository tooling only.
