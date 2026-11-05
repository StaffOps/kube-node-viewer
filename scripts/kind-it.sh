#!/usr/bin/env bash
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# Integration test on a throwaway kind cluster: creates it from
# cmd/kube-node-viewer/testdata/kind/cluster.yaml, runs TestKindCluster
# (go test -tags integration), which applies the made-up workload itself,
# and deletes the cluster. Needs Docker and kind (no kubectl). The kubeconfig
# is a temporary file, so the contexts of ~/.kube/config are never read or
# changed. Refuses to run when a kind cluster of that name already exists.
#
#   scripts/kind-it.sh           # KNV_IT_KEEP=1 keeps the cluster
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
name=${KNV_IT_CLUSTER:-knv-it}
fixtures="$root/cmd/kube-node-viewer/testdata/kind"
work=$(mktemp -d)
kubeconfig="$work/kubeconfig"
created=

cleanup() {
  if [[ -n "$created" && -n "${KNV_IT_KEEP:-}" ]]; then
    echo "kept cluster $name; kubeconfig $kubeconfig"
    return
  fi
  if [[ -n "$created" ]]; then
    kind delete cluster --name "$name" --kubeconfig "$kubeconfig" >/dev/null 2>&1 || true
  fi
  rm -rf "$work"
}
trap cleanup EXIT

if kind get clusters 2>/dev/null | grep -qx "$name"; then
  echo "kind-it: a kind cluster named $name already exists; delete it or set KNV_IT_CLUSTER" >&2
  exit 1
fi

create() {
  kind create cluster --name "$name" --config "$fixtures/cluster.yaml" --kubeconfig "$kubeconfig" --wait 120s
}
# the name was free, so from here on the cluster is ours to delete, also
# after a failed create
created=1
if ! create; then
  echo "kind-it: create failed, retrying once" >&2
  kind delete cluster --name "$name" --kubeconfig "$kubeconfig" >/dev/null 2>&1 || true
  create
fi

KNV_IT_KUBECONFIG="$kubeconfig" KNV_IT_CONTEXT="kind-$name" \
  go test -tags integration -count=1 -run TestKindCluster -v "$root/cmd/kube-node-viewer"
