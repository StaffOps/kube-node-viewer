#!/usr/bin/env python3
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
"""Fail when what a user sees changes with no spec or docs change.

    scripts/drift-check.py [BASE]

Reads, at BASE and in the working tree:
  - flags: each flag registered in cmd/kube-node-viewer/flag.go, with its
    default and usage text;
  - config keys: the ~/.kube-node-viewer keys read in cmd/kube-node-viewer
    (cfg.getValue("…"), cfg["…"]) and the config.yaml keys
    (mapping(…, "key", …) in settings.go and configview.go);
  - keys: the helpKeys entries (key and text) in pkg/model/panels.go;
  - fields: the Field constants (columns, sorts, filters) in
    pkg/model/view.go.

Rules:
  1. A new flag, config key or help key must be named in README.md.
  2. Any other change to them must come with a change to a spec
     (specs/<spec>/…, not specs/README.md), README.md or docs/, or a commit
     trailer on a commit that changes one of the files above:
         Drift-check: skip — <reason>
  3. Every key the main view handles (`case "…"` in pkg/model/uimodel.go)
     is in the help, whatever the docs say.
  4. Each list must find something in the working tree: an empty one means
     the code moved and this script must follow it.

BASE defaults as in scripts/changelog-check.py. Reading the sets rather than
the diff lets a refactor that moves code around pass untouched.
"""
import os
import re
import subprocess
import sys

TRAILER = re.compile(r"^Drift-check: skip [—-]+ \S", re.M)
CMD = "cmd/kube-node-viewer"
ARROWS = {"↑": "up", "↓": "down", "←": "left", "→": "right"}


def git(*args):
    return subprocess.run(["git", *args], capture_output=True, text=True)


def default_base():
    if os.environ.get("GITHUB_EVENT_NAME") == "pull_request":
        return git("rev-parse", "-q", "--verify", "HEAD^1").stdout.strip()
    return git("merge-base", "HEAD", "origin/main").stdout.strip()


def flags(src):
    """{name: the rest of the call (default and usage)}."""
    return {m.group(1): " ".join(m.group(2).split())
            for m in re.finditer(r'flagSet\.\w+Var\(&[\w.]+,\s*"([^"]+)",(.*)\)\s*$', src, re.M)}


def config_keys(src):
    keys = set(re.findall(r'cfg\.get\w*Value\("([^"]+)"', src))
    keys |= set(re.findall(r'cfg\["([^"]+)"\]', src))
    for m in re.finditer(r"\bmapping\(([^()]*)\)", src):
        keys |= set(re.findall(r'"([^"]*)"', m.group(1))[1:])  # the first is the path
    return {k: "" for k in keys if re.fullmatch(r"[a-z][a-z0-9-]*", k)}


def help_keys(src):
    m = re.search(r"^var helpKeys = \[\]\[2\]string\{\n(.*?)^\}", src, re.M | re.S)
    if not m:
        return {}
    out = {}
    for k, text in re.findall(r'\{"((?:[^"\\]|\\.)*)",\s*"((?:[^"\\]|\\.)*)"\}', m.group(1)):
        out[f"{k}: {text}"] = ""
    return out


def fields(src):
    return {f: "" for f in re.findall(r'^\s*Field\w*\s+Field\s*=\s*"([^"]+)"', src, re.M)}


SURFACES = [
    ("flag", [f"{CMD}/flag.go"], flags, True),
    ("config key", [f"{CMD}/flag.go", f"{CMD}/recommend.go", f"{CMD}/settings.go", f"{CMD}/configview.go"], config_keys, True),
    ("help key", ["pkg/model/panels.go"], help_keys, True),
    ("field", ["pkg/model/view.go"], fields, False),
]


def read(rev, path):
    if rev is None:
        return open(path).read() if os.path.exists(path) else ""
    return git("show", f"{rev}:{path}").stdout


def collect(rev, paths, extract):
    out = {}
    for p in paths:
        out.update(extract(read(rev, p)))
    return out


def help_tokens(src):
    """Every key the help names: the key column split up, arrows as names,
    and the words of the texts (`space` in "space turns a filter on/off")."""
    toks = set()
    for entry in help_keys(src):
        k, _, text = entry.partition(": ")
        toks.add(k)
        for t in re.split(r"[\s,/]+", k):
            toks.add(ARROWS.get(t, t))
        toks |= set(re.findall(r"[\w+]+", text))
    return toks


def handled_keys(src):
    out = set()
    for m in re.finditer(r'^\s*case ((?:"[^"]*",?\s*)+):', src, re.M):
        out |= set(re.findall(r'"([^"]*)"', m.group(1)))
    return {"space" if k == " " else k for k in out}


def main():
    top = git("rev-parse", "--show-toplevel").stdout.strip()
    if top:
        os.chdir(top)
    base = sys.argv[1] if len(sys.argv) > 1 else default_base()
    problems, changes = [], []
    readme = read(None, "README.md")

    for name, paths, extract, documented in SURFACES:
        new = collect(None, paths, extract)
        if not new:
            problems.append(f"no {name} found in {', '.join(paths)}: update scripts/drift-check.py")
            continue
        if not base:
            continue
        old = collect(base, paths, extract)
        for k in sorted(set(new) - set(old)):
            changes.append(f"{name}: added {k[:90]}")
            word = k.split(": ")[0] if name == "help key" else k
            if documented and word not in readme:
                problems.append(f"new {name} {word!r} is not in README.md")
        for k in sorted(set(old) - set(new)):
            changes.append(f"{name}: removed {k[:90]}")
        for k in sorted(set(old) & set(new)):
            if old[k] != new[k]:
                changes.append(f"{name}: changed {k}: {new[k][:80]}")

    ui = read(None, "pkg/model/uimodel.go")
    missing = sorted(handled_keys(ui) - help_tokens(read(None, "pkg/model/panels.go")))
    for k in missing:
        problems.append(f"key {k!r} is handled in pkg/model/uimodel.go but not in helpKeys")

    if not base:
        print("drift-check: no base to compare with; only the help check ran")
    elif changes and not problems:
        changed = git("diff", "--name-only", base).stdout.split()
        docs = [f for f in changed if f == "README.md" or f.startswith("docs/")
                or (f.startswith("specs/") and f.count("/") >= 2)]
        surface = sorted({p for _, paths, _, _ in SURFACES for p in paths})
        skipped = any(TRAILER.search(git("log", "-1", "--format=%B", c).stdout)
                      for c in git("rev-list", f"{base}..HEAD", "--", *surface).stdout.split())
        if not docs and not skipped:
            problems.append("flags, config keys, help keys or fields changed with no change to a spec, "
                            "README.md or docs/; update one, or if none needs it, add a trailer "
                            "'Drift-check: skip — <reason>' to the commit that changes them")
    if problems:
        print("Drift between what users see and the docs (drift-check):", file=sys.stderr)
        for p in problems + changes:
            print("  " + p, file=sys.stderr)
        return 1
    print(f"drift-check: ok ({len(changes)} change(s))")
    return 0


if __name__ == "__main__":
    sys.exit(main())
