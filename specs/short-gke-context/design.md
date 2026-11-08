# Design: short-gke-context

> Implements [requirements](requirements.md).

## Current state

`shortContext` (`pkg/model/header.go:324-329`) returns the cluster name of an
EKS ARN and any other name as it is; it feeds the three headers through the
`context :=` line (`header.go:160`). `exportBase` (`pkg/model/export.go:243-248`)
builds `kube-node-viewer-<context>-<time>`, replacing characters outside
`[A-Za-z0-9._-]` with `_`, or using `cluster` for an empty context.
`writeExport` calls it with `doc.Context` (`export.go:254,258`), `createPair`
creates both files with `O_EXCL` and adds `-2`, `-3`… to a name in use
(`export.go:286,294`). The CSV has no context column; the JSON has a `context`
field (`export.go:144`). The notice prints the CSV path (`export.go:385`).

## Target state

`shortContext` also returns the cluster of a GKE context (D2). `exportBase`
reduces `shortContext(context)` instead of `context`, so the file name and the
notice follow; `doc.Context` stays the full name.

## Components touched

| Path | Change |
|------|--------|
| `pkg/model/header.go` | the GKE pattern in `shortContext` |
| `pkg/model/export.go` | `exportBase` uses `shortContext` |
| `scripts/checks/content.sh`, `scripts/checks/test-checks.sh` | the `gke` rule also catches the domain form and `connectgateway_` names, with cases |
| `pkg/model/header_internal_test.go` | GKE rows in `TestShortContextName` (the row at line 151, a GKE name staying unchanged, flips to a short-gke-context R1 row), `TestHeadersShowTheShortGKEName`, `TestShortGKENameOnlyInTheHeaders`, `TestGKELocationIsNotThePricingRegion` |
| `pkg/model/export_internal_test.go` | `TestExportBaseName` as a table (its ARN expectation at lines 55-56 flips), `TestExportFileNameIsShort`, `TestSameShortNameDoesNotOverwrite` |
| `pkg/client/client_test.go` | GKE case in `TestClusterKeyKeepsTheFullContext` |
| `README.md` | both header paragraphs and the export section |
| `CHANGELOG.md` | `[Unreleased]` `Changed` |
| `specs/short-arn-context/requirements.md` | dated notes: R2's GKE example, R3 and Out of scope (export file name, notice) |
| `specs/views-and-export/requirements.md` | dated note on the file name rule |
| `specs/interactive-tui/requirements.md` | dated note under R16 |
| `TODO.md` | remove both items; add `connectgateway_` and multicloud context names, and project ids elsewhere on screen |

## Decisions

### D1: The export file name uses the short name; the JSON keeps the full one

**Choice**: `exportBase` shortens EKS ARN and GKE names. The JSON `context`
field and every other place stay as they are.

**Justification, strongest first**:
1. The file name is what gets attached and pasted, and the footer shows it on
   the screen; it is the one place short-arn-context left carrying the account
   id on screen.
2. The notice must name the real file (short-arn-context R3), so shortening
   the file name is the only way to shorten the notice honestly.
3. The JSON still says exactly which cluster it came from.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| Same-named clusters in different accounts, projects or regions give file names that differ only by time | the JSON `context` tells them apart, and `createPair`'s `O_EXCL` never overwrites |
| The CSV has no context column, so a CSV alone only names the short cluster | the JSON written beside it has the full name; adding a column would change the CSV format |
| Scripts that glob the old long file name stop matching | README and views-and-export documented `<context>` in the name, so this is a change of a documented name: CHANGELOG `Changed` says so, and views-and-export gets a dated note |

**When this would be wrong**: users sort exports of several accounts in one
directory by file name and need the account in it, or tools parse the context
out of the file name.

**Reversal action**: call `exportBase` with `context` unshortened (remove the
`shortContext` call in `export.go`) and restore the test table; minutes.

**Alternatives discarded**: masking only the account or project in the name
(keeps it long, and a partial id still identifies); a hash of the full context
in the name (unreadable, and no use when the JSON already has it); shortening
only the footer notice (it would name a file that does not exist).

### D2: A strict GKE pattern, anything else unchanged

**Choice**: shorten only a name matching, in full,
`^gke_((?:[a-z0-9-]+(?:\.[a-z0-9-]+)+:)?[a-z][a-z0-9-]{4,28}[a-z0-9])_[a-z]+-[a-z]+[0-9]+(?:-[a-z0-9]+)?_([a-z](?:[a-z0-9-]{0,38}[a-z0-9])?)$`.

