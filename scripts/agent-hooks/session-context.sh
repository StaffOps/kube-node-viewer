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
# SessionStart hook (Claude Code and Kiro): print branch, dirty count and open
# spec tasks as plain text; both tools add stdout to the session context.
cd "${CLAUDE_PROJECT_DIR:-$(git rev-parse --show-toplevel 2>/dev/null || echo .)}" 2>/dev/null || exit 0

branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null)
dirty=$(git status --porcelain 2>/dev/null | wc -l | tr -d ' ')

echo "kube-node-viewer session context"
if [ -d .githooks ] && [ "$(git config core.hooksPath)" != .githooks ]; then
  git config core.hooksPath .githooks && echo "git hooks: installed .githooks (pre-commit, commit-msg, pre-push) for this clone"
fi
echo "branch: $branch, uncommitted files: $dirty"
skills=$(for d in .kiro/skills/*/SKILL.md; do [ -f "$d" ] && basename "$(dirname "$d")"; done | tr '\n' ' ' | sed 's/ $//; s/ /, /g')
echo "Rules: AGENTS.md. Specs: specs/README.md. Backlog: TODO.md. Skills: ${skills:-none}. Reviewer: spec-refuter."
echo "Flow: branch -> PR. CI (verify, lint, build, secrets, guardrails, deps, workflows) green on a PR from a trusted author -> approved and merged automatically (label hold to stop). CHANGELOG [Unreleased] entries -> next version, released Mondays 12:00 UTC or on demand (gh workflow run release.yaml, or label a PR release) (Fixed=patch, Added/Changed/Removed=minor). Code + spec PRs need every spec task [x], the cluster run included (draft until then). Never push to main or tags."
echo "Open spec tasks ([ ] / [-], max 10 per spec):"
for f in specs/*/tasks.md; do
  [ -f "$f" ] || continue
  d=$(basename "$(dirname "$f")")
  grep -E '^- \[( |-)\] T[0-9]+' "$f" | head -10 | sed -E "s/^- \[(.)\] /  $d [\1] /" | cut -c1-120
done
exit 0
