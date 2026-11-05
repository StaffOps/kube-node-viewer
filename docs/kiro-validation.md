# Validating the Kiro hooks in a real session

The Kiro wiring in this repository (`.kiro/hooks/kube-node-viewer.json`,
`.kiro/agents/*.json`) follows the Kiro documentation, and
`scripts/agent-hooks/kiro-selftest.sh` checks it offline. A real Kiro
session has never run it. This checklist is for a Kiro user and takes about
15 minutes. Run it on a scratch branch. Only tests 5, 6 and 9 run real
commands, and those touch nothing beyond the scratch branch; the other
commands are prefixed with `echo`, so they do nothing even if a hook fails
to fire.

Docs this wiring follows (read 2026-11-05):
[Hooks](https://kiro.dev/docs/hooks/),
[Hook types](https://kiro.dev/docs/hooks/types/),
[Hook actions](https://kiro.dev/docs/hooks/actions/),
[Agent configuration](https://kiro.dev/docs/custom-agents/configuration-reference/),
[Permissions](https://kiro.dev/docs/permissions/),
[CLI V3 hooks migration](https://kiro.dev/docs/cli/v3/hooks-migration/).

## Before you start

1. Kiro IDE 1.x or Kiro CLI V3 (`kiro-cli --version`). The standalone
   `.kiro/hooks/*.json` file needs one of them; CLI 2.x does not read it
   (the git hooks and CI still apply there).
2. A clone of this repository, on a scratch branch with a clean tree:
   `git switch -c kiro-validation`.
3. `make hooks` (the git hooks; the session-context hook also does it).
4. `jq` and `python3` on `PATH` (`brew install jq`). Without `jq` every
   hook lets every command through.
5. The offline self-test: `make kiro-selftest`. Every line must say
   `PASS`. Fix a `FAIL` before you open Kiro; its message says what is wrong.

## Tests

Run them in the CLI (`kiro-cli chat --agent kube-node-viewer`) or in the
IDE chat. For each one, ask the agent to run the command exactly as written.

When a command is refused, also note the hook's text and the exit status
Kiro reports. The docs say any non-zero exit blocks a tool in one place
and only exit 2 in another; the IDE and the CLI may differ. The hooks run
`sh scripts/agent-hooks/…` with a relative path, so they need Kiro to run
them from the repository root. A `No such file` error means it does not:
record the directory Kiro used.

| # | Action | Expected | If it differs, copy back |
|---|--------|----------|--------------------------|
| 1 | Open a new session. | The agent's context has the block starting `kube-node-viewer session context` (branch, open spec tasks), **exactly once**. IDE: the Agent Hooks panel lists the five `kube-node-viewer: …` hooks. | The session's first messages, `/hooks` output (CLI) or a screenshot of the hooks panel. Zero copies under `--agent` means the CLI did not load the hook file (see "If test 1 fails"); two copies mean the hooks run twice. |
| 2 | Add a temporary payload capture: create `.kiro/hooks/zz-capture.json` with `{"version":"v1","hooks":[{"name":"capture","trigger":"PreToolUse","matcher":"shell","action":{"type":"command","command":"cat > /tmp/kiro-hook-payload.json"}}]}`, start a new session, ask the agent to run `git status`. | `/tmp/kiro-hook-payload.json` has `tool_input.command` equal to `git status`. Note `hook_event_name` and `tool_name` too (the docs give `execute_bash`; a V3 harness may report another ID). Then delete `zz-capture.json`. | The file's content (with your home path replaced by `~`). The hooks read `tool_input.command`: any other field name means none of them sees the command. |
| 3 | `echo git push --force origin kiro-validation` | Refused before it runs: `kube-node-viewer guard: blocked (git push that overwrites or deletes remote refs)…`. | The agent's message, the hook text and the exit status. |
| 4 | `echo git reset --hard HEAD` | Refused by the same guard: `kube-node-viewer guard: blocked (discards working-tree changes)…`. | As in 3. |
| 5 | `echo node,cpu > kube-node-viewer-validation.csv && git add -f kube-node-viewer-validation.csv`, then `git commit -m "test: kiro validation"` | The commit is refused by `kube-node-viewer commit guard: blocked (…export file…)`, before git runs. (`-f` is needed because exports are git-ignored.) | The hook text. If git ran and its `pre-commit` hook refused instead, say so: the Kiro hook did not fire. |
| 6 | `git rm -q --cached kube-node-viewer-validation.csv && rm kube-node-viewer-validation.csv`, then `git commit --allow-empty -m "bad subject"` | Refused by the **git** `commit-msg` hook: `commit-msg: blocked: subject must be a Conventional Commit…`. (Git checks the subject; no Kiro hook does.) | Git's output. |
| 7 | After 6, ask the agent: did it receive a `Pre-commit checklist (kube-node-viewer; …` message before running the commit? | Unknown; this test decides it. The docs disagree: Hook actions says a hook's stdout on exit 0 is added to the agent's context, while another passage says a PreToolUse hook's stdout is ignored. | Whether the agent saw it, and where: in its context, as a warning shown only to you, or nowhere. |
| 8 | `echo gh pr create --fill` | Runs (it only prints the words): with nothing wrong on the branch, the PR guard lets it through. | Any hook message. |
| 9 | Add a line under the newest released heading: `python3 -c "p='CHANGELOG.md';t=open(p).read();i=t.index(chr(10),t.index(chr(10)+'## [0.')+1);open(p,'w').write(t[:i]+chr(10)+'- test'+t[i:])"`, then `echo gh pr create --fill` | Refused: `kube-node-viewer PR guard: blocked. … released sections must not change …`. Afterwards: `git checkout CHANGELOG.md`. | The hook text and exit status. |
| 10 | `ls` | Runs, with no hook message. | Any hook message. |
| 11 | Start the reviewer: `kiro-cli chat --agent spec-refuter` (IDE: pick the `spec-refuter` agent). Ask it to run `git log -1 --oneline`, then `git commit --allow-empty -m "test: x"`. | The first runs without a prompt. The second is denied by the agent's permission rules (it may not commit, push or write files). | What happened to each command, including any prompt shown. |

## Cleanup (run yourself)

```sh
git status                      # a test commit should not have landed
git reset --soft HEAD~1         # only if one did
rm -f kube-node-viewer-validation.csv .kiro/hooks/zz-capture.json /tmp/kiro-hook-payload.json
git checkout CHANGELOG.md
git switch main && git branch -D kiro-validation
```

## If test 1 fails

The agent profile `.kiro/agents/kube-node-viewer.json` holds no hooks: the
five hooks live only in the hook file, so there is one source of truth.
The docs say CLI V3 loads `.kiro/hooks/*.json` in every session, profile
or not; that is not yet confirmed. If the session context does not appear
under `--agent kube-node-viewer` on CLI V3, put the hooks back into the
profile as an inline `hooks` array (the same five definitions as the hook
file) and report it.

## Report

Paste this into an issue or a PR comment:

```text
Kiro validation (docs/kiro-validation.md)
Date:
Kiro: IDE <version> | CLI <kiro-cli --version>
OS / shell:
make kiro-selftest: <N passed, N failed, N skipped>
1 session context exactly once:    ok | differs: …
2 tool_input.command:              ok | differs: <paste, home path as ~>; tool_name: …
3 force push refused:              ok | differs: … (exit status: …)
4 reset --hard refused:            ok | differs: … (exit status: …)
5 export file commit refused:      ok | differs: …
6 bad subject refused by git:      ok | differs: …
7 checklist reached the agent:     yes | no | shown to me only
8 PR guard quiet:                  ok | differs: …
9 PR guard refused CHANGELOG edit: ok | differs: … (exit status: …)
10 ls quiet:                       ok | differs: …
11 spec-refuter permissions:       ok | differs: …
Hook working directory issues:     none | …
Notes:
```

Leave out cluster names, endpoints, account ids and company names, as
everywhere in this repository.
