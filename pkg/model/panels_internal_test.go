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
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func press(u *UIModel, keys ...string) tea.Cmd {
	var cmd tea.Cmd
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		case "right":
			msg = tea.KeyMsg{Type: tea.KeyRight}
		case " ":
			msg = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")}
		case "ctrl+c":
			msg = tea.KeyMsg{Type: tea.KeyCtrlC}
		case "backspace":
			msg = tea.KeyMsg{Type: tea.KeyBackspace}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		_, cmd = u.Update(msg)
	}
	return cmd
}

func quits(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

// capacityUI has nodes alternating on-demand and spot, across two pools.
func capacityUI(t *testing.T, count int) *UIModel {
	u := newTestUI(t)
	for i := 0; i < count; i++ {
		n := NewNode(&v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("node-%02d", i), CreationTimestamp: metav1.Now(),
				Labels: map[string]string{
					"karpenter.sh/capacity-type": []string{"on-demand", "spot"}[i%2],
					"karpenter.sh/nodepool":      []string{"default", "gpu"}[i%2],
				},
			},
			Spec: v1.NodeSpec{ProviderID: fmt.Sprintf("aws:///us-east-1a/i-%d", i)},
			Status: v1.NodeStatus{Allocatable: v1.ResourceList{
				v1.ResourceCPU: resource.MustParse("2"), v1.ResourcePods: resource.MustParse("10"),
			}},
		})
		u.cluster.AddNode(n).Show()
	}
	u.Refresh()
	return u
}

func TestQuitKeys(t *testing.T) {
	u := capacityUI(t, 1)
	if quits(press(u, "esc")) {
		t.Error("esc quit from the main view (R12)")
	}
	if !quits(press(u, "q")) {
		t.Error("q did not quit from the main view")
	}
	press(u, "f")
	if quits(press(u, "q")) {
		t.Error("q quit with a panel open")
	}
	if !quits(press(u, "ctrl+c")) {
		t.Error("ctrl+c did not quit with a panel open")
	}
}

func TestEscClosesPanels(t *testing.T) {
	u := capacityUI(t, 1)
	press(u, "?")
	if u.panel != panelHelp || !strings.Contains(u.View(), "Keys") {
		t.Fatal("? did not open help")
	}
	press(u, "esc")
	if u.panel != panelNone {
		t.Error("esc did not close help")
	}
	press(u, "f")
	if u.panel != panelFilters {
		t.Fatal("f did not open filters")
	}
	press(u, "esc")
	if u.panel != panelNone {
		t.Error("esc did not close filters")
	}
}

func TestAddFilterFromPanel(t *testing.T) {
	u := capacityUI(t, 4)
	var saved int
	u.OnViewChange(func(ViewState) { saved++ })

	// f lists every field; enter opens capacity (the first) with its values
	press(u, "f")
	out := u.View()
	for _, want := range []string{"Filters", "capacity", "nodegroup", "karpenter.sh/nodepool"} {
		if !strings.Contains(out, want) {
			t.Fatalf("panel misses %q:\n%s", want, out)
		}
	}
	press(u, "enter")
	if out = u.View(); !strings.Contains(out, "OnDemand  2") || !strings.Contains(out, "Spot  2") {
		t.Fatalf("values with counts:\n%s", out)
	}
	// Spot is the second value; ticking filters right away
	press(u, "down", "down", " ")
	want := []Filter{{Field: FieldCapacity, Values: []string{"Spot"}, Enabled: true}}
	if !reflect.DeepEqual(u.view.Filters, want) {
		t.Fatalf("filters %+v", u.view.Filters)
	}
	if saved != 1 {
		t.Errorf("view changes reported: %d", saved)
	}
	press(u, "esc", "esc")
	out = u.View()
	if u.panel != panelNone || strings.Contains(out, "node-00") || !strings.Contains(out, "node-01") {
		t.Errorf("filter not applied:\n%s", out)
	}
	if !strings.Contains(out, "capacity=Spot") {
		t.Error("header does not show the filter")
	}
}

