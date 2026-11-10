# Requirements: azure-live-skus

> Status: draft · Owner: Carlos Gomes · Reviewed: — (spec-refuter, 2026-11-10)

## Context

The Recommendations screen on AKS ([aks-recommendations](../aks-recommendations/requirements.md))
reads Azure VM sizes (family, vCPU, memory, architecture) from a table copied
into the binary from the cluster autoscaler (aks-recommendations D1,
`pkg/azure/zz_generated_instance_types.go`). That table goes stale:
- **Upstream lags.** Its last change was 2026-03-17 (aks-recommendations T1
  note); families released after it get no advice.
- **The repo and the binary lag.** Nothing refreshes it automatically (the
  workflow step was deferred, aks-recommendations tasks Deferred), and an
  installed binary keeps the table it was built with.
- **It is wrong in places.** It truncates memory to whole GB
  (`Standard_DS1_v2` reads 3 GiB, `Standard_B1ls` 0) and gives constrained
  sizes their full vCPU count (aks-recommendations R2).

The owner decided on 2026-11-10: this tool is for system administrators, who
can read the subscription their cluster runs in. The size catalog SHALL come
live from the Azure Resource SKUs API with the Azure credential already on
the machine, and when there is no credential or no permission the screen
SHALL say so, instead of advising from an old table. No IAM role or app
registration is created for this; only an existing sign-in is used.

## Usage examples

An admin signed in with `az login`, Reader on the subscription, on an AKS
cluster in `eastus` (made-up names, as in the aks-recommendations fixture):

```text
$ kube-node-viewer          # then 6 (Recommendations)
 Recommendations   (Azure VM sizes: loading)
 idle         agent pool userpool (4 nodes)   …
 ...
# a second or two later, sizes and prices arrived:
 rightsize    agent pool legacy (1 nodes)   Standard_D4s_v3 → Standard_D2s_v3 …
 arm          agent pool userpool (4 nodes) Standard_D8s_v5 → Standard_D8ps_v5 …
```

The math popup of a rightsize row says where the size data came from:

```text
 Standard_D2s_v3: 2 vCPU, 8 GiB, x86_64 (Azure Resource SKUs API)
```

Failure examples (each one line on the Recommendations screen, in the notes
area; the idle, consolidation and old-node rows still show):

```text
 Azure VM sizes: the Azure CLI (az) was not found; install it and run az login
 Azure VM sizes: not signed in to Azure; run az login
 Azure VM sizes: the signed-in account can't see the nodes' subscription; az login with an account that can
 Azure VM sizes: no permission to read VM sizes (Microsoft.Compute/skus/read on the subscription, e.g. the Reader role)
 Azure VM sizes: the nodes' subscription was not found or is disabled
 Azure VM sizes: Azure Resource Manager unavailable (503); retrying in 1m
 Azure VM sizes: the nodes' provider ID names no subscription
```

