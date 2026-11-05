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

// Tests of the owner change of 2026-11-03 to node-pods, written against
// specs/node-pods/requirements.md: the all-pods view (R6), the keys (R8),
// the DaemonSets view (R9), the sort of its rows (R7) and the DaemonSets
// line of the node list (R10).

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// tenCPU10Gi makes the percents round: 1 CPU is 10%, 1Gi is 10%.
var tenCPU10Gi = v1.ResourceList{
	v1.ResourceCPU: resource.MustParse("10"), v1.ResourceMemory: resource.MustParse("10Gi"), v1.ResourcePods: resource.MustParse("110"),
}

// dsPod is a pod of the DaemonSet set (no name recorded when set is "").
func dsPod(ns, pod, set, cpu, mem string) podSpec {
	return podSpec{ns: ns, name: pod, cpu: cpu, mem: mem, ownerKind: "DaemonSet", ownerName: set}
}

// totalLine is the fixed Total line of the DaemonSets view, without the box.
func totalLine(t *testing.T, u *UIModel) string {
	t.Helper()
	for _, l := range box(u) {
		if strings.Contains(l, "Total (") {
			return strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(l), "│")), "│")
		}
	}
	t.Fatalf("no Total line in:\n%s", shown(u))
	return ""
}

// node-pods R6: every pod is its own row, DaemonSet pods marked ds, none grouped.
func TestAllPodsViewListsEveryPodAsARow(t *testing.T) {
	u, _ := podsUI(t, 160, tenCPU10Gi,
		podSpec{name: "web", cpu: "3"},
		dsPod("kube-system", "proxy-x", "proxy", "500m", "128Mi"),
		dsPod("kube-system", "proxy-y", "proxy", "500m", "128Mi"), // a second pod of the same set on the node
		dsPod("kube-system", "orphan", "", "100m", ""),
		podSpec{ns: "kube-system", name: "static", cpu: "200m", ownerKind: "Node", ownerName: podsNodeName},
	)
	got := names(u)
	want := []string{"default/web", "kube-system/proxy-x ds", "kube-system/proxy-y ds", "kube-system/static", "kube-system/orphan ds"}
	if !slices.Equal(got, want) {
		t.Errorf("rows %q, want %q (cost ↓, ties on namespace/name)", got, want)
	}
	for _, r := range podRowsShown(u) {
		if strings.ContainsAny(r, "▸▾├└") || strings.Contains(r, "DaemonSets") || strings.Contains(r, "Total (") {
			t.Errorf("a grouping or folding row in the all-pods view: %q", r)
		}
	}
	if line := box(u)[1]; !strings.Contains(line, "Details  [Pods]") || !strings.Contains(line, "view: all pods") {
		t.Errorf("tab line %q", line)
	}
	if !strings.Contains(box(u)[3], "POD") {
		t.Errorf("all-pods header %q, want POD", box(u)[3])
	}
}

// node-pods R6: the ds mark follows the pod's own sort keys, name included.
func TestAllPodsViewDaemonSetPodsShareTheSort(t *testing.T) {
	u, _ := podsUI(t, 160, tenCPU10Gi,
		podSpec{ns: "b", name: "app", cpu: "1"},
		dsPod("a", "agent", "agent", "2", ""),
		podSpec{ns: "c", name: "app", cpu: "3"},
	)
	press(u, "s", "s", "s") // name ↑
	if got := names(u); !slices.Equal(got, []string{"a/agent ds", "b/app", "c/app"}) {
		t.Errorf("name ↑: %q", got)
	}
	press(u, "S")
	if got := names(u); !slices.Equal(got, []string{"c/app", "b/app", "a/agent ds"}) {
		t.Errorf("name ↓: %q", got)
	}
}

