# Design: context-visibility

> **Corrected 2026-11-18.** `readContextLists` became `readPatternLists`, one function for the label and the context lists. The R6 notice text and the `main` wiring order are checked by the T6 run, not by a unit test.

> Implements [requirements](requirements.md).

## Current state

- `config.yaml` is read by `readSettingsFile` (`cmd/kube-node-viewer/settings.go`):
  the top level allows `labels`, `views`, `default-view`, `start-with` and
  `clusters`; `readLabelLists` checks `labels.allow` and `labels.deny` with
  `mapping`, `list` and `patternList`.
- `model.LabelPolicy` (`pkg/model/labelpolicy.go`) compiles allow and deny
  patterns with `*` and answers `Shows(key)`: deny first, then allow, whole
  string, case-sensitive.
- The context picker (`c`) gets its names from `sessions.Contexts`
  (`cmd/kube-node-viewer/session.go`), which calls `client.Contexts`.
- The Clusters screen gets its contexts from `sessions.fleetContexts`
  (`cmd/kube-node-viewer/fleet.go`), and its saved choice through
  `fleetStore` (`cmd/kube-node-viewer/main.go`) from the state file.
- The model shows a saved context that is not in the kubeconfig as `missing`
  in the Clusters picker (`pkg/model/clusters_picker.go`).
- The start context is `flags.Context`, else the kubeconfig's current one,
  resolved in `sessions.Switch` by `client.ResolveCluster`.

## Target state

```text
config.yaml contexts.allow/deny ──> settings.contexts (model.LabelPolicy)
main, after loadSettings, before SetFleet:
  effectivePolicy(start, policy) ── start hidden? ──yes──> zero policy, off
                                                └─no──> policy
  the same value to sessions.contexts and fleetStore; off: go Send(NoticeMsg),
  model.SetContextListsOff (picker titles)
sessions.Contexts      ── names, filtered ──> context picker (c)
sessions.fleetContexts ── infos, filtered ──> Clusters picker, fleetSync
fleetStore.Fleet       ── saved choice, hidden ones left out ──> model
fleetStore.SaveFleet   ── model's choice + the hidden ones of the file
```

## Components touched

| Path | Change |
|------|--------|
| `cmd/kube-node-viewer/settings.go` | `contexts` key: `readContextLists` with the label lists' checks; `settings.contexts` |
| `cmd/kube-node-viewer/session.go` | `sessions.contexts` (the policy in effect, zero = all); `Contexts` filters the names |
| `cmd/kube-node-viewer/fleet.go` | `fleetContexts` filters the infos |
| `cmd/kube-node-viewer/main.go` | `effectivePolicy(start, policy) (LabelPolicy, off bool)`, called after `loadSettings` and before `SetFleet`; its result goes to `sessions` and `fleetStore`; off sends the notice with `go program.Send` (Send blocks until the program runs) and sets the picker titles; `fleetStore` filters on read and keeps hidden entries on save |
| `pkg/model/contexts.go`, `pkg/model/clusters_picker.go` | `SetContextListsOff`: both picker titles say `lists off` |
| `README.md`, `CHANGELOG.md` | the setting |

## Decisions

### D1: Reuse `model.LabelPolicy` for contexts

**Choice**: compile the context lists with `model.NewLabelPolicy` and ask
`Shows(name)`; no new matcher.

**Justification, strongest first**:
1. The owner asked for the same allow/deny with wildcards as labels; one
   matcher keeps one set of rules (R3 is label-visibility R4 with a context
   name in place of a key).
2. It is tested already (`labelpolicy_test.go`).

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| The type is named for labels | a comment at the use says it is the same rule; renaming touches label code for no behaviour |

**When this would be wrong**: contexts need a rule labels do not (case-
insensitive match, short names) — then a matcher of their own.

**Reversal action**: a `ContextPolicy` type with its own matcher; the
settings and session code change one call.

**Alternatives discarded**: `path.Match` globs — `*` stops at `/`, which ARN
contexts hold.

### D2: Filter in `cmd`, not in the model

**Choice**: the lists are applied where the names come from:
`sessions.Contexts`, `sessions.fleetContexts` and `fleetStore`. The model
never sees a hidden context.

**Justification, strongest first**:
1. Three small filters at the sources cover the two pickers, the Clusters
   screen's wanted list and its saved choice; the model's code (`missing`,
   de-duplication by server, the scope) works unchanged on what it gets.
