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
# Local gate before every commit: gofmt, go vet, go test -race, spec lint.
# `hack/` is excluded: several upstream main packages share that directory.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
# no __pycache__ next to the scripts: bytecode records local paths
export PYTHONDONTWRITEBYTECODE=1

PKGS="./cmd/... ./pkg/..."

echo "==> gofmt"
unformatted=$(gofmt -s -l cmd pkg)
if [ -n "$unformatted" ]; then
  echo "gofmt -s needed on:" >&2
  echo "$unformatted" >&2
  exit 1
fi

echo "==> go vet"
# shellcheck disable=SC2086
go vet $PKGS

echo "==> go test -race"
# shellcheck disable=SC2086
go test -race $PKGS

echo "==> spec lint"
scripts/spec-lint.sh
echo "==> real-environment identifiers"
sh scripts/checks/content.sh
echo "==> changelog"
python3 scripts/changelog-check.py
echo "==> CI tool pins (one version per tool)"
python3 scripts/checks/tool-versions.py --pins-only
echo "==> spec tasks"
python3 scripts/spec-tasks-check.py
echo "==> spec trace"
python3 scripts/spec-trace.py
echo "==> drift"
python3 scripts/drift-check.py
echo "==> license headers"
python3 scripts/checks/headers.py

echo "verify: ok"
