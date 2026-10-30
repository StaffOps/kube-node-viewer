#!/bin/sh
# PreToolUse hook (Claude Code): on `git commit`, remind the docs/spec checklist.
command -v jq >/dev/null 2>&1 || exit 0
# shellcheck disable=SC1003 # '\n\\' is a tr set (newline, backslash), not an escaped quote
cmd=$(jq -r '.tool_input.command // ""' | tr '\n\\' '  ' | sed -E "s/'[^']*'|\"[^\"]*\"//g")
printf '%s\n' "$cmd" | grep -Eq '(^|[;&|(/[:space:]])git([[:space:]]+(-C|-c)[[:space:]]+[^[:space:]]+|[[:space:]]+-[^[:space:]]+)*[[:space:]]+commit([[:space:]]|$)' || exit 0
jq -n '{hookSpecificOutput: {hookEventName: "PreToolUse", additionalContext: "Pre-commit checklist (kube-node-viewer): scripts/verify.sh green; task status updated in specs/<feature>/tasks.md; CHANGELOG.md [Unreleased]; README usage if flags/keys changed; Apache headers on new files; Conventional Commit message with no AI trailers."}}'
