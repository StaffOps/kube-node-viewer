#!/usr/bin/env bash
# Check the mechanical rules of specs/README.md. Exit 1 on any error.
set -uo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.." || exit 1

errors=0
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

  reqs=$(grep -oE '^\*\*R[0-9]+\*\*' "$reqfile" | tr -d '*' | sort -u)
  [ -n "$reqs" ] || err "$name: no '**R<n>** WHEN … SHALL' requirement in $(basename "$reqfile")"
  while IFS= read -r line; do
    printf '%s\n' "$line" | grep -q 'WHEN.*THEN.*SHALL' ||
      err "$name: requirement not in WHEN/THEN/SHALL form: ${line:0:80}"
  done < <(awk '/^\*\*R[0-9]+\*\*/{p=$0; f=1; next} f&&NF&&!/^ *-/{p=p " " $0; next} f{print p; f=0} END{if(f) print p}' "$reqfile")

  for r in $reqs; do
    grep -qE "(^|[^0-9A-Za-z])$r([^0-9]|$)" "$dir/tasks.md" || err "$name: $r not covered by any task in tasks.md"
  done

  grep -qE '^- \[( |-|x)\] T[0-9]+[a-z]?\. ' "$dir/tasks.md" || err "$name: tasks.md has no '- [ ] T<n>. <task>' items"
  bad=$(grep -E '^- \[.\]' "$dir/tasks.md" | grep -vE '^- \[( |-|x)\] T[0-9]+[a-z]?\. ' || true)
  [ -z "$bad" ] || err "$name: task item not in '- [ |-|x] T<n>. …' form: $(printf '%s' "$bad" | head -1 | cut -c1-60)"

  if grep -qE '^> Status: (active|shipped)' "$reqfile" && grep -q '\[TO CONFIRM\]' "$reqfile"; then
    err "$name: status active/shipped but [TO CONFIRM] remains"
  fi

  if [ -f "$dir/design.md" ]; then
    decisions=$(grep -cE '^### D[0-9]+' "$dir/design.md")
    wrong=$(grep -cE '\*\*When this would be wrong\*\*' "$dir/design.md")
    reversal=$(grep -cE '\*\*Reversal action\*\*' "$dir/design.md")
    [ "$wrong" -ge "$decisions" ] || err "$name: $decisions decisions but $wrong 'When this would be wrong'"
    [ "$reversal" -ge "$decisions" ] || err "$name: $decisions decisions but $reversal 'Reversal action'"
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
