# TODO

Backlog for kube-node-viewer: what the owner has to do, follow-ups found
while working, ideas, and deferred work. Rules in
[`AGENTS.md`](AGENTS.md#backlog-todomd): add what you find out of scope,
remove an item in the commit that does it, and a spec comes first when the
change needs one. Deferred tables in `specs/*/tasks.md` stay the record for
their feature; the rows below link them.

## Owner actions

- [ ] Decide whether to purge the export files committed by mistake in
      `08735f6` (node names and private IPs of a test cluster) from history.
      They are removed from `main`, but stay in older commits and in the
      v0.9.0 source archives. Purging needs a history rewrite, a force-push
      and re-tagging; a normal removal commit is already in.
- [ ] Optional: an IAM role with `pricing:GetProducts` trusted by GitHub
      OIDC, its ARN in the repo variable `AWS_CATALOG_ROLE_ARN`, so the
      monthly `refresh-data.yaml` also refreshes the instance catalog (today
      it refreshes only the static price tables).

## Follow-ups

- [ ] AWS pricing region when the API server is not an EKS URL (private
      endpoint, proxy): prices use the SDK region while the header shows the
      node label `topology.kubernetes.io/region`. Use the node label for
      pricing too.
- [ ] Spec status: `sort-invert` moves to **shipped** in the release that
      carries it (skill `release`).

## Ideas

- [ ] Recommendations for AKS: Azure VM catalog and family tables, so the
      Recommendations screen works on Azure nodes
      ([aks-pricing deferred](specs/aks-pricing/tasks.md#deferred),
      [recommendations deferred](specs/recommendations/tasks.md#deferred)).
- [ ] GKE: node pool labels and prices (stage 2, multi-cloud;
      [interactive-tui deferred](specs/interactive-tui/tasks.md#deferred)).
- [ ] GPU resource and columns (stage 3;
      [interactive-tui deferred](specs/interactive-tui/tasks.md#deferred)).

## Deferred (with promotion trigger)

| Item | Trigger | Source |
|------|---------|--------|
| Typed filter syntax in `/` (`cap:spot pool:gpu`) | panels feel slow for frequent filters | [interactive-tui](specs/interactive-tui/tasks.md#deferred), [views-and-export](specs/views-and-export/tasks.md#deferred) |
| OR across filter fields | owner asks | [interactive-tui](specs/interactive-tui/tasks.md#deferred) |
| Choosing the export directory | users run it from read-only directories | [views-and-export](specs/views-and-export/tasks.md#deferred) |
| Pod-level rightsizing (requests too high) | owner asks | [recommendations](specs/recommendations/tasks.md#deferred) |
| Commitments: Savings Plans, RIs, Azure reservations | someone needs effective rather than list cost | [recommendations](specs/recommendations/tasks.md#deferred), [aks-pricing](specs/aks-pricing/tasks.md#deferred) |
| A second Recommendations total including Spot | owner asks | [recommendations](specs/recommendations/tasks.md#deferred) |
| Drop the `eks-node-viewer/*` label prefix fallback | a major release, and no known user carries the old names | [rebrand](specs/rebrand/tasks.md#deferred) |
