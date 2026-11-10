# Requirements: hide-cloud-ids

> Status: active · Owner: Carlos Gomes · Reviewed: 2026-11-09 (spec-refuter; 1 HIGH, 7 MEDIUM, 9 LOW, applied)

## Context

[short-gke-context](../short-gke-context/requirements.md) took the project id
out of the headers and the export file name for
`gke_<project>_<location>_<cluster>` contexts, and left gaps:

- gcloud writes two more context forms this project handles here, for GKE
  multi-cloud and attached clusters and for fleet memberships reached through
  the Connect gateway. They still show whole in the headers and the file name.
- Those contexts' API server URL holds `/projects/<project>/`, and the full
  panel prints it in the `Cluster:` row.
- The Provider ID row of node details (`pkg/model/nodedetails.go:125`) shows
  the id as the node carries it. On Azure that is the subscription id and the
  resource group, on GCE the project.

A node Karpenter created but that has no name yet is listed by its provider
ID (`pkg/model/node.go:268-270`, `:288-300`), so on a cloud other than AWS the
node list can show the same ids.

No GKE, multi-cloud or Connect gateway cluster is available to this project;
the names below are made up, and the formats come from the gcloud source
(design D1, D2). gcloud's default Connect gateway path takes the context name
from the API response, so that form could not be proved from the source
(design D1).

## Usage examples

With made-up values, a Connect gateway context:

```text
(full panel, before)
 Context:  connectgateway_example-project_global_demo-fleet
 Cluster:  https://connectgateway.googleapis.com/v1/projects/123456789/locations/global/gkeMemberships/demo-fleet
(full panel, after)
 Context:  demo-fleet
 Cluster:  https://connectgateway.googleapis.com/v1/projects/…/locations/global/gkeMemberships/demo-fleet
```

A GKE attached cluster, `gke_attached_<project>_us-west1_demo-attached`, shows
`demo-attached`. The contexts popup (`c`) keeps every full name.

The Provider ID row of node details:

```text
(GCE, before)
 Provider ID  gce://example-project/us-central1-a/gke-demo-pool-1-abcd
(GCE, after)
 Provider ID  gce://…/us-central1-a/gke-demo-pool-1-abcd

(Azure, before)
 Provider ID  azure:///subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/example-rg/providers/Microsoft.Compute/virtualMachineScaleSets/aks-demo-vmss/virtualMachines/3
(Azure, after)
 Provider ID  azure:///…/virtualMachineScaleSets/aks-demo-vmss/virtualMachines/3

(AWS, unchanged: it holds no account id)
 Provider ID  aws:///us-east-1a/i-0123456789abcdef0
```

## Requirements

**R1** WHEN the context name is exactly
`gke_<kind>_<project>_<location>_<cluster>`, `<kind>` being `aws`, `azure` or
`attached`, `<project>` and `<location>` as in short-gke-context R1 and
`<cluster>` 1 to 63 lower-case letters, digits or `-`, starting and ending with
a letter or a digit, THEN the three headers SHALL show `<cluster>` in place of
the context name.

- [x] Each of the three kinds shows the cluster.
- [x] With a server URL of R10's shape, no header contains the project.
- [x] A domain-scoped project (`example.com:example-project`) matches.

_Test: `// hide-cloud-ids R1` on `TestShortFleetContextName` (table) and
`TestHeadersHideTheFleetProject` (the three headers), pkg/model._

**R2** WHEN the context name is exactly
`connectgateway_<project>_<location>_<membership>`, optionally followed by
`_ns-<namespace>`, `<project>` as in short-gke-context R1, `<location>` either
`global` or as in short-gke-context R1, and `<membership>` and `<namespace>` 1
to 63 lower-case letters, digits or `-`, starting and ending with a letter or a
digit, THEN the three headers SHALL show `<membership>` in place of the context
name.

- [x] `global` and a region both match.
- [x] With and without `_ns-team-a` the header shows the membership alone.
- [x] A membership starting with a digit matches.

_Test: `// hide-cloud-ids R2` on `TestShortFleetContextName` and
`TestHeadersHideTheFleetProject`, pkg/model._

**R3** WHEN a context name starts with `gke_` or `connectgateway_` and is
neither form above nor the form of short-gke-context R1 THEN the headers SHALL
show it unchanged: another `<kind>`; a project, location, cluster or membership
outside its rule; a name of 64 characters; upper-case or non-ASCII letters;
a missing or extra field; extra text before or after; a trailing newline; an
empty namespace.

- [x] Each of those renders exactly as it is.
- [x] Every name short-gke-context R1 and R2 accept or refuse keeps its result,
  except its two named forms, `connectgateway_…` and `gke_{kind}_…`, which R1
  and R2 here now shorten. A project named `attached` in the
  three-field form stays short-gke-context's case.

_Test: `// hide-cloud-ids R3` on `TestShortFleetContextName`, pkg/model._

**R4** WHEN the context is a form of R1 or R2 THEN the export file name SHALL
use the short name as short-gke-context R3 says, and the contexts popup and its
"connecting to" line, the JSON export's `context`, the saved-state key, the
`--context` flag, the `context` config key, log lines and error messages SHALL
keep the full name.

- [x] The file name and the footer notice hold no project.
- [x] The popup, the "connecting to" row and the JSON `context` hold the full
  name.
- [x] The saved-state key is unchanged.

_Test: `// hide-cloud-ids R4` on `TestFleetNameInExportAndPopup`, pkg/model, and
`TestClusterKeyKeepsTheFullContext`, pkg/client._

_Test: manual — only the `--context` flag, the `context` config key and log and
error lines: they read the kubeconfig name and are untouched by the diff._

