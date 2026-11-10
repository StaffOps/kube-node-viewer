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
      session and report** (IDE 1.x or CLI V3; the report template goes in
      an issue or PR comment). Checked offline on 2026-11-05 against the
      Kiro docs (hooks, hook types and actions, agent configuration,
      permissions, CLI V3 migration): the hook file schema, triggers and
      `shell` matcher, each script wired once in `kiro` mode, the agent
      profiles and the reviewer's permission rules, and every hook command
      run with Kiro's documented payloads (`make kiro-selftest`, also in
      `test-checks.sh`). Open, for the real session: whether Kiro sends
      `tool_input.command`; whether each hook fires exactly once with no
      hooks in the profile (unverified; if none fire under `--agent` on CLI
      V3, put them back in the profile); whether a PreToolUse hook's stdout
      reaches the agent (the docs disagree); and which exit codes block in
      the IDE and the CLI.
      _Why:_ if Kiro ignores a field, its agent loses the early warnings
      without any error; the git hooks and CI still hold.

- [ ] **Watch the Monday schedules.** On 2026-11-09, the first Monday they
      existed, GitHub started all three 9 to 10 hours late (`test.yaml`
      06:00 → 14:54 UTC, Scorecard 06:30 → 16:41, `release.yaml` 12:00 →
      22:10), during a day of GitHub incidents. The fallback is a manual
      `gh workflow run release.yaml` (skill `release`). If it keeps
      happening, consider a check that warns when Monday passes with
      `[Unreleased]` entries and no release. _Why:_ a release that starts
      late can collide with the day's merges and fail (rerun it).

## Spec-driven sync

Keep specs and code telling the same story, checked by CI rather than by
memory. Listed by impact.

## Follow-ups

- [ ] Project or account ids still on screen, and in `/` search and the export:
      node labels (on AKS `kubernetes.azure.com/cluster` holds the resource
      group) and node names when a cloud puts one in them
      ([hide-cloud-ids](specs/hide-cloud-ids/requirements.md), out of scope).
      _Why:_ a screenshot of node details shares them.
- [ ] gcloud context names with a project number in place of the project id
      (`gke_<number>_…`, `connectgateway_<number>_…`, `edgecontainer_<number>_…`)
      are refused by the short-name patterns and show whole
      ([short-edge-context](specs/short-edge-context/requirements.md), out of
      scope). _Why:_ a project number identifies the project too; unknown
      whether gcloud ever writes one.
- [ ] `content.sh` still lets these through
      ([content-cloud-ids](specs/content-cloud-ids/requirements.md#out-of-scope)):
      resource groups in `az` table output, as `az group … --name`, in a
      Terraform or Bicep block (`name = …`, `resourceGroup('…')`), across
      YAML lines (an ASO `owner:`), unquoted after `= ` or before more text,
      or starting with a letter whose UTF-8 lead byte is `\342`, as
      `New-AzResourceGroup -Name`, double-encoded (`%252F`, `\\/`); GCP projects
      in a `--project` flag, a `project`/`project_id` key, image registries,
      `<project>.svc.id.goog`, service account e-mails in commit messages, or
      other collections (`serviceAccounts`, `secrets`, `topics`,
      `aggregated`); project numbers outside `projects/<number>/<collection>`;
      Azure subscription ids outside `subscriptions/<guid>`
      (`"subscriptionId": …`). _Why:_ a pasted CLI output or Terraform file
      still carries them.
- [ ] `Node.Ready` takes the write lock on every call for a Ready node, to clear
      the NodeClaim creation time, and the view calls it on every render
      (`pkg/model/node.go`). _Why:_ needless contention with the informers;
      clear it once, in `Update`.
- [ ] `Pod.FargateCapacityProvisioned` logs a `CapacityProvisioned` annotation
      it cannot parse and then indexes the failed match (`pkg/model/pod.go`).
      _Why:_ a malformed annotation panics the program.

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
