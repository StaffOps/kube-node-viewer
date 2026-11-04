# Views

A view is what the node list shows: columns and their order, group-by,
sort, resources, filters and folding. You can build one in the TUI (`f`,
`g`, `s`, `o`, `r`) and save it with `v`, or write it by hand in
`config.yaml`, pick one to open every cluster with, and switch between them
with `v`. This page is about the views written by hand.

Contents:

- [Quick start](#quick-start)
- [Keys of a view](#keys-of-a-view)
- [Field names](#field-names)
- [Recipes](#recipes)
- [The start view](#the-start-view)
- [The Views panel](#the-views-panel)
- [Errors](#errors)

## Quick start

Open the file. It is the one of
[Label visibility](../README.md#label-visibility):
`~/.config/kube-node-viewer/config.yaml`, or
`$XDG_CONFIG_HOME/kube-node-viewer/config.yaml` when `XDG_CONFIG_HOME` is
set to an absolute path. Add a `views` list and, if you want one to open
with, `default-view`:

```yaml
views:
  - name: cost
    columns:
      - instance-type
      - price/h
      - price/m
      - idle/m
      - capacity
      - nodegroup
      - status
    group-by:
      - nodegroup
      - zone
    sort: price
    order: descending
    resources:
      - cpu
      - memory

  - name: spot
    filters:
      - field: capacity
        values:
          - Spot
      - field: zone
        exclude:
          - us-east-1c
    group-by:
      - karpenter.sh/nodepool
    groups-only: true

default-view: cost
start-with: default-view
```

Start `kube-node-viewer`. Every cluster opens with `cost`; `v` lists
`cost (config)` and `spot (config)` to switch between them.

Write every list as a block list, one `- item` per line.

## Keys of a view

Each item of `views` is one view. Only `name` is required; a key left out
takes the built-in default.

| Key | Value |
|-----|-------|
| `name` | the name the Views panel shows; unique |
| `columns` | list of columns, in screen order, named by their title in lower case (`price/h`, `price/m`) |
| `group-by` | list of one to three fields, top level first |
| `sort` | one field |
| `order` | `ascending` or `descending`. Without it, the direction the Sort panel starts with: descending for `creation`, `price`, `idle`, `pods` and usage, ascending for the rest. Without `sort`, it sets the direction of the `creation` sort |
| `resources` | list of resource names, in full: `cpu`, `memory`, `nvidia.com/gpu` |
| `filters` | list of filters, each with `field` and either `values` (keep nodes with any of them) or `exclude` (drop nodes with any of them); one filter per field; they start turned on |
| `groups-only` | `true` starts folded to the groups, as `--groups-only`; needs `group-by` |

The top-level keys:

| Key | Value |
|-----|-------|
| `default-view` | the name of a config view to open the clusters with |
| `start-with` | `default-view` (when not set) or `last-view` |

## Field names

Each key accepts the built-in names of the panel that sets it; any other name is a node label key, such as `karpenter.sh/nodepool`.
`label:<key>` always names a label, for a label whose key is also a
built-in name (`label:zone`).

| Key | Built-in names |
|-----|----------------|
| `group-by`, `filters[].field` | `capacity`, `nodegroup`, `zone`, `region`, `instance-type`, `status` |
| `sort` | `creation`, `name`, `ip`, `price`, `idle`, `pods`, `usage:<resource>` (`usage:cpu`), and the `group-by` names |
| `columns` | see the table below |

Columns, by the title they show:

| Column on screen | Name in `columns` | Also accepted |
|------------------|-------------------|---------------|
| `INSTANCE` | `instance-type` | |
| `PRICE/H` | `price/h` | `price` |
| `PRICE/M` | `price/m` | `price-month` |
| `IDLE/H` | `idle/h` | `idle` |
| `IDLE/M` | `idle/m` | `idle-month` |
| `CAPACITY` | `capacity` | |
| `NODEGROUP` | `nodegroup` | |
| `ZONE` | `zone` | |
| `STATUS` | `status` | |
| `REGION` | `region` | |
| `AGE` | `age` | |
| `IP` | `ip` | |

`sort: price` orders by price, and so both `PRICE/H` and `PRICE/M`: the
monthly price is the hourly one × 730. `sort: idle` does the same for
`IDLE/H` and `IDLE/M`.

A built-in name in a key that does not list it (`group-by` on `price`) is an
error. A typo in a built-in name (`zon`) is read as a label, which gives an
empty column or a `<none>` group. The [computed labels](../README.md#computed-labels)
are labels: `kube-node-viewer/node-age`.

`zone` in `config.yaml` is the derived field, the node's availability zone.
On the command line `--group-by zone` names the node label `zone`; there,
`--group-by topology.kubernetes.io/zone` groups by zone.

## Recipes

Each block is a whole `config.yaml`; put the views you want under one
`views:` key.

Both resources, cpu and memory, with a bar each per node:

```yaml
views:
  - name: cost
    columns:
      - instance-type
      - price/h
      - price/m
      - idle/m
      - capacity
      - nodegroup
      - status
    group-by:
      - nodegroup
    sort: price
    order: descending
    resources:
      - cpu
      - memory
default-view: cost
```

A view without `resources` shows cpu only. `r` still cycles the resources
on screen.

Spot nodes only, folded to zones and instance types:

```yaml
views:
  - name: spot-by-zone
    filters:
      - field: capacity
        values:
          - Spot
    group-by:
      - zone
      - instance-type
    groups-only: true
```

Every node except Spot, fullest memory first:

```yaml
views:
  - name: on-demand
    filters:
      - field: capacity
        exclude:
          - Spot
    sort: usage:memory
    resources:
      - memory
```

arm64 nodes by a node label, busiest cpu first:

```yaml
views:
  - name: arm
    filters:
      - field: kubernetes.io/arch
        values:
          - arm64
    sort: usage:cpu
```

Open each cluster as it was left, and use `cost` only for a cluster opened
for the first time:

```yaml
views:
  - name: cost
    sort: price
default-view: cost
start-with: last-view
```

## The start view

With `default-view` set and `start-with: default-view`
(or no `start-with`), each cluster opens with that view the first time the
run connects to it, at start or with `c`. With `start-with: last-view`, each
cluster opens with its saved view, and a cluster with no saved view opens
with the default view. Either way:

- the start view replaces the built-in defaults and the view options of
  `~/.kube-node-viewer` (`group-by`, `node-sort`, `resources`,
  `extra-labels`, `groups-only`), as a saved view does;
- the header size, the screen and the groups folded one by one come from
  the cluster's saved view;
- flags given on the command line apply on top, on the cluster the run
  starts on;
- changes made in the TUI are saved per cluster as usual, and going back
  to a cluster with `c` shows it as it was left in this run. In
  `default-view` mode the next start opens the default view again.

Without `default-view` each cluster opens with its saved view, as before.

## The Views panel

In the Views panel config views come first, in file order, marked
`(config)`. `enter` applies them like any named view. `d` and saving under
their name are refused with a notice: the TUI never writes `config.yaml`. A
view saved from the TUI under a name a config view now has is not listed;
it stays in the state file and comes back if the config view is renamed.

A view may use a label the label lists hide: it is applied, and the panels
keep listing the label while the view uses it.

## Errors

Lists are YAML block lists (`- item`); flow lists (`[a, b]`)
are read too. Keys compare exactly (`Name:` is unknown). Strings that YAML
reads as something else (`yes`, `123`, `~`) must be quoted; `yes`/`no` and
`on`/`off` are read as `true`/`false` for `groups-only`. The start stops,
before the TUI, with the file and the place of the problem for an unknown
key, a value of the wrong type, a missing, empty or repeated `name`, a
name with leading or trailing spaces, an empty list in a view or a repeated
item, more than three `group-by` levels, a
built-in name not allowed in that key, an `order` or `start-with` value
not listed above, a filter without `field`, with both or neither of
`values` and `exclude`, or a second filter on the same field,
`groups-only: true` without `group-by`, a `default-view` naming no config
view, and `start-with: default-view` without `default-view`.

Names are checked the way Kubernetes checks them: a label key (bare or
after `label:`) or a resource name (`resources`, `usage:`) with leading or
trailing whitespace, a comma, an upper-case title with a `/` (`PRICE/M`;
write `price/m`) or any other character a label key cannot have is an
error, and so are filter values and `labels.allow`/`labels.deny` patterns
with leading or trailing whitespace. Upper-case names without a `/`
(`PRICE`, `MyLabel`) are valid label keys, so they are read as labels.
`- label: zone`, with a space after the colon, is a YAML map: write
`label:zone`. A price or idle column in `sort` (`price/m`) says to write
`sort: price`, and `sort: age` says `sort: creation`:

```text
$ kube-node-viewer
2026/11/02 22:10:05 config file /home/me/.config/kube-node-viewer/config.yaml: views[1].filters[0]: needs values or exclude
```
