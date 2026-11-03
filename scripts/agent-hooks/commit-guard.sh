#!/bin/sh
# PreToolUse hook: on `git commit`, block what must never be committed.
#   commit-guard.sh claude  -> JSON "deny" decision (Claude Code)
#   commit-guard.sh kiro    -> reason on stderr + exit 2 (Kiro blocks)
# Runs scripts/checks/staged.sh (the same checks as .githooks/pre-commit)
# before the commit is attempted, so the agent sees why: exports, released
# CHANGELOG sections, gofmt, Apache headers on new files.
mode=${1:-claude}
command -v jq >/dev/null 2>&1 || exit 0
raw=$(jq -r '.tool_input.command // .tool_input.cmd // ""' 2>/dev/null)
# shellcheck disable=SC1003 # '\n\\' is a tr set (newline, backslash), not an escaped quote
cmd=$(printf '%s' "$raw" | tr '\n\\' '  ' | sed -E "s/'[^']*'|\"[^\"]*\"//g")
printf '%s\n' "$cmd" | grep -Eq '(^|[;&|(/[:space:]])git([[:space:]]+(-C|-c)[[:space:]]+[^[:space:]]+|[[:space:]]+-[^[:space:]]+)*[[:space:]]+commit([[:space:]]|$)' || exit 0

cd "${CLAUDE_PROJECT_DIR:-$(git rev-parse --show-toplevel 2>/dev/null || echo .)}" 2>/dev/null || exit 0
[ -f scripts/checks/staged.sh ] || exit 0
all=""
printf '%s\n' "$cmd" | grep -Eq '[[:space:]]commit([[:space:]]+[^;&|]*)?[[:space:]](-[a-zA-Z]*a[a-zA-Z]*|--all)([[:space:]]|$)' && all="--all"
# shellcheck disable=SC2086 # $all is empty or one flag
reason=$(sh scripts/checks/staged.sh $all | tr '\n' ';' | sed 's/;$//')
[ -n "$reason" ] || exit 0

if [ "$mode" = kiro ]; then
  printf '%s\n' "kube-node-viewer commit guard: blocked ($reason)." >&2
  exit 2
fi
jq -n --arg r "kube-node-viewer commit guard: $reason." '{hookSpecificOutput: {hookEventName: "PreToolUse", permissionDecision: "deny", permissionDecisionReason: $r}}'
