/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package model

import (
	"os"
	"strings"
	"testing"
)

// Tests for specs/search-regex: the matcher (R1-R3) and the footer, header
// and help wording (R3, R4). Node selection is in search_test.go.

// search-regex R1, R2, R3: regex, literal text, and an invalid regex read as text.
func TestSearchMatcher(t *testing.T) {
	cases := []struct {
		text, value string
		want, re    bool // re: a valid regex (words or compiled)
		invalid     bool
	}{
		{"batch", "workload-BATCH-1", true, false, false},
		{"203.0.113.5", "203.0.113.5", true, false, false},
		{"203.0.113.5", "ip-203-0-113-5.example.com", false, false, false}, // `.` stays literal (D1)
		{"m5.large", "m5.large", true, false, false},
		{"search|batch", "batch", true, true, false},
		{"search|batch", "web", false, true, false},
		{"^node-1$", "node-1", true, true, false},
		{"^node-1$", "node-12", false, true, false},
		{"BATCH|x", "batch", true, true, false},
		{"(?-i)BATCH|x", "batch", false, true, false},
		{"atc|x", "batch", true, true, false},
		{"m7g\\.(large|xl)", "m7g.xlarge", true, true, false},
		{"m7g\\.(large|xl)", "m7g.medium", false, true, false},
		{"203.0.113.5|zz", "ip-203-0-113-5.example.com", true, true, false}, // in a regex `.` is any character
		{"search | batch", "batch", false, true, false},                     // spaces inside count
		{"a|", "zzz", true, true, false},                                    // an empty alternative matches everything
		{"a||b", "zzz", true, true, false},                                  // (design, Failure modes)
		{"ſ|zz", "s", true, true, false},                                    // non-ASCII: the regex, whose (?i) folds ſ to s
		{"a(b", "xa(by", true, false, true},
		{"a(b", "ab", false, false, true},
		{"[", "a[b", true, false, true},
		{"*a", "x*ay", true, false, true},
		{"x{2", "x{2", true, true, false}, // RE2 reads an unfinished { as a literal
		{"trail\\", "trail\\", true, false, true},
		{"  batch  ", "batch", true, false, false},
	}
	for _, c := range cases {
		m := newSearchMatcher(c.text)
		if got := m.match(c.value); got != c.want {
			t.Errorf("%q on %q: got %v, want %v", c.text, c.value, got, c.want)
		}
		if m.hasRegex != (c.re || c.invalid) || m.invalid != c.invalid {
			t.Errorf("%q: regex characters %v invalid %v, want %v %v", c.text, m.hasRegex, m.invalid, c.re || c.invalid, c.invalid)
		}
	}
	if m := newSearchMatcher("search|batch"); m.re != nil || len(m.words) != 2 {
		t.Errorf("words joined by | are not matched as words: %+v", m)
	}
	if m := newSearchMatcher("203.0.113.5|zz"); m.re == nil {
		t.Error("a pattern with . is matched as words")
	}
	for _, text := range []string{"", "   "} {
		if !newSearchMatcher(text).empty() {
			t.Errorf("%q: not empty", text)
		}
	}
}

// search-regex R3 and R4: the footer and header say an invalid pattern is
// read as text; the help says the search takes a regex.
func TestSearchFooterHeaderAndHelp(t *testing.T) {
	u := footerUI(t, 200, 40)
	u.View()
	press(u, "/", "n")
	if got := u.helpText(); got != "type to search (regex) • enter: keep • esc: clear" {
		t.Errorf("plain text footer: %q", got)
	}
	press(u, "o", "d", "e", "|", "x")
	if got := u.helpText(); got != "type to search (regex) • enter: keep • esc: clear" {
		t.Errorf("valid pattern footer: %q", got)
	}
	press(u, "backspace", "backspace", "(")
	if got := u.helpText(); got != "type to search (regex) • invalid regex: as text • enter: keep • esc: clear" {
		t.Errorf("invalid pattern footer: %q", got)
	}
	press(u, "enter")
	if out := screen(u); !strings.Contains(out, "/node( (as text)") {
		t.Errorf("header after an invalid search lacks the mark:\n%s", out)
	}

	u = footerUI(t, 200, 40)
	u.View()
	press(u, "/", "n", "o", "d", "e", "|", "x", "enter")
	if out := screen(u); !strings.Contains(out, "/node|x") || strings.Contains(out, "(as text)") {
		t.Errorf("header after a valid search:\n%s", out)
	}

	found := false
	for _, k := range helpKeys {
		if k[0] == "/" {
			found = k[1] == "search name, IP, instance type, labels (regex, any case); esc clears"
		}
	}
	if !found {
		t.Error("help line for / does not mention regex")
	}
}

// search-regex R4: the README says the search takes a regular expression:
// the key table with an a|b example, the Search section with RE2 and the
// escaped dot, and the Export section on how the search is recorded.
func TestSearchREADMESaysRegex(t *testing.T) {
	b, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	readme := string(b)
	section := func(title string) string {
		i := strings.Index(readme, "\n"+title+"\n")
		if i < 0 {
			t.Fatalf("README has no %q section", title)
		}
		rest := readme[i+len(title)+2:]
		if j := strings.Index(rest, "\n#"); j >= 0 {
			rest = rest[:j]
		}
		return rest
	}
	row := ""
	for _, line := range strings.Split(readme, "\n") {
		if strings.HasPrefix(line, "| `/` |") {
			row = line
		}
	}
	if !strings.Contains(row, "regular expression") || !strings.Contains(row, `search\|batch`) {
		t.Errorf("README key row for / lacks the regex wording or its a|b example: %q", row)
	}
	search := section("### Search (`/`)")
	for _, want := range []string{"regular expression", "RE2", `\.`, "invalid regex: as text"} {
		if !strings.Contains(search, want) {
			t.Errorf("README Search section lacks %q", want)
		}
	}
	if !strings.Contains(section("### Export (`e`)"), "regular expression") {
		t.Error("README Export section does not say the search may be a regular expression")
	}
}
