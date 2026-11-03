# Requirements: label-visibility

> Status: active · Owner: sule · Reviewed: 2026-11-02 (spec-refuter, two rounds; findings fixed; approved by owner 2026-11-02)

## Context

The Filters, Group by, Sort and Columns panels list every label key found on
the watched nodes, after the derived fields. On the owner's clusters a node
carries more than 40 labels and at most 10 are useful, so the useful ones are
hard to find in every panel.

This feature adds a YAML config file in `~/.config/kube-node-viewer/` with an
allow list and a deny list of label key patterns. They decide which node
labels the panels offer. The derived fields (`capacity`, `nodegroup`, `zone`,
`region`, `instance-type`, `status`) and the computed labels
(`kube-node-viewer/node-age` and the usage labels, offered today only by
Columns) are not node labels and stay offered where they are today.

## Usage examples

`~/.config/kube-node-viewer/config.yaml`:

```yaml
labels:
  allow:
    - karpenter.sh/nodepool
    - karpenter.sh/capacity-type
    - topology.kubernetes.io/zone
    - team
    - app.example.com/*
  deny:
    - app.example.com/internal-*
```

On a node with 43 labels, `g` opens Group by with the derived fields and only
the labels matching an allow pattern, minus
`app.example.com/internal-*`:

```text
 Group by
   capacity
   nodegroup
   zone
   ...
   karpenter.sh/capacity-type
   karpenter.sh/nodepool
   team
   topology.kubernetes.io/zone
   app.example.com/owner
 38 labels hidden by config • a: show all
 ↑/↓: move • space/enter: add/remove level • K/J: move level up/down • esc: close
```

Pressing `a` lists all 43 labels until the panel closes; the file is not
changed.

Deny only, no allow list:

```yaml
labels:
  deny:
    - "*.amazonaws.com/*"
    - beta.kubernetes.io/*
    - kubernetes.io/hostname
```

Every label shows except the denied ones.

One run with a different list, without editing the file:

```text
$ kube-node-viewer --labels-allow 'karpenter.sh/*,team'
```

A typo in the file stops the start with the reason:

```text
$ kube-node-viewer
config file /home/sule/.config/kube-node-viewer/config.yaml: unknown field "lables"
$ echo $?
1
```

## Requirements

**R1** WHEN the config file sets `labels.allow` THEN the system SHALL list,
in the Filters, Group by, Sort and Columns panels, only the node label keys
that match at least one allow pattern, in addition to the derived fields and
the computed labels each panel offers today.

- [ ] With `allow: [team]` on nodes labeled `team`, `env` and `kubernetes.io/os`, each of the four panels lists `team` and none of `env`, `kubernetes.io/os`.
- [ ] The derived fields and the computed labels each panel listed before are still listed.
- [ ] A real node label with the `kube-node-viewer/` prefix (such as `kube-node-viewer/instance-price`) follows the lists like any other label.

**R2** WHEN the config file sets `labels.deny` THEN the system SHALL leave
out of those panels every label key that matches a deny pattern, also when
it matches an allow pattern.

- [ ] With `allow: [app.example.com/*]` and `deny: [app.example.com/internal-*]`, `app.example.com/owner` is listed and `app.example.com/internal-id` is not.
- [ ] With only `deny: [env]`, every label except `env` is listed.

**R3** WHEN there is no config file, or it sets neither list THEN the system
SHALL list every label key in the panels, as before this feature.

- [ ] Existing panel tests pass unchanged with no config file.
- [ ] An empty `allow: []` behaves as no allow list.

**R4** WHEN a pattern is compared with a label key THEN the system SHALL
treat `*` as any run of characters, `/` and `.` included, every other
character as itself, match the whole key, and compare case-sensitively.

- [ ] `*.amazonaws.com/*` matches `eks.amazonaws.com/nodegroup` and does not match `amazonaws.com`.
- [ ] `karpenter.sh/*` matches `karpenter.sh/nodepool`; `karpenter` does not match `karpenter.sh/nodepool`.
- [ ] `Team` does not match `team`; `a.b/c` does not match `aXb/c` (`.` is literal).

