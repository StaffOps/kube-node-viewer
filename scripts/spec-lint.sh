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
# Check the mechanical rules of specs/README.md. Exit 1 on any error.
set -uo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.." || exit 1

errors=0
# Shipped before the review gate without a spec-refuter review (owner or
# code-review agent instead). Listed, not hidden; a spec leaves the list when
# a spec-refuter review is recorded on it. Never add a new spec here.
UNREVIEWED=""

err() { echo "spec-lint: $1" >&2; errors=$((errors + 1)); }

index=specs/README.md
[ -f "$index" ] || { err "$index missing"; exit 1; }

for dir in specs/*/; do
  name=$(basename "$dir")
  case "$name" in decisions) continue ;; esac

  case "$name" in
    fix-*) required="bugfix.md tasks.md"; reqfile="$dir/bugfix.md" ;;
    *)     required="requirements.md design.md tasks.md"; reqfile="$dir/requirements.md" ;;
  esac
  missing=false
  for f in $required; do
    [ -f "$dir/$f" ] || { err "$name: missing $f"; missing=true; }
  done
  $missing && continue

  grep -q "($name/)" "$index" || err "$name: not linked from $index"

  reqs=$(grep -oE '^\*\*R[0-9]+[a-z]?\*\*' "$reqfile" | tr -d '*' | sort -u)
  [ -n "$reqs" ] || err "$name: no '**R<n>** WHEN … SHALL' requirement in $(basename "$reqfile")"
  # capture first: a failing awk must fail the lint, not pass it silently
  if reqtext=$(awk '/^\*\*R[0-9]+[a-z]?\*\*/{p=$0; f=1; next} f&&NF&&!/^ *-/{p=p " " $0; next} f{print p; f=0} END{if(f) print p}' "$reqfile"); then
    while IFS= read -r line; do
      [ -n "$line" ] || continue
      printf '%s\n' "$line" | grep -q 'WHEN.*THEN.*SHALL' ||
        err "$name: requirement not in WHEN/THEN/SHALL form: ${line:0:80}"
    done <<<"$reqtext"
  else
    err "$name: requirement check failed (awk)"
  fi

  for r in $reqs; do
    grep -qE "(^|[^0-9A-Za-z])$r([^0-9a-z]|$)" "$dir/tasks.md" || err "$name: $r not covered by any task in tasks.md"
  done

  grep -qE '^- \[( |-|x)\] T[0-9]+[a-z]?\. ' "$dir/tasks.md" || err "$name: tasks.md has no '- [ ] T<n>. <task>' items"
  bad=$(grep -E '^- \[.\]' "$dir/tasks.md" | grep -vE '^- \[( |-|x)\] T[0-9]+[a-z]?\. ' || true)
  [ -z "$bad" ] || err "$name: task item not in '- [ |-|x] T<n>. …' form: $(printf '%s' "$bad" | head -1 | cut -c1-60)"

  if grep -qE '^> Status: (active|shipped)' "$reqfile" && grep -q '\[TO CONFIRM\]' "$reqfile"; then
    err "$name: status active/shipped but [TO CONFIRM] remains"
  fi

  status=$(sed -n 's/^> Status: \([a-z]*\).*/\1/p' "$reqfile" | head -1)
  case "$status" in
    draft | active | shipped | superseded) ;;
    *) err "$name: status '${status:-none}' on '> Status:' is not draft, active, shipped or superseded" ;;
  esac

  # the index entry says the same status as the spec (fix-spec-tooling R3)
  listed=$(grep -E "\]\($name/\) — \*\*[a-z]+\*\*" "$index" | sed -E 's/.* — \*\*([a-z]+)\*\*.*/\1/' | head -1)
  [ -z "$listed" ] || [ "$listed" = "$status" ] || err "$name: $index lists it as $listed, its status line says ${status:-none}"

  # an active or shipped spec names its independent review on the status
  # line: 'Reviewed: <date> (spec-refuter…' or 'Reviewed: spec-refuter
  # <date>' (AGENTS.md step 2)
  if [ "$status" = active ] || [ "$status" = shipped ]; then
    if grep -m1 -E '^> Status:' "$reqfile" |
      grep -qE 'Reviewed: ([0-9]{4}-[0-9]{2}-[0-9]{2} \(spec-refuter|spec-refuter [0-9]{4}-[0-9]{2}-[0-9]{2})'; then
      case " $UNREVIEWED " in *" $name "*) err "$name: reviewed by spec-refuter now; remove it from UNREVIEWED in scripts/spec-lint.sh" ;; esac
    else
      case " $UNREVIEWED " in
        *" $name "*) ;;
        *) err "$name: status active/shipped needs 'Reviewed: <date> (spec-refuter…)' or 'Reviewed: spec-refuter <date>' on its status line; run the spec-refuter review first (draft until then)" ;;
      esac
    fi
  fi

  if [ -f "$dir/design.md" ]; then
    # each decision ('## D1. …', '### D1: …', not '## Decisions';
    # fix-spec-tooling R2) carries both markers in its own section, which
    # ends at the next decision or the next heading of its level or higher;
    # fenced code blocks are skipped (fix-spec-lint-markers R1). No {n,m}
    # in the regexes, for older mawk.
    if markers=$(awk '
      function flush() {
        if (cur == "") return
        if (!w) printf "%s (design.md:%d) has no '\''When this would be wrong'\''\n", cur, at
        if (!r) printf "%s (design.md:%d) has no '\''Reversal action'\''\n", cur, at
        cur = ""
      }
      {
        # up to three leading spaces do not change a fence or a heading
        t = $0; n = 0
        while (n < 3 && substr(t, 1, 1) == " ") { t = substr(t, 2); n++ }
        c = substr(t, 1, 1)
        if (c == "`" || c == "~") {
          k = 0
          while (substr(t, k + 1, 1) == c) k++
          if (k >= 3) {
            if (fence == "") { fence = c; flen = k; next }
            if (c == fence && k >= flen && substr(t, k + 1) ~ /^[ \t]*$/) { fence = ""; next }
          }
        }
        if (fence != "") next
        if (t ~ /^#+( |$)/) {
          level = 0
          while (substr(t, level + 1, 1) == "#") level++
          if (t ~ /^###? D[0-9]+([.: ]|$)/) {
            flush()
            split(t, f, /[ .:]+/); cur = f[2]; at = NR; lvl = level; w = 0; r = 0
            next
          }
          if (cur != "" && level <= lvl) flush()
        }
        if (cur != "") {
          if (index($0, "**When this would be wrong**")) w = 1
          if (index($0, "**Reversal action**")) r = 1
        }
      }
      END { flush() }' "$dir/design.md"); then
      while IFS= read -r missing; do
        [ -n "$missing" ] && err "$name: $missing"
      done <<<"$markers"
    else
      err "$name: design.md marker check failed (awk)"
    fi
  fi
done

for adr in specs/decisions/*.md; do
  [ -f "$adr" ] || continue
  base=$(basename "$adr")
  [[ "$base" =~ ^ADR-[0-9]{3}-[a-z0-9-]+\.md$ ]] || err "decisions/$base: name must be ADR-NNN-<kebab-title>.md"
  grep -q "decisions/$base" "$index" || err "decisions/$base: not in the ADR index of $index"
done

if [ "$errors" -gt 0 ]; then
  echo "spec-lint: $errors error(s)" >&2
  exit 1
fi
echo "spec-lint: ok"