// node-pods R6 with R2: on a narrow window a long DaemonSet pod name is cut with …,
// and the row is still marked ds.
func TestAllPodsViewDsMarkOnANarrowWindow(t *testing.T) {
	long := "fluent-bit-a-very-long-daemonset-pod-name-x7k2q"
	for _, width := range []int{60, 80} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			u, _ := podsUI(t, width, tenCPU10Gi, dsPod("logging", long, "fluent-bit", "100m", ""))
			row := podRowOf(t, u, "logging/")
			if !strings.Contains(row, "…") {
				t.Errorf("the name was not cut: %q", row)
			}
			if !strings.Contains(row, " ds") {
				t.Errorf("the ds mark was cut off with the name: %q", row)
			}
		})
	}
}

// node-pods R8: enter and space in the Pods tab neither close the popup nor change the
// view, the sort or the cursor, in both views.
func TestPodsTabEnterAndSpaceDoNothing(t *testing.T) {
	u, _ := podsUI(t, 160, tenCPU10Gi,
		podSpec{name: "web", cpu: "3"},
		dsPod("kube-system", "proxy-x", "proxy", "500m", ""),
		dsPod("kube-system", "dns-x", "dns", "100m", ""),
	)
	for _, view := range []string{"all pods", "daemonsets"} {
		press(u, "down")
		before, cursor := shown(u), cursorRow(t, u)
		for _, k := range []string{"enter", " "} {
			press(u, k)
			if u.panel != panelNode || u.detailsTab != detailsTabPods {
				t.Fatalf("%s: %q closed the popup or left the tab (panel %v, tab %v)", view, k, u.panel, u.detailsTab)
			}
			if shown(u) != before || cursorRow(t, u) != cursor {
				t.Errorf("%s: %q changed the tab:\n%s\nwas:\n%s", view, k, shown(u), before)
			}
		}
		press(u, "d")
	}
}

// node-pods R8: d does nothing in the Details tab; the Pods tab still opens on all pods.
func TestDInDetailsDoesNothing(t *testing.T) {
	u := newTestUI(t)
	podsNode(u, tenCPU10Gi, nil)
	addPods(u, dsPod("kube-system", "proxy-x", "proxy", "500m", ""))
	u.Update(tea.WindowSizeMsg{Width: 160, Height: 60})
	u.View()
	press(u, "enter")
	before := shown(u)
	press(u, "d")
	if u.panel != panelNode || u.detailsTab != detailsTabDetails || shown(u) != before {
		t.Fatalf("d in Details changed it (panel %v, tab %v):\n%s", u.panel, u.detailsTab, shown(u))
	}
	press(u, "tab")
	if !strings.Contains(box(u)[1], "view: all pods") {
		t.Errorf("d in Details switched the Pods view: %q", box(u)[1])
	}
}

// node-pods R8: d puts the cursor on the first row of the view it switches to, each
// time, wherever the cursor was.
func TestDPutsTheCursorOnTheFirstRow(t *testing.T) {
	u, _ := podsUI(t, 160, tenCPU10Gi, append(manyPods(5),
		dsPod("kube-system", "a-1", "a", "900m", ""),
		dsPod("kube-system", "b-1", "b", "800m", ""))...)
	press(u, "down", "down", "down")
	press(u, "d")
	if got := names(u); len(got) != 2 || !strings.Contains(cursorRow(t, u), got[0]) {
		t.Errorf("after d the cursor is on %q, rows %q", cursorRow(t, u), got)
	}
	press(u, "down", "d")
	if got := names(u); !strings.Contains(cursorRow(t, u), got[0]) {
		t.Errorf("after d back the cursor is on %q, want %q", cursorRow(t, u), got[0])
	}
}

