# Specs

Planning artefacts for `kube-node-viewer`. Principles and the change loop live
in [`../AGENTS.md`](../AGENTS.md); what shipped lives in
[`../CHANGELOG.md`](../CHANGELOG.md).

## Reading order

1. [`rebrand/`](rebrand/) — **shipped** in v0.10.0. Rename module, binary, config file and
   docs away from the upstream name.
   [requirements](rebrand/requirements.md) · [design](rebrand/design.md) ·
   [tasks](rebrand/tasks.md)
2. [`interactive-tui/`](interactive-tui/) — **shipped** in v0.8.0. Stage 1
   of the sofka-style rework: no-flag start, popup panels (Filters, Group by,
   Sort, Columns, Context), folding, per-cluster state, change-driven
   redraw. The owner's UX changes are dated notes in its requirements.
   [requirements](interactive-tui/requirements.md) ·
   [design](interactive-tui/design.md) · [tasks](interactive-tui/tasks.md)
3. [`views-and-export/`](views-and-export/) — **shipped** in v0.9.0. Excluding filters,
   named views (`v`), export of the screen to CSV and JSON (`e`).
   [requirements](views-and-export/requirements.md) ·
   [design](views-and-export/design.md) · [tasks](views-and-export/tasks.md)
4. [`recommendations/`](recommendations/) — **shipped** in v0.9.0. A Recommendations
   page next to Overview, reached with `:` or `tab`: idle cost,
   consolidation, rightsizing, newer generation, Graviton, old nodes, Spot
   last; rules, no AI.
   [requirements](recommendations/requirements.md) ·
   [design](recommendations/design.md) · [tasks](recommendations/tasks.md)
5. [`aks-pricing/`](aks-pricing/) — **shipped** in v0.9.0. AKS node prices from the
   public Azure Retail Prices API, per node by provider-ID cloud.
   [requirements](aks-pricing/requirements.md) ·
   [design](aks-pricing/design.md) · [tasks](aks-pricing/tasks.md)
6. [`node-details/`](node-details/) — **shipped** in v0.9.0. `enter` on a node opens
   a popup with its details: resources, conditions, taints, labels.
   [requirements](node-details/requirements.md) ·
   [design](node-details/design.md) · [tasks](node-details/tasks.md)
7. [`label-visibility/`](label-visibility/) — **shipped** in v0.10.0. Allow and deny
   lists of label key patterns in `~/.config/kube-node-viewer/config.yaml`
   decide which node labels the panels offer; `a` shows all.
   [requirements](label-visibility/requirements.md) ·
   [design](label-visibility/design.md) · [tasks](label-visibility/tasks.md)
8. [`sort-invert/`](sort-invert/) — **shipped** in v0.9.1. `I` (shift+i) flips the
   sort between ascending and descending from the main screen.
   [requirements](sort-invert/requirements.md) ·
   [design](sort-invert/design.md) · [tasks](sort-invert/tasks.md)
9. [`monthly-price-columns/`](monthly-price-columns/) — **shipped** in v0.11.0. `PRICE/M`
   and `IDLE/M` columns, shown by default.
   [requirements](monthly-price-columns/requirements.md) ·
   [design](monthly-price-columns/design.md) · [tasks](monthly-price-columns/tasks.md)
10. [`config-views/`](config-views/) — **shipped** in v0.12.0. Views written in
    `config.yaml`, a default view to open with, switched with `v`.
    [requirements](config-views/requirements.md) ·
    [design](config-views/design.md) · [tasks](config-views/tasks.md)
11. [`node-pods/`](node-pods/) — **shipped** in v0.13.0. A Pods tab in the node details
    popup: each pod's requests, share of the node and cost, all pods or
    DaemonSets only (`d`); the DaemonSets' share of the selected node on the
    node list.
    [requirements](node-pods/requirements.md) ·
    [design](node-pods/design.md) · [tasks](node-pods/tasks.md)
12. [`search-regex/`](search-regex/) — **shipped** in v0.14.0. The `/` search takes a regular
    expression (`a|b`, `^name$`); text with no regex characters but `.`
    matches as before.
    [requirements](search-regex/requirements.md) ·
    [design](search-regex/design.md) · [tasks](search-regex/tasks.md)
