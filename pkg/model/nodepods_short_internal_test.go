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
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// screen is the whole frame as the user sees it, without styles.
func screen(u *UIModel) string { return ansi.Strip(u.View()) }

// The Pods tab keeps its key line on short windows: the node line and the
// column header give way before it, the tab line stays (node-pods R1, R8).
func TestNodePodsShortWindowKeepsTheKeyLine(t *testing.T) {
	for _, height := range []int{10, 12, 14, 16} {
		t.Run(fmt.Sprint(height), func(t *testing.T) {
			u, _ := podsUI(t, 100, fourCPU16Gi, manyPods(30)...)
			u.Update(tea.WindowSizeMsg{Width: 100, Height: height})
			out := screen(u)
			if lines := strings.Count(out, "\n") + 1; lines > height {
				t.Errorf("frame has %d lines on a %d-line window", lines, height)
			}
			if !strings.Contains(out, "tab: details") {
				t.Errorf("key line missing at height %d:\n%s", height, out)
			}
			if !strings.Contains(out, "[Pods]") {
				t.Errorf("tab line missing at height %d:\n%s", height, out)
			}
			if c := cursorRow(t, u); c == "" {
				t.Errorf("no cursor row at height %d", height)
			}
		})
	}
}

// On a tall window nothing is dropped: tab line, node line, header, key line.
func TestNodePodsTallWindowShowsEveryFixedLine(t *testing.T) {
	u, _ := podsUI(t, 100, fourCPU16Gi, manyPods(3)...)
	out := screen(u)
	for _, want := range []string{"[Pods]", "pods ·", "POD", "COST/H", "tab: details"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q missing:\n%s", want, out)
		}
	}
}

// When the DaemonSet row under the cursor goes away in the DaemonSets view,
// the cursor moves to the row now at its position (node-pods R8, R9).
func TestNodePodsCursorOnVanishedDaemonSetFallsBack(t *testing.T) {
	ds := func(name, set, cpu string) podSpec {
		return podSpec{ns: "kube-system", name: name, cpu: cpu, mem: "64Mi", ownerKind: "DaemonSet", ownerName: set}
	}
	pods := append(manyPods(20),
		ds("big-x", "big", "900m"),
		ds("small-x", "small", "10m"),
	)
	u, _ := podsUI(t, 120, fourCPU16Gi, pods...)
	press(u, "d")
	// the first DaemonSet row (big, sorted by cost)
	if !strings.Contains(cursorRow(t, u), "kube-system/big") {
		t.Fatalf("cursor not on the big DaemonSet row: %q", cursorRow(t, u))
	}
	u.cluster.DeletePod("kube-system", "big-x")
	u.Refresh()
	if got := cursorRow(t, u); !strings.Contains(got, "kube-system/small") {
		t.Errorf("cursor after the DaemonSet row went away = %q, want the row now at its position", got)
	}
}