// node-pods R9 with R3: the Total share comes from the summed requests. A DaemonSet
// with 10% CPU and none of the memory and one with 20% of the memory and no
// CPU take 20% of the node together, not 30%.
func TestDaemonSetsTotalFromSummedRequests(t *testing.T) {
	u, _ := podsUI(t, 160, tenCPU10Gi,
		podSpec{name: "web", cpu: "3", mem: "1Gi"},
		dsPod("kube-system", "cpu-x", "cpu-only", "1", ""),
		dsPod("kube-system", "mem-x", "mem-only", "", "2Gi"),
	)
	press(u, "d")
	lines := box(u)
	if !strings.Contains(lines[1], "view: daemonsets") {
		t.Errorf("tab line %q", lines[1])
	}
	if !strings.Contains(lines[3], "DAEMONSET") || !strings.Contains(lines[4], "Total (") {
		t.Fatalf("header then Total expected:\n%s", strings.Join(lines, "\n"))
	}
	total := totalLine(t, u)
	if !strings.HasPrefix(total, "Total (2 pods, 2 sets)") {
		t.Errorf("Total label %q", total)
	}
	// cpu 1.00 10%, memory 2.0Gi 20%, share 20%, cost 20% of $0.40/h
	if got := cells(total, 6); !slices.Equal(got, []string{"1.00", "10%", "2.0Gi", "20%", "20%", "$0.0800"}) {
		t.Errorf("Total cells %q, want share 20%% from the summed requests", got)
	}
	if got := cells(podRowOf(t, u, "kube-system/cpu-only"), 2); got[0] != "10%" {
		t.Errorf("cpu-only row share %q", got)
	}
	if got := cells(podRowOf(t, u, "kube-system/mem-only"), 2); got[0] != "20%" {
		t.Errorf("mem-only row share %q", got)
	}
	// the Total is not a row: the cursor rows are the two DaemonSets
	if got := names(u); !slices.Equal(got, []string{"kube-system/mem-only", "kube-system/cpu-only"}) {
		t.Errorf("DaemonSet rows %q", got)
	}
}

// node-pods R9: <m> counts distinct namespace/name pairs; <unnamed> pods count in the
// pods of the Total but not in its sets.
func TestDaemonSetsSetCountAndUnnamed(t *testing.T) {
	tests := []struct {
		name  string
		pods  []podSpec
		total string
		rows  []string
	}{
		{
			name: "same name in two namespaces, two pods of one set",
			pods: []podSpec{
				dsPod("kube-system", "agent-1", "agent", "300m", ""),
				dsPod("kube-system", "agent-2", "agent", "300m", ""),
				dsPod("logging", "agent-3", "agent", "200m", ""),
			},
			total: "Total (3 pods, 2 sets)",
			rows:  []string{"kube-system/agent", "logging/agent"},
		},
		{
			name: "unnamed pods only",
			pods: []podSpec{
				dsPod("kube-system", "x", "", "100m", ""),
				dsPod("monitoring", "y", "", "200m", ""),
			},
			total: "Total (2 pods, 0 sets)",
			rows:  []string{"<unnamed>"},
		},
		{
			name: "named and unnamed",
			pods: []podSpec{
				dsPod("kube-system", "a-1", "a", "500m", ""),
				dsPod("kube-system", "b-1", "b", "400m", ""),
				dsPod("kube-system", "x", "", "100m", ""),
				dsPod("monitoring", "y", "", "100m", ""),
			},
			total: "Total (4 pods, 2 sets)",
			rows:  []string{"kube-system/a", "kube-system/b", "<unnamed>"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, _ := podsUI(t, 160, tenCPU10Gi, append([]podSpec{{name: "web", cpu: "1"}}, tt.pods...)...)
			press(u, "d")
			if got := totalLine(t, u); !strings.HasPrefix(got, tt.total) {
				t.Errorf("Total %q, want %q", got, tt.total)
			}
			if got := names(u); !slices.Equal(got, tt.rows) {
				t.Errorf("rows %q, want %q", got, tt.rows)
			}
		})
	}
	// the unnamed row sums every unnamed pod: 300m of 10 CPU
	u, _ := podsUI(t, 160, tenCPU10Gi, tests[1].pods...)
	press(u, "d")
	if got := cells(podRowOf(t, u, "<unnamed>"), 6); got[0] != "0.30" || got[4] != "3%" {
		t.Errorf("<unnamed> row %q, want 0.30 cores and share 3%%", got)
	}
}

