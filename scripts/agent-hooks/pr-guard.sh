#!/bin/sh
# PreToolUse hook: before `gh pr create` (not --draft), `gh pr ready` or
# `gh pr merge`, run the checks CI runs before a merge (and so a release):
#   scripts/changelog-check.py   no edits to released CHANGELOG sections
#   scripts/spec-tasks-check.py  no open spec tasks (verify = run on a cluster)
#   pr-guard.sh claude  -> JSON "deny" decision (Claude Code)
#   pr-guard.sh kiro    -> reason on stderr + exit 2 (Kiro blocks)
mode=${1:-claude}
command -v jq >/dev/null 2>&1 || exit 0
command -v python3 >/dev/null 2>&1 || exit 0
raw=$(jq -r '.tool_input.command // .tool_input.cmd // ""' 2>/dev/null)
# shellcheck disable=SC1003 # '\n\\' is a tr set (newline, backslash), not an escaped quote
cmd=$(printf '%s' "$raw" | tr '\n\\' '  ' | sed -E "s/'[^']*'|\"[^\"]*\"//g")
printf '%s\n' "$cmd" | grep -Eq '(^|[;&|(/[:space:]])gh[[:space:]]+pr[[:space:]]+(create|ready|merge)([[:space:]]|$)' || exit 0
if printf '%s\n' "$cmd" | grep -Eq 'gh[[:space:]]+pr[[:space:]]+create' && printf '%s\n' "$cmd" | grep -Eq '[[:space:]](-d|--draft)([[:space:]]|$)'; then
  exit 0 # a draft PR may still have open tasks
fi

cd "${CLAUDE_PROJECT_DIR:-$(git rev-parse --show-toplevel 2>/dev/null || echo .)}" 2>/dev/null || exit 0
out=""; failed=false
for check in scripts/changelog-check.py scripts/spec-tasks-check.py; do
  [ -f "$check" ] || continue # a branch cut before the check existed
  o=$(python3 -B "$check" 2>&1) || { failed=true; out="$out $o"; }
done
$failed || exit 0
reason=$(printf '%s' "$out" | grep -v ': ok' | tr '\n' ' ' | cut -c1-600)

if [ "$mode" = kiro ]; then
  printf '%s\n' "kube-node-viewer PR guard: blocked. $reason Open it as a draft (gh pr create --draft) until then." >&2
  exit 2
fi
jq -n --arg r "kube-node-viewer PR guard: $reason Open it as a draft (gh pr create --draft) until then." '{hookSpecificOutput: {hookEventName: "PreToolUse", permissionDecision: "deny", permissionDecisionReason: $r}}'
