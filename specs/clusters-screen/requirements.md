# Requirements: clusters-screen

> Status: active · Owner: Carlos Gomes · Reviewed: 2026-11-18 (spec-refuter: first pass 5 blocking, 6 high; second pass 3 blocking, 5 high; third pass 2026-11-18 1 blocking, 6 high, 7 medium, 7 low; fourth pass 2026-11-18 0 blocking, 2 high, 7 medium, 6 low; re-checks 2026-11-18, ready for active)

> **Revised 2026-11-18.** The owner asked to see the cost of one cloud's
> clusters at once and to keep clouds apart. Changes: clusters are grouped by
> cloud with a total per cloud and none across clouds (R4, R12);
> `:clusters <cloud>` shows one cloud only (R14); a cluster is first watched
> for its nodes only, and its pods are watched when its section is unfolded,
> so a large fleet does not open a pod watch per cluster (R15, R2, R3, R11);
> the picker ticks a whole cloud at once (R8).
>
> **Revised 2026-11-18, after the third review.** Picker ticks apply on
> `enter` only (R8); a closed list of pod figures shown as `…` before the pods
> (R15); Fargate nodes priced from a Fargate-only pod watch (R16); one section
> per API server (R17); the cloud of a cluster is final when its nodes synced,
> with `mixed` and `other` buckets (R3, R14); `tab` stays Overview ↔
> Recommendations, Clusters is reached with `:clusters` (R1); header sizes
> (R1); bare `:clusters` resets the scope (R14).
>
> **Revised 2026-11-18, after the fourth review.** Stopped watches (R7); an
> empty cluster keeps a known guessed cloud (R3, R14); filters on usage labels
> and `IDLE/H` gated (R15); Fargate pods by scheduler name (R16); first by
> name, normalized URLs (R17); default ticks follow a given scope (R8).

> **Corrected 2026-11-18, owner's review of the first build.** Each
> section shows the requests over allocatable of its nodes, a line and a bar
> per resource, as Overview's summary does, and its pods are watched as it
> connects (R3, R15, R18). The lighter mode of watching pods only for
> unfolded sections stays, set by `clusters.pods: unfolded` in `config.yaml`
> (R18, D10). Usage examples of the first draft show the light mode.

## Context

The viewer shows one kubeconfig context at a time; `c` switches and the
previous connection is closed. Someone who runs many clusters of one provider
(many EKS clusters) cannot see what they cost together without visiting each
one and adding the totals by hand.

This adds a third screen, **Clusters**, next to Overview and Recommendations.
It connects to several contexts at once and shows, for each cluster, its cost,
and for each cloud the total of its clusters. Prices of different clouds come
from different sources (AWS on-demand list prices, the Azure Retail Prices
API), so the screen never adds them into one figure; `:clusters aws` shows
one cloud alone. The filters, search, sort and group-by of the current view
apply inside each cluster's section. `enter` on a cluster opens that context
in Overview.

Cost needs the nodes only (Fargate aside, R16); idle cost, pod counts and the
request bars need the pods. A pod watch across all namespaces is the heaviest
thing the tool does, so a cluster is watched for its nodes first and for its
pods only when its section is unfolded ([design D10](design.md)).

The focus is EKS and AKS, which have prices. GKE nodes have no price source
today (`pkg/` has AWS and Azure providers only), so a GKE cluster shows its
nodes and no cost, and says so ([design D4](design.md)). GKE prices are a
separate spec, in `TODO.md`.

The tool still only reads: nothing here writes to a cluster.

## Usage examples

```text
$ kube-node-viewer
:clusters aws↵
 Clusters · aws · 3 clusters · 117 nodes · $15,990.000/month
 ▸ eks-prod-us   aws    64 nodes  $9,810.000/month  idle …  ok
 ▸ eks-prod-eu   aws    41 nodes  $5,200.000/month  idle …  ok
 ▸ eks-stg       aws    12 nodes    $980.000/month  idle …  ok
```

`space` on `eks-prod-us` unfolds it: its pods are watched, and until they
have synced the pod figures read `…`. With the group-by on the node group, the
section lists each node group and its nodes:

```text
 ▾ eks-prod-us   aws    64 nodes  $9,810.000/month  idle …  ok
   general      40 nodes  … pods  cpu …  $8.356/hour | $6,100.000/month  idle …
     NAME …  INSTANCE  PRICE/H  …
   batch-spot   24 nodes  … pods  cpu …  $5.082/hour | $3,710.000/month  idle …
     …
```

