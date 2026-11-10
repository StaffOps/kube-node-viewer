# Requirements: content-cloud-ids

> Status: active · Owner: Carlos Gomes · Reviewed: 2026-11-10 (spec-refuter; 1 HIGH, 4 MEDIUM, 6 LOW, applied; gaps R5-R10: 2 HIGH, 6 MEDIUM, 7 LOW, applied)

> **Corrected 2026-11-10 (gaps).** R5 to R10 close the gaps this spec left
> out of scope: the example allowance of `gke_ok`, `gce_ok` and
> `resourcegroup_ok` no longer takes any name that only starts with
> `example`; the other spellings of a resource group, escaped resource ids and
> names with a leading `-`, `(` or non-ASCII letter are blocked; GCP project
> ids and numbers in resource paths are a new rule, `gcp`. What stays out is
> listed under "Out of scope" below.

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

**R5** WHEN a GKE context name (`gke_`, `connectgateway_`, `edgecontainer_`),
a `gce://` id, a `projects/` path, a resource group (R2, R6, R7), an AKS host
or an AKS node resource group (`MC_…`) is checked THEN it SHALL pass as an
example only when its project, group or host label is `example` alone or
`example` followed by `-` or a digit (after `mc_` for a node resource group),
and, when a domain precedes a GCP project, the domain is exactly
`example.com`.

- [x] `gke_example<letters>_<zone>_<cluster>` is blocked, and the same with
  `connectgateway_`, `edgecontainer_`, `gce://`, `resourceGroups/`,
  `MC_example<letters>_…` and `example<letters>.hcp.…`.
- [x] A resource group named `example_` and a name is blocked.
- [x] `gke_example.com:<real project>_<zone>_<cluster>` is blocked;
  `gke_example.com:example-project_us-central1_demo` passes.
- [x] Every example the repository uses (`example-project`, `example-a`,
  `example1-demo1…`, `example-rg`, `MC_example-rg_…`, `MC_example…`) passes.

_Test: `# content-cloud-ids R5` on the cases in `scripts/checks/test-checks.sh`._

**R6** WHEN a checked line names a resource group in another spelling (the
`--resource-group <name>` or `--node-resource-group <name>` flag, `-g <name>`
in an `az` command, Az PowerShell's `-ResourceGroupName <name>`, a quoted value
of a key matching `resource[_-]?group(_?name)?` (`resourceGroup`,
`resourceGroupName`, `nodeResourceGroup`, `resource_group`,
`resource_group_name`, `AZURE_RESOURCE_GROUP`, an annotation ending
`-resource-group`) after `:`, `=` or `:=`, the unquoted value of such a key
that ends a YAML line (a trailing `#` comment allowed), or `<key>=<name>` with
no space, ended by a space, `;`, `&`, `|`, a quote or the end of the line
(shell, environment, `--set`)) THEN the content
check SHALL report it under the rule `resourcegroup`, unless the name is an
example as in R5.

- [x] Each spelling with a real name is blocked; with `example-rg` it passes.
- [x] A Go field or variable assigned from an identifier
  (`ResourceGroup: group,`, `resourceGroup := parts[4]`), a shell variable
  (`--resource-group "$RG"`) and a placeholder (`<name>`) pass.
- [x] `-g` outside an `az` command (`npm install -g <pkg>`, also after `az`
  past a `;`) passes.
- [x] A Python keyword argument from a variable
  (`resource_group_name=<variable>,`) passes.
- [x] An unquoted YAML value with a non-ASCII letter is blocked.

_Test: `# content-cloud-ids R6` on the cases in `scripts/checks/test-checks.sh`._

**R7** WHEN a resource id writes the `/` after `resourceGroups` as `%2F`
(URL-encoded, any case) or `\/` (JSON-escaped), or the group name starts with
`-`, a non-ASCII letter, or `(` with the name then running to `/`, a quote,
a space, `,`, `;`, `&`, `%`, `\` or the end of the line THEN the content
check SHALL report it under the rule `resourcegroup`, unless the name is an
example as in R5.

- [x] Both escaped forms with a real name are blocked; with `example-rg` they
  pass.
- [x] Names starting `-`, `(`, an accented Latin letter, a Vietnamese letter,
  a CJK character or a 4-byte letter are blocked, and a `(` name holding an
  accented letter or ended by each character listed above.
- [x] An example name followed by a non-ASCII letter is blocked; `©` or `…`
  after `resourceGroups/` passes.
- [x] Regular expressions (`([^/]+)`, `(.*)`, `(?P<g>[^/]+)`, `(\w+)`,
  `(mc_)?example…`, `(<a>|<b>)`) and `resourceGroups/…` pass.

_Test: `# content-cloud-ids R7` on the cases in `scripts/checks/test-checks.sh`._

**R8** WHEN a checked line holds `projects/` followed by a GCP project id (the
R1 shape, optionally after a domain and a colon) or a project number (6 or
more digits), then `/` and one of `zones`, `regions`, `locations` or
`global` (each `/` may be `%2F`- or `\/`-escaped) THEN the content check
SHALL report it under the rule `gcp`, unless
the project is an example as in R5 or the number is `123456789`,
`123456789012` or all zeros.

- [x] A real project id, a real number, a domain-scoped project, the GKE PD
  CSI node id (`projects/<project>/zones/<zone>/instances/<name>`) and a
  compute API URL with a real project are blocked, escaped or not;
  `projects/example<letters>/…` and an example project after a real domain
  are blocked.
- [x] `projects/example-project/locations/global/…`,
  `projects/123456789/zones/…` and all zeros pass.
- [x] A `projects/` path not followed by one of those collections (a local
  directory, a GitHub project board), or by a longer word (`zonesfoo`),
  passes.

_Test: `# content-cloud-ids R8` on the cases in `scripts/checks/test-checks.sh`._

**R9** WHEN the content check runs in any of its modes THEN R5 to R8 SHALL
apply as R3 says.

- [x] A commit message with a real value of R6 or R8 is blocked.
- [x] The full-tree mode blocks each R6 and R7 form (flag, `az … -g` after a
  tab, quoted key, YAML key, `KEY=`, `%2F`, `\/`, leading `-`, non-ASCII
  start) and an R8 path, and passes their examples.

_Test: `# content-cloud-ids R9` on the cases in `scripts/checks/test-checks.sh`._

**R10** WHEN R5 to R9 are in place THEN every tracked file SHALL still pass
the full-tree check, with no new `content-check: allow` line.

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

> **Corrected 2026-11-10 (gaps).** R5 to R8 now cover the spellings,
> escaped ids, leading characters, project ids in resource paths and the
> `gke_ok` allowance above. Still out of scope, in the script's known gaps
> and `TODO.md`: resource groups in `az` table output, as `az group …
> --name`, in a Terraform or Bicep block (`name = …`, `resourceGroup('…')`),
> across YAML lines (an ASO `owner:`), unquoted after `= ` or before more
> text, or starting with a letter whose UTF-8 lead byte is `\342`; GCP
> resource groups as `New-AzResourceGroup -Name`, or double-encoded
> (`%252F`, `\\/`); GCP projects in a `--project` flag, a `project`/`project_id` key, image
> registries, `<project>.svc.id.goog`, service account e-mails in commit
> messages, or other collections (`serviceAccounts`, `secrets`, `topics`,
> `aggregated`); project numbers outside `projects/<number>/<collection>`
> (bare digits, `gke_<number>_…`); Azure subscription ids outside
> `subscriptions/<guid>` (`"subscriptionId": …`).
