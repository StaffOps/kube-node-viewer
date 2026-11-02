[![License](https://img.shields.io/badge/License-Apache%202.0-ff69b4.svg)](LICENSE)
[![Go code tests](https://github.com/StaffOps/kube-node-viewer/actions/workflows/test.yaml/badge.svg)](https://github.com/StaffOps/kube-node-viewer/actions/workflows/test.yaml)

# kube-node-viewer

`kube-node-viewer` is a terminal UI for watching node usage and cost in a
Kubernetes cluster. For every node it shows the resources requested by the
pods scheduled on it against the node's allocatable capacity, the pod count
and the node price, and it sums them per group and for the whole cluster.

It reads **requests**, not actual usage: it answers "how full are my nodes
from the scheduler's point of view, and what do they cost", which is what
drives node count and the bill. It needs nothing installed in the cluster,
only read access through your kubeconfig.

Start it with no flags and drive it from inside: filter, group and sort the
nodes, pick columns and resources, search, fold groups and switch cluster
without restarting. The last view used on each cluster comes back the next
time.

![kube-node-viewer: grouping by node group and zone, folding with n, details with d](.static/demo.gif)

The screenshots and the animation use a made-up cluster; they are regenerated
with `scripts/screenshots.sh`.

## Contents

- [Installation](#installation)
- [Quick start](#quick-start)
- [What is on screen](#what-is-on-screen)
- [Keys](#keys)
- [Panels](#panels)
- [Saved view per cluster](#saved-view-per-cluster)
- [Flags and config file](#flags-and-config-file)
- [Pricing](#pricing)
- [Computed labels](#computed-labels)
- [Troubleshooting](#troubleshooting)
- [How it works](#how-it-works)
- [Development](#development)

## Installation

Download a binary from the
[releases](https://github.com/StaffOps/kube-node-viewer/releases) page, or
install with Go:

```shell
go install github.com/staffops/kube-node-viewer/cmd/kube-node-viewer@latest
```

This installs to your `GOBIN` directory, typically `~/go/bin`.

To build from a checkout, run `scripts/build.sh` (binary in
`bin/kube-node-viewer`); `scripts/build.sh -i` installs it and
`scripts/build.sh -a` cross-compiles for every release platform.

## Quick start

```shell
kube-node-viewer
```

It connects to the current kubeconfig context. Then:

1. `r` until the header shows `cpu+memory`.
2. `g`, tick `nodegroup` (or `zone`, `capacity`, any label) with `space`,
   then `esc`: nodes are grouped, with subtotals.
3. `n` to fold the list down to the groups, each with its usage bars.
4. `f`, open `capacity` with `enter`, tick `Spot`: only spot nodes remain.
5. `c` to look at another cluster.

Quit with `q`. The next time you open the same cluster, the view is as you
left it.

## What is on screen

![The main screen: header box, cluster summary and the node list](.static/main.png)

**Header box.** The context, API server URL and pricing region; how many nodes
the filters and search keep out of all watched nodes, with their pod count
(and pending pods, when there are any); the active view (filters, group-by,
sort, resources); the main keys. `ctrl+e` switches between this full panel
and a two-line box. On a very short window it shrinks to one line.

**Cluster summary.** Requested over allocatable, the percentage and a bar for
each resource, and the cost per hour and per month. It covers every watched
node, whatever the filters.

**Node list.** One block per node: a usage bar per resource (stacked, with a
blank line between nodes when more than one resource is shown), the pods on
the node over its pod capacity, then the columns you picked. Default columns:

| Column | What it shows |
|--------|---------------|
| `INSTANCE` | instance type (`<cpu>vCPU-<mem>GB` for a Fargate pod) |
| `PRICE/H` | hourly price; `~$` marks an on-demand equivalent for a reserved node (see [Pricing](#pricing)); `-` when unknown |
| `CAPACITY` | `OnDemand`, `Spot`, `Reserved`, `Fargate` or `Unknown`, with `/Auto` for EKS Auto Mode |
| `NODEGROUP` | the Karpenter NodePool, or the EKS managed node group name in blue |
| `ZONE` | availability zone |
| `STATUS` | `Ready` or `NotReady/<for how long>`, plus `Cordoned`, `Deleting`, `Launching` (a Karpenter NodeClaim whose node has not registered yet) |

The Columns panel (`o`) also offers `IP`, `REGION`, `AGE`, the computed labels
and any node label.

**Groups.** With a group-by, nodes nest under up to three levels. Top-level
groups are in bold blue with a blank line between them; sub-groups hang off
them with `├─`/`└─` and show only their own value. Every heading carries node
and pod counts, usage per resource and cost; a cost prefixed with `>=` means
some node in the group has no known price.

**Colours.**

| Colour | Meaning |
|--------|---------|
| green / yellow / red percentage and bar | usage above 80% / 60–80% / below 60%: an under-used node is the costly case |
| green `Ready` · red `NotReady`, `Deleting` · yellow `Cordoned`, `Launching` | node status |
| orange `Spot` · magenta `Reserved` · blue `Fargate` | capacity types that change what a node costs |
| blue node group name | an EKS managed node group; plain is a Karpenter NodePool |

The three usage colours come from `--style`.

**Pages.** When the list is taller than the window it is cut into pages
(`←`/`→`); a page that continues a group repeats its path with `(cont.)`. On
a narrow window the least important columns are dropped first, and the footer
says how many are hidden. Lines never run past the window edge.


With `ctrl+e` the header shrinks to two lines:

![Compact header](.static/header-compact.png)

## Keys

| Key | Action |
|-----|--------|
| `f` | Filters panel |
| `g` | Group by panel |
| `s` | Sort panel |
| `o` | Columns panel |
| `/` | Search name, IP, instance type and label values as you type; `enter` keeps it, `esc` clears it |
| `r` | Resources: cpu → memory → cpu+memory (→ the `--resources` list, if different) |
| `n` | Fold one level per press when grouping: the nodes, then each sub-group level down to the top level; the next press unfolds all |
| `d` | While folded: list the next group level under each group as one-line rows, or hide them |
| `c` | Switch to another kubeconfig context |
| `ctrl+e` | Compact or expand the header |
| `←`/`→`, `h`/`l`, `home` | Previous / next / first page |
| `?` | All keys and colours |
| `q`, `ctrl+c` | Quit |

Panels open as a popup over the node list, with their own keys on their last
line. While a panel or the search is open, keys go to it: `q` or `f` are typed,
not commands, and `esc` closes the panel or steps back. `ctrl+c` and `ctrl+e`
work everywhere. `esc` never quits.

## Panels

### Filters (`f`)

![Filters popup with capacity open and Spot ticked](.static/filters.png)

Every field is listed (`capacity`, `nodegroup`, `zone`, `region`,
`instance-type`, `status` and every label found on the nodes), the ones
already filtered first. `enter` (or `→`) opens a field in place and lists its
values with the number of nodes having each; `space` ticks a value and the
list filters at once. On a field row, `space` turns its filter off and on
without losing the values, and `d` clears it. `←` or `esc` folds the field.

Values of one field combine with OR, fields with AND: `capacity ∈ {Spot}` and
`zone ∈ {us-east-1a, us-east-1b}` keeps spot nodes in either zone. A status
filter keeps a node that has any of the ticked flags.

### Group by (`g`)

![Group by popup with node group and zone ticked as levels 1 and 2](.static/group-by.png)

The same list of fields. `space` ticks a field as the next level (the first
ticked is the top level, each next one a sub-group of the previous), up to
three; the rows show `[1]`, `[2]`, `[3]`. `K`/`J` move the level under the
cursor up or down; unticking a level moves the ones below it up. Nodes
without a value for a level go to a `<none>` group, always last.

Folding with `n`: the deepest level on screen carries the usage bars and the
levels above it are one-line summaries. With `zone › instance-type`:

| Presses | On screen |
|---------|-----------|
| 0 | zones and instance types as one-line headings, nodes with bars |
| 1 | zones as one-line headings, instance types with bars |
| 2 | zones with bars (`d` adds their instance types as one-line rows) |
| 3 | back to everything |


One press of `n` (zones carry the bars):

![Grouped by node group and zone, folded once](.static/groups-folded.png)

Two presses and `d` (node groups carry the bars, their zones as one-line rows):

![Folded to the node groups, with details](.static/groups-details.png)

### Sort (`s`)

Creation time, name, IP, price, pods, usage of each resource, any derived
field or label. `enter` sorts by the row under the cursor; on the current
sort it flips the direction, and so does `space`. Price, pods, usage and
creation start biggest or newest first. Nodes without a value sort last in
either direction; ties break on the node name.

### Columns (`o`)

![Columns popup](.static/columns.png)

Shown columns first, in screen order, then the others and every label.
`space` shows or hides the column under the cursor (a new one goes at the
end) and `K`/`J` move it left or right.

### Context (`c`)

![Context popup](.static/contexts.png)

The kubeconfig contexts, the current one marked. Picking another connects to
it without restarting: its watches start, the previous cluster's stop, the
pricing region follows the new cluster, and its saved view comes back.
Nothing from the previous cluster shows after the switch. A connection error
stays in the panel and the current cluster keeps running.

## Saved view per cluster

Filters, group-by, sort, resources, columns, folding depth, details and the
header size are saved per cluster (context name plus API server URL, since
merged kubeconfigs can repeat context names) in
`$XDG_STATE_HOME/kube-node-viewer/state.json`, by default
`~/.local/state/kube-node-viewer/state.json`. Writes are batched and atomic,
so the file survives the tool being killed. The search text is not saved.

At start the view is built from, lowest to highest:

1. built-in defaults;
2. the config file `~/.kube-node-viewer`;
3. the saved view for this cluster, which replaces both;
4. flags given on the command line, for this run only: they are saved only
   once you change the view from the TUI.

A corrupt or unreadable state file is ignored with a warning before the TUI
starts. To start a cluster over, delete its entry (or the whole file) while
the tool is not running.

## Flags and config file

```text
Usage of kube-node-viewer:
  -attribution
    	Show the Open Source Attribution
  -context string
    	Name of the kubernetes context to use
  -disable-pricing
    	Disable pricing lookups
  -extra-labels string
    	A comma separated set of extra node labels to display
  -group-by string
    	Node label to summarize usage and cost by, optionally with a filter on a comma separated list of its values, e.g. 'karpenter.sh/nodepool' or 'karpenter.sh/nodepool=default,gpu'. Applies to this run; the last view chosen in the TUI is restored per cluster
  -groups-only
    	Start folded to the --group-by groups, with the node list hidden. At runtime 'n' folds and unfolds one level per press
  -kubeconfig string
    	Absolute path to the kubeconfig file (default "~/.kube/config")
  -node-selector string
    	Node label selector used to filter nodes, if empty all nodes are selected
  -node-sort string
    	Sort order for the nodes, either 'creation' or a label name. The sort order defaults to ascending and can be controlled by appending =asc or =dsc to the value. (default "creation=dsc")
  -resources string
    	List of comma separated resources to monitor (default "cpu")
  -style string
    	Three color to use for styling 'good','ok' and 'bad' values. These are also used in the gradients displayed from bad -> good. (default "#04B575,#FFFF00,#FF0000")
  -v	Display kube-node-viewer version
  -version
    	Display kube-node-viewer version
```

How the flags map to the view:

| Flag | Effect |
|------|--------|
| `--node-selector` | the only server-side scope: nodes outside it are never watched |
| `--group-by key` | group-by level 1 = the label `key` |
| `--group-by key=a,b` | the same, plus a filter `key ∈ {a, b}` you can turn off in the Filters panel |
| `--groups-only` | start folded to the groups |
| `--node-sort` | the sort (`creation` or a label, `=asc` / `=dsc`) |
| `--extra-labels` | label columns after the default ones |
| `--resources` | the resources; a list other than cpu / memory / both becomes a fourth step of `r` |
| `--context`, `--kubeconfig` | which cluster; view flags apply to the cluster the run starts on |
| `--style`, `--disable-pricing` | run options, not saved |

`-kubeconfig` also reads `KUBECONFIG` (flag > env var > config file).

Examples:

```shell
# Karpenter nodes only, watched server-side
kube-node-viewer --node-selector karpenter.sh/nodepool
# Start on cpu and memory, grouped by nodepool and folded to the groups
kube-node-viewer --resources cpu,memory --group-by karpenter.sh/nodepool --groups-only
# A few nodepools at once
kube-node-viewer --group-by karpenter.sh/nodepool=default,gpu,spot
# Sort by CPU usage, highest first
kube-node-viewer --node-sort=kube-node-viewer/node-cpu-usage=dsc
# Another cluster, AWS profile for pricing
AWS_PROFILE=myprofile kube-node-viewer --context staging
```

### Config file

`~/.kube-node-viewer` holds standing defaults, one `option-name=value` per
line, option names being the flag names. A saved view for a cluster takes
precedence over it.

```text
# select only Karpenter managed nodes
node-selector=karpenter.sh/nodepool

# display both CPU and memory
resources=cpu,memory

# group by nodepool, starting folded to the groups
group-by=karpenter.sh/nodepool
groups-only=true

# extra label columns
extra-labels=topology.kubernetes.io/zone

# newest nodes last
node-sort=creation=asc

# change the good / ok / bad colours
style=#2E91D2,#ffff00,#D55E00
```

## Pricing

Prices come from AWS: on-demand and Fargate from the Price List API, spot from
the EC2 spot price history, refreshed periodically, with a static on-demand
table as a fallback until the first lookup answers. On other clouds, run with
`--disable-pricing` or set prices with the override label below.

**Region.** For an EKS cluster the pricing region comes from its API server
URL (`https://<id>.gr7.<region>.eks.amazonaws.com`), also after a context
switch. For other servers it is the AWS SDK's configured region (`AWS_REGION`
or the profile).

**Reserved capacity.** Nodes on Karpenter reserved capacity or EKS capacity
blocks are prepaid, so their real hourly cost is unknown here. They show the
on-demand price as an equivalent (`~$0.1344`), and cluster and group totals
leave them out and say so: `$12.10/hour (+1 reserved, not priced)`.

**Override label.** A node labeled `kube-node-viewer/instance-price=<price per
hour>` uses that price instead of a lookup, counted as a real price.

**AWS permissions.** Node and pod data comes from the Kubernetes API. AWS is
only called for pricing, which requires:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "pricing:GetProducts",
        "ec2:DescribeSpotPriceHistory"
      ],
      "Resource": "*"
    }
  ]
}
```

Neither action supports resource-level scoping, so `Resource` is `*`. With
`--disable-pricing`, no AWS permissions are needed.

## Computed labels

These names work anywhere a label does: columns, sort, group-by, filters,
`--extra-labels`, `--node-sort`, `--group-by`.

- `kube-node-viewer/node-age`: age of the node
- `kube-node-viewer/node-cpu-usage`: CPU requests over allocatable
- `kube-node-viewer/node-memory-usage`: memory requests over allocatable
- `kube-node-viewer/node-pods-usage`: pods over pod capacity
- `kube-node-viewer/node-ephemeral-storage-usage`: ephemeral storage requests over allocatable

## Troubleshooting

**The view is not what I expect on one cluster.** It is that cluster's saved
view. Change it from the TUI, or quit and delete its entry from
`~/.local/state/kube-node-viewer/state.json`.

**A new default column or order does not show.** Columns saved for a cluster
win over the defaults; adjust them with `o` (`space`, `K`/`J`).

**NoCredentialProviders: no valid providers in chain.** Pricing needs AWS
credentials unless you use `--disable-pricing`. Configure them through
`~/.aws/credentials`, `~/.aws/config`, environment variables or another
provider in the
[default credential chain](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/configure-gosdk.html).

**`creating client, exec plugin: invalid apiVersion "client.authentication.k8s.io/v1alpha1"`.**
Update the AWS CLI and
[regenerate your kubeconfig](https://docs.aws.amazon.com/cli/latest/reference/eks/update-kubeconfig.html).

**`could not open a new TTY`.** The TUI needs a real terminal; run it directly,
not through a pipe or a tool without a TTY.

## How it works

- Three informers watch nodes and Karpenter NodeClaims (within
  `--node-selector`) and all pods. Filters, grouping, sorting and search work
  on memory and never restart them; only a context switch does.
- Pods are slimmed at the informer to what the totals need (requests, phase,
  node) and the store keeps running totals per node, so drawing a frame never
  walks pods: with 500 nodes a frame takes about 1.7 ms whether the cluster
  has 15,000 or 150,000 pods, and 150,000 pods take about 150 MiB of memory.
- The screen redraws when the cluster or prices change (at most every
  100 ms), plus a 1 s tick for ages and NotReady durations.
- Each context switch builds a new store with its own watches and pricing;
  late events from the previous cluster land in its own store, which is no
  longer shown.

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md) and [AGENTS.md](AGENTS.md). In short:
`scripts/verify.sh` checks formatting, vet, tests (with the race detector) and
specs, and `scripts/build.sh` builds the binary.

## Credits

`kube-node-viewer` is based on
[`awslabs/eks-node-viewer`](https://github.com/awslabs/eks-node-viewer),
licensed under the Apache License 2.0, which was originally developed at AWS
for demonstrating consolidation with [Karpenter](https://karpenter.sh/). It
was presented in
[Containers from the Couch: Workload Consolidation with Karpenter](https://www.youtube.com/watch?v=BnksdJ3oOEs)
and
[AWS re:Invent 2022 - Kubernetes virtually anywhere, for everyone](https://www.youtube.com/watch?v=OB7IZolZk78).
This project is not affiliated with or endorsed by AWS.

Thanks to:

- the authors and contributors of `eks-node-viewer`, whose code this project
  starts from;
- [João Pedro Alcantara](https://github.com/sule26), for the `--group-by` and
  `--groups-only` features, imported from the `sule26/eks-node-viewer` fork.

## License

[Apache License 2.0](LICENSE). See [NOTICE](NOTICE) for attribution.