All selected clusters, more than one cloud:

```text
:clusters↵
 Clusters · 5 clusters · 230 nodes
   aws     4 clusters  212 nodes  $18,420.000/month
   azure   1 cluster    18 nodes   $2,050.000/month
 aws
 ▸ eks-prod-us   aws    64 nodes  $9,810.000/month  idle …  ok
 …
 azure
 ▸ aks-prod      azure  18 nodes  $2,050.000/month  idle …  ok
```

With a filter `capacity=Spot` the headings and the cloud totals count only
Spot nodes and say `(filtered: 9 of 64 nodes)`.

Failure examples: one cluster's token expired → its heading shows the reason
and its cloud's total reads `>= $… (1 cluster not connected)`; a GKE
cluster → its heading shows `prices unknown`; a kubeconfig with one context →
the screen shows that one cluster.

## Requirements

**R1** WHEN the user runs `:clusters` (or `:cl`) THEN the system SHALL show
the Clusters screen; `:overview`, `:recommendations` and `tab` (to Overview)
leave it. `tab` on Overview and Recommendations keeps switching between those
two.

- [ ] The header names the screen `Clusters`; the `:` hint and the unknown
  command message name `:clusters`.
- [ ] The screen is saved with the view like Recommendations is
  (`ViewState.Screen`).
- [ ] A saved `screen` this version does not know opens Overview.
- [ ] Header, one line: `Clusters · <cloud> · N clusters · N nodes · $…/month`
  (cloud and cost only with one cloud on screen, R4). Compact and full sizes:
  the Context, Cluster and Region rows give way to Scope, Clusters, Nodes and
  Cost rows; no pod count is shown in any size. The header's cost carries the
  same `>=` and `pods not loaded` marks as its cloud's total.

**R2** WHEN the Clusters screen is on screen, however it got there
(`:clusters`, start-up with a saved screen, `c` into a cluster saved on it)
THEN the system SHALL connect to each selected context in the cloud scope
(R14), show one section per cluster (R17), and fill each section as its data
arrives.

- [ ] Before the first answer a section shows `connecting…`; a section is
  `connecting…` until the cluster's node watch has synced or failed.
- [ ] The context on screen in Overview is not connected a second time: its
  existing store is shown, and the screen never closes it.
- [ ] No more than 4 connections are being made at once.
- [ ] The rows show the resources the view has chosen (`r`), in every
  unfolded section.
- [ ] The context on screen is always a section, ticked or not, unless the
  cloud scope (R14) leaves its cloud out.
- [ ] WHEN no choice is saved THEN the screen opens the cluster picker first
  and connects only the context on screen until `enter` in it (R8).
- [ ] Leaving the screen while members are still connecting leaves none of
  them running (R9).
- [ ] A saved context that is no longer in the kubeconfig is not connected;
  the picker lists it as `missing` until it is unticked.

**R3** WHEN a cluster section is shown THEN the system SHALL put in its
heading the short cluster name (the context name when it has no short form),
its cloud, node count, `$/month` and idle `$/month` of the nodes shown, and
its status.

- [ ] The cloud is the provider-ID scheme shared by all its nodes (`aws`,
  `azure`, `gce`); another scheme is `other` (the scheme shown), nodes of two
  schemes are `mixed`. Until the node watch has synced it is the cloud
  guessed from the kubeconfig, or `-`; a synced cluster with no node keeps
  that guess.
- [ ] Monthly figures use the same hours-per-month factor as Overview.
- [ ] Idle reads `…` while the cluster's pods are not watched or not yet
  synced (R15); it is never shown as `$0` or as the full price.
- [ ] Status is `ok`, `connecting…`, or the last error of its watches, without
  the API server URL.

**R4** WHEN the Clusters screen is shown THEN the system SHALL show, for each
cloud on screen, the total cost of its clusters' nodes shown, and how many
clusters and nodes it covers; WHEN only one cloud is on screen THEN that
total is the header's figure; the system SHALL NOT add costs of different
clouds into one figure.

- [ ] Each cloud's total is the sum of its clusters' `$/hour`, not of rounded
  figures.
- [ ] With two or more clouds on screen there is one line per cloud and no
  figure across clouds; the header shows the cluster and node counts only.
- [ ] WHEN a cluster of a cloud is not connected, or some node of it has no
  price, THEN that cloud's total is marked `>=` and says how many clusters
  are not connected and how many nodes are unpriced.
