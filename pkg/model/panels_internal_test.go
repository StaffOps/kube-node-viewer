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
	"strings"
	"testing"

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
	var saved []ViewState
	u.OnViewChange(func(v ViewState) { saved = append(saved, v) })

	// f → "+ add filter" → capacity (first field) → values listed with counts
	press(u, "f", "enter", "enter")
	out := u.View()
	if !strings.Contains(out, "OnDemand  2") || !strings.Contains(out, "Spot      2") {
		t.Fatalf("value picker:\n%s", out)
	}
	// Spot is the second value
	press(u, "down", " ", "enter")

	want := []Filter{{Field: FieldCapacity, Values: []string{"Spot"}, Enabled: true}}
	if !reflect.DeepEqual(u.view.Filters, want) {
		t.Fatalf("filters %+v", u.view.Filters)
	}
	if len(saved) != 1 {
		t.Errorf("view changes reported: %d", len(saved))
	}
	press(u, "esc")
	out = u.View()
	if strings.Contains(out, "node-00") || !strings.Contains(out, "node-01") {
		t.Errorf("filter not applied:\n%s", out)
	}
	if !strings.Contains(out, "capacity=Spot") {
		t.Error("header does not show the filter")
	}
}

func TestToggleAndRemoveFilter(t *testing.T) {
	u := capacityUI(t, 2)
	v := u.view
	v.Filters = []Filter{{Field: FieldCapacity, Values: []string{"Spot"}, Enabled: true}}
	u.SetView(v)

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
		t.Errorf("d did not remove: %+v", u.view.Filters)
	}
}

func TestEditFilterKeepsSelectionAndEmptyRemoves(t *testing.T) {
	u := capacityUI(t, 2)
	v := u.view
	v.Filters = []Filter{{Field: FieldCapacity, Values: []string{"Spot"}, Enabled: true}}
	u.SetView(v)

	press(u, "f", "enter")
	if !u.filters.selected["Spot"] {
		t.Fatal("editing lost the current values")
	}
	// unselect Spot (second row) and apply: no values left removes the filter
	press(u, "down", " ", "enter")
	if len(u.view.Filters) != 0 {
		t.Errorf("filters %+v", u.view.Filters)
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
	u.Update(tea.WindowSizeMsg{Width: 160, Height: 15})
	short := u.View()
	if strings.Contains(short, "Context:") || !strings.Contains(strings.SplitN(short, "\n", 2)[0], "ctx-a │ 2 of 2 shown") {
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