**R5** WHEN a node's provider ID is exactly `gce://<project>/<zone>/<instance>`,
each part non-empty and with no `/` or white space, THEN the Provider ID row of
node details SHALL show `gce://…/<zone>/<instance>`.

- [x] The row holds the zone and the instance, and not the project.
- [x] A domain-scoped project (`example.com:example-project`) is hidden too.

_Test: `// hide-cloud-ids R5` on `TestDisplayProviderID` (table) and
`TestNodeDetailsHideTheCloudIDs`, pkg/model._

**R6** WHEN a node's provider ID is `azure:`, one or more `/`, then
`subscriptions/<id>/resourceGroups/<group>/providers/<namespace>/<rest>`, the
words `subscriptions`, `resourceGroups` and `providers` in any case, `<id>`,
`<group>` and `<namespace>` non-empty with no `/` or white space, and `<rest>`
non-empty with no white space, THEN the row SHALL show `azure:///…/<rest>`.

- [x] A scale set instance shows the scale set and the instance number, and
  neither the subscription nor the resource group.
- [x] A standalone VM (`virtualMachines/<name>`) shows its name.
- [x] `resourcegroups` in lower case and `azure://` with two slashes match.

_Test: `// hide-cloud-ids R6` on `TestDisplayProviderID` and
`TestNodeDetailsHideTheCloudIDs`, pkg/model._

**R7** WHEN a provider ID is neither form above THEN the row SHALL show it
unchanged, and an empty one SHALL still show `-`.

- [x] `aws:///us-east-1a/i-0abc` and a `kind://` id render exactly as they are.
- [x] So do `gce://p/zone` (a part missing), `gce://p/zone/i/extra`, `gce:///zone/i`,
  `azure:subscriptions/…` (no slash), an empty subscription, an empty group, and
  an Azure id with nothing after the provider namespace.

_Test: `// hide-cloud-ids R7` on `TestDisplayProviderID`, pkg/model._

**R8** WHEN a node has no name yet (known only from its NodeClaim, whose
provider ID is never empty here) THEN the name shown wherever a node's name
shows (the list, details and Pods titles, the DaemonSets line, recommendations,
the export) and matched by `/` search SHALL be the provider ID as R5 to R7 show
it, and on AWS SHALL stay the instance id.

- [x] A nameless GCE node and a nameless Azure node hold no project,
  subscription or resource group in their name.
- [x] A nameless EC2 node is named by its instance id.

_Test: `// hide-cloud-ids R8` on `TestNamelessNodeHidesTheCloudIDs`, pkg/model._

**R9** WHEN a node's id is masked for display THEN the store SHALL still tell
nodes apart by the whole id, and the cloud, the zone and the capacity type read
from a provider ID SHALL be what they were.

- [x] Two GCE nodes whose ids differ only in the project stay two nodes in the
  store.
- [x] The cloud of a GCE and an Azure node, the zone of an EC2 node with no
  zone label, and the capacity type of an Azure node are unchanged.

_Test: `// hide-cloud-ids R9` on `TestProviderIDStaysWholeInTheStore`, pkg/model._

**R10** WHEN the API server URL is exactly
`https://<host>/<version>/projects/<project>/locations/<rest>`, `<version>` a
`v`, a digit and any letters or digits, THEN the `Cluster:` row of the full
panel SHALL show `…` in place of `<project>`. Any other URL SHALL be shown as
it is, and the JSON export's `server` SHALL keep the whole URL.

- [x] The row holds the rest of the URL and not the project.
- [x] An EKS URL and an AKS URL render as they are.
- [x] The JSON `server` holds the whole URL.

_Test: `// hide-cloud-ids R10` on `TestDisplayServer` (table) and
`TestHeadersHideTheFleetProject`, and on `TestFleetNameInExportAndPopup` (the
JSON), pkg/model._

> **Corrected 2026-11-09 (code review).** In R10, `<host>` and `<project>` are
> non-empty with no `/` or white space, `<version>` is `v`, a digit, then
> lower-case letters or digits, and `<rest>` is non-empty with no white space.
> "White space" in R5, R6 and R10 is ASCII white space (tab, newline, form feed,
> carriage return, space); other spaces count as part of an id. R8's `/` search
> and the other places it lists read the same name as the node list; only the
> name itself is tested.

## Counter-examples

- The system SHALL NOT shorten a name in the contexts popup, where the user
  picks among contexts.
- The system SHALL NOT remove the JSON export's `context` field, or mask its
  `server`.
- The system SHALL NOT mask a provider ID it does not recognise by guessing
  which part is an id.
- The system SHALL NOT change the store's key or what pricing receives.
- The system SHALL NOT add a flag or config key to see the full id:
  `kubectl get node -o yaml` shows it.

## Out of scope

- Node labels, shown as they are because showing every label is the feature:
  in node details, in `/` search and in the export. On AKS
  `kubernetes.azure.com/cluster` holds the resource group; a known limit,
  recorded in `TODO.md`.
- Node names, which are what `kubectl` takes and the cloud chooses (`TODO.md`).
- gcloud's `edgecontainer_{project_id}_{location}_{cluster_id}` form, for
  Distributed Cloud clusters (`TODO.md`).

  > **Corrected 2026-11-10.** [short-edge-context](../short-edge-context/requirements.md)
  > shortens this form, and its offline-credential variant, in the headers and
  > the export file name.
- Context names people choose, and any other cloud's context format.
- Masking ids in logs and error messages.
- Telling apart two nameless nodes whose masked ids are the same, which needs one
  cluster with the same zone and instance in two projects (design D2).