// node-pods R5 with R9: DaemonSets whose summed requests have neither CPU nor memory
// show no requests, on their row and on the Total, and such a row sorts last.
func TestDaemonSetsNoRequests(t *testing.T) {
	t.Run("all without requests", func(t *testing.T) {
		u, _ := podsUI(t, 160, tenCPU10Gi,
			podSpec{name: "web", cpu: "1"},
			dsPod("kube-system", "gpu-x", "gpu-plugin", "", ""),
		)
		press(u, "d")
		if got := totalLine(t, u); !strings.Contains(got, "no requests") || strings.Contains(got, "$") {
			t.Errorf("Total with no requests %q", got)
		}
		if got := podRowOf(t, u, "kube-system/gpu-plugin"); !strings.Contains(got, "no requests") {
			t.Errorf("DaemonSet row with no requests %q", got)
		}
	})
	t.Run("sorts last in every sort", func(t *testing.T) {
		u, _ := podsUI(t, 160, tenCPU10Gi,
			dsPod("kube-system", "z-1", "z", "100m", "64Mi"),
			podSpec{ns: "kube-system", name: "a-1", gpu: "1", ownerKind: "DaemonSet", ownerName: "a"},
			dsPod("kube-system", "m-1", "m", "300m", "64Mi"),
		)
		press(u, "d")
		for i := range 8 { // cost, cpu, memory, name, each both directions
			got := names(u)
			if got[len(got)-1] != "kube-system/a" {
				t.Errorf("%s: %q, the no-requests DaemonSet not last", sortShown(t, u), got)
			}
			if i%2 == 0 {
				press(u, "S")
			} else {
				press(u, "S", "s")
			}
		}
	})
}

// node-pods R9: the cursor key is the DaemonSet namespace/name and survives updates that
// reorder the rows; header and Total stay fixed while the rows scroll.
func TestDaemonSetsCursorKeptAcrossUpdates(t *testing.T) {
	u, _ := podsUI(t, 160, tenCPU10Gi,
		dsPod("kube-system", "a-1", "a", "300m", ""),
		dsPod("kube-system", "b-1", "b", "200m", ""),
		dsPod("kube-system", "c-1", "c", "100m", ""),
	)
	press(u, "d", "down") // cost ↓: a, b, c; on b
	if !strings.Contains(cursorRow(t, u), "kube-system/b") {
		t.Fatalf("cursor on %q", cursorRow(t, u))
	}
	// a new DaemonSet sorts first, and b gets a second pod that moves it up
	addPods(u, dsPod("kube-system", "big-1", "big", "2", ""), dsPod("kube-system", "b-2", "b", "250m", ""))
	if got := names(u); !slices.Equal(got, []string{"kube-system/big", "kube-system/b", "kube-system/a", "kube-system/c"}) {
		t.Errorf("rows after the update %q", got)
	}
	if got := cursorRow(t, u); !strings.Contains(got, "kube-system/b") {
		t.Errorf("cursor on %q after the update, want kube-system/b", got)
	}
	if got := totalLine(t, u); !strings.HasPrefix(got, "Total (5 pods, 4 sets)") {
		t.Errorf("Total did not follow the live node: %q", got)
	}
}

func TestDaemonSetsFixedLinesWhileScrolling(t *testing.T) {
	var pods []podSpec
	for i := range 30 {
		pods = append(pods, dsPod("kube-system", fmt.Sprintf("ds-%02d-x", i), fmt.Sprintf("ds-%02d", i), fmt.Sprintf("%dm", 10+i), ""))
	}
	u, _ := podsUI(t, 160, tenCPU10Gi, pods...)
	u.Update(tea.WindowSizeMsg{Width: 160, Height: 20})
	press(u, "d", "s", "s", "s") // name ↑
	for range 25 {
		press(u, "down")
	}
	out := screen(u)
	for _, want := range []string{"view: daemonsets", "DAEMONSET", "Total (30 pods, 30 sets)", "kube-system/ds-25"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q missing after scrolling:\n%s", want, out)
		}
	}
	if strings.Contains(out, "kube-system/ds-00 ") {
		t.Errorf("the DaemonSet rows did not scroll:\n%s", out)
	}
	press(u, "home", "up", "k", "pgup")
	if got := cursorRow(t, u); !strings.Contains(got, "kube-system/ds-00") {
		t.Errorf("the cursor left the rows: %q", got)
	}
}

