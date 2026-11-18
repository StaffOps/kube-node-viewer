# Tasks: context-visibility

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done
> (Kiro's task-list format — the Kiro spec panel tracks these checkboxes).

- [x] T1. `contexts.allow` / `contexts.deny` in `config.yaml`: `readContextLists`, errors naming the place with a `contexts…` prefix, `settings.contexts` (D1)
  - _Covers: R3, R4, R8 · Depends on: — · Verified by: settings tests_
- [x] T2. Filter `sessions.Contexts` and `sessions.fleetContexts`; `fleetStore` filters on read and keeps hidden entries on save (D2, D4)
  - _Covers: R1, R2, R7 · Depends on: T1 · Verified by: session and store tests_
- [x] T3. `effectivePolicy(start, policy)` in `main` before `SetFleet`; off sends the notice and sets `SetContextListsOff` for the picker titles (D3)
  - _Covers: R5, R6 · Depends on: T2 · Verified by: `effectivePolicy` table test; model test of the titles_
- [x] T4. README: a "Context visibility" section next to Label visibility naming `contexts.allow` / `contexts.deny`, the full-name match, the lists-off rule and `deny: ["*"]`; CHANGELOG `[Unreleased]`
  - _Covers: — · Depends on: T1–T3 · Verified by: manual — the section read against the requirements (drift-check alone passes on the words already in the README)_
- [x] T5. Tests by an independent author carry `// context-visibility R<n>`
  - _Covers: all · Depends on: T1–T3 · Verified by: `scripts/spec-trace.py`_
- [x] T6. Verify: `scripts/verify.sh`, `make lint`; the binary on two kind clusters with a deny list, from a scratch directory; note what was checked, never a cluster's name
  - _Covers: all · Depends on: T1–T5 · Verified by: the run, recorded here as a dated note_

> **2026-11-18, T6.** Checked: `scripts/verify.sh` green but for this
> spec's tasks before this note, `make lint` 0 issues, `go test -race` on
> every package; tests by an independent author for R1–R8. The binary, from
> a scratch directory with its own HOME and `config.yaml`
> (`contexts.deny: ["*-cv-b"]`), on two throwaway kind clusters: started on
> the allowed context, `c` and the Clusters picker list only it (R1, R2);
> started on the denied one, `c` lists both under `Context (lists off)`, the
> Clusters picker under `Clusters (lists off)`, and the footer shows
> `contexts.allow/deny not applied: the context on screen is hidden by them`
> right after start (R5, R6). T4's README section was read against R1–R8.

## Deferred

| Task | Promotion trigger |
|------|-------------------|
| `--contexts-allow` / `--contexts-deny` flags | owner asks |
| Matching the short name | owner asks |
