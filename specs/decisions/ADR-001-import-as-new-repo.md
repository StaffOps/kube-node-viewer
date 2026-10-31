# ADR-001: Import upstream into a new repository instead of a GitHub fork

> Status: Accepted · Date: 2026-10-30 · Recorded after the fact (no
> independent review at decision time).

## Context

- `awslabs/eks-node-viewer` is Apache 2.0 (`LICENSE`, `NOTICE`:
  `Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.`).
- The project is low-traffic but alive: last non-bot commit 2026-10-05
  (`971c440`), previous 2026-05-05 (`0374223`). It is not abandoned.
- This project wants its own name, roadmap and release cadence, and will take
  changes from other forks (`sule26/eks-node-viewer`).

## Decision

Create `StaffOps/kube-node-viewer` as a new repository whose first content
commit (`af96088`, on top of the GitHub-created `6b6779a`) is a verbatim copy of upstream `fe35ed41e18b` — not a GitHub fork,
not a rewrite.

## Rationale

1. Apache 2.0 obligations attach to the code, not to the repository: a
   verbatim import with `LICENSE`, `NOTICE` and headers preserved meets
   section 4 exactly as a fork would.
2. A GitHub fork ties the project's identity, default PR target and search
   visibility to upstream; this project diverges by design.
3. The import commit message records the upstream SHA, so provenance stays
   checkable without carrying upstream's full history.

## Consequences

- Obligations (permanent): keep `LICENSE`, `NOTICE` and Amazon copyright
  headers; mark modified files; no AWS/Amazon/EKS branding implying
  endorsement (Apache 2.0 §6 grants no trademark rights).
- Upstream changes are pulled by cherry-pick (`git cherry-pick -x`), never by
  merge — histories are unrelated.
- Third-party fork changes keep their original author (cherry-pick preserves
  it) and are credited in `README.md`.

## When this would be wrong

- If upstream becomes very active and this project only adds small patches,
  carrying a fork with rebases would cost less than repeated cherry-picks.

## Alternatives discarded

- **GitHub fork** — keeps the upstream name and PR target; see rationale 2.
- **Clean-room rewrite** — high cost, and reading the code while rewriting
  would still produce a derivative work.
