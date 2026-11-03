# Security policy

## Supported versions

Only the latest release gets fixes. Releases go out on Mondays when there
are changes, so a fix ships in the next one; a security fix is released
right away.

| Version | Supported |
|---------|-----------|
| latest release | yes |
| older releases | no — upgrade (`brew upgrade kube-node-viewer`) |

## Reporting a vulnerability

Please **do not open a public issue**. Report it privately through GitHub:
**Security → Report a vulnerability** on
[this repository](https://github.com/StaffOps/kube-node-viewer/security/advisories/new).

Include what you found, how to reproduce it, and the version
(`kube-node-viewer -version`). Do not include data from a real cluster
(names, endpoints, IPs, account ids); made-up values are enough.

You can expect an answer within a week. Once it is confirmed, the fix is
released and a GitHub security advisory is published, crediting you unless
you prefer otherwise.

## Scope

`kube-node-viewer` reads your cluster through your kubeconfig, with your
permissions, and only reads: it never writes to the cluster. It calls the
AWS Price List and the EC2 Spot price history (with your AWS credentials)
and the public Azure Retail Prices API, and writes only its state file, its
log and the exports you ask for (`e`). Reports about any of these, the
release binaries, the Homebrew cask or this repository's workflows are in
scope.

Vulnerabilities in a dependency are in scope when they reach this tool;
otherwise please report them upstream. Known advisories are already
checked on every PR, weekly and before each release (`make vuln`).