With `--disable-pricing`, no SKU request is made and the screen keeps its
current note ("pricing disabled: Azure rows that need a price are not
shown").

## Requirements

**R1** WHEN the Recommendations screen analyses Azure nodes THEN the system
SHALL read the VM sizes of each (subscription, region) of those nodes from
the Azure Resource SKUs API (`GET
/subscriptions/<id>/providers/Microsoft.Compute/skus?api-version=2021-07-01&$filter=location
eq '<region>'`), once per session, without blocking the screen.
- The subscription comes from the node's provider ID
  (`azure:///subscriptions/<id>/resourceGroups/…`).
- Nothing is asked before Recommendations is shown, so a user who never
  opens it makes no Resource Manager request and sees no error.
- While the answer is awaited the notes say `Azure VM sizes: loading`, and
  the analysis runs again when it arrives (as aks-recommendations R3 does
  for prices).

- [ ] Test (fake Resource Manager): opening Recommendations with two Azure
  nodes in one region sends one request; the note shows, then the rows that
  need sizes appear after the answer.
- [ ] Test: no request before Recommendations is shown.
- [ ] Test: nodes in two regions → two requests; one subscription id and
  region per request, read from the provider ID.

**R2** WHEN the system reads a SKU answer THEN it SHALL keep only
`resourceType` `virtualMachines` and take, per size:
- vCPU: `vCPUsAvailable` when present, else `vCPUs`, so a constrained size
  (`Standard_E8-4ds_v5`) counts its usable 4 and is still flagged
  constrained (it has `vCPUsAvailable` < `vCPUs`, or a `-` in the name);
- memory: `MemoryGB`, fractional (`Standard_DS1_v2` is 3.5 GiB);
- architecture: `CpuArchitectureType` `x64` → `x86_64`, `Arm64` →
  `arm64`;
- family: `family`, normalised as aks-recommendations R2 (lower case,
  without `standard` and `Family`).

An entry missing any of these, or with 0 vCPU or 0 memory, counts as
missing (aks-recommendations R12).

- [ ] Test (recorded made-up answer): D64ps_v5 64 vCPU / 208 GiB arm64;
  DS1_v2 3.5 GiB; E8-4ds_v5 4 usable vCPU and constrained; a `disks` entry
  ignored; an entry without `MemoryGB` missing.

**R3** WHEN a size has a restriction in the answer THEN it SHALL not be a
candidate or a target for a node it restricts:
- `type` `Location` (e.g. `reasonCode` `NotAvailableForSubscription`): for
  every node of that subscription and region;
- `type` `Zone`: for a node in one of the restricted zones.

The node's own size is still read from the answer, restricted or not, so
the node itself can be compared.

- [ ] Test: a cheaper candidate with a Location restriction is never
  offered; a zone-restricted one is offered only to nodes outside the
  restricted zones; a node whose own size is restricted is still rightsized.

**R4** WHEN the system needs a token THEN it SHALL use only a credential
already on the machine, never prompt, and never create anything:
- the Azure CLI's sign-in: `az account get-access-token --resource-type arm
  --subscription <id> --output json` (design D1);
- no browser, device-code or interactive flow; no credential is stored by
  this tool.

- [ ] Test: a fake `az` on `PATH` returning a token → the request carries
  `Authorization: Bearer <token>`; the subscription passed to `az` is the
  node's.
- [ ] Test: the token is reused for its lifetime (at most one `az` run per
  subscription while it is valid).

**R5** WHEN the sizes can't be read THEN the Recommendations notes SHALL show
one line saying why and what to do, from this table, and log it once (the
header issue log, as price failures):

| Cause | Line (after `Azure VM sizes: `) |
|---|---|
| `az` not on `PATH` | `the Azure CLI (az) was not found; install it and run az login` |
| `az` says no sign-in, expired, or interaction needed (`az login`, `AADSTS`) | `not signed in to Azure; run az login` |
| `az` doesn't know the subscription | `the signed-in account can't see the nodes' subscription; az login with an account that can` |
| `az` fails otherwise, or times out (30 s) | `the Azure CLI failed to give a token; run az account get-access-token to see why` |
| `401` (`InvalidAuthenticationToken…`) | `Azure refused the token; run az login` |
| `403` `AuthorizationFailed` | `no permission to read VM sizes (Microsoft.Compute/skus/read on the subscription, e.g. the Reader role)` |
| `404` `SubscriptionNotFound` | `the nodes' subscription was not found or is disabled` |
| `429`, `5xx`, network error | `Azure Resource Manager unavailable (<status>); retrying in <d>` |
| provider ID without a subscription | `the nodes' provider ID names no subscription` |

- No line carries a subscription id, tenant id, account name or token, and
  `az`'s own output is never shown (it can carry them).
- While the line shows, rows that need sizes (rightsize, generation, arm,
  Spot) don't appear; idle, consolidation and old-node rows do.

- [ ] Test: a table test drives each cause (fake `az`, fake Resource
  Manager) and checks the exact line, that it contains no `00000000-` and
  no token, and the row kinds shown.

**R6** WHEN a size read failed for sign-in, permission or subscription (R5 rows 1–7)
THEN the system SHALL try again at most once a minute while Recommendations
is shown, so an admin can run `az login` or get the role and come back
without restarting. `429`, `5xx` and network errors are retried as the
price provider does (fix-azure-retry: the answer's retry header, at least a
minute, at most 10 minutes).

- [ ] Test (fake clock): after a `403`, no request for 59 s, one at 60 s
  while shown, none while another screen is shown; after the fake `az`
  starts returning a token, the rows appear.

**R7** WHEN pricing is disabled (`--disable-pricing`) THEN the system SHALL
make no SKU request: every row that needs a size also needs a price, so none
could show (aks-recommendations R3). The note stays the pricing-disabled
one.

- [ ] Test: with pricing disabled, no `az` run and no request.

**R8** WHEN a rule uses an Azure size THEN the math popup SHALL name the
source, `(Azure Resource SKUs API)`, and the subscription id SHALL appear
nowhere on screen, in the search or in the export (hide-cloud-ids).

- [ ] Test: the rightsize math of the fixture ends its size line with the
  source; no rendered screen of the fixture contains the subscription id.

**R9** WHEN this ships THEN the copied autoscaler table, `make
azure-catalog`, its README credit and the deferred workflow task SHALL be
removed: the live answer is the only Azure size source. The aks-recommendations
fixtures and table-coverage tests SHALL run on a recorded made-up SKU answer
in `pkg/azure/testdata/` (design D5).

- [ ] Test: `go build` has no `InstanceTypes` symbol in `pkg/azure`; the
  aks-recommendations fixture still totals $322.66 and the mixed one
  $565.60 on the recorded answer.

## Counter-examples

- The system SHALL NOT open a browser, ask for a device code, or read or
  write any credential file itself; `az` does its own.
- The system SHALL NOT fall back to an embedded size table when the live
  read fails (the owner's decision): it shows the R5 line.
- The system SHALL NOT make a Resource Manager request when no Azure node is
  shown, or before Recommendations is shown.
- The system SHALL NOT show or log a subscription id, tenant id, account or
  token, nor `az`'s raw output.
- The system SHALL NOT change anything for AWS nodes or AWS-only clusters
  (the AWS catalog stays as it is).

## Out of scope

- AWS: the EC2 catalog stays the embedded one; a live
  `DescribeInstanceTypes` is a possible later spec.
- Credentials other than the Azure CLI: managed identity, workload identity,
  service-principal environment variables, `azd` (design D1 alternatives;
  deferred with a trigger).
- Sovereign clouds (Azure China, US Government): the Resource Manager host
  is `management.azure.com` only.
- Using the live answer for prices (prices stay on the Retail Prices API).
- Checking the temp-disk table (aks-recommendations D5) against
  `MaxResourceVolumeMB`; the fixed tables stay.