// node-pods R9: with no DaemonSet pods the cursor keys do nothing.
func TestDaemonSetsEmptyViewKeys(t *testing.T) {
	u, _ := podsUI(t, 160, tenCPU10Gi, manyPods(3)...)
	press(u, "d")
	before := shown(u)
	if !strings.Contains(before, "no DaemonSets") || strings.Contains(before, "Total (") {
		t.Errorf("empty DaemonSets view:\n%s", before)
	}
	press(u, "down", "up", "j", "k", "pgdown", "pgup", "home", "enter", " ")
	if shown(u) != before || cursorRow(t, u) != "" || u.panel != panelNode {
		t.Errorf("the cursor keys did something:\n%s", shown(u))
	}
}

// node-pods R8: the view resets to all pods with its cursor on the first row, even when
// the popup closed in the DaemonSets view on a later row.
func TestDaemonSetsViewResetsOnReopen(t *testing.T) {
	u, _ := podsUI(t, 160, tenCPU10Gi,
		podSpec{name: "web", cpu: "3"},
		dsPod("kube-system", "a-1", "a", "300m", ""),
		dsPod("kube-system", "b-1", "b", "200m", ""),
	)
	first := cursorRow(t, u)
	press(u, "d", "down", "S", "esc", "enter", "tab")
	if !strings.Contains(box(u)[1], "view: all pods") || strings.Contains(shown(u), "DAEMONSET") {
		t.Errorf("reopened in the DaemonSets view:\n%s", shown(u))
	}
	if got := cursorRow(t, u); got != first {
		t.Errorf("cursor %q after reopening, want %q", got, first)
	}
}

// --- node-pods R10: the DaemonSets line of the node list ---

// footerNode adds a node with tenCPU10Gi in the zone, priced $0.40/h.
func footerNode(u *UIModel, name, zone string) {
	n := NewNode(&v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name, CreationTimestamp: metav1.Now(), Labels: map[string]string{
			v1.LabelInstanceTypeStable: "m6i.2xlarge", v1.LabelTopologyZone: zone,
		}},
		Spec:   v1.NodeSpec{ProviderID: "aws:///" + zone + "/i-" + name},
		Status: v1.NodeStatus{Allocatable: tenCPU10Gi, Capacity: tenCPU10Gi},
	})
	u.cluster.AddNode(n).Show()
	n.SetPrice(0.40)
}

// footerUI has node-a with two DaemonSets (10% CPU, 20% memory) and node-b
// with none, sorted by name so node-a is first.
func footerUI(t *testing.T, width, height int) *UIModel {
	t.Helper()
	u := newTestUI(t)
	footerNode(u, "node-a", "us-east-1a")
	footerNode(u, "node-b", "us-east-1b")
	v := DefaultViewState()
	v.Sort = SortSpec{Field: FieldName}
	u.SetView(v)
	addPods(u,
		podSpec{name: "web", cpu: "3", mem: "1Gi", node: "node-a"},
		dsPod("kube-system", "cpu-x", "cpu-only", "1", "").onNode("node-a"),
		dsPod("kube-system", "mem-x", "mem-only", "", "2Gi").onNode("node-a"),
		podSpec{name: "api", cpu: "2", node: "node-b"},
	)
	u.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return u
}

func (s podSpec) onNode(node string) podSpec { s.node = node; return s }

// dsLines are the lines of the frame that carry the DaemonSets line.
func dsLines(u *UIModel) []string {
	var out []string
	for _, l := range strings.Split(screen(u), "\n") {
		// the footer line, not the Details tab's DaemonSets section in a popup
		if strings.Contains(l, " · DaemonSets:") || strings.Contains(l, " · no DaemonSets") {
			out = append(out, strings.TrimSpace(l))
		}
	}
	return out
}