func TestToggleAndClearFilter(t *testing.T) {
	u := capacityUI(t, 2)
	v := u.view
	v.Filters = []Filter{{Field: FieldCapacity, Values: []string{"Spot"}, Enabled: true}}
	u.SetView(v)

	// the field with a filter is listed first
	press(u, "f", " ")
	if u.view.Filters[0].Enabled {
		t.Fatal("space did not turn the filter off")
	}
	if !strings.Contains(u.View(), "(off)") {
		t.Error("disabled filter not marked")
	}
	press(u, "esc")
	if !strings.Contains(u.View(), "node-00") {
		t.Error("a disabled filter still filters")
	}
	press(u, "f", "d")
	if len(u.view.Filters) != 0 {
		t.Errorf("d did not clear: %+v", u.view.Filters)
	}
}

func TestUntickingLastValueRemovesFilter(t *testing.T) {
	u := capacityUI(t, 2)
	v := u.view
	v.Filters = []Filter{{Field: FieldCapacity, Values: []string{"Spot"}, Enabled: true}}
	u.SetView(v)

	press(u, "f", "enter", "down", "down", " ")
	if len(u.view.Filters) != 0 {
		t.Errorf("filters %+v", u.view.Filters)
	}
}

func TestPanelIsAPopupOverTheNodeList(t *testing.T) {
	u := capacityUI(t, 3)
	u.Update(tea.WindowSizeMsg{Width: 140, Height: 30})
	press(u, "s")
	out := u.View()
	if !strings.Contains(out, "╭─ Sort by") || !strings.Contains(out, "╰") {
		t.Fatalf("no popup box:\n%s", out)
	}
	if !strings.Contains(out, "NAME") {
		t.Errorf("node list hidden behind the popup:\n%s", out)
	}
}

func TestPaginationRepeatsGroupPath(t *testing.T) {
	u := capacityUI(t, 30)
	u.Update(tea.WindowSizeMsg{Width: 160, Height: 20})
	v := u.view
	v.SetGroupBy(FieldNodeGroup)
	v.Sort = SortSpec{Field: FieldName}
	u.SetView(v)

	first := u.View()
	if !strings.Contains(first, "default  15 nodes") {
		t.Fatalf("first page:\n%s", first)
	}
	press(u, "right")
	second := u.View()
	if !strings.Contains(second, "default (cont.)") {
		t.Errorf("second page does not repeat the group:\n%s", second)
	}
	if lines := strings.Count(second, "\n") + 1; lines > 20 {
		t.Errorf("page is %d lines on a 20-line screen", lines)
	}
}

func TestHeaderPanelAndOneLineFallback(t *testing.T) {
	u := capacityUI(t, 2)
	u.SetClusterInfo(ClusterInfo{Context: "ctx-a", Server: "https://a.example", Region: "us-east-1"})

	u.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	tall := u.View()
	for _, want := range []string{"Context:", "ctx-a", "https://a.example", "us-east-1", "2 of 2 shown", "Filters: none"} {
		if !strings.Contains(tall, want) {
			t.Errorf("header panel misses %q", want)
		}
	}
	u.Update(tea.WindowSizeMsg{Width: 160, Height: 20})
	compact := u.View()
	compactHeader := strings.Join(strings.SplitN(compact, "\n", 5)[:4], "\n")
	for _, want := range []string{"╭", "ctx-a", "https://a.example", "2 of 2 shown", "Filters: none", "╰"} {
		if !strings.Contains(compactHeader, want) {
			t.Errorf("compact header misses %q:\n%s", want, compactHeader)
		}
	}

	u.Update(tea.WindowSizeMsg{Width: 160, Height: 10})
	short := u.View()
	if strings.Contains(short, "╭") || !strings.Contains(strings.SplitN(short, "\n", 2)[0], "ctx-a │ 2 of 2 shown") {
		t.Errorf("short screen header:\n%s", short)
	}
}

