# TODO

Backlog for kube-node-viewer: what the owner has to do, follow-ups found
while working, ideas, and deferred work. Rules in
[`AGENTS.md`](AGENTS.md#backlog-todomd): add what you find out of scope,
remove an item in the commit that does it, and a spec comes first when the
change needs one. Deferred tables in `specs/*/tasks.md` stay the record for
their feature; the rows below link them.

## Guardrails

The layers in `AGENTS.md` ("Guardrails") stop honest mistakes; these close
what they cannot. Listed by impact.

- [ ] **Release push with a GitHub App instead of the deploy key**: the
      rulesets on `main` and `v*` tags let deploy keys bypass, and
      `release.yaml` pushes with `RELEASE_DEPLOY_KEY`. An app of the
      StaffOps org (Contents: read & write, installed on this repository
      only) with `actions/create-github-app-token` gives a one-hour token
      instead; add the app as a bypass actor next to the deploy key, switch
      the workflow, release once, then remove the key.
      _Why:_ the deploy key is a long-lived credential that can push to
      `main`; it has to be rotated by hand, and a leak lasts until then.
- [ ] **Confirm the Kiro hooks in a real session**: the Kiro config
      follows the Kiro docs but has not run in Kiro yet; a `git commit`
      with a non-conventional subject should be refused by both the git
      hook and the Kiro `commit-guard`.
      _Why:_ if Kiro ignores a field, its agent loses the early warnings
      without any error; the git hooks and CI still hold.

## Spec-driven sync

Keep specs and code telling the same story, checked by CI rather than by
memory. Listed by impact.

- [ ] **Spec review as a CI gate**: a spec may be **active** only with a
      `Reviewed:` header naming `spec-refuter` and a date; a PR may not add
      code under a spec that is still **draft**.
      _Why:_ `AGENTS.md` makes the independent review mandatory, but 4 of
      9 specs skipped it (`aks-pricing`, `node-details`: owner only;
      `monthly-price-columns`, `sort-invert`: code-review agent). A rule
      nothing checks erodes the first time someone is in a hurry.
- [ ] **Requirement → test traceability in CI**: every `**R<n>**` of an
      active or shipped spec must be named by a test (`// <spec> R<n>`);
      a requirement without one fails the PR.
      _Why:_ only 9 test files cite requirements today, so nothing shows
      which requirements are untested. This is what keeps a spec from
      describing behaviour the code no longer has.
- [ ] **Drift detector**: when key bindings (`helpKeys`), flags
      (`flag.go`) or the column list change without a change to a spec
      or to `README.md`, CI flags the PR.
      _Why:_ behaviour changes made "while there" are the usual way
      specs and docs go stale; the spec-task gate only fires when a spec
      is touched.
- [ ] **ADRs for the cross-cutting decisions**: automerge for writers,
      release from CHANGELOG headings, guardrail layers (git hooks + agent
      hooks + CI), history rewrite policy.
      _Why:_ only ADR-001 exists; these decisions change how everyone
      works, and without a record of the alternatives they will be
      reargued or undone by accident.

## Security

Beyond the Guardrails section above.

- [ ] **Build provenance (SLSA)**: `actions/attest-build-provenance` on
      the archives in `release.yaml` (`attestations: write`), checked with
      `gh attestation verify`; `scripts/release-check.sh --strict` would
      then require it.
      _Why:_ the checksum signature proves the files came from
      `release.yaml`; provenance also records the inputs of the build, and
      OpenSSF Scorecard gives Signed-Releases its top score only with it.
      In the same change, move signing to its own job: today
      `id-token: write` covers the whole release job, including
      `make vuln licenses`, which runs downloaded tools.
- [ ] **A goreleaser snapshot in CI**: `goreleaser release --snapshot
      --skip=sign,publish` with syft on PRs that touch `.goreleaser.yaml`
      or `release.yaml`.
      _Why:_ a broken SBOM or archive setting shows today only on the
      Monday release, after the tag is pushed.

## Evolution

- [ ] **Integration tests on a `kind` cluster in CI**: start kind, apply
      fixture nodes and pods, run the TUI headless, assert on the screen.
      _Why:_ the "ran on a cluster" rule depends on someone having one;
      on 2026-11-03 none was available. A kind run makes the basic
      cluster check automatic, leaving real clusters for cloud-specific
      behaviour (prices, node pools).
- [ ] **Golden-file screen snapshots in PRs**: render the main screens
      from the made-up cluster used for screenshots and show the text
      diff in the PR.
      _Why:_ UI changes are reviewed today by reading code or running the
      binary; a diff of the screen makes layout regressions visible in
      review.

## Follow-ups

- [ ] The JSON export has no `view.search`: `exportDoc.View` is a
      `ViewState`, whose `Search` is `json:"-"` (so the state file never
      saves it), and `export.go` sets `doc.View.Search` anyway; README
      ("Export") says the JSON holds the search text. Found in the
      [search-negation](specs/search-negation/tasks.md) cluster run.
      _Why:_ an export of a searched view does not say which search made
      it.

## Ideas

- [ ] Recommendations for AKS: Azure VM catalog and family tables, so the
      Recommendations screen works on Azure nodes
      ([aks-pricing deferred](specs/aks-pricing/tasks.md#deferred),
      [recommendations deferred](specs/recommendations/tasks.md#deferred)).
- [ ] GKE: node pool labels and prices (stage 2, multi-cloud;
      [interactive-tui deferred](specs/interactive-tui/tasks.md#deferred)).
- [ ] GPU resource and columns (stage 3;
      [interactive-tui deferred](specs/interactive-tui/tasks.md#deferred)).

## Deferred (with promotion trigger)

| Item | Trigger | Source |
|------|---------|--------|
| Typed filter syntax in `/` (`cap:spot pool:gpu`) | panels feel slow for frequent filters | [interactive-tui](specs/interactive-tui/tasks.md#deferred), [views-and-export](specs/views-and-export/tasks.md#deferred) |
| OR across filter fields | owner asks | [interactive-tui](specs/interactive-tui/tasks.md#deferred) |
| Choosing the export directory | users run it from read-only directories | [views-and-export](specs/views-and-export/tasks.md#deferred) |
| Pod-level rightsizing (requests too high) | owner asks | [recommendations](specs/recommendations/tasks.md#deferred) |
| Commitments: Savings Plans, RIs, Azure reservations | someone needs effective rather than list cost | [recommendations](specs/recommendations/tasks.md#deferred), [aks-pricing](specs/aks-pricing/tasks.md#deferred) |
| A second Recommendations total including Spot | owner asks | [recommendations](specs/recommendations/tasks.md#deferred) |
| Drop the `eks-node-viewer/*` label prefix fallback | a major release, and no known user carries the old names | [rebrand](specs/rebrand/tasks.md#deferred) |