13. [`fix-config-errors/`](fix-config-errors/) — **shipped** in v0.14.0. `config.yaml`
    refuses label keys and resource names Kubernetes would refuse (padded,
    `zone,region`, `PRICE/M`), with hints for `- label: zone` and
    `sort: price/m`. [bugfix](fix-config-errors/bugfix.md) ·
    [tasks](fix-config-errors/tasks.md)
14. [`fix-hook-tests-git-env/`](fix-hook-tests-git-env/) — **shipped** in v0.14.0. The
    hook tests run from `pre-push` no longer write into the repository
    being pushed. [bugfix](fix-hook-tests-git-env/bugfix.md) ·
    [tasks](fix-hook-tests-git-env/tasks.md)
15. [`fix-aws-pricing-region/`](fix-aws-pricing-region/) — **shipped** in v0.14.0. AWS
    nodes priced in their own region when the API server URL names none.
    [bugfix](fix-aws-pricing-region/bugfix.md) ·
    [design](fix-aws-pricing-region/design.md) ·
    [tasks](fix-aws-pricing-region/tasks.md)
16. [`fix-config-case/`](fix-config-case/) — **shipped** in v0.14.0. `config.yaml`
    refuses a built-in name or column title in another case (`INSTANCE`,
    `Zone`, `PRICE/M`) instead of reading it as an empty label; YAML errors
    on one line. [bugfix](fix-config-case/bugfix.md) ·
    [tasks](fix-config-case/tasks.md)
17. [`release-provenance/`](release-provenance/) — **shipped** in v0.14.0. SLSA build
    provenance for every release file, signing in its own job, and a
    goreleaser snapshot on PRs that change the release.
    [requirements](release-provenance/requirements.md) ·
    [design](release-provenance/design.md) ·
    [tasks](release-provenance/tasks.md)
18. [`group-focus/`](group-focus/) — **shipped** in v0.14.0. `enter` on a group heading
    shows only that group's nodes; `esc` goes back; folding moves to
    `space`. [requirements](group-focus/requirements.md) ·
    [design](group-focus/design.md) · [tasks](group-focus/tasks.md)
19. [`search-negation/`](search-negation/) — **shipped** in v0.14.0. `/!text` keeps the
    nodes the text does not match; text with `=` also matches labels as
    `key=value` (`/key=` for "has the label").
    [requirements](search-negation/requirements.md) ·
    [design](search-negation/design.md) · [tasks](search-negation/tasks.md)
20. [`fix-export-search/`](fix-export-search/) — **shipped** in v0.14.0. The JSON
    export writes the search as `view.search`.
    [bugfix](fix-export-search/bugfix.md) · [tasks](fix-export-search/tasks.md)
21. [`fix-azure-price-product/`](fix-azure-price-product/) — **shipped** in v0.14.0. AKS
    Linux nodes of some series took the Cloud Services (Windows) price.
    [bugfix](fix-azure-price-product/bugfix.md) ·
    [tasks](fix-azure-price-product/tasks.md)
22. [`fix-config-filter-values/`](fix-config-filter-values/) — **shipped** in v0.15.0. `config.yaml`
    filters on `capacity` or `status` refuse a value no node can have.
    [bugfix](fix-config-filter-values/bugfix.md) · [tasks](fix-config-filter-values/tasks.md)
23. [`fix-spec-tooling/`](fix-spec-tooling/) — **shipped** in v0.15.0. Release tooling
    ships bugfix specs; spec-lint reads `## D<n>` decisions.
    [bugfix](fix-spec-tooling/bugfix.md) · [tasks](fix-spec-tooling/tasks.md)
24. [`fix-azure-retry/`](fix-azure-retry/) — **shipped** in v0.15.0. Azure prices honour
    `Retry-After`, retry per region, and clear a resolved error from the header.
    [bugfix](fix-azure-retry/bugfix.md) · [design](fix-azure-retry/design.md) · [tasks](fix-azure-retry/tasks.md)
25. [`fix-spec-lint-markers/`](fix-spec-lint-markers/) — **shipped** in v0.15.0. spec-lint checks
    each decision's own markers.
    [bugfix](fix-spec-lint-markers/bugfix.md) · [tasks](fix-spec-lint-markers/tasks.md)