- [ ] Clusters under `-`, `other` or `mixed` are counted in no cloud's total;
  the header says how many there are and how many of them are not connected.
- [ ] Reserved nodes are counted as Overview counts them (`+N reserved, not
  priced`).
- [ ] A node of a cloud with no price provider (`gce`) is never priced by
  another cloud's provider, on this screen or on Overview; the
  `kube-node-viewer/instance-price` label still prices it, as on Overview.

**R5** WHEN a filter, a search, a sort or a group-by is set THEN the system
SHALL apply it inside each cluster's section, and the headings and the cloud
totals SHALL count the nodes kept.

- [ ] A section with the filters on says `(filtered: x of y nodes)`, as
  Overview's summary does; so does each cloud total.
- [ ] A cluster none of whose nodes match keeps its heading, with 0 nodes.
- [ ] The view applied is the live view of the context on screen in Overview
  (changes made on this screen are saved under that context); the other
  clusters' saved views are not read for it.
- [ ] With a group-by, each unfolded section lists its groups with their
  totals, always unfolded, whatever folds, depth or details the saved view
  has; folding and focus of groups are Overview's.
- [ ] A group's node count and cost are shown as soon as the node watch has
  synced; its pod, usage and idle parts follow R15.
- [ ] The value lists of the Filters and Group by panels offer the values of
  every connected cluster's nodes.

**R6** WHEN the user presses `enter` on a cluster heading or on any row of its
section THEN the system SHALL open that context in Overview, the same as
choosing it with `c`, with the Overview screen.

- [ ] The view shown is the one saved for that cluster, or the default, with
  the screen forced to Overview even when the saved one is Clusters.
- [ ] `enter` on a cluster not connected yet still opens it, as `c` would; if
  it fails the footer shows the error and the screen stays.
- [ ] `enter` on the context already on screen just shows Overview.
- [ ] A second `enter` while a switch is pending does nothing.
- [ ] The Clusters screen's connections are closed by this, including the
  member of the target context; the new Overview's connection is the only one.
- [ ] `enter` on a cloud total line does nothing.

**R7** WHEN a cluster fails to connect or its watches report an error THEN the
system SHALL show the failure in that cluster's heading and keep the others
working.

- [ ] An invalid context, an expired token and an unreachable API server each
  leave a heading with the reason and no effect on the other sections.
- [ ] A watch that recovers returns its heading to `ok`.
- [ ] A pod watch that fails after the node watch synced shows its error in
  the heading and keeps the cost.
- [ ] A member's errors are shown in its heading only, not in the header's
  log issue line.
- [ ] A watch refused by the API server (403), or whose credentials fail
  again after a refresh (401 twice, or the exec plugin failing), stops
  retrying and keeps its error; the cluster's other watches go on. Leaving
  and opening the screen again reconnects it. The section of the context on
  screen keeps Overview's retries.

**R8** WHEN the user opens the cluster picker (`c` on the Clusters screen)
THEN the system SHALL list the kubeconfig contexts under their guessed cloud
(`aws`, `azure`, `gce`, `unknown`), let `space` tick a context, or every
context of a cloud on that cloud's line, and apply the ticks on `enter`.

- [ ] The first time, the context on screen is ticked, and so are the
  contexts of the same cloud when that cloud is known (D5); the others are
  listed unticked. A context on screen of unknown cloud ticks only itself.
  WHEN the first visit is `:clusters <cloud>` THEN that cloud's contexts are
  the ones ticked.
  > **Corrected 2026-11-18.** The context on screen stays ticked too, as on
  > any first use; the other contexts of its cloud are not.
- [ ] `space` on a cloud's line ticks all its contexts when any is unticked,
  and unticks them all otherwise.
- [ ] Ticks are pending: nothing connects or closes until `enter`.
- [ ] `enter` saves the choice in the state file and applies it through the
  cloud scope (R14): a ticked context of another cloud is saved, not
  connected.
- [ ] `esc` drops the pending ticks. The first time, `esc` saves nothing and
  the screen shows the context on screen alone; the picker opens again on the
  next visit.
- [ ] A context with the same API server as another is listed with `same
  cluster as <name>` (R17).

**R9** WHEN the user leaves the Clusters screen, or quits, THEN the system
SHALL close the connections it opened.

- [ ] After leaving, no watch of a cluster other than Overview's is left
  running, node or pod; Overview's own connection is untouched.
- [ ] Quitting closes all of them.

