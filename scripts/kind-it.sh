#!/usr/bin/env bash
# Licensed under the Apache License, Version 2.0 (the "License"); see LICENSE.
#
# Integration test on a throwaway kind cluster: creates it from
# cmd/kube-node-viewer/testdata/kind/, applies the made-up workload, runs
# TestKindCluster (go test -tags integration) against it and deletes it.
# Needs Docker, kind and kubectl. The kubeconfig is a temporary file, so the
# contexts of ~/.kube/config are never read or changed.
#
#   scripts/kind-it.sh           # KNV_IT_KEEP=1 keeps the cluster
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
name=${KNV_IT_CLUSTER:-knv-it}
fixtures="$root/cmd/kube-node-viewer/testdata/kind"
work=$(mktemp -d)
export KUBECONFIG="$work/kubeconfig"

cleanup() {
  if [[ -z "${KNV_IT_KEEP:-}" ]]; then
    kind delete cluster --name "$name" --kubeconfig "$KUBECONFIG" >/dev/null 2>&1 || true
    rm -rf "$work"
  else
    echo "kept cluster $name; kubeconfig $KUBECONFIG"
  fi
}
trap cleanup EXIT

kind create cluster --name "$name" --config "$fixtures/cluster.yaml" --kubeconfig "$KUBECONFIG" --wait 120s
kubectl apply -f "$fixtures/workload.yaml"
kubectl -n knv-it rollout status deployment/it-app --timeout=180s

KNV_IT_KUBECONFIG="$KUBECONFIG" KNV_IT_CONTEXT="kind-$name" \
  go test -tags integration -count=1 -run TestKindCluster -v "$root/cmd/kube-node-viewer"