**Justification, strongest first**:
1. A context name is free text; `gke_` plus underscores is an easy shape to
   hit by accident, so a loose rule would rename contexts the user chose.
2. The fields follow Google's documentation: project ids are 6 to 30 lower-case
   letters, digits and hyphens, start with a letter and cannot end with a hyphen
   (https://cloud.google.com/resource-manager/docs/creating-managing-projects);
   cluster names are up to 40 lower-case letters, digits and hyphens, start with
   a letter and end with a letter or a digit
   (https://cloud.google.com/kubernetes-engine/docs/reference/rest/v1/projects.locations.clusters).
3. gcloud writes that form: `gke_{project}_{zone}_{cluster}`
   (`googlecloudsdk/api_lib/container/util.py:505` in the installed SDK).
4. None of the three fields can hold `_`, so the split is unambiguous.
5. Anything outside the pattern renders as today: the worst case of too strict
   a pattern is the current behaviour.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| Verified only against made-up names: no GKE cluster is available | `gcloud`'s format and Google's rules are documented; a miss shows the full name, as today |
| A location shape the pattern rejects keeps the full name | all 43 regions and their zones listed on https://cloud.google.com/compute/docs/regions-zones on 2026-11-08 match, the AI zones (`us-west4-ai2b`) included; S3NS's `u-france-east1` does not and stays long, accepted; widening is a test case away |
| A legacy domain-scoped project is accepted by a simple domain shape, unverified against Google's documentation or a real context (only a 2016 thread describes it), and its length rule is assumed the same | it is the case that leaks most (the organization's domain), and a false match needs a `gke_` name with a dotted prefix and a colon |

**When this would be wrong**: real `gcloud` contexts have a shape the pattern
rejects, or a hand-named context matches it and the owner wanted the long name.

**Reversal action**: narrow or widen the regular expression and add the case
to `TestShortContextName`; minutes.

**Alternatives discarded**: splitting on `_` without a pattern (accepts any
name with three underscores); dropping only the project and keeping the location in the name
(longer, and the region has its own row when known).

## Invariants

- Requests, never usage; no new network call.
- `u.info.Context`, the kubeconfig names, the state key and `doc.Context` are
  unchanged; only the three headers and the export file name differ.
- For a context that is neither an EKS ARN nor a GKE name, the headers and the
  file name render byte-identically (the golden screens stay unchanged).
- Existing export files are never overwritten.

## Failure modes

| Failure | Behavior | How the user notices |
|---------|----------|----------------------|
| Malformed GKE name | Shown as it is | The full name in the header, as today |
| Same cluster name in two projects | Same header name and file name base | The JSON `context`; a file exported in the same second gets `-2` |

## Test plan

- `TestShortContextName`, table: R1's region, zone and domain-scoped names,
  project lengths 6 and 30, cluster lengths 1 and 40; R2's project of 5 and 31
  characters, starting with a digit, ending with `-`; missing and extra fields;
  upper case; cluster of 41 characters, starting with a digit, ending with `-`;
  a location without digits, `u-france-east1`; leading and trailing text;
  trailing newline; a Cyrillic letter; malformed domains; `connectgateway_…`
  and `gke_{kind}_…`. Rows whose literal `content.sh` would flag (not
  `gke_example…`) are built by concatenation (`"gke_" + p + "_us-central1-a_demo"`).
- `TestHeadersShowTheShortGKEName`: the three headers show `demo-prod`, and
  none contains `example-project`; a domain-scoped case with a server host
  other than the project's domain, so `example.com` in the output can only
  come from the context.
- `TestShortGKENameOnlyInTheHeaders`: popup rows, "connecting to" row and the
  export `context` keep the full name.
- `TestGKELocationIsNotThePricingRegion`: no region from the server, no nodes →
  `Region:` is `-`, and the compact box has no `us-central1`.
- `TestExportBaseName`: table for an ARN, a GKE name, a plain name, an unsafe
  name and an empty one.
- `TestExportFileNameIsShort`: the files written and the notice hold neither
  the account nor the project; the JSON `context` is the full name.
- `TestSameShortNameDoesNotOverwrite`: two different full contexts with the
  same short name, exported at the same time, give `-2`.
- `TestClusterKeyKeepsTheFullContext`: a GKE case.
- The golden screens stay byte-identical.
- Run on a local kind cluster with its context renamed to a made-up GKE name,
  and to a made-up ARN with account 111122223333: three headers, `c`, `e`.