26. [`compact-header-no-url/`](compact-header-no-url/) — **shipped** in v0.15.0. The compact
    header box drops the API server URL; the full panel keeps it.
    [requirements](compact-header-no-url/requirements.md) · [design](compact-header-no-url/design.md) · [tasks](compact-header-no-url/tasks.md)
27. [`short-arn-context/`](short-arn-context/) — **shipped** in v0.15.0. The headers show an
    EKS ARN context as its cluster name.
    [requirements](short-arn-context/requirements.md) · [design](short-arn-context/design.md) · [tasks](short-arn-context/tasks.md)
28. [`short-gke-context/`](short-gke-context/) — **shipped** in v0.15.0. The headers show a
    GKE context as its cluster name, and export file names use the short name.
    [requirements](short-gke-context/requirements.md) · [design](short-gke-context/design.md) · [tasks](short-gke-context/tasks.md)
29. [`hide-cloud-ids/`](hide-cloud-ids/) — **shipped** in v0.16.0. The headers shorten gcloud's multi-cloud
    and Connect gateway contexts, and node details stop showing the project or subscription.
    [requirements](hide-cloud-ids/requirements.md) · [design](hide-cloud-ids/design.md) · [tasks](hide-cloud-ids/tasks.md)
30. [`short-edge-context/`](short-edge-context/) — **shipped** in v0.16.0. The headers and the export file
    name show a Distributed Cloud (`edgecontainer_`) context as its cluster name.
    [requirements](short-edge-context/requirements.md) · [design](short-edge-context/design.md) · [tasks](short-edge-context/tasks.md)
31. [`fix-node-name-lock/`](fix-node-name-lock/) — **shipped** in v0.16.0. `Node` takes its lock once per
    call: no recursive read lock, no read outside the lock.
    [bugfix](fix-node-name-lock/bugfix.md) · [tasks](fix-node-name-lock/tasks.md)
32. [`content-cloud-ids/`](content-cloud-ids/) — **shipped** in v0.16.0. The content check blocks real GCE
    projects and Azure resource groups in provider IDs.
    [requirements](content-cloud-ids/requirements.md) · [design](content-cloud-ids/design.md) · [tasks](content-cloud-ids/tasks.md)
33. [`fix-ready-lock-fargate-panic/`](fix-ready-lock-fargate-panic/) — **shipped** in v0.16.0. `Ready` only
    reads (`Update` clears the NodeClaim time); a malformed Fargate annotation no longer panics.
    [bugfix](fix-ready-lock-fargate-panic/bugfix.md) · [tasks](fix-ready-lock-fargate-panic/tasks.md)
34. [`fix-azure-price-batch/`](fix-azure-price-batch/) — **shipped** in v0.16.0. Azure prices are asked in
    batches of at most 17 sizes; a request refused as over the API's cap is not sent again, another
    `400` waits 10 minutes.
    [bugfix](fix-azure-price-batch/bugfix.md) · [design](fix-azure-price-batch/design.md) · [tasks](fix-azure-price-batch/tasks.md)
35. [`aks-recommendations/`](aks-recommendations/) — **shipped** in v0.16.0. The Recommendations screen on
    AKS nodes: the cluster autoscaler's Azure size table, candidate prices from the Retail Prices
    API, agent pools, Arm v5 counterparts, regional Spot, system-pool limits.
    [requirements](aks-recommendations/requirements.md) · [design](aks-recommendations/design.md) · [tasks](aks-recommendations/tasks.md)
36. [`azure-live-skus/`](azure-live-skus/) — **shipped** in v0.16.0. Azure VM sizes read live from the
    Resource SKUs API with the machine's `az login`; no permission or no sign-in is said on the
    Recommendations screen; the copied autoscaler table goes.
    [requirements](azure-live-skus/requirements.md) · [design](azure-live-skus/design.md) · [tasks](azure-live-skus/tasks.md)
37. [`decisions/`](decisions/) — decision records (index below).

Status values: **draft** (not reviewed) → **active** (reviewed, being
implemented; the status line names the review, `Reviewed: <date>
(spec-refuter…)`, and no code may change under a draft spec) → **shipped** (all tasks done; historical, only dated
corrections) → **superseded** (link to the replacement).

