#!/bin/sh
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
# Provenance over the whole tree (AGENTS.md "Provenance and licensing",
# rebrand R8), not only the new files pre-commit sees:
# - every tracked .go/.sh/.py file and git hook has the Apache header;
# - every .go/.sh/.py file that came from upstream and changed since the
#   import carries the modification note (rebrand design D5);
# - LICENSE is the imported one and NOTICE keeps its first line.
# The static price tables (pkg/aws/zz_generated_*.pricing.go) are exempt from
# the note: `make prices` copies them verbatim from Karpenter, header
# included, and they are never edited by hand.
#   scripts/checks/headers.sh    (verify.sh runs it; needs the import commit,
#                                 so a clone with full history)
# KNV_IMPORT_COMMIT overrides the import commit (ADR-001, ADR-005); the
# guardrail tests use it.
cd "$(git rev-parse --show-toplevel 2>/dev/null || echo .)" || exit 2
import=${KNV_IMPORT_COMMIT:-f3aceaa}
note='Modified by the kube-node-viewer authors'
problems=""
add() { problems="$problems
$1
  $2"; }

if ! git cat-file -e "$import^{commit}" 2>/dev/null; then
  echo "headers: the import commit $import is not in this clone; fetch the full history (git fetch --unshallow)" >&2
  exit 2
fi

for f in $(git ls-files '*.go' '*.sh' '*.py' '.githooks/*'); do
  [ -f "$f" ] || continue
  grep -q 'Licensed under the Apache License' "$f" || add "$f" "no Apache license header"
done

for f in $(git ls-files '*.go' '*.sh' '*.py'); do
  [ -f "$f" ] || continue
  case $f in pkg/aws/zz_generated_*.pricing.go) continue ;; esac
  git cat-file -e "$import:$f" 2>/dev/null || continue
  git diff --quiet "$import" -- "$f" && continue
  grep -q "$note" "$f" || add "$f" "upstream file changed since the import without the modification note"
done

git diff --quiet "$import" -- LICENSE || add LICENSE "LICENSE differs from the imported one"
first=$(git show "$import:NOTICE" 2>/dev/null | head -n 1)
[ "$(head -n 1 NOTICE 2>/dev/null)" = "$first" ] || add NOTICE "NOTICE first line changed"

if [ -n "$problems" ]; then
  printf 'headers: missing provenance%s\n' "$problems" >&2
  echo "fix: copy the header of scripts/checks/headers.sh (make boilerplate for Go); add '// $note, 2026.' (or '# …' in a script) after the license block" >&2
  exit 1
fi
echo "headers: ok"
