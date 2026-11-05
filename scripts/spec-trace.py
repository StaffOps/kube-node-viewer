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

Every **R<n>** in specs/<spec>/requirements.md (bugfix.md for fix-*) whose
status is active or shipped needs one of:

  - a test citing it: a comment line in a test file (*_test.go, or
    scripts/checks/test-checks.sh) that names the spec and then the
    requirement, e.g. `// node-pods R3` or `// search-regex R1, R2 and R4`
    (the spec's folder name; a range R1-R4 or R1–R4 names R1 to R4);
  - a marker inside the requirement, up to the next **R<n>** or heading:
    `Test: manual — <reason>` when no automated test can check it (docs,
    CI behaviour, a run on a cluster), or `Test: withdrawn — <reason>`
    for a requirement a dated note withdrew.

Run from anywhere in the repository; -v lists where each one is covered.
"""
import pathlib
import re
import subprocess
import sys

REQ = re.compile(r"^\*\*R(\d+)\*\*")
MARK = re.compile(r"Test: (manual|withdrawn) [—-]+ \S")
STATUS = re.compile(r"^> Status: (\w+)", re.M)
TOKEN = re.compile(r"\bR(\d+)(?:\s*[-–]\s*R(\d+))?\b")
COMMENT = re.compile(r"^\s*(//|#)")


def root():
    out = subprocess.run(["git", "rev-parse", "--show-toplevel"], capture_output=True, text=True)
    return pathlib.Path(out.stdout.strip() or ".")


def requirements(path):
    """{n: marked} for each **R<n>** of the file; marked is the marker kind or None."""
    reqs, cur = {}, None
    for line in path.read_text().splitlines():
        m = REQ.match(line)
        if m:
            cur = int(m.group(1))
            reqs.setdefault(cur, None)
        elif line.startswith("#"):
            cur = None
        if cur is not None:
            mk = MARK.search(line)
            if mk:
                reqs[cur] = mk.group(1)
    return reqs


def test_files(top):
    files = [p for d in ("cmd", "pkg", "hack") for p in (top / d).rglob("*_test.go")]
    hooks = top / "scripts" / "checks" / "test-checks.sh"
    return files + ([hooks] if hooks.exists() else [])


def citations(top, specs):
    """{(spec, n): [file:line]} from comment lines naming a spec, then R<n>."""
    names = sorted(specs, key=len, reverse=True)  # fix-x-y before fix-x
    pattern = re.compile(r"(?<![\w/-])(?:specs/)?(" + "|".join(map(re.escape, names)) + r")(?![\w-])")
    cited = {}
    for f in test_files(top):
        for i, line in enumerate(f.read_text(errors="replace").splitlines(), 1):
            if not COMMENT.match(line):
                continue
            hits = list(pattern.finditer(line))
            for j, m in enumerate(hits):
                end = hits[j + 1].start() if j + 1 < len(hits) else len(line)
                for t in TOKEN.finditer(line, m.end(), end):
                    lo = int(t.group(1))
                    for n in range(lo, int(t.group(2) or lo) + 1):
                        cited.setdefault((m.group(1), n), []).append(f"{f.relative_to(top)}:{i}")
    return cited


def main():
    verbose = "-v" in sys.argv[1:]
    top = root()
    specs = {}
    for d in sorted((top / "specs").iterdir()):
        if not d.is_dir() or d.name in ("decisions", ".templates"):
            continue
        f = d / ("bugfix.md" if d.name.startswith("fix-") else "requirements.md")
        if not f.exists():
            continue
        st = STATUS.search(f.read_text())
        if st and st.group(1) in ("active", "shipped"):
            specs[d.name] = f
    cited = citations(top, specs)
    missing, total = [], 0
    for name, f in specs.items():
        for n, mark in sorted(requirements(f).items()):
            total += 1
            where = cited.get((name, n))
            if verbose:
                print(f"{name} R{n}: {mark or ', '.join(where or ['MISSING'])}")
            if not where and not mark:
                missing.append(f"{f.relative_to(top)}: R{n}")
    if missing:
        print("Requirements no test names (spec-trace). Cite each in a test comment as "
              "'// <spec> R<n>' (the spec's folder name), or, when no automated "
              "test can check it, add 'Test: manual — <reason>' inside the requirement:",
              file=sys.stderr)
        for m in missing:
            print("  " + m, file=sys.stderr)
        return 1
    print(f"spec-trace: ok ({total} requirements in {len(specs)} specs)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