func TestDaemonSetLineOfTheCursorNode(t *testing.T) {
	u := footerUI(t, 160, 40)
	got := dsLines(u)
	want := "node-a · DaemonSets: 20% of the node (cpu 1000m 10% · mem 2.0Gi 20%)"
	if !slices.Equal(got, []string{want}) {
		t.Fatalf("DaemonSets line %q, want %q:\n%s", got, want, screen(u))
	}
	if strings.Contains(got[0], "$") {
		t.Errorf("the DaemonSets line shows a cost: %q", got[0])
	}
	// above the key hints
	lines := strings.Split(screen(u), "\n")
	at := slices.IndexFunc(lines, func(l string) bool { return strings.Contains(l, want) })
	hints := slices.IndexFunc(lines, func(l string) bool { return strings.Contains(l, "f: filters") || strings.Contains(l, "?:") })
	if hints >= 0 && at > hints {
		t.Errorf("the DaemonSets line is below the key hints:\n%s", screen(u))
	}

	press(u, "down")
	if got := dsLines(u); !slices.Equal(got, []string{"node-b · no DaemonSets"}) {
		t.Errorf("node without DaemonSets: %q", got)
	}
	press(u, "up")
	if got := dsLines(u); !slices.Equal(got, []string{want}) {
		t.Errorf("back on node-a: %q", got)
	}
}

// node-pods R10: the line follows the live node.
func TestDaemonSetLineFollowsTheLiveNode(t *testing.T) {
	u := footerUI(t, 160, 40)
	addPods(u, dsPod("kube-system", "more-x", "more", "2", "").onNode("node-a"))
	if got := dsLines(u); !slices.Equal(got, []string{"node-a · DaemonSets: 30% of the node (cpu 3000m 30% · mem 2.0Gi 20%)"}) {
		t.Errorf("after a DaemonSet pod was bound: %q", got)
	}
}

func TestDaemonSetLineDirect(t *testing.T) {
	u := newTestUI(t)
	n := podsNode(u, tenCPU10Gi, nil)
	if got := DaemonSetLine(n); got != podsNodeName+" · no DaemonSets" {
		t.Errorf("no pods: %q", got)
	}
	addPods(u, podSpec{name: "web", cpu: "1"}, podSpec{ns: "kube-system", name: "static", cpu: "1", ownerKind: "Node", ownerName: podsNodeName})
	if got := DaemonSetLine(n); got != podsNodeName+" · no DaemonSets" {
		t.Errorf("only non-DaemonSet pods: %q", got)
	}
	addPods(u, dsPod("kube-system", "x", "", "500m", "1Gi"))
	if got, want := DaemonSetLine(n), podsNodeName+" · DaemonSets: 10% of the node (cpu 500m 5% · mem 1.0Gi 10%)"; got != want {
		t.Errorf("an unnamed DaemonSet pod counts: %q, want %q", got, want)
	}

	// R3: a percent whose allocatable is missing shows - and is left out of
	// the share; with both missing the share is -
	noMem := v1.ResourceList{v1.ResourceCPU: resource.MustParse("10"), v1.ResourcePods: resource.MustParse("110")}
	u2 := newTestUI(t)
	n2 := podsNode(u2, noMem, nil)
	addPods(u2, dsPod("kube-system", "x", "x", "1", "4Gi"))
	if got, want := DaemonSetLine(n2), podsNodeName+" · DaemonSets: 10% of the node (cpu 1000m 10% · mem 4.0Gi -)"; got != want {
		t.Errorf("no memory allocatable: %q, want %q", got, want)
	}
	u3 := newTestUI(t)
	n3 := podsNode(u3, v1.ResourceList{v1.ResourcePods: resource.MustParse("110")}, nil)
	addPods(u3, dsPod("kube-system", "x", "x", "1", "4Gi"))
	if got, want := DaemonSetLine(n3), podsNodeName+" · DaemonSets: - of the node (cpu 1000m - · mem 4.0Gi -)"; got != want {
		t.Errorf("no allocatable: %q, want %q", got, want)
	}
}

