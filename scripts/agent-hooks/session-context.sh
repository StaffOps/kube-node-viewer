#!/bin/sh
# SessionStart hook (Claude Code and Kiro): print branch, dirty count and open
# spec tasks as plain text; both tools add stdout to the session context.
cd "${CLAUDE_PROJECT_DIR:-$(git rev-parse --show-toplevel 2>/dev/null || echo .)}" 2>/dev/null || exit 0

branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null)
dirty=$(git status --porcelain 2>/dev/null | wc -l | tr -d ' ')

echo "kube-node-viewer session context"
echo "branch: $branch, uncommitted files: $dirty"
echo "Rules: AGENTS.md. Specs: specs/README.md. Skills: spec-new, change-loop, upstream-sync. Reviewer: spec-refuter."
echo "Open spec tasks ([ ] / [-], max 10 per spec):"
for f in specs/*/tasks.md; do
  [ -f "$f" ] || continue
  d=$(basename "$(dirname "$f")")
  grep -E '^- \[( |-)\] T[0-9]+' "$f" | head -10 | sed -E "s/^- \[(.)\] /  $d [\1] /" | cut -c1-120
done
exit 0