2. A hidden context never reaches the model, so no screen can show it by
   mistake.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| `fleetStore` must keep the hidden entries on save | a few lines, tested (R7) |
| The model cannot say "N contexts hidden" | not asked; the README says how the lists work |

**When this would be wrong**: the screens should say how many contexts the
lists hide — then pass the count with the names.

**Reversal action**: pass the policy to the model and filter there.

**Alternatives discarded**: filter in `client.Contexts` — `client` would
read the settings, a layering it does not have today.

### D3: A hidden start context turns the lists off for the run

**Choice**: `main` takes the start context from the first session's info
(`first.Info.Context`, which `Switch` resolved from `--context` or the
kubeconfig's current context) and calls
`effectivePolicy(name, policy)` after `loadSettings` and before `SetFleet`
(which reads the saved choice once). A non-empty name the policy hides gives
the zero policy (everything listed) and `off`; an empty name (no context in
the kubeconfig) is never hidden. The one value goes to `sessions.contexts`
and to `fleetStore`, so the pickers, the wanted list and the saved choice
agree. With `off`, the program gets one `NoticeMsg` (sent with `go`, as
`Send` blocks until the program runs) and both picker titles say
`lists off`, since a notice lasts seconds.

**Justification, strongest first**:
1. The owner's choice: always open on the current context, and list
   everything when it is a hidden one.
2. A picker that hides the context on screen would not show where the user
   is, nor let them come back to it after `c`.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| A hidden context as current shows every context for the whole run | the footer says why; the next run on an allowed context applies the lists |

**When this would be wrong**: users start on a hidden context on purpose and
want the lists anyway — then apply them and keep only the context on screen.

**Reversal action**: keep the policy and add the context on screen to the
listed names.

**Alternatives discarded**: refuse to start — the owner wants it to open;
switch to the first allowed context — changes what `kubectl` users expect.

### D4: Hidden saved contexts are kept in the state file

**Choice**: `fleetStore.Fleet` returns the saved choice without the hidden
contexts; `SaveFleet` writes the model's choice plus the hidden contexts of
the file's previous choice.

**Justification, strongest first**:
1. Lists change; a context hidden today and allowed tomorrow comes back
   ticked (R7).
2. The model's choice is what it can see; it cannot keep what it never got.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| A hidden context the user meant to untick stays saved | it is hidden, so it is never connected |

**When this would be wrong**: the state file grows with contexts deleted
long ago — then drop hidden entries not in the kubeconfig.

**Reversal action**: save the model's choice only.

**Alternatives discarded**: drop hidden entries on save — a list change loses
the choice.

## Invariants

- The start context is never changed by the lists.
- Per-cluster view state (`context@server`) and named views are untouched; a
  hidden context's saved view is kept.
- The pickers, the Clusters screen's wanted list and its saved choice see the
  same policy.
- A hidden context is never connected by the Clusters screen, unless the
  lists are off for the run (D3).
- No change to the kubeconfig.

## Failure modes

| Failure | Behavior | How the user notices |
|---------|----------|----------------------|
| Invalid lists in the file | the tool does not start | error naming the place |
| Lists hide every context but the current | pickers list only it | nothing else listed |
| `deny: ["*"]` | the start context is always hidden: the lists are off, unless the run has no context name, which lists nothing | `lists off` in the picker titles |
| Start context hidden | lists off for the run | footer notice |
| Saved choice holds a hidden context | not connected, kept in the file | no section |

## Test plan

- `cmd`: settings tests for `contexts` (R8, R4), errors prefixed
  `contexts…`; sessions `Contexts` and `fleetContexts` filtered (R1–R3);
  `effectivePolicy` table: hidden start, allowed start with a hidden
  kubeconfig current, empty name (R5, R6); `fleetStore` read and save with
  hidden entries, and with the lists off (R6, R7).
- `cmd`: from a kubeconfig where a hidden context and an allowed one share a
  server, the hidden one is in neither `fleetContexts` nor `fleetStore.Fleet`
  (R7).
- `pkg/model`: picker titles with the lists off (R6); with only the allowed
  context in the infos, it is wanted and no `same cluster as` mark appears
  (R7).
- The matcher is `LabelPolicy`'s, already tested; add cases with ARN names
  (R3).
- Manual: the binary on two kind clusters with a deny list, as in the
  clusters-screen run.