func TestNarrowScreenHidesColumns(t *testing.T) {
	u := capacityUI(t, 2)
	u.Update(tea.WindowSizeMsg{Width: 70, Height: 40})
	out := u.View()
	if !strings.Contains(out, "columns hidden") || !strings.Contains(out, "STATUS") {
		t.Errorf("narrow screen:\n%s", out)
	}
}

func TestCtrlETogglesHeaderAndSaves(t *testing.T) {
	u := capacityUI(t, 2)
	u.SetClusterInfo(ClusterInfo{Context: "ctx-a"})
	var saved []ViewState
	u.OnViewChange(func(v ViewState) { saved = append(saved, v) })
	ctrlE := tea.KeyMsg{Type: tea.KeyCtrlE}

	u.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	if !strings.Contains(u.View(), "Context:") {
		t.Fatal("tall screen does not start with the full panel")
	}
	u.Update(ctrlE)
	if out := u.View(); strings.Contains(out, "Context:") || !strings.Contains(out, "╭") {
		t.Errorf("ctrl+e did not compact the header:\n%s", out)
	}
	if len(saved) != 1 || saved[0].Header != HeaderCompact {
		t.Errorf("pick not saved: %+v", saved)
	}

	// on a short screen the compact box is the default; ctrl+e expands it
	u.Update(tea.WindowSizeMsg{Width: 160, Height: 20})
	u.view.Header = ""
	u.Update(ctrlE)
	if !strings.Contains(u.View(), "Context:") {
		t.Error("ctrl+e did not expand the header on a short screen")
	}

	// works with a panel open, and the panel stays open
	press(u, "f")
	u.Update(ctrlE)
	if u.panel != panelFilters || strings.Contains(u.View(), "Context:") {
		t.Error("ctrl+e in a panel")
	}
}

func TestManagedNodeGroupShownByNameInColour(t *testing.T) {
	n := NewNode(&v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "n", Labels: map[string]string{"eks.amazonaws.com/nodegroup": "workers"}}})
	u := newTestUI(t)
	cell := u.cell(n, FieldNodeGroup, time.Now())
	if !strings.Contains(cell, "workers") || strings.Contains(cell, "ManagedNodeGroup") {
		t.Errorf("cell %q", cell)
	}
	if textWidth(cell) != len("workers") {
		t.Errorf("colour codes counted in the width: %d", textWidth(cell))
	}
}

func TestGroupByPanel(t *testing.T) {
	u := capacityUI(t, 4)
	var saved int
	u.OnViewChange(func(ViewState) { saved++ })

	// g lists every field; space ticks capacity (first) as level 1
	press(u, "g", " ")
	if !reflect.DeepEqual(u.view.GroupBy, []Field{FieldCapacity}) {
		t.Fatalf("group-by %v", u.view.GroupBy)
	}
	// nodegroup is the second row: tick it as level 2, then move it up
	press(u, "down", " ")
	if !reflect.DeepEqual(u.view.GroupBy, []Field{FieldCapacity, FieldNodeGroup}) {
		t.Fatalf("group-by %v", u.view.GroupBy)
	}
	if !strings.Contains(u.View(), "[2] nodegroup") {
		t.Error("level number not shown")
	}
	press(u, "K")
	if !reflect.DeepEqual(u.view.GroupBy, []Field{FieldNodeGroup, FieldCapacity}) {
		t.Fatalf("after K: %v", u.view.GroupBy)
	}
	// untick nodegroup (cursor on it, level 1)
	press(u, " ")
	if !reflect.DeepEqual(u.view.GroupBy, []Field{FieldCapacity}) {
		t.Fatalf("after untick: %v", u.view.GroupBy)
	}
	if saved != 4 {
		t.Errorf("saved %d changes, want 4", saved)
	}
	press(u, "esc")
	if out := u.View(); !strings.Contains(out, "OnDemand  2 nodes") {
		t.Errorf("groups not shown:\n%s", out)
	}
}

