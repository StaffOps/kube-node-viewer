# Requirements: node-details

> Status: shipped (v0.9.0, 2026-11-02) · Owner: Carlos Gomes · Reviewed: 2026-11-04 (spec-refuter, retroactive; 3 MEDIUM, 2 LOW; applied), earlier 2026-11-02 (owner)

> **Corrected 2026-11-05.** The Details tab has changed since this spec
> shipped, and the spec was not updated: pods left Resources for their own
> section, capacity shows only when it differs from allocatable, a Pods tab
> and a DaemonSets section were added, and the hint grew `home: top • tab:
> pods`. Those changes are specified in
> [node-pods](../node-pods/requirements.md) R1 (the tab, the key line) and
> R11 (the Resources, Pods and DaemonSets sections, capacity), with its
> 2026-11-03 owner changes; where they differ, node-pods wins. In particular the
> usage example (`1820m / 1930m (94%) / 2`, the hint), R1's "pods" in
> Resources and the counter-example "MUST NOT list pods" are superseded.
> The 2026-11-02 review was by the owner, who is also the author; the spec
> and the code landed in one commit.

## Context

The node list shows one line of facts per node. To see why a node holds
pods, won't take one, or costs what it does, the owner leaves the viewer for
`kubectl describe node`. A popup over the selected node answers that in place.
Pods are left out: the list already counts them, and `kubectl` lists them.

## Usage example

> **Corrected 2026-11-03.** The node and node group names below are
> made-up example values, as `scripts/checks/content.sh` requires.

```text
↓ ↓ (cursor on ip-10-0-133-127.ec2.internal) → enter
╭─ ip-10-0-133-127.ec2.internal ───────────────────────────╮
│ Instance     r6g.large · OnDemand · $0.1008/h            │
│ Node group   example-search-1                            │
│ Zone         us-east-1a (us-east-1)                      │
│ Status       Ready · age 2d5h · created 2026-10-31 15:23 │
│ …                                                        │
│ Resources    requested / allocatable / capacity          │
│   cpu        1820m / 1930m (94%) / 2                     │
│ Conditions                                               │
│   Ready      True   KubeletReady                         │
│ Taints       dedicated=elasticsearch:NoSchedule          │
│ Labels (24)                                              │
│   kubernetes.io/arch=arm64                               │
├──────────────────────────────────────────────────────────┤
│ ↑/↓: scroll • esc: close                                 │
╰─────────────────────────────────────────── 1-20 of 52 ───╯
```

## Requirements

**R1** WHEN the cursor is on a node row and the user presses `enter` THEN the
system SHALL open a popup with that node's details; `enter` on a group
heading still folds it.

> **Corrected 2026-11-04 (group-focus).** `enter` on a group heading shows
> only that group's nodes, and `space` folds it
> ([group-focus](../group-focus/requirements.md) R1, R5); `enter` on a node
> is unchanged.

- [x] Sections: instance (type, capacity, price), node group, zone and
  region, status (flags, age, creation time), system (kubelet, OS image,
  kernel, architecture, runtime), provider ID and internal IP, resources
  (requested / allocatable / capacity for cpu, memory, pods, ephemeral
  storage and any extended resource such as `nvidia.com/gpu`), conditions
  (type, status, reason; a condition in its bad state in red), taints, labels
  sorted by key.
- [x] The popup follows the live node: values update as the cluster changes;
  a node that goes away says so instead of closing.
- [x] `↑/↓` (`k/j`) scroll, `home` goes to the top, `esc` or `enter` closes.

## Counter-examples

- MUST NOT list pods or annotations (annotations are noise, often large).
- MUST NOT call the API server: everything comes from the watched node.

## Out of scope

Editing labels or taints, cordon/drain, events.
