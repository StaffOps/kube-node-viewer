# Requirements: context-visibility

> Status: active · Owner: Carlos Gomes · Reviewed: 2026-11-18 (spec-refuter: first pass 2 high, 4 medium, 5 low; second pass ready for active, 1 medium and 3 low fixed after it)

## Context

A kubeconfig often holds many contexts, and someone may only want some of them
in the tool: the production clusters of one team, not the sandboxes, not a
customer's. Today the context picker (`c`) and the Clusters screen's picker
list every context of the kubeconfig.

This adds `contexts.allow` and `contexts.deny` to `config.yaml`: lists of
patterns, with `*` as a wildcard, that decide which contexts the pickers
offer. They follow the rules of `labels.allow` and `labels.deny`
([label-visibility](../label-visibility/requirements.md)).

This is about what the tool shows, not access: the kubeconfig still reaches
every cluster with `kubectl` or any other tool.

## Usage examples

```yaml
contexts:
  allow:
    - "prod-*"
    - "arn:aws:eks:*:111122223333:cluster/*"
  deny:
    - "*-sandbox"
```

With contexts `prod-us`, `prod-sandbox`, `staging` and
`arn:aws:eks:us-east-1:111122223333:cluster/billing`, the pickers list
`prod-us` and the ARN; `prod-sandbox` is denied; `staging` matches no allow
pattern.

Deny only: `contexts: {deny: ["kind-*"]}` lists everything but the kind
contexts.

The tool starts on `staging` (the kubeconfig's current context) with the
lists above: it opens on `staging`, the lists are not applied in that run,
every context is listed, and the footer says why.

## Requirements

**R1** WHEN `config.yaml` sets `contexts.allow` THEN the system SHALL list, in
the context picker (`c`) and in the Clusters screen's picker, only the
contexts whose name matches at least one allow pattern.

- [ ] With `allow: [prod-*]` and contexts `prod-us`, `staging`, both pickers
  list `prod-us` and not `staging`.
- [ ] An empty `allow: []` behaves as no allow list.

**R2** WHEN `config.yaml` sets `contexts.deny` THEN the system SHALL leave out
of both pickers every context whose name matches a deny pattern, also when it
matches an allow pattern.

- [ ] With `allow: [prod-*]` and `deny: [*-sandbox]`, `prod-sandbox` is not
  listed and `prod-us` is.
- [ ] With only `deny: [kind-*]`, every context but `kind-…` is listed.

**R3** WHEN a pattern is compared with a context THEN the system SHALL compare
it with the context's full name, as the kubeconfig writes it, where `*`
matches any run of characters (`:`, `/` and `.` included) and every other
character only itself, case-sensitively, the whole name.

- [ ] `arn:aws:eks:*:111122223333:cluster/*` matches
  `arn:aws:eks:us-east-1:111122223333:cluster/billing`.
- [ ] `billing` does not match `arn:aws:eks:us-east-1:111122223333:cluster/billing`.
- [ ] `Prod-*` does not match `prod-us`.

**R4** WHEN there is no config file, or it sets neither list THEN the system
SHALL list every context, as today.

**R5** WHEN the tool starts THEN the system SHALL open on the context it
starts on today (`--context`, else the kubeconfig's current context), whatever
the lists say; only that context, the one the run opens on, is checked
against the lists.

- [ ] A start context hidden by the lists opens as any other.
- [ ] Started with `--context prod-us` (allowed) while the kubeconfig's
  current context is `staging` (denied), the lists apply and `staging` is
  not listed.
- [ ] A run with no context name (no `--context`, no current context in the
  kubeconfig) is never "hidden": the lists apply.

**R6** WHEN the context the tool starts on is hidden by the lists THEN the
system SHALL apply neither list in that run: both pickers list every context,
and the footer says so once, naming the setting.

- [ ] With `deny: [staging]` and the tool started on `staging`, `c` lists
  every context, `staging` marked current.
- [ ] The footer shows `contexts.allow/deny not applied: the context on
  screen is hidden by them` once after start.
- [ ] Started on an allowed context, the lists apply as in R1–R2.
- [ ] Started with `--context staging` (denied) while the kubeconfig's
  current context is allowed, the lists are off.
- [ ] The lists stay off for the whole run, also after `c` to an allowed
  context.
- [ ] While they are off, the title of both pickers says `lists off`.
- [ ] While they are off, a saved Clusters choice holding a hidden context
  ticks and connects it as any other.

**R7** WHEN the Clusters screen's saved choice holds a context the lists hide,
and the lists apply (R6), THEN the system SHALL neither connect nor list it,
and SHALL keep it in the saved choice.

- [ ] A hidden saved context has no section and no picker row, and is not
  shown as `missing`.
- [ ] After `enter` in the Clusters picker, the state file still holds the
  hidden context with the ticked ones, so a later change of the lists finds
  it again.
- [ ] A hidden saved context sharing its API server with an allowed chosen
  one: the allowed one is connected, and no `same cluster as` mark names the
  hidden one.

**R8** WHEN `contexts`, `contexts.allow` or `contexts.deny` is not what the
file allows THEN the system SHALL refuse to start with an error naming the
place in the file, as for the other keys.

- [ ] `contexts: [a]` (not a map), `contexts: {other: x}` (unknown field),
  `allow: prod` (not a list), `allow: [""]`, `allow: [1]` or
  `allow: [" prod-*"]` (padded) each give an error naming `contexts…`.

## Counter-examples

- The lists SHALL NOT stop the tool from starting, nor change which context
  it starts on.
- The system SHALL NOT write to the kubeconfig or change its current context.
- A context hidden by the lists SHALL NOT be connected by the Clusters screen.
- A hidden context SHALL NOT be removed from the saved Clusters choice.

## Out of scope

- Command-line flags for the lists (labels have `--labels-allow`; no one asked
  for contexts).
- Matching the short name (the cluster name of an ARN or gcloud context); the
  owner names contexts with aliases.
- Hiding contexts from other tools, or any access control.
- Matching a literal `*` in a context name: `*` is always the wildcard.

## Open questions

None.
