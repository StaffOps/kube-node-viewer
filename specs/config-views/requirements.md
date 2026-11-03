# Requirements: config-views

> Status: active · Owner: sule · Reviewed: 2026-11-02 (code-review agent, round 1; findings applied; approved by owner 2026-11-03)

## Context

Every time the owner opens the viewer they set up the same view by hand:
these columns in this order, this group-by, this sort. Named views (`v`)
help, but they live in the state file, are built by clicking through the
panels, and the viewer opens with each cluster's last view, not a chosen one.

This feature lets `config.yaml` (the file of
[label-visibility](../label-visibility/requirements.md)) define views by
hand, choose one to open with, and switch between them with `v` like the
named views already saved from the TUI.

Depends on: `label-visibility` (the `config.yaml` loader). The monthly column
names (`price-month`, `idle-month`) are accepted once `monthly-price-columns`
is merged; the names come from the model, so nothing here changes for them.

## Usage examples

`~/.config/kube-node-viewer/config.yaml`:

```yaml
labels:
  allow:
    - karpenter.sh/nodepool
    - karpenter.sh/capacity-type

views:
  - name: cost
    columns:
      - instance-type
      - price
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
    group-by:
      - karpenter.sh/nodepool
    groups-only: true

default-view: cost
start-with: default-view
```

Opening the viewer on any cluster shows `cost`. `v` lists:

```text
 Views
   cost (config)
   spot (config)
   my-saved-view
   + save current view as…
```

`enter` on `spot` applies it. Changes made in the TUI last until the viewer
closes; the next start opens `cost` again. With `start-with: last-view`,
each cluster opens with the view it was left in, and `cost` is only the
starting point of a cluster opened for the first time.

## Requirements

**R1** WHEN `config.yaml` has a `views` list THEN the system SHALL read each
item as a view with a `name` and any of `columns`, `group-by`, `sort`,
`order`, `resources`, `filters` and `groups-only`; a key left out takes the
built-in default (the start view replaces `~/.kube-node-viewer` too, see R4).

- [x] `columns`: list of fields, in screen order.
- [x] `group-by`: list of one to three fields, top level first.
- [x] `sort`: one field. `order`: `ascending` or `descending`. Without
  `order`, the direction the Sort panel starts with for that field:
  descending for `creation`, `price`, `idle`, `pods` and usage, ascending for
  the rest. `order` without `sort` sets the direction of the `creation` sort.
- [x] `resources`: list of resource names, written in full (`cpu`, `memory`,
  `nvidia.com/gpu`).
- [x] `filters`: list of items, each with `field` and either `values` (keep
  nodes with any of them) or `exclude` (drop nodes with any of them). Filters
  start turned on.
- [x] `groups-only`: starts folded to the groups, as `--groups-only`. YAML
  reads `true`/`false` and also `yes`/`no`, `on`/`off`.
- [x] Every list is a YAML block list (`- item`). A flow list (`[a, b]`)
  is read too; the docs show only block lists.

**R2** WHEN a view names a field THEN the system SHALL accept, per key, these
built-in names, and read any other name as a node label key:

| Key | Built-in names |
|-----|----------------|
| `group-by`, `filters[].field` | `capacity`, `nodegroup`, `zone`, `region`, `instance-type`, `status` |
| `sort` | `creation`, `name`, `ip`, `price`, `idle`, `pods`, `usage:<resource>`, and the `group-by` names |
| `columns` | `instance-type`, `price`, `capacity`, `nodegroup`, `zone`, `status`, `idle`, `region`, `age`, `ip` (plus `price-month`, `idle-month` with monthly-price-columns) |

- [x] `karpenter.sh/nodepool` is the node label of that key in any key.
- [x] `label:zone` names the node label `zone`, not the derived field.
- [x] In `columns`, the lower-cased screen titles name the columns:
  `price/h`, `price/m`, `idle/h`, `idle/m` (`price`, `price-month`, `idle`,
  `idle-month` are accepted too). `sort: price` orders `PRICE/H` and
  `PRICE/M` alike.
- [x] A built-in name used in a key that does not list it (`group-by:
  - price`) is an error (R6), not a label.