**R5** WHEN the current view uses a label that the lists hide, as a filter,
a group-by level, the sort or a column (from saved state, a named view, a
flag or the TUI) THEN the system SHALL keep applying it and SHALL list it in
the panels so it can be changed or removed.

- [ ] A saved state grouped by `env` with `deny: [env]` still groups by `env`, and Group by lists `env` as level `[1]`.
- [ ] `--extra-labels env` with `deny: [env]` shows the `env` column and the Columns panel lists it.
- [ ] A field the view used when the panel opened stays listed until the panel closes, so unticking it can be undone.
- [ ] Once the owner removes `env` from the view, the panels stop listing it the next time they open.

**R6** WHEN a panel is open and the lists hide at least one label key THEN
the system SHALL show in the panel how many keys are hidden and that `a`
shows them, and pressing `a` SHALL list every label key in that panel until
it closes, without changing the config file.

- [ ] The status line above the key hints reads `<n> labels hidden by config • a: show all`, where `<n>` is the number of distinct label keys on the current nodes that the lists hide and the view does not use, recomputed on every frame.
- [ ] With no nodes yet, or when nodes change while the panel is open, the count follows the current nodes.
- [ ] On a 10-line window the panel still shows at least one row and the status line.
- [ ] After `a` the status line reads `showing all labels • a: hide`, and `a` again goes back.
- [ ] Closing and reopening the panel starts with the lists applied again.
- [ ] With nothing hidden, the panel shows no count and `a` does nothing.
- [ ] Turning `a` off while a Filters field opened under show-all is open and now hidden closes that field.
- [ ] A context switch closes the panel, as today, so show-all starts off on the next cluster.

**R7** WHEN `--labels-allow` or `--labels-deny` is given THEN the system
SHALL use its comma-separated patterns in place of the matching list of the
config file for that run only.

- [ ] `--labels-allow 'karpenter.sh/*'` replaces the file's allow list and keeps its deny list.
- [ ] `--labels-allow ''` means no allow list for the run.
- [ ] Neither flag is written to the state file or the config file.

**R8** WHEN the config file cannot be read or is not valid (YAML syntax,
unknown key, a list item that is not a YAML string or is empty, such as `123`,
`yes`, `~` or `''`) THEN the system SHALL exit with a
non-zero status before the TUI starts, with a message naming the file and
the problem.

- [ ] Each case above has a test that checks the message names the file.
- [ ] A missing file or a missing directory is not an error.
- [ ] `allow: null` (or `allow:` with no value) means no allow list.

**R9** WHEN the config file is looked up THEN the system SHALL read
`$XDG_CONFIG_HOME/kube-node-viewer/config.yaml`, or
`~/.config/kube-node-viewer/config.yaml` when `XDG_CONFIG_HOME` is unset,
empty or not an absolute path.

- [ ] Tested with `XDG_CONFIG_HOME` set, unset, empty and relative; a relative value is ignored, as the XDG Base Directory spec says.

## Counter-examples

- The system SHALL NOT hide labels in the node details popup: it shows every label, like `kubectl describe node`.
- The system SHALL NOT change the `/` search: it still matches every label value.
- The system SHALL NOT hide the derived fields or the computed labels, even when a pattern matches them.
- The system SHALL NOT add computed labels to panels that do not offer them today.
- The system SHALL NOT drop a filter, group-by level, sort or column from the view because its label is hidden.
- The system SHALL NOT read the allow or deny lists from `~/.kube-node-viewer`.

## Out of scope

- Allow or deny lists per kubeconfig context.
- Moving the other options of `~/.kube-node-viewer` into the YAML file.
- Hiding label values, or patterns other than `*`.
- Hiding labels in the CSV/JSON export beyond the columns the owner picked.

## Open questions

- None.
