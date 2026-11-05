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
"""Render a file of ANSI-coloured terminal text as a standalone HTML page.

Usage: ansi2html.py <in.ansi> <out.html> [caption-file]
"""
import html, re, sys

BASIC = ["#1c1c1c", "#d75f5f", "#87af5f", "#d7af5f", "#5f87d7", "#af87d7", "#5fafaf", "#d0d0d0",
         "#626262", "#ff8787", "#afd787", "#ffd787", "#87afff", "#d7afff", "#87d7d7", "#ffffff"]
FG, BG = "#d0d0d0", "#1e1e1e"


def x256(n):
    if n < 16:
        return BASIC[n]
    if n < 232:
        n -= 16
        steps = [0, 95, 135, 175, 215, 255]
        return "#%02x%02x%02x" % (steps[n // 36], steps[(n // 6) % 6], steps[n % 6])
    v = 8 + (n - 232) * 10
    return "#%02x%02x%02x" % (v, v, v)


def render(text):
    state = {"fg": None, "bg": None, "bold": False, "faint": False}
    out = []
    for part in re.split(r"(\x1b\[[0-9;]*m)", text):
        m = re.fullmatch(r"\x1b\[([0-9;]*)m", part)
        if m:
            codes = [int(c) if c else 0 for c in m.group(1).split(";")] if m.group(1) else [0]
            i = 0
            while i < len(codes):
                c = codes[i]
                if c == 0:
                    state = {"fg": None, "bg": None, "bold": False, "faint": False}
                elif c == 1:
                    state["bold"] = True
                elif c == 2:
                    state["faint"] = True
                elif c == 22:
                    state["bold"] = state["faint"] = False
                elif c == 39:
                    state["fg"] = None
                elif c == 49:
                    state["bg"] = None
                elif 30 <= c <= 37:
                    state["fg"] = BASIC[c - 30]
                elif 90 <= c <= 97:
                    state["fg"] = BASIC[c - 90 + 8]
                elif 40 <= c <= 47:
                    state["bg"] = BASIC[c - 40]
                elif c in (38, 48) and i + 1 < len(codes):
                    key = "fg" if c == 38 else "bg"
                    if codes[i + 1] == 2 and i + 4 < len(codes):
                        state[key] = "#%02x%02x%02x" % tuple(codes[i + 2:i + 5])
                        i += 4
                    elif codes[i + 1] == 5 and i + 2 < len(codes):
                        state[key] = x256(codes[i + 2])
                        i += 2
                i += 1
            continue
        if not part:
            continue
        style = []
        if state["fg"]:
            style.append("color:" + state["fg"])
        if state["bg"]:
            style.append("background:" + state["bg"])
        if state["bold"]:
            style.append("font-weight:bold")
        if state["faint"]:
            style.append("opacity:.6")
        esc = html.escape(part)
        out.append('<span style="%s">%s</span>' % (";".join(style), esc) if style else esc)
    return "".join(out)


src, dst = sys.argv[1], sys.argv[2]
body = render(open(src, encoding="utf-8").read())
caption = ""
if len(sys.argv) > 3:
    caption = '<div class="cap">%s</div>' % html.escape(open(sys.argv[3], encoding="utf-8").read().strip())
open(dst, "w", encoding="utf-8").write("""<!doctype html><meta charset="utf-8">
<style>
html,body{margin:0;background:%s}
pre{margin:0;padding:14px 20px 18px;color:%s;font:13px/1 Menlo,"SF Mono",monospace;white-space:pre}
.cap{padding:9px 20px;background:#2a2a40;color:#ffd75f;font:bold 14px Menlo,"SF Mono",monospace}
</style>%s<pre>%s</pre>""" % (BG, FG, caption, body))
