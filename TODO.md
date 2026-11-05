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

- [ ] **Retroactive spec-refuter reviews**: `aks-pricing`, `config-views`,
      `monthly-price-columns`, `node-details` and `sort-invert` shipped
      reviewed by the owner or the code-review agent only; they pass the
      review gate through `UNREVIEWED` in `scripts/spec-lint.sh`. Review
      each, record findings as dated notes, drop it from the list.
      _Why:_ the list is the gate's only exception; it should end empty.

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

- [ ] **License headers everywhere** (rebrand R8): about ten shell scripts
      (`.githooks/*`, `scripts/agent-hooks/*.sh`, `scripts/build.sh`,
      `hack/gen_licenses.sh`) have no Apache header, and five upstream Go
      files changed since the import (`pkg/model/stats.go`, `style.go`,
      `pkg/text/colortabwriter.go`, the two `zz_generated` price tables) lack
      the D5 note. Add them, then a check over every file cited for R8.
      _Why:_ `pre-commit` checks only new files, so R8 is manual today.
- [ ] **Check the first attested release** (0.14.0): the release run's
      `check` job, then `gh attestation verify` (README "Verifying a
      release") on a downloaded archive and on an SBOM; record it under
      R1 of [release-provenance](specs/release-provenance/requirements.md),
      and under R5 the next PR's `release-snapshot` "no release files
      changed" pass.
      _Why:_ attestation can only be proven on a published release.
- [ ] `config.yaml` filter values in another case match nothing: `values:
      [Spot]` for `capacity` keeps no node, since capacity values are
      lower case (`spot`), with no error
      ([fix-config-case](specs/fix-config-case/bugfix.md)). _Why:_ the
      same silent empty result the case check fixed for field names.
- [ ] Bugfix specs are never marked shipped: `ship_specs` in
      `scripts/release-prepare.py` reads only `requirements.md`, so a
      `fix-*/bugfix.md` stays **active** after its release
      ([ADR-003](specs/decisions/ADR-003-release-from-changelog-headings.md)).
      _Why:_ the specs index keeps showing released fixes as **active**,
      as if still in progress.
- [ ] Regenerate `.static/demo.gif` with `scripts/screenshots.sh`: its
      captions now fold with `space` and show `enter` focusing a group
      ([group-focus](specs/group-focus/tasks.md) T4). _Why:_ the GIF still
      shows `enter` folding; the script needs ffmpeg, missing on the
      machine that made the change.
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