// node-pods R10: a cursor key whose node is gone gives a blank line, not a stale one.
func TestDaemonSetFooterNodeGone(t *testing.T) {
	u := footerUI(t, 160, 40)
	u.View()
	u.cursorKey = "n:no-such-node"
	if got := u.daemonSetFooter(); got != "" {
		t.Errorf("footer for a node that is gone: %q", got)
	}
	u.cursorKey = "n:node-a"
	if got := u.daemonSetFooter(); !strings.HasPrefix(got, "node-a · DaemonSets:") {
		t.Errorf("footer for node-a: %q", got)
	}
}

// node-pods R10: absent on a group heading, while a panel, the search or the command
// line is open, and on Recommendations.
func TestDaemonSetLineAbsent(t *testing.T) {
	want := "node-a · DaemonSets: 20% of the node (cpu 1000m 10% · mem 2.0Gi 20%)"
	t.Run("group heading", func(t *testing.T) {
		u := footerUI(t, 160, 40)
		v := u.view
		v.SetGroupBy(FieldZone)
		u.SetView(v)
		u.View()
		press(u, "home")
		if got := dsLines(u); len(got) != 0 {
			t.Errorf("on a heading: %q\n%s", got, screen(u))
		}
		press(u, "down")
		if got := dsLines(u); !slices.Equal(got, []string{want}) {
			t.Errorf("on node-a under its heading: %q\n%s", got, screen(u))
		}
	})
	t.Run("panel", func(t *testing.T) {
		for _, key := range []string{"f", "g", "s", "o", "?", "enter"} {
			u := footerUI(t, 160, 40)
			u.View()
			press(u, key)
			if u.panel == panelNone {
				t.Fatalf("%q opened no panel", key)
			}
			if got := dsLines(u); len(got) != 0 {
				t.Errorf("with panel %q open: %q", key, got)
			}
			press(u, "esc")
			if u.panel == panelNone {
				if got := dsLines(u); !slices.Equal(got, []string{want}) {
					t.Errorf("after closing %q: %q", key, got)
				}
			}
		}
	})
	t.Run("search", func(t *testing.T) {
		u := footerUI(t, 160, 40)
		u.View()
		press(u, "/")
		if got := dsLines(u); len(got) != 0 {
			t.Errorf("while searching: %q", got)
		}
		press(u, "n", "o", "d", "e", "-", "a")
		if got := dsLines(u); len(got) != 0 {
			t.Errorf("while typing a search: %q", got)
		}
		press(u, "enter")
		if got := dsLines(u); !slices.Equal(got, []string{want}) {
			t.Errorf("after the search was kept: %q\n%s", got, screen(u))
		}
	})
	t.Run("command line", func(t *testing.T) {
		u := footerUI(t, 160, 40)
		u.View()
		press(u, ":")
		if !strings.Contains(screen(u), ":█") {
			t.Fatalf("command line not open:\n%s", screen(u))
		}
		if got := dsLines(u); len(got) != 0 {
			t.Errorf("in the command line: %q", got)
		}
		press(u, "esc")
		if got := dsLines(u); !slices.Equal(got, []string{want}) {
			t.Errorf("after the command line closed: %q", got)
		}
	})
	t.Run("recommendations", func(t *testing.T) {
		u := footerUI(t, 160, 40)
		u.advisor = fakeAdvisor{}
		u.View()
		pressAll(u, "tab")
		if !u.onRecommendations() {
			t.Fatal("tab did not switch to Recommendations")
		}
		if got := dsLines(u); len(got) != 0 {
			t.Errorf("on Recommendations: %q", got)
		}
	})
}

