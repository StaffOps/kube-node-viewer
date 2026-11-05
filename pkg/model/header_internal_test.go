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
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// firstHeaderLine is the first line inside the header box.
func firstHeaderLine(t *testing.T, view string) string {
	t.Helper()
	lines := strings.Split(view, "\n")
	if len(lines) < 2 || !strings.HasPrefix(lines[0], "╭") {
		t.Fatalf("no header box:\n%s", view)
	}
	return strings.TrimSpace(strings.Trim(lines[1], "│"))
}

// compact-header-no-url R1
func TestCompactHeaderHasNoServerURL(t *testing.T) {
	const server = "https://0A1B2C3D4E5F.gr7.us-east-1.eks.amazonaws.com"
	for _, tc := range []struct {
		name   string
		height int
		ctrlE  bool
		region string
		want   string
	}{
		{"ctrl+e, region known", 40, true, "us-east-1", "ctx-a · us-east-1 · 2 of 2 shown · 0 pods · $0.000/month (global cluster)"},
		{"by height, region known", 20, false, "us-east-1", "ctx-a · us-east-1 · 2 of 2 shown · 0 pods · $0.000/month (global cluster)"},
		{"ctrl+e, no region", 40, true, "", "ctx-a · 2 of 2 shown · 0 pods · $0.000/month (global cluster)"},
		{"by height, no region", 20, false, "", "ctx-a · 2 of 2 shown · 0 pods · $0.000/month (global cluster)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// capacityUI's nodes carry no region label, so "no region" has none at all
			u := capacityUI(t, 2)
			u.SetClusterInfo(ClusterInfo{Context: "ctx-a", Server: server, Region: tc.region})
			u.Update(tea.WindowSizeMsg{Width: 200, Height: tc.height})
			if tc.ctrlE {
				u.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
			}
			if u.headerSize() != headerCompact {
				t.Fatalf("header size %v, want compact", u.headerSize())
			}
			line := firstHeaderLine(t, u.View())
			// the counts and cost are the full panel's Nodes row, unchanged
			if line != tc.want {
				t.Errorf("first line %q, want %q", line, tc.want)
			}
			for _, part := range []string{server, "0A1B2C3D4E5F.gr7.us-east-1.eks.amazonaws.com", "https://"} {
				if strings.Contains(line, part) {
					t.Errorf("first line %q contains %q", line, part)
				}
			}
		})
	}
}

// compact-header-no-url R2
func TestFullHeaderKeepsServerURL(t *testing.T) {
	u := capacityUI(t, 2)
	u.SetClusterInfo(ClusterInfo{Context: "ctx-a", Server: "https://a.example", Region: "us-east-1"})
	u.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	if u.headerSize() != headerFull {
		t.Fatalf("header size %v, want full", u.headerSize())
	}
	if full := u.View(); !strings.Contains(full, "Cluster:") || !strings.Contains(full, "https://a.example") {
		t.Errorf("full panel lost the server URL:\n%s", full)
	}
	u.Update(tea.WindowSizeMsg{Width: 200, Height: 10})
	if first := strings.SplitN(u.View(), "\n", 2)[0]; strings.Contains(first, "a.example") {
		t.Errorf("one-line header shows the server URL: %q", first)
	}
}
