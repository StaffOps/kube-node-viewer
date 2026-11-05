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
"""Fail when a requirement of an active or shipped spec names no test.

    scripts/spec-trace.py [-v]

Every **R<n>** (or **R<n><letter>**) in specs/<spec>/requirements.md
(bugfix.md for fix-*) whose status is active or shipped needs one of:

  - a test citing it: in the comment block right above a `func Test…` or a
    `t.Run(` line of a *_test.go file (above any case line of
    scripts/checks/test-checks.sh), a line that names the spec and then the
    requirement: `// node-pods R3`, `// search-regex R1, R2 and R4`,
    `// config-views R1-R3` (a range names each one). A file header does
    not count: name the requirement on the test that checks it;
  - a marker in the spec: `_Test: manual — <how it is checked>._` (docs,
    a run on a cluster), `_Test: ci — <the CI job>._` (CI behaviour) or
    `_Test: withdrawn — <the dated note>._`, inside the requirement's
    block (up to the next **R<n>** or heading), or anywhere with the
    requirement named: `_Test: manual (R5) — …_`.

A citation is refused when it names a spec that does not exist, a
requirement the spec does not have, or a draft spec: any `word-word R<n>`
is read as a spec reference.

Run from anywhere in the repository; -v lists where each one is covered.
"""
import pathlib
import re
import subprocess
import sys

RID = r"R\d+[a-z]?"
REQ = re.compile(rf"^\*\*({RID})\*\*")
MARK = re.compile(rf"Test: (manual|ci|withdrawn)(?: \(({RID})\))? [—-]+ \S")
STATUS = re.compile(r"^> Status: (\w+)", re.M)
TOKEN = re.compile(r"\b(R(\d+)([a-z]?))(?:\s*[-–]\s*R(\d+))?\b")
COMMENT = re.compile(r"^\s*(//|#)")
CODE = re.compile(r"^\s*(//|#)\s*[\w.]+\(.*\)\s*;?\s*$|\"\)")  # commented-out code
ANCHOR_GO = re.compile(r"^\s*(func Test\w*\(|.*\bt\.Run\()")
SPECREF = re.compile(r"(?<![\w/.-])(?:specs/)?([a-z0-9]+(?:-[a-z0-9]+)+)(?=\s+R\d)")


def root():
    out = subprocess.run(["git", "rev-parse", "--show-toplevel"], capture_output=True, text=True)
    return pathlib.Path(out.stdout.strip() or ".")


def requirements(path):
    """({id: marker kind or None}, [marker ids the spec lacks]), outside code fences."""
    reqs, marks, cur, fenced = {}, {}, None, False
    for line in path.read_text().splitlines():
        if line.lstrip().startswith("```"):
            fenced = not fenced
            continue
        if fenced:
            continue
        m = REQ.match(line)
        if m:
            cur = m.group(1)
            reqs.setdefault(cur, None)
        elif line.startswith("#"):
            cur = None
        for mk in MARK.finditer(line):
            target = mk.group(2) or cur
            # an explicit (R<n>) wins over a block's marker
            if target and (mk.group(2) or target not in marks):
                marks[target] = mk.group(1)
    for k in reqs:
        reqs[k] = marks.get(k)
    return reqs, sorted(set(marks) - set(reqs))


def test_files(top):
    files = [p for d in ("cmd", "pkg", "hack") for p in (top / d).rglob("*_test.go")]
    hooks = top / "scripts" / "checks" / "test-checks.sh"
    return files + ([hooks] if hooks.exists() else [])


def cited_lines(path):
    """(line number, text) of the comment lines in the block right above an anchor."""
    lines = path.read_text(errors="replace").splitlines()
    go = path.suffix == ".go"
    out = []
    for i, line in enumerate(lines):
        anchor = ANCHOR_GO.match(line) if go else (line.strip() and not COMMENT.match(line))
        if not anchor:
            continue
        j = i - 1
        while j >= 0 and COMMENT.match(lines[j]):
            out.append((j + 1, lines[j]))
            j -= 1
    return out


def ids(tok):
    """The requirement ids a token names: R3, R4a, or each of R1-R4."""
    if tok.group(4):
        return [f"R{n}" for n in range(int(tok.group(2)), int(tok.group(4)) + 1)]
    return [tok.group(1)]


def citations(top, specs, status, errors):
    """{(spec, id): [file:line]} from the cited comment lines."""
    names = sorted(status, key=len, reverse=True)
    known = re.compile(r"(?<![\w/.-])(?:specs/)?(" + "|".join(map(re.escape, names)) + r")(?![\w-])")
    cited = {}
    for f in test_files(top):
        for n, line in cited_lines(f):
            if CODE.search(line):
                continue
            where = f"{f.relative_to(top)}:{n}"
            refs = {m.start(1): m.group(1) for m in known.finditer(line)}
            for m in SPECREF.finditer(line):
                refs.setdefault(m.start(1), m.group(1))
            starts = sorted(refs)
            for k, pos in enumerate(starts):
                name = refs[pos]
                # an unknown name ends the previous spec's requirements too
                end = starts[k + 1] if k + 1 < len(starts) else len(line)
                if name not in status:
                    errors.append(f"{where}: no spec named {name}")
                    continue
                if status[name] == "draft":
                    errors.append(f"{where}: {name} is a draft spec; cite it once it is active")
                    continue
                if name not in specs:
                    continue
                for tok in TOKEN.finditer(line, pos + len(name), end):
                    for rid in ids(tok):
                        if rid not in specs[name][0]:
                            errors.append(f"{where}: {name} has no {rid}")
                        else:
                            cited.setdefault((name, rid), []).append(where)
    return cited


def key(rid):
    m = re.match(r"R(\d+)([a-z]?)", rid)
    return int(m.group(1)), m.group(2)


def main():
    verbose = "-v" in sys.argv[1:]
    top = root()
    specs, status = {}, {}
    for d in sorted((top / "specs").iterdir()):
        if not d.is_dir() or d.name in ("decisions", ".templates"):
            continue
        f = d / ("bugfix.md" if d.name.startswith("fix-") else "requirements.md")
        if not f.exists():
            continue
        st = STATUS.search(f.read_text())
        status[d.name] = st.group(1) if st else "draft"
        if status[d.name] in ("active", "shipped"):
            specs[d.name] = (*requirements(f), f)
    errors = []
    for name, (_, unknown, f) in specs.items():
        for rid in unknown:
            errors.append(f"{f.relative_to(top)}: a Test marker names {rid}, which the spec does not have")
    cited = citations(top, specs, status, errors)
    missing, total = [], 0
    for name, (reqs, _, f) in specs.items():
        for rid in sorted(reqs, key=key):
            mark = reqs[rid]
            total += 1
            where = cited.get((name, rid))
            if verbose:
                print(f"{name} {rid}: {mark or ', '.join(where or ['MISSING'])}")
            if not where and not mark:
                missing.append(f"{f.relative_to(top)}: {rid}")
    if errors:
        print("Bad requirement citations or markers (spec-trace):", file=sys.stderr)
        for e in errors:
            print("  " + e, file=sys.stderr)
    if missing:
        print("Requirements no test names (spec-trace). Cite each right above the test "
              "that checks it ('// <spec> R<n>' above func Test… or t.Run), or, when no "
              "automated test can check it, add '_Test: manual — <how>._' (or ci, withdrawn) "
              "inside the requirement:", file=sys.stderr)
        for m in missing:
            print("  " + m, file=sys.stderr)
    if errors or missing:
        return 1
    print(f"spec-trace: ok ({total} requirements in {len(specs)} specs)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