// node-pods R10: on a narrow window the line is cut with …; no line runs past the window.
func TestDaemonSetLineNarrowWindow(t *testing.T) {
	for _, width := range []int{20, 30, 45} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			u := footerUI(t, width, 40)
			out := screen(u)
			for _, l := range strings.Split(out, "\n") {
				if w := ansi.StringWidth(l); w > width {
					t.Errorf("line %d wide on a %d-column window: %q", w, width, l)
				}
			}
			got := dsLines(u)
			var cut bool
			for _, l := range strings.Split(out, "\n") {
				if strings.Contains(l, "node-a · ") && strings.HasSuffix(strings.TrimRight(l, " "), "…") {
					cut = true
				}
			}
			if !cut {
				t.Errorf("no DaemonSets line cut with … (lines %q):\n%s", got, out)
			}
		})
	}
}

// node-pods R10: the frame never grows past the window, and the node list gives up at
// most one row to the DaemonSets line: with one-line nodes and more nodes
// than fit, the frame fills the window exactly, so only the reserved line
// is taken from the list.
func TestDaemonSetLineFrameHeights(t *testing.T) {
	for height := 8; height <= 30; height++ {
		t.Run(fmt.Sprint(height), func(t *testing.T) {
			u := footerUI(t, 120, height)
			for i := range 40 {
				footerNode(u, fmt.Sprintf("node-x%02d", i), "us-east-1c")
			}
			u.Refresh()
			out := screen(u)
			lines := strings.Split(out, "\n")
			if len(lines) > height {
				t.Fatalf("frame %d lines on a %d-line window:\n%s", len(lines), height, out)
			}
			at := slices.IndexFunc(lines, func(l string) bool { return strings.Contains(l, "node-a · DaemonSets:") })
			if at < 0 {
				// on a window too short to keep three list lines besides
				// it, the line is dropped and the selected node stays on
				// screen (R10, corrected 2026-11-03)
				if height >= 12 {
					t.Fatalf("no DaemonSets line on a %d-line window:\n%s", height, out)
				}
				if !slices.ContainsFunc(lines, func(l string) bool { return strings.Contains(l, "node-a ") }) {
					t.Fatalf("DaemonSets line dropped and the selected node off screen:\n%s", out)
				}
				return
			}
			// the list rows: every node line between the summary and the
			// DaemonSets line
			first := slices.IndexFunc(lines, func(l string) bool { return strings.Contains(l, "node-a ") && !strings.Contains(l, "DaemonSets") })
			if first < 0 || first > at {
				t.Fatalf("no node row above the DaemonSets line:\n%s", out)
			}
			// without the DaemonSets line the list would end two lines above
			// the bottom of the window; with it, one row is gone: the list
			// runs to exactly three lines above the bottom
			if listEnd := at; len(lines) == height && listEnd != height-3 {
				t.Errorf("list ends at line %d, want %d (one row given up):\n%s", listEnd, height-3, out)
			}
			if len(lines) < height && at-first < 3 {
				t.Errorf("a short frame (%d of %d lines) and a list of %d rows:\n%s", len(lines), height, at-first, out)
			}
		})
	}
}

// node-pods R10: pages do not change as the cursor moves between headings and nodes.
func TestDaemonSetLinePagesStable(t *testing.T) {
	u := footerUI(t, 120, 14)
	for i := range 20 {
		footerNode(u, fmt.Sprintf("node-x%02d", i), []string{"us-east-1a", "us-east-1b"}[i%2])
	}
	v := u.view
	v.SetGroupBy(FieldZone)
	u.SetView(v)
	u.Refresh()
	listRows := func() []string {
		var out []string
		for _, l := range strings.Split(screen(u), "\n") {
			// the cursor mark in the margin is not part of the page
			l = strings.TrimSpace(strings.Replace(l, "▌", " ", 1))
			if strings.Contains(l, "DaemonSets") || l == "" {
				continue
			}
			out = append(out, l)
		}
		return out
	}
	press(u, "home")
	onHeading := listRows()
	press(u, "down")
	if got := dsLines(u); len(got) != 1 {
		t.Fatalf("cursor not on a node: %q", got)
	}
	onNode := listRows()
	if !slices.Equal(onHeading, onNode) {
		t.Errorf("the page changed with the cursor:\non a heading:\n%s\non a node:\n%s",
			strings.Join(onHeading, "\n"), strings.Join(onNode, "\n"))
	}
}
