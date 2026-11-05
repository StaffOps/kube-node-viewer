#!/usr/bin/env bash
# Local gate before every commit: gofmt, go vet, go test -race, spec lint.
# `hack/` is excluded: several upstream main packages share that directory.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

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
echo "==> spec tasks"
python3 scripts/spec-tasks-check.py
echo "==> spec trace"
python3 scripts/spec-trace.py
echo "==> drift"
python3 scripts/drift-check.py


echo "verify: ok"