## Decision records

| ADR | Decision | Status |
|-----|----------|--------|
| [001](decisions/ADR-001-import-as-new-repo.md) | Import upstream into a new repository instead of a GitHub fork | Accepted |
| [002](decisions/ADR-002-merge-green-prs-from-writers.md) | Merge green PRs from writers, and patch-only Dependabot PRs, without a human approval | Accepted |
| [003](decisions/ADR-003-release-from-changelog-headings.md) | Release in batches, with the version decided by the CHANGELOG headings | Accepted |
| [004](decisions/ADR-004-three-guardrail-layers.md) | Check the same rules in three layers: git hooks, agent hooks and CI | Accepted |
| [005](decisions/ADR-005-never-rewrite-published-history.md) | Never rewrite published history; fix forward, and rotate what leaked | Accepted |

## Format

Copy from [`.templates/`](.templates/). `scripts/spec-lint.sh` enforces the
mechanical parts.

- **Feature spec** `<feature>/` (kebab-case):
  - `requirements.md` — context, concrete usage examples, requirements as
    `**R<n>** WHEN <condition> THEN the system SHALL <behavior>`, each with
    checkable acceptance criteria, counter-examples, explicit **Out of scope**.
  - `design.md` — current vs target state, components touched, and for each
    significant decision: choice, justification (strongest first),
    trade-offs, **when it would be wrong**, **reversal action**, alternatives.
    Invariants. Pure reference tables are exempt from the decision format.
    Each decision is a level-2 or level-3 heading `D<n>` (`## D1. …`,
    `### D1: …`) whose own section, up to the next decision or the next
    heading of its level or higher, holds the markers
    `**When this would be wrong**` and `**Reversal action**` (`spec-lint.sh`
    checks it; a marker in a code block does not count).
  - Each requirement is named by the test that checks it, in the comment
    block right above its `func Test…` (or `t.Run(`), on a line that gives
    the spec's folder name and then the requirement: `// node-pods R3`,
    `// search-regex R1, R2` or `// config-views R1-R3` (a range names each
    one). A file header does not count. One no automated test can check
    carries a marker inside its block, or anywhere with its id:
    `_Test: manual — <how it is checked>._` (docs, a run on a cluster),
    `_Test: ci — <the CI job>._` (CI behaviour), or
    `_Test: withdrawn (R6) — <the dated note>._`. Markers may be added to a
    shipped spec, dated. `scripts/spec-trace.py` fails CI on a requirement
    with neither, and on a citation of an unknown or draft spec or of a
    requirement the spec lacks.
  - `tasks.md` — Kiro checkbox list: `- [ ] T<n>. <task>` (`[-]` in
    progress, `[x]` done) with a `_Covers: R<n> · Depends on: … · Verified
    by: …_` line under each. Deferred work goes in a table with its
    promotion trigger.
- **Bugfix spec** `fix-<slug>/`: `bugfix.md` (current behavior with
  reproduction, expected, unchanged behavior that must not regress) +
  `tasks.md`; `design.md` only when the fix involves a decision.
- **Decision record** `decisions/ADR-NNN-<imperative-title>.md`: context with
  checkable evidence (file:line, command output, links), decision, rationale,
  consequences, when wrong, alternatives. Add it to the index above.

## Kiro and Claude Code

`.kiro/specs` is a symlink to this directory, so the Kiro spec panel lists
every folder here: `decisions/` shows up as an entry but is not a spec;
templates are hidden in `.templates/`. Claude Code reaches specs through
`AGENTS.md`.

## Rules

- Spec before code for anything in the "spec needed" table in `AGENTS.md`.
- No invented numbers: write `unmeasured` when a figure was not measured.
  Mark guesses `[ASSUMPTION]` and open questions `[TO CONFIRM]`; a spec with
  `[TO CONFIRM]` cannot move to **active**.
- Every spec set gets an independent adversarial review (`spec-refuter`)
  before it moves to **active**; record the review date in the spec header.
- Corrections are recorded in place as a dated note
  (`> **Corrected YYYY-MM-DD.** …`), never applied silently.
- Requirements describe behavior a user can observe, not function names.
