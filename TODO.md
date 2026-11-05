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
- [ ] Regenerate the README images (`scripts/screenshots.sh`): the demo
      nodes' internal IPs moved to 192.0.2.0/24 for the golden screens,
      and `.static/` still shows the old ones. The node names stay
      `ip-10-0-…` (the allowed example names in `scripts/checks/content.sh`),
      so a node's name no longer matches its IP; rename them only together
      with that allowlist.
- [ ] `pkg/model/node.go` imports `aws-sdk-go-v2/service/ec2/types`, which
      crosses the "AWS SDK only in `pkg/aws`" rule (upstream code, there at
      the import). _Why:_ the model should not depend on a cloud SDK.
- [ ] spec-lint checks the decision sections only for `### D<n>` headings;
      `aks-pricing` and `node-details` use `## D<n>.` and pass unchecked.
      _Why:_ a design without "When this would be wrong" passed the gate.
- [ ] Azure prices: one failure time is shared by every region, so a
      failure in one region delays new sizes in all of them by a minute
      ([aks-pricing](specs/aks-pricing/design.md) D2). _Why:_ clusters
      across regions wait for a failure that is not theirs.

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