func TestGroupByPanelAtMostThreeLevels(t *testing.T) {
	u := capacityUI(t, 2)
	v := u.view
	v.SetGroupBy(FieldCapacity, FieldNodeGroup, FieldZone)
	u.SetView(v)
	// the fourth row is the first field not grouped on
	press(u, "g", "down", "down", "down", " ")
	if len(u.view.GroupBy) != 3 {
		t.Errorf("a fourth level was added: %v", u.view.GroupBy)
	}
}

func TestSortPanel(t *testing.T) {
	u := capacityUI(t, 3)
	// s → name (second choice) sorts ascending
	press(u, "s", "down", "enter")
	if u.panel != panelNone || u.view.Sort != (SortSpec{Field: FieldName}) {
		t.Fatalf("sort %+v panel %v", u.view.Sort, u.panel)
	}
	out := u.View()
	if strings.Index(out, "node-00") > strings.Index(out, "node-02") {
		t.Error("not sorted by name")
	}
	// enter on the current field flips it
	press(u, "s", "enter")
	if !u.view.Sort.Descending {
		t.Error("enter on the sorted field did not flip it")
	}
	// price starts descending
	press(u, "s", "down", "down", "enter")
	if u.view.Sort != (SortSpec{Field: FieldPrice, Descending: true}) {
		t.Errorf("sort %+v", u.view.Sort)
	}
}

func TestColumnsPanel(t *testing.T) {
	u := capacityUI(t, 2)
	// the first row is the first shown column (instance-type): hide it
	press(u, "o", " ")
	if slices.Contains(u.view.Columns, FieldInstanceType) {
		t.Fatal("space did not hide the column")
	}
	if u.columns.fields[u.columns.cursor] != FieldInstanceType {
		t.Error("cursor left the column it toggled")
	}
	press(u, " ")
	if u.view.Columns[len(u.view.Columns)-1] != FieldInstanceType {
		t.Fatalf("shown again at the end: %v", u.view.Columns)
	}
	press(u, "K")
	if u.view.Columns[len(u.view.Columns)-2] != FieldInstanceType {
		t.Errorf("K did not move it left: %v", u.view.Columns)
	}
	press(u, "esc")
	var title string
	for _, line := range strings.Split(u.View(), "\n") {
		if strings.Contains(line, "NAME") {
			title = line
		}
	}
	if !strings.HasSuffix(strings.TrimSpace(title), "INSTANCE  STATUS") {
		t.Errorf("column order on screen: %q", title)
	}
}

func TestSearchTypesKeysAndEscClears(t *testing.T) {
	u := capacityUI(t, 12)
	var saved int
	u.OnViewChange(func(ViewState) { saved++ })

	press(u, "/", "n", "o", "d", "e", "-", "1", "1")
	if u.view.Search != "node-11" {
		t.Fatalf("search %q", u.view.Search)
	}
	out := u.View()
	if !strings.Contains(out, "node-11") || strings.Contains(out, "node-10") {
		t.Errorf("not filtered while typing:\n%s", out)
	}
	// q and f are typed, not commands (R12)
	if quits(press(u, "q")) || u.panel != panelNone {
		t.Error("a command key acted while searching")
	}
	press(u, "backspace", "enter")
	if u.searching || u.view.Search != "node-11" {
		t.Errorf("enter: searching %v text %q", u.searching, u.view.Search)
	}
	press(u, "/", "esc")
	if u.view.Search != "" {
		t.Error("esc did not clear the search")
	}
	if saved != 0 {
		t.Error("search text was saved")
	}
}