**R10** WHEN the pricing is disabled (`--disable-pricing`) THEN the screen
SHALL show nodes and counts and no cost, and no `>=` marker, as Overview
does.

> **Corrected 2026-11-18.** Below its heading line each section shows one
> line per resource of the view (`r`), as Overview's summary does: requested
> / allocatable, the percentage, the resource and a bar, and on the first
> line the cost `$…/hour | $…/month` with its marks and the idle cost. The
> heading line keeps the name, cloud, node count and status. Until the pods
> have synced the requested, percentage and bar read `…` (R15); with
> `--disable-pricing` there is no cost. Folded sections keep these lines.
> _Test: the R3 citations; the golden screens `clusters`, `clusters-pending`._

**R11** WHEN the user presses `space` on a cluster heading THEN the system
SHALL unfold or fold that cluster's section; on any other row it does
nothing.

- [ ] Sections start folded: the heading only.
- [ ] Unfolding a section starts the cluster's pod watch (R15) and lists its
  nodes, or its groups and their nodes with a group-by.
- [ ] Unfolding a section still connecting is remembered: its pods are
  watched once it is connected.
- [ ] Folded sections keep their heading and their share of the cloud total.
- [ ] The fold is not saved, and is forgotten when the screen is left or the
  member is closed by a scope change.

**R12** WHEN clusters are listed THEN the system SHALL put them under their
cloud, clouds ordered by total cost, highest first, then clusters inside a
cloud by monthly cost, highest first, with clusters that have no cost last,
and ties by name.

- [ ] `mixed`, `other` and `-` come last, in that order.
- [ ] The order follows the prices as they arrive, and the cursor stays on
  the same cluster, even when its section moves.

**R13** WHEN the Clusters screen has the keys THEN the system SHALL act as
this table says.

| Key | On the Clusters screen |
|-----|------------------------|
| `↑/↓ j/k` | move over cluster headings and rows |
| `←/→ h/l pgup/pgdown home` | change page |
| `enter` | open the cluster in Overview (R6) |
| `space` | on a heading, unfold or fold its section (R11) |
| `f g s I o r /` | as in Overview, applied to every section |
| `c` | the cluster picker (R8) |
| `tab` | Overview |
| `v`, `:`, `?`, `q`, `ctrl+e` | as in Overview |
| `esc` | clears a kept search |
| `n d e` | do nothing (fold levels, details and export are Overview's) |

- [ ] Each row of the table is checked by a key-driven test.
- [ ] The node details popup is closed and the focus is not drawn on entering
  the screen.

**R14** WHEN the user runs `:clusters <cloud>` (or `:cl <cloud>`), `<cloud>`
being `aws`, `azure` or `gce`, THEN the system SHALL show only the selected
clusters of that cloud; `:clusters` alone shows every selected cluster.

- [ ] The header names the cloud (`Clusters · aws · …`) and its total is the
  header's figure (R4).
- [ ] Clusters of other clouds are not connected; a scope change made while
  on the screen closes the members it leaves out at once.
- [ ] A selected context is connected for its nodes when its guessed cloud is
  the scope's or unknown. Once its node watch has synced, its cloud (R3) is
  final: a member whose cloud is not the scope's (`other` and `mixed`
  included) is closed, and left out while the scope excludes its learned
  cloud; a later scope that includes that cloud, or a bare `:clusters`,
  connects it again. A
  synced cluster with no node keeps a known guessed cloud; with an `unknown`
  guess it is left out of a scope. A member that fails before syncing stays
  under `-`, counted in the header (R4).
- [ ] An unknown cloud word leaves the screen as it is and says
  `unknown cloud "<word>" (aws, azure, gce)`.
- [ ] The scope is saved with the cluster choice; a bare `:clusters` sets and
  saves the scope to every cloud.
- [ ] `tab` on the command line completes the cloud word as it completes
  commands.

**R15** WHEN a cluster is connected on the Clusters screen THEN the system
SHALL watch its nodes, and watch its pods only once its section is unfolded;
until its pods have synced, every figure made from pods SHALL read `…` and
order nothing.

- [ ] A folded section that was never unfolded has no pod watch (Fargate
  aside, R16).
- [ ] Unfolding starts the pod watch once; folding again keeps it until the
  screen is left.
- [ ] Figures made from pods, each read `…` until the pods have synced: idle
  `$/hour` and `$/month` (heading, group, `IDLE/H` and `IDLE/M` columns); pod count (`PODS`
  column, group line); requests and their bars (`usage:*` columns, group
  usage); the computed `kube-node-viewer/node-<resource>-usage` labels.
