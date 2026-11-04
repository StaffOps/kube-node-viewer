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
"""Report CI tools pinned by version that Dependabot does not update.

    scripts/checks/tool-versions.py [--min-age DAYS]

For each pin below, reads the version in the file and the newest release
(Go module proxy, or PyPI for zizmor). A tool is behind when a newer
release is at least --min-age days old (default 14: a release that young
is not adopted yet). Also fails when the golangci-lint versions of the
Makefile and test.yaml differ. Exit 1 when anything is behind; the weekly
CI run fails, which mails the owner. Needs network; no token.
"""

import argparse
import datetime
import json
import re
import sys
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]

# name, file, regex with one group (the version), where the newest is
PINS = [
    ("golangci-lint", "Makefile", r"^GOLANGCI_LINT_VERSION=v?(\S+)", "go:github.com/golangci/golangci-lint/v2"),
    ("golangci-lint (CI)", ".github/workflows/test.yaml", r"^\s+version: v?(\S+)", "go:github.com/golangci/golangci-lint/v2"),
    ("govulncheck", "Makefile", r"^GOVULNCHECK_VERSION=v?(\S+)", "go:golang.org/x/vuln"),
    ("go-licenses", "Makefile", r"^GO_LICENSES_VERSION=v?(\S+)", "go:github.com/google/go-licenses/v2"),
    ("actionlint", ".github/workflows/test.yaml", r"actionlint/cmd/actionlint@v?(\S+)", "go:github.com/rhysd/actionlint"),
    ("zizmor", ".github/workflows/test.yaml", r"zizmor==(\S+)", "pypi:zizmor"),
    ("gitleaks", ".github/actions/gitleaks/action.yml", r"GITLEAKS_VERSION: v?(\S+)", "go:github.com/zricethezav/gitleaks/v8"),
    ("cosign", ".github/workflows/release.yaml", r"cosign-release: v?(\S+)", "go:github.com/sigstore/cosign/v3"),
    ("syft", ".github/workflows/release.yaml", r"syft-version: v?(\S+)", "go:github.com/anchore/syft"),
]


def get_json(url):
    req = urllib.request.Request(url, headers={"User-Agent": "kube-node-viewer-tool-versions"})
    with urllib.request.urlopen(req, timeout=30) as r:
        return json.load(r)


def parse_time(s):
    return datetime.datetime.fromisoformat(s.replace("Z", "+00:00"))


def newest(source):
    """The newest release: (version without v, release time)."""
    kind, name = source.split(":", 1)
    if kind == "go":
        info = get_json(f"https://proxy.golang.org/{name}/@latest")
        return info["Version"].lstrip("v"), parse_time(info["Time"])
    info = get_json(f"https://pypi.org/pypi/{name}/json")
    version = info["info"]["version"]
    files = info["releases"].get(version) or [{"upload_time_iso_8601": "1970-01-01T00:00:00Z"}]
    return version, parse_time(files[0]["upload_time_iso_8601"])


def key(version):
    return tuple(int(p) if p.isdigit() else 0 for p in re.split(r"[.+-]", version))


def pinned(file, pattern):
    for line in (ROOT / file).read_text().splitlines():
        m = re.search(pattern, line)
        if m:
            return m.group(1)
    raise SystemExit(f"{file}: no line matches {pattern}")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--min-age", type=int, default=14, metavar="DAYS")
    args = ap.parse_args()
    now = datetime.datetime.now(datetime.timezone.utc)
    behind = 0
    seen = {}
    for name, file, pattern, source in PINS:
        have = pinned(file, pattern)
        if source not in seen:
            seen[source] = newest(source)
        latest, when = seen[source]
        age = (now - when).days
        if key(latest) > key(have) and age >= args.min_age:
            behind += 1
            print(f"BEHIND  {name}: {have} in {file}; {latest} released {age} days ago")
        elif key(latest) > key(have):
            print(f"ok      {name}: {have} ({latest} is {age} days old, under {args.min_age})")
        else:
            print(f"ok      {name}: {have}")
    lint = {pinned(f, p) for n, f, p, _ in PINS if n.startswith("golangci-lint")}
    if len(lint) > 1:
        behind += 1
        print(f"DIFFER  golangci-lint: Makefile and test.yaml pin {sorted(lint)}")
    return 1 if behind else 0


if __name__ == "__main__":
    sys.exit(main())
