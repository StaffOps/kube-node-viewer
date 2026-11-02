[![License](https://img.shields.io/badge/License-Apache%202.0-ff69b4.svg)](LICENSE)
[![Go code tests](https://github.com/StaffOps/kube-node-viewer/actions/workflows/test.yaml/badge.svg)](https://github.com/StaffOps/kube-node-viewer/actions/workflows/test.yaml)

# kube-node-viewer

`kube-node-viewer` is a terminal UI for watching node usage in a Kubernetes
cluster. It shows the scheduled pod resource requests against each node's
allocatable capacity, plus the node price. It *does not* look at actual pod
resource usage.

Prices come from AWS (on-demand, spot and Fargate). On other clouds, run with
`--disable-pricing`, or set prices per node with the
`kube-node-viewer/instance-price` label.

![](./.static/screenshot.png)

Start it with no flags and drive it from inside: filter, group and sort nodes,
pick columns and resources, search, and switch cluster without restarting.
The last view used on each cluster comes back the next time.

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

## Usage

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
    	Start with the individual node list hidden, leaving only the --group-by summary. The node list is toggled at runtime with 'n'
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

### Keys

| Key | Action |
|-----|--------|
| `f` | Filters: every field and label is listed; `enter` opens one to tick its values (with node counts), `space` turns a filter on/off, `d` clears it |
| `g` | Group by: tick up to three fields as nested levels (`K`/`J` reorder), each with node, pod, usage and cost subtotals |
| `s` | Sort by creation, name, IP, price, pods, usage per resource or any label; `enter` on the current sort flips it |
| `o` | Columns: show, hide and reorder (`K`/`J`) columns and labels |
| `/` | Search name, IP, instance type and label values as you type; `enter` keeps it, `esc` clears it |
| `r` | Resources: cpu → memory → cpu+memory (→ the `--resources` list, if different) |
| `c` | Switch to another kubeconfig context |
| `n` | Fold one level per press when grouping: the nodes, then each sub-group level down to the top level; the next press unfolds all. Folded, only the deepest level shown has usage bars |
| `d` | Folded: list the next group level under each barred group as one-line rows, or hide them |
| `ctrl+e` | Compact or expand the header |
| `←/→` | Previous / next page |
| `?` | All keys |
| `q`, `ctrl+c` | Quit |

Panels open as a popup over the node list. While a panel or the search is
open, keys go to it: `q` or `f` are typed, not
commands, and `esc` closes the panel or steps back. `esc` no longer quits.
Filters on different fields combine with AND, several values of one field with
OR. In the node group column, EKS managed node groups are shown in their own
colour; plain names are Karpenter NodePools. A reserved node shows its
on-demand price as an equivalent (`~$`), left out of the totals.

### Saved view per cluster

Filters, group-by, sort, resources, columns, the node list toggle and the
header size are saved per cluster (context name plus API server URL) in
`$XDG_STATE_HOME/kube-node-viewer/state.json`, by default
`~/.local/state/kube-node-viewer/state.json`. At start the view is built from
the built-in defaults, then `~/.kube-node-viewer`, then the saved state, then
flags given on the command line. Those flags apply to that run only, until you
change the view from the TUI. A corrupt state file is ignored with a warning;
delete it to start over. The search text is not saved.

### Examples

```shell
# Standard usage
kube-node-viewer
# Karpenter nodes only
kube-node-viewer --node-selector karpenter.sh/nodepool
# Display both CPU and memory usage
kube-node-viewer --resources cpu,memory
# Display extra labels, i.e. AZ
kube-node-viewer --extra-labels topology.kubernetes.io/zone
# Sort by CPU usage in descending order
kube-node-viewer --node-sort=kube-node-viewer/node-cpu-usage=dsc
# Cost and usage per nodepool, without the per-node detail
kube-node-viewer --group-by karpenter.sh/nodepool --groups-only
# Cost and usage per team, using your own node label
kube-node-viewer --group-by company.com/team
# A few nodepools at once, instead of one terminal per nodepool
kube-node-viewer --group-by karpenter.sh/nodepool=default,gpu,spot
# Specify a particular AWS profile and region for pricing
AWS_PROFILE=myprofile AWS_REGION=us-west-2 kube-node-viewer
```

### Grouping by label

`--group-by` adds a summary block that aggregates every node sharing a value
for the given label. Each group shows its node count, pod count, resource
requests vs allocatable, the percentage in use and the cost, which answers
questions like "how much is this nodepool costing me" or "how much are the
nodes belonging to this team costing me":

```shell
kube-node-viewer --group-by karpenter.sh/nodepool
```

```text
6 nodes  (   98304Mi/377176Mi)  26.1% memory  ██████████░░░░░░░░░░░░  $4.608/hour | $3,363.840/month
48 pods (0 pending 48 running 48 bound)

default  4 nodes  32 pods  (65536Mi/251451Mi)  26.1% memory  ██████████░░░░░░░░░░░░  $3.072/hour | $2,242.560/month
gpu      2 nodes  16 pods  (32768Mi/125725Mi)  26.1% memory  ██████████░░░░░░░░░░░░  $1.536/hour | $1,121.280/month
```

Appending a comma separated list of values adds a filter on them, so a single
flag both picks the nodepools to look at and breaks the cost down by them. The
filter can be turned off from the Filters panel (`f`):

```shell
kube-node-viewer --group-by karpenter.sh/nodepool=default,gpu
```

A few notes:

- The label is any node label, written out in full. Grouping by team, zone,
  instance type or capacity type is the same flag with a different key.
- Values are optional. A bare `--group-by` label only says how to aggregate.
  `--node-selector` is the only flag that limits which nodes are watched;
  everything else filters in memory.
- The percentage is the usage of the group as a whole, so it is weighted by
  node size rather than being the plain average of each node's percentage. It
  also covers every resource passed to `--resources`.
- Nodes without the label are collected into a `<none>` group, which always
  sorts last.
- A cost prefixed with `>=` means at least one node in the group has an
  unknown price, so the real cost is higher than the number shown. A group
  with no known price at all shows no cost.
- Add `--groups-only` to start with the per-node list hidden; `n` shows it
  again. `g` opens the Group by panel for up to three levels.

### Computed labels

These label names can be passed to `--extra-labels`, `--node-sort` and
`--group-by` to show computed node information:

- `kube-node-viewer/node-age` - age of the node
- `kube-node-viewer/node-cpu-usage` - CPU usage (requests)
- `kube-node-viewer/node-memory-usage` - memory usage (requests)
- `kube-node-viewer/node-pods-usage` - pod usage (requests)
- `kube-node-viewer/node-ephemeral-storage-usage` - ephemeral storage usage (requests)

### Pricing region

For an EKS cluster the pricing region comes from its API server URL
(`https://<id>.gr7.<region>.eks.amazonaws.com`), also after a context switch.
For other servers it is the AWS SDK's configured region (`AWS_REGION` or the
profile).

### Price override label

A node labeled `kube-node-viewer/instance-price=<price per hour>` uses that
price instead of a pricing lookup.

### Default options

You can supply default options in a file named `.kube-node-viewer` in your
home directory. The format is `option-name=value`, where the option names are
the command line flags:

```text
# select only Karpenter managed nodes
node-selector=karpenter.sh/nodepool

# display both CPU and memory
resources=cpu,memory

# show the zone and nodepool name by default
extra-labels=topology.kubernetes.io/zone,karpenter.sh/nodepool

# summarize cost and usage per nodepool, optionally only for some of them
group-by=karpenter.sh/nodepool

# and start with the node list hidden
groups-only=true

# sort so that the newest nodes are first
node-sort=creation=asc

# change default color style
style=#2E91D2,#ffff00,#D55E00
```

### Coming from eks-node-viewer

- If `~/.kube-node-viewer` doesn't exist, `~/.eks-node-viewer` is read instead
  and a warning names the new path. The old file is never changed.
- The `eks-node-viewer/` prefix still works for the price override label and
  the computed label names. When a node carries both price labels,
  `kube-node-viewer/instance-price` wins.

### AWS permissions for pricing

Node and pod data comes from the Kubernetes API through your kubeconfig. AWS
is only called for pricing, which requires:

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

`pricing:GetProducts` is used for on-demand and Fargate pricing, and
`ec2:DescribeSpotPriceHistory` for spot pricing. Neither action supports
resource-level scoping, so `Resource` is `*`. With `--disable-pricing`, no AWS
permissions are needed.

### Troubleshooting

#### NoCredentialProviders: no valid providers in chain

Pricing needs AWS credentials unless you use `--disable-pricing`. Configure
them through `~/.aws/credentials`, `~/.aws/config`, environment variables or
another provider in the
[default credential chain](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/configure-gosdk.html).

#### `creating client, exec plugin: invalid apiVersion "client.authentication.k8s.io/v1alpha1"`

Update the AWS CLI and
[regenerate your kubeconfig](https://docs.aws.amazon.com/cli/latest/reference/eks/update-kubeconfig.html).

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md) and [AGENTS.md](AGENTS.md). In short:
`scripts/verify.sh` checks formatting, vet, tests and specs, and
`scripts/build.sh` builds the binary.

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