func TestResourcesCycle(t *testing.T) {
	u := capacityUI(t, 1)
	var got []string
	for i := 0; i < 4; i++ {
		press(u, "r")
		got = append(got, strings.Join(u.view.Resources, "+"))
	}
	want := []string{"memory", "cpu+memory", "cpu", "memory"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("cycle %v", got)
	}

	// a list from --resources is a fourth step
	v := u.view
	v.Resources = []string{"cpu", "nvidia.com/gpu"}
	u.SetView(v)
	got = nil
	for i := 0; i < 4; i++ {
		press(u, "r")
		got = append(got, strings.Join(u.view.Resources, "+"))
	}
	want = []string{"cpu", "memory", "cpu+memory", "cpu+nvidia.com/gpu"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("cycle with custom list %v", got)
	}
}

func TestCollapsedGroupsShowUsageBars(t *testing.T) {
	u := capacityUI(t, 4)
	v := u.view
	v.SetGroupBy(FieldCapacity)
	v.Resources = []string{"cpu", "memory"}
	u.SetView(v)
	press(u, "n")
	out := u.View()
	if strings.Contains(out, "node-00") {
		t.Fatal("nodes listed while collapsed")
	}
	// one bar line per resource per group, each ending in its percentage
	// count from the first group heading, past the cluster summary
	bars := 0
	groups := out[strings.Index(out, "OnDemand"):]
	for _, line := range strings.Split(groups, "\n") {
		if strings.ContainsAny(line, "▇█") && strings.Contains(line, "%") {
			bars++
		}
	}
	if bars != 4 {
		t.Errorf("want 4 group bars (2 groups x cpu, memory), got %d:\n%s", bars, out)
	}
}

func TestGroupHeadingsAreATree(t *testing.T) {
	u := capacityUI(t, 4)
	v := u.view
	v.SetGroupBy(FieldCapacity, FieldNodeGroup)
	u.SetView(v)
	for _, collapsed := range []bool{false, true} {
		if collapsed {
			press(u, "n")
		}
		out := u.View()
		if strings.Contains(out, "OnDemand › default") {
			t.Errorf("collapsed=%v: sub-group repeats its parent:\n%s", collapsed, out)
		}
		if !strings.Contains(out, "└─ default") || !strings.Contains(out, "└─ gpu") {
			t.Errorf("collapsed=%v: sub-groups not hung off their parent:\n%s", collapsed, out)
		}
	}
}

func TestNFoldsOneLevelPerPress(t *testing.T) {
	u := capacityUI(t, 4)
	v := u.view
	v.SetGroupBy(FieldCapacity, FieldNodeGroup)
	u.SetView(v)

	type screen struct{ nodes, sub, top bool }
	look := func() screen {
		out := u.View()
		return screen{
			nodes: strings.Contains(out, "node-00"),
			sub:   strings.Contains(out, "└─ default"),
			top:   strings.Contains(out, "OnDemand  2 nodes") || strings.Contains(out, "OnDemand ") && strings.Contains(out, "2 nodes"),
		}
	}
	want := []screen{
		{nodes: true, sub: true, top: true},   // start: everything
		{nodes: false, sub: true, top: true},  // n: groups and sub-groups
		{nodes: false, sub: false, top: true}, // n: top-level groups only
		{nodes: true, sub: true, top: true},   // n: everything again
	}
	for i, w := range want {
		if i > 0 {
			press(u, "n")
		}
		if got := look(); got != w {
			t.Errorf("after %d presses: %+v, want %+v\n%s", i, got, w, u.View())
		}
	}

	// at the most folded step, d lists the sub-groups as one-line rows
	press(u, "n", "n")
	press(u, "d")
	if got := look(); got != (screen{nodes: false, sub: true, top: true}) {
		t.Errorf("details on: %+v\n%s", got, u.View())
	}
	if out := u.View(); strings.Contains(out, "└─ default") && strings.ContainsAny(out, "▇█") {
		for _, line := range strings.Split(out, "\n") {
			if strings.Contains(line, "└─ default") && strings.ContainsAny(line, "▇█") {
				t.Errorf("a detail row has a bar: %q", line)
			}
		}
	}
	press(u, "d")
	if got := look(); got.sub {
		t.Error("d did not hide the details again")
	}
}
