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
- [ ] **Run [docs/kiro-validation.md](docs/kiro-validation.md) in a real Kiro
      session and report** (IDE 1.x or CLI V3; its report template goes in
      an issue or PR comment). Checked offline on 2026-11-05 against the
      Kiro docs (hooks, hook types and actions, agent configuration, CLI V3
      migration): the hook file schema, triggers and `shell` matcher, the
      agent profiles, and every hook script fed Kiro's documented payloads
      (`make kiro-selftest`, also in `test-checks.sh`). Open: whether
      `tool_input.command` is the field Kiro sends, whether each hook fires
      once, and whether the commit checklist (stdout of a PreToolUse hook)
      reaches the agent; the docs say that stdout is ignored.
      _Why:_ if Kiro ignores a field, its agent loses the early warnings
      without any error; the git hooks and CI still hold.

## Spec-driven sync

Keep specs and code telling the same story, checked by CI rather than by
memory. Listed by impact.

## Follow-ups

- [ ] Demo node names no longer match their IPs: the screenshot and golden
      fixtures name nodes `ip-10-0-…` (the example names allowed by
      `scripts/checks/content.sh`) while their internal IPs are in
      192.0.2.0/24. _Why:_ cosmetic in the README images; rename the nodes
      only together with that allowlist.
- [ ] EKS contexts named by ARN (`arn:aws:eks:<region>:<account>:cluster/<name>`,
      the default of `aws eks update-kubeconfig`) show the account id in both
      headers, and take most of the compact line
      ([compact-header-no-url](specs/compact-header-no-url/requirements.md),
      out of scope). _Why:_ a screenshot shares the account id; showing
      `<name>` (and the region) would be shorter.

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
