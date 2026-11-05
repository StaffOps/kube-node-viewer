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
- [ ] **Owner: does the `kind` run count as the cluster run?** The
      `kind` job (`scripts/kind-it.sh`) runs on every PR, informational
      (not in automerge's needs, not a required check). Decide whether it
      counts as the "run on a cluster" of a verify task for changes that
      are not cloud-specific (AGENTS.md, "Tested before it ships"), leaving
      real clusters for prices and node pools; if so, make it gate
      automerge and a required check.
      _Why:_ until then every change still waits for a real cluster.
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

## Follow-ups

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
- [ ] Regenerate the README images (`scripts/screenshots.sh`): the demo
      nodes' internal IPs moved to 192.0.2.0/24 for the golden screens,
      and `.static/` still shows the old ones. The node names stay
      `ip-10-0-…` (the allowed example names in `scripts/checks/content.sh`),
      so a node's name no longer matches its IP; rename them only together
      with that allowlist.

## Ideas

- [ ] **Search: negation and label keys** (`/`): `!text` to keep the nodes
      that do not match, and matching label keys (`key=value`) as well as
      values. Out of scope of [`search-regex`](specs/search-regex/).
      _Why:_ "everything but the system nodes" and "nodes that have label
      X" are the next two questions after `a|b`.
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
