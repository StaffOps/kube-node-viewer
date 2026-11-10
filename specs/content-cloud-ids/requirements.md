# Requirements: content-cloud-ids

> Status: active · Owner: Carlos Gomes · Reviewed: 2026-11-10 (spec-refuter; 1 HIGH, 4 MEDIUM, 6 LOW, applied)

## Context

[hide-cloud-ids](../hide-cloud-ids/requirements.md) stopped the screen from
showing the project of a GCE provider ID and the resource group of an Azure
one, because they identify the owner's account. The content check
(`scripts/checks/content.sh`), which keeps real-environment data out of this
public repository, does not flag either:

- `gce://<project>/<zone>/<instance>`: no rule names `gce://`.
- `resourceGroups/<name>` of an Azure resource id: only an AKS node resource
  group (`MC_<group>_<cluster>_<region>`) and the subscription id are rules.

So a provider ID pasted from `kubectl get node -o yaml` into a test, a spec or
a commit message passes the pre-commit hook and CI.

This spec never writes a blocked value literally: a real-looking one would be
blocked by the check it describes. The cases in `scripts/checks/test-checks.sh`
(a file the check skips) hold the concrete values.

## Usage examples

```text
$ git commit -m "test: a node from the cluster"   # a file adds gce://<real project>/us-central1-a/gke-x
real-environment data (rules in scripts/checks/content.sh); replace it with example values:
  pkg/model/x_test.go:12  gce  …

$ git commit -m "test: an AKS node"   # a file adds …/resourceGroups/<real name>/providers/…
real-environment data (rules in scripts/checks/content.sh); replace it with example values:
  pkg/model/x_test.go:12  resourcegroup  …
```

`gce://example-project/us-central1-a/gke-x` and
`…/resourceGroups/example-rg/…` pass.

## Requirements

**R1** WHEN a checked line holds `gce://` followed by a GCP project id (6 to 30
lower-case letters, digits or `-`, starting with a letter and not ending with
`-`, optionally after a domain and a colon) and `/`, in any case, THEN the
content check SHALL report it under the rule `gce`, unless the project starts
with `example` and any domain is exactly `example.com`.

- [x] A real project, the same in upper case, and a real project after a real
  domain are blocked.
- [x] A real project after `example.com:`, and any project after a domain that
  only starts with `example.` (`example.<real domain>:`), are blocked.
- [x] `gce://example-project/…` and `gce://example.com:example-project/…` pass.
- [x] A real project after an example one on the same line is blocked.
- [x] Two example values on one line pass.

_Test: `# content-cloud-ids R1` on the cases in `scripts/checks/test-checks.sh`._

**R2** WHEN a checked line holds `resourceGroups/` (in any case) followed by a
name that starts with a letter, a digit or `_` and goes on with Azure resource
group characters (letters, digits, `.`, `_`, `-`, `(`, `)`) THEN the content
check SHALL report it under the rule `resourcegroup`, unless the name starts
with `example` or with `mc_example`.

- [x] A real name, and the same in upper case, are blocked.
- [x] A real `MC_<group>_<cluster>_<region>` name is blocked (also by `aks`).
- [x] `resourceGroups/example-rg` and `resourceGroups/MC_example-rg_example-aks_eastus`
  pass.
- [x] A real name after an example one on the same line is blocked.
- [x] A one- or two-letter name is blocked like any other: the repository's
  fixtures use example names instead.
- [x] A regular expression such as `resourceGroups/([^/]+)/` passes.

_Test: `# content-cloud-ids R2` on the cases in `scripts/checks/test-checks.sh`._

**R3** WHEN the content check runs in any of its modes (every tracked file,
`--staged`, `--message`, `--commits`) THEN R1 and R2 SHALL apply, and a report
SHALL show only the place and the rule under GitHub Actions, as for every
other rule.

- [x] A commit message holding a real `gce://` project is blocked.
- [x] The full-tree mode finds a committed real resource group.

_Test: `# content-cloud-ids R3` on the cases in `scripts/checks/test-checks.sh`._

**R4** WHEN the rules are added THEN every tracked file of the repository SHALL
still pass the full-tree check.

- [x] `scripts/verify.sh` is green.

_Test: ci — the `verify` job runs `content.sh` on the whole tree._

## Counter-examples

- The check SHALL NOT allow short names wholesale (`rg`, `g`): an allow list
  that broad would also let a real short name through; fixtures move to
  example values instead.
- The check SHALL NOT flag `gce://` text that is not a project id (`gce://…/`,
  `gce://<project>/`, a regular expression such as `gce://[^/`), nor
  `resourceGroups/<group>` or a regular expression after `resourceGroups/`.

## Out of scope

- Other spellings of a resource group: `--resource-group <name>`,
  `"resourceGroup": "<name>"`, `resource_group = …`; URL-encoded (`%2F`) and
  JSON-escaped (`\/`) resource ids; project numbers (bare digits) and project
  ids outside a `gce://` id. They stay a `TODO.md` item and a known gap in the
  script header.
- Tightening `gke_ok`, which accepts any project after `example.com:`
  (`TODO.md`).
- The deny list, which still catches names the owner lists there.