**R3** WHEN the user opens the Views panel (`v`) THEN the system SHALL list
the config views first, in file order, marked `(config)`, then the views
saved from the TUI; `enter` SHALL apply either kind the same way.

- [x] `d` on a config view does not delete it and says it is defined in
  `config.yaml`.
- [x] Saving from the TUI under a config view's name is refused with a
  notice.
- [x] A view saved earlier under a name a config view now has is not listed
  and stays in the state file; the config view is the one applied.

**R4** WHEN `default-view` names a config view and `start-with` is
`default-view` or not set THEN the system SHALL open each cluster with that
view the first time the run connects to it, at start or with `c`.

- [x] The start view replaces the built-in defaults and the view options of
  `~/.kube-node-viewer` (`group-by`, `node-sort`, `resources`,
  `extra-labels`, `groups-only`), as a saved view does today.
- [x] Header size, screen (Overview / Recommendations) and groups folded one
  by one come from the cluster's saved view, as when a view is applied
  with `v`.
- [x] Changes made in the TUI are saved per cluster as today. Returning to a
  cluster with `c` in the same run shows it as it was left in this run.
- [x] Flags given on the command line (`--group-by`, `--node-sort`, …) apply
  on top, on the cluster the run starts on.

**R5** WHEN `start-with` is `last-view` THEN the system SHALL open each
cluster with its saved view, and a cluster with no saved view with the
`default-view` (or the built-in defaults and `~/.kube-node-viewer` without
one).

**R6** WHEN `config.yaml` has a view or setting it cannot use THEN the system
SHALL exit before the TUI starts with the file and the path of the problem
(label-visibility R8 rules), for:

- [x] an unknown key, compared exactly (`Name:` is unknown);
- [x] a value of the wrong type, including `yes`, `123` or `~` where a string
  is expected ("not a string; quote it");
- [x] a missing, empty or repeated `name`, or one with leading or trailing
  spaces;
- [x] an empty list, or a list item repeated (`columns`, `group-by`,
  `resources`, filter `values`/`exclude`), or more than three `group-by`
  levels;
- [x] a built-in name not allowed in that key (R2);
- [x] `order` not `ascending`/`descending`; `start-with` not
  `default-view`/`last-view`;
- [x] a filter without `field`, with both or neither of `values` and
  `exclude`, or a second filter on the same field;
- [x] `groups-only: true` without `group-by`;
- [x] `default-view` naming no config view; `start-with: default-view`
  without `default-view`.
- [x] Example: `config file …/config.yaml: views[1].filters[0]: needs values or exclude`.

**R7** WHEN a view uses a label the label lists hide THEN the system SHALL
apply it and keep it listed in the panels (label-visibility R5).

**R8** WHEN the user reads the README THEN the system SHALL find a "Config
views" section with a complete example, every key and its values, the field
names per key, how the start view is chosen, the errors, and that `zone` in
`config.yaml` is the derived field while `--group-by zone` is the label
`zone`. Every YAML example uses block lists and is decoded by a test.

## Counter-examples

- MUST NOT write to `config.yaml`; the TUI never edits it.
- MUST NOT drop or change the named views of the state file.
- MUST NOT change what happens without `views` and `default-view`: each
  cluster opens with its saved view, as today.

## Out of scope

- Per-cluster default views (`default-view` per kubeconfig context).
- Saving a TUI view into `config.yaml`.
- Folding depth, details, folded groups, header size and screen in config
  views.
- Moving the `~/.kube-node-viewer` options into `config.yaml`.

## Notes

- 2026-11-03 (owner): the full reference moves to `docs/views.md`; the
  README keeps a short example and links it (R8 reads "the docs").
- 2026-11-03 (owner): `price` alone was ambiguous next to `PRICE/H` and
  `PRICE/M`. Columns are named by their lower-cased screen titles
  (`price/h`, `price/m`, `idle/h`, `idle/m`), the field names stay
  accepted. `sort` also takes the derived fields the Sort panel offers.
  View names with leading or trailing spaces are errors. The Columns panel
  now offers `IP` for everyone (it shares `ColumnFields()` with the config).
