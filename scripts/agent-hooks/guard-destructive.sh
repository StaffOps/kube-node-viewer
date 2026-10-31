#!/bin/sh
# PreToolUse hook: stop shell commands that rewrite or delete published git
# state or wipe local work, until a human confirms.
#   guard-destructive.sh claude  -> JSON "ask" decision (Claude Code)
#   guard-destructive.sh kiro    -> reason on stderr + exit 2 (Kiro blocks)
# Reads the hook payload (JSON with .tool_input.command) on stdin.
mode=${1:-claude}

if ! command -v jq >/dev/null 2>&1; then
  [ "$mode" = kiro ] && exit 0
  printf '%s\n' '{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"ask","permissionDecisionReason":"kube-node-viewer guard: jq missing, cannot inspect the command"}}'
  exit 0
fi

cmd=$(jq -r '.tool_input.command // .tool_input.cmd // ""' 2>/dev/null)
[ -n "$cmd" ] || exit 0
# Join lines and drop quoted strings so commit messages or greps don't match.
# shellcheck disable=SC1003 # '\n\\' is a tr set (newline, backslash), not an escaped quote
plain=$(printf '%s' "$cmd" | tr '\n\\' '  ' | sed -E "s/'[^']*'|\"[^\"]*\"//g")

reason=""
B='(^|[;&|(`/[:space:]])'
O='([[:space:]]+-[^[:space:]]+([[:space:]]+[^-[:space:]][^[:space:]]*)?)*'
A='([[:space:]]+[^;&|]*)?'
scan() { # scan <text>
  check() { printf '%s\n' "$1" | grep -Eq "$2" && reason="$3"; }
  t=$1
  check "$t" "${B}git${O}[[:space:]]+push${A}[[:space:]](-[a-zA-Z]*[fd][a-zA-Z]*|--force(-with-lease)?(=[^[:space:]]*)?|--delete|--mirror|--prune|\+[^[:space:]]+|[^[:space:]]*:\+[^[:space:]]*|:[^[:space:]]*)([[:space:]]|$)" "git push that overwrites or deletes remote refs"
  check "$t" "${B}git${O}[[:space:]]+tag${A}[[:space:]](-[a-zA-Z]*d[a-zA-Z]*|--delete)([[:space:]]|$)" "deletes git tags"
  check "$t" "${B}git${O}[[:space:]]+(filter-repo|filter-branch)([[:space:]]|$)" "rewrites git history"
  check "$t" "${B}git${O}[[:space:]]+reset${A}[[:space:]]--hard([[:space:]]|$)" "discards working-tree changes"
  check "$t" "${B}git${O}[[:space:]]+clean${A}[[:space:]](-[a-zA-Z]*f|--force)" "deletes untracked files"
  check "$t" "${B}git${O}[[:space:]]+branch${A}[[:space:]](-D[a-zA-Z]*|-[a-zA-Z]*D|--delete[[:space:]]+--force)([[:space:]]|$)" "force-deletes a branch"
  check "$t" "${B}git${O}[[:space:]]+(checkout|switch)${A}[[:space:]](-f|--force|--discard-changes)([[:space:]]|$)" "discards working-tree changes"
  check "$t" "${B}git${O}[[:space:]]+checkout${A}[[:space:]]--[[:space:]]" "discards working-tree changes in paths"
  if printf '%s\n' "$t" | grep -Eq "${B}git${O}[[:space:]]+restore([[:space:]]|$)" && ! printf '%s\n' "$t" | grep -Eq -- '--staged' ; then
    check "$t" "${B}git${O}[[:space:]]+restore${A}[[:space:]][^-[:space:]]" "discards working-tree changes in paths"
  fi
  check "$t" "${B}git${O}[[:space:]]+stash[[:space:]]+(drop|clear)([[:space:]]|$)" "deletes stashed work"
  check "$t" "${B}git${O}[[:space:]]+update-ref${A}[[:space:]]-d([[:space:]]|$)" "deletes a ref"
  check "$t" "${B}git${O}[[:space:]]+reflog[[:space:]]+(expire|delete)([[:space:]]|$)" "drops reflog recovery points"
}
scan "$plain"
# Quoted text is skipped above so messages don't match; but `sh -c '…'` and
# `eval "…"` run their quoted argument, and `git push` refspecs may be quoted
# ("+main"), so rescan those with only the quotes removed.
if [ -z "$reason" ] && printf '%s\n' "$plain" | grep -Eq "${B}((ba|z)?sh[[:space:]]+(-[a-z]+[[:space:]]+)*-[a-z]*c|eval)([[:space:]]|$)|${B}git${O}[[:space:]]+push([[:space:]]|$)"; then
  # shellcheck disable=SC1003 # '\n\\' is a tr set (newline, backslash), not an escaped quote
  scan "$(printf '%s' "$cmd" | tr '\n\\' '  ' | tr -d "'\"")"
fi
[ -n "$reason" ] || exit 0

msg="kube-node-viewer guard: $reason. Proceed only if the owner asked for it."
if [ "$mode" = kiro ]; then
  # Kiro: exit 2 blocks outright; the owner has to run it themselves.
  printf '%s\n' "kube-node-viewer guard: blocked ($reason). Do not retry; ask the owner to run it." >&2
  exit 2
fi
jq -n --arg r "$msg" '{hookSpecificOutput: {hookEventName: "PreToolUse", permissionDecision: "ask", permissionDecisionReason: $r}}'
