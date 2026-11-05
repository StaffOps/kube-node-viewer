# Validating the Kiro hooks in a real session

The Kiro wiring in this repository (`.kiro/hooks/kube-node-viewer.json`,
`.kiro/agents/*.json`) follows the Kiro documentation, and
`scripts/agent-hooks/kiro-selftest.sh` checks it offline. What has never
been checked is a real Kiro session. This checklist is for a Kiro user: it
takes about 15 minutes and every step is harmless. The destructive commands
are prefixed with `echo`, so nothing runs even if a hook fails to fire.

Docs this wiring follows (read 2026-11-05):
[Hooks](https://kiro.dev/docs/hooks/),
[Hook types](https://kiro.dev/docs/hooks/types/),
[Hook actions](https://kiro.dev/docs/hooks/actions/),
[Agent configuration](https://kiro.dev/docs/custom-agents/configuration-reference/),
[CLI V3 hooks migration](https://kiro.dev/docs/cli/v3/hooks-migration/).

## Before you start

1. Kiro IDE 1.x or Kiro CLI V3 (`kiro-cli --version`). The standalone
   `.kiro/hooks/*.json` format needs one of them; CLI 2.x does not read it.
2. A clone of this repository, on a scratch branch with a clean tree:
   `git switch -c kiro-validation`.
3. `make hooks` (the git hooks; the session-context hook also does it).
4. `jq` and `python3` on `PATH` (`brew install jq`). Without `jq` every
   hook lets every command through.
5. The offline self-test: `make kiro-selftest`. Every line must say
   `PASS`; fix a `FAIL` before opening Kiro (its message says what is wrong).

## Tests

Run them in the CLI (`kiro-cli chat --agent kube-node-viewer`) or in the
IDE chat. For each one, ask the agent to run the command exactly as written.

| # | Action | Expected | If it differs, copy back |
|---|--------|----------|--------------------------|
| 1 | Open a new session. | The agent's context has the block starting `kube-node-viewer session context` (branch, open spec tasks), **once**. IDE: the Agent Hooks panel lists the five `kube-node-viewer: …` hooks. | The session's first messages, `/hooks` output (CLI) or a screenshot of the hooks panel. |
| 2 | Add a temporary payload capture: create `.kiro/hooks/zz-capture.json` with `{"version":"v1","hooks":[{"name":"capture","trigger":"PreToolUse","matcher":"shell","action":{"type":"command","command":"cat > /tmp/kiro-hook-payload.json"}}]}`, start a new session, ask the agent to run `git status`. | `/tmp/kiro-hook-payload.json` has `hook_event_name`, `tool_name` (`execute_bash`) and `tool_input.command` = `git status`. Then delete `zz-capture.json`. | The file's content (replace your home path with `~`). The hooks read `tool_input.command`; another field name means none of them sees the command. |
| 3 | `echo git push --force origin kiro-validation` | Refused before it runs: `kube-node-viewer guard: blocked (git push that overwrites or deletes remote refs)…`. | The agent's message and the hook output. |
| 4 | `echo git reset --hard HEAD` | Refused by the same guard: `kube-node-viewer guard: blocked (discards working-tree changes)…`. | As in 3. |
| 5 | `echo node,cpu > kube-node-viewer-validation.csv && git add kube-node-viewer-validation.csv`, then `git commit -m "test: kiro validation"` | The commit is refused by `kube-node-viewer commit guard: blocked (…export file…)`, before git runs. Afterwards: `git rm --cached kube-node-viewer-validation.csv && rm kube-node-viewer-validation.csv`. | The hook output. If git ran and its `pre-commit` hook refused instead, say so: the Kiro hook did not fire. |
| 6 | `git commit --allow-empty -m "bad subject"` | Refused by the **git** `commit-msg` hook: `commit-msg: blocked: subject must be a Conventional Commit…`. (A subject is checked by git, not by a Kiro hook.) | Git's output. |
| 7 | Same command as 6. Did the agent receive the pre-commit checklist (`Pre-commit checklist (kube-node-viewer; …`)? Ask it. | Unknown, and the point of this test: the docs say a PreToolUse command's stdout on exit 0 is ignored, so the checklist may never reach the agent. | Whether the agent saw it, and where (context, a warning shown to you, nothing). |
| 8 | `echo gh pr create --fill` | Runs (prints the words): on a branch with no open spec task the PR guard lets it through. The blocking case is covered by the self-test. | The hook output, if any. |
| 9 | `ls` | Runs with no hook message. | Any hook message. |

## Report

Paste this into an issue or a PR comment:

```text
Kiro validation (docs/kiro-validation.md)
Date:
Kiro: IDE <version> | CLI <kiro-cli --version>
OS / shell:
make kiro-selftest: <N passed, N failed, N skipped>
1 session context once:      ok | differs: …
2 payload fields:            ok | differs: <paste, home path as ~>
3 force push refused:        ok | differs: …
4 reset --hard refused:      ok | differs: …
5 export file commit refused: ok | differs: …
6 bad subject refused by git: ok | differs: …
7 checklist reached agent:   yes | no | shown to me only
8 PR guard quiet:            ok | differs: …
9 ls quiet:                  ok | differs: …
Notes:
```

Leave out cluster names, endpoints, account ids and company names, as
everywhere in this repository.