- [ ] A sort on any of those fields (`idle`, `pods`, `usage:*`, the computed
  labels) keeps a section in name order until its pods have synced; a
  group-by on a computed usage label puts its nodes under `…`; a filter on
  one keeps no node and the section says `(filtered: … pods not loaded)`;
  its cloud's total says `N clusters pods not loaded`.
- [ ] The context on screen in Overview is gated the same way: its section
  shows pod figures once its pods have synced, at once when they already
  have.

**R16** WHEN a connected cluster has Fargate nodes THEN the system SHALL
price them without watching all of the cluster's pods.

- [ ] A Fargate node is priced as on Overview, from its pod's provisioned
  capacity, read by a watch of Fargate pods only, kept until the screen is
  left.
- [ ] A cluster with no Fargate node, or `--disable-pricing`, opens no such
  watch.

> **Corrected 2026-11-18.** With `clusters.pods: all` (the default, R18) a
> member's pod watch starts as it connects, still at most 4 starting at a
> time; unfolding starts nothing. With `clusters.pods: unfolded` the
> criteria above hold as written.

**R17** WHEN two selected contexts point at the same API server THEN the
system SHALL show and count that cluster once.

- [ ] The context on screen in Overview wins; otherwise the first by name.
- [ ] Servers are compared with the host in lower case, no trailing `/` and
  no default `:443`.
- [ ] A cloud total never counts the same API server twice.

**R18** WHEN `config.yaml` sets `clusters.pods` THEN the system SHALL watch
the Clusters screen members' pods as it says: `all`, the default, as each
member connects; `unfolded`, only once its section is unfolded (R15).

- [ ] With no `clusters` key, or `pods: all`, every connected member's
  section shows its requests and idle once its pods sync, folded or not.
- [ ] With `pods: unfolded`, a folded section that was never unfolded has
  no pod watch, and its request lines read `…`.
- [ ] Any other value, a key other than `pods` under `clusters`, or a
  `clusters` that is not a map, is an error naming the place in the file,
  as for the other keys.
- [ ] The setting changes nothing for Overview's own connection.

## Counter-examples

- A cloud's total SHALL NOT read as complete when one of its clusters is
  missing or a node is unpriced.
- The system SHALL NOT add AWS and Azure costs into one figure, nor put a
  `mixed` cluster's cost in a cloud's total.
- The system SHALL NOT read usage (metrics-server) or write to any cluster.
- Leaving the screen SHALL NOT leave watches running on clusters nobody is
  looking at.
- Opening the screen on 30 clusters SHALL NOT open 30 full pod watches.
- Idle SHALL NOT read as the node's whole price because its pods are not
  watched yet.
- The same API server SHALL NOT be counted twice in a cloud total.
- A GKE cluster's nodes SHALL NOT be priced with AWS prices.
- Opening the screen SHALL NOT stop, restart or change the connection
  Overview has.
- A cursor or a fold in one cluster SHALL NOT move or fold another cluster
  that has a node or a group with the same name.

## Out of scope

- A total across clouds, and any currency or price-model conversion.
- `tab` reaching the Clusters screen (it would connect every selected cluster
  on a round trip between Overview and Recommendations).
- GKE prices (own spec; `TODO.md`).
- Recommendations across clusters; the Recommendations screen stays on the
  context on screen.
- Export of the Clusters screen.
- Per-cluster filters; the one view applies to every section.
- Merging nodes of different clusters in one list.
- Folding and focus of groups inside a section.
- A key that unfolds every section at once (it would open every pod watch).

## Open questions

- [ASSUMPTION] Ordering clouds and clusters by cost (R12); the owner asked
  only for the totals and a view per cluster. Cheap to change.
- [ASSUMPTION] A pod watch, once opened by an unfold, stays until the screen
  is left (R15): folding and unfolding again does not re-list the pods.
  Closing it on fold is one call if memory says so (T12).
- [ASSUMPTION] Memory, CPU and API load of node-only members and of the
  unfolded clusters' pod watches are acceptable, and a node watch is small
  next to a pod watch. Unmeasured; T12 measures them and notes the result.
- [ASSUMPTION] EKS Fargate pods have `spec.schedulerName` set to
  `fargate-scheduler`, so R16's watch can select them on the server. T12
  checks it on a cluster with Fargate; if not, R16 falls back to unpriced
  Fargate nodes marked as such.
