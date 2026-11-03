#!/bin/sh
# PreToolUse hook: on `git commit`, block what must never be committed.
#   commit-guard.sh claude  -> JSON "deny" decision (Claude Code)
#   commit-guard.sh kiro    -> reason on stderr + exit 2 (Kiro blocks)
# Checks the staged changes (plus tracked changes when the commit uses -a):
# - exports from the TUI (kube-node-viewer-*.csv / *.json): cluster data;
# - lines added inside a released CHANGELOG section (## [x.y.z]): changes
#   go under ## [Unreleased]; release.yaml writes the version sections.
mode=${1:-claude}
command -v jq >/dev/null 2>&1 || exit 0
raw=$(jq -r '.tool_input.command // .tool_input.cmd // ""' 2>/dev/null)
# shellcheck disable=SC1003 # '\n\\' is a tr set (newline, backslash), not an escaped quote
cmd=$(printf '%s' "$raw" | tr '\n\\' '  ' | sed -E "s/'[^']*'|\"[^\"]*\"//g")
printf '%s\n' "$cmd" | grep -Eq '(^|[;&|(/[:space:]])git([[:space:]]+(-C|-c)[[:space:]]+[^[:space:]]+|[[:space:]]+-[^[:space:]]+)*[[:space:]]+commit([[:space:]]|$)' || exit 0

cd "${CLAUDE_PROJECT_DIR:-$(git rev-parse --show-toplevel 2>/dev/null || echo .)}" 2>/dev/null || exit 0
diffargs="--cached"
printf '%s\n' "$cmd" | grep -Eq '[[:space:]]commit([[:space:]]+[^;&|]*)?[[:space:]](-[a-zA-Z]*a[a-zA-Z]*|--all)([[:space:]]|$)' && diffargs="HEAD"

reason=""
exports=$(git diff $diffargs --name-only --diff-filter=AM 2>/dev/null | grep -E '(^|/)kube-node-viewer-[^/]*\.(csv|json)$')
[ -n "$exports" ] && reason="export files with cluster data staged: $(printf "%s" "$exports" | tr "\n" " ")"

if [ -z "$reason" ] && git diff $diffargs --name-only 2>/dev/null | grep -qx CHANGELOG.md; then
  # new-file line numbers of added lines, compared with where the first
  # released section starts in the new file
  first=$(git show ":CHANGELOG.md" 2>/dev/null | grep -nE '^## \[[0-9]+\.[0-9]+\.[0-9]+\]' | head -1 | cut -d: -f1)
  [ "$diffargs" = HEAD ] && first=$(grep -nE '^## \[[0-9]+\.[0-9]+\.[0-9]+\]' CHANGELOG.md | head -1 | cut -d: -f1)
  if [ -n "$first" ]; then
    late=$(git diff $diffargs -U0 -- CHANGELOG.md | awk -v first="$first" '
      /^@@/ { split($3, a, ","); n = substr(a[1], 2) + 0; next }
      /^\+\+\+/ { next }
      /^\+/ { if (n > first && $0 !~ /^\+## \[/ && $0 !~ /^\+\[[0-9.]+\]: / && $0 !~ /^\+\[Unreleased\]: / && $0 != "+") print n; n++; next }
    ' | head -1)
    [ -n "$late" ] && reason="CHANGELOG.md line $late adds to a released section; put changes under ## [Unreleased]"
  fi
fi
[ -n "$reason" ] || exit 0

if [ "$mode" = kiro ]; then
  printf '%s\n' "kube-node-viewer commit guard: blocked ($reason)." >&2
  exit 2
fi
jq -n --arg r "kube-node-viewer commit guard: $reason." '{hookSpecificOutput: {hookEventName: "PreToolUse", permissionDecision: "deny", permissionDecisionReason: $r}}'
