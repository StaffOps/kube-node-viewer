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
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Tests for specs/sort-invert: `I` on the Overview main screen flips the
// direction of the current sort (R1–R4).

const teamLabel = "example.com/team"

// teamUI has nodes with and without the team label; node-tb and node-tc tie.
func teamUI(t *testing.T) *UIModel {
	t.Helper()
	u := newTestUI(t)
	for name, team := range map[string]string{
		"node-ta": "a", "node-tb": "b", "node-tc": "b", "node-zx": "", "node-zy": "",
	} {
		labels := map[string]string{}
		if team != "" {
			labels[teamLabel] = team
		}
		u.cluster.AddNode(NewNode(&v1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: name, CreationTimestamp: metav1.Now(), Labels: labels},
			Spec:       v1.NodeSpec{ProviderID: "aws:///us-east-1a/i-" + name},
			Status: v1.NodeStatus{Allocatable: v1.ResourceList{
				v1.ResourceCPU: resource.MustParse("2"), v1.ResourcePods: resource.MustParse("10"),
			}},
		})).Show()
	}
	u.Refresh()
	return u
}

// screenOrder returns the given names in the order they appear on screen,
// failing when one is missing.
func screenOrder(t *testing.T, out string, names ...string) []string {
	t.Helper()
	pos := map[string]int{}
	for _, n := range names {
		i := strings.Index(out, n)
		if i < 0 {
			t.Fatalf("%s not on screen:\n%s", n, out)
		}
		pos[n] = i
	}
	sorted := append([]string(nil), names...)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && pos[sorted[j]] < pos[sorted[j-1]]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	return sorted
}

// sort-invert R1: I flips the current sort's direction, keeping its field.
func TestFlipSortBothWaysFromANamedSort(t *testing.T) {
	cases := []struct {
		name  string
		start SortSpec
	}{
		{"name ascending", SortSpec{Field: FieldName}},
		{"name descending", SortSpec{Field: FieldName, Descending: true}},
		{"price descending", SortSpec{Field: FieldPrice, Descending: true}},
		{"label ascending", SortSpec{Field: LabelField(teamLabel)}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			u := capacityUI(t, 3)
			v := u.view
			v.Sort = c.start
			u.SetView(v)

			press(u, "I")
			want := SortSpec{Field: c.start.Field, Descending: !c.start.Descending}
			if u.view.Sort != want {
				t.Fatalf("after I: %+v, want %+v", u.view.Sort, want)
			}
			press(u, "I")
			if u.view.Sort != c.start {
				t.Errorf("after I I: %+v, want %+v", u.view.Sort, c.start)
			}
		})
	}
}

func TestSortInvertRedrawsTheList(t *testing.T) {
	u := capacityUI(t, 3)
	v := u.view
	v.Sort = SortSpec{Field: FieldName}
	u.SetView(v)
	names := []string{"node-00", "node-01", "node-02"}
	if got := screenOrder(t, u.View(), names...); strings.Join(got, ",") != "node-00,node-01,node-02" {
		t.Fatalf("ascending order on screen: %v", got)
	}
	press(u, "I")
	out := u.View()
	if got := screenOrder(t, out, names...); strings.Join(got, ",") != "node-02,node-01,node-00" {
		t.Errorf("descending order on screen: %v", got)
	}
	if !strings.Contains(out, "name ↓") {
		t.Errorf("header does not show the new arrow:\n%s", out)
	}
}

func TestSortInvertDefaultCreation(t *testing.T) {
	u := capacityUI(t, 2)
	if u.view.Sort != (SortSpec{Field: FieldCreation, Descending: true}) {
		t.Fatalf("default sort %+v", u.view.Sort)
	}
	if !strings.Contains(u.View(), "creation ↓") {
		t.Fatalf("default header arrow:\n%s", u.View())
	}
	press(u, "I")
	if u.view.Sort != (SortSpec{Field: FieldCreation}) {
		t.Errorf("after I: %+v", u.view.Sort)
	}
	if !strings.Contains(u.View(), "creation ↑") {
		t.Errorf("header after I:\n%s", u.View())
	}
}

func TestSortInvertEmptyField(t *testing.T) {
	u := capacityUI(t, 2)
	v := u.view
	v.Sort = SortSpec{}
	u.SetView(v)
	if !strings.Contains(u.View(), "creation ↑") {
		t.Fatalf("empty field does not show as creation:\n%s", u.View())
	}
	press(u, "I")
	if !u.view.Sort.Descending {
		t.Fatalf("I did not flip an empty-field sort: %+v", u.view.Sort)
	}
	if f := u.view.Sort.Field; f != "" && f != FieldCreation {
		t.Errorf("I changed the field to %q", f)
	}
	if !strings.Contains(u.View(), "creation ↓") {
		t.Errorf("header after I:\n%s", u.View())
	}
}

func TestSortInvertResetsPage(t *testing.T) {
	u := capacityUI(t, 30)
	u.Update(tea.WindowSizeMsg{Width: 160, Height: 20})
	v := u.view
	v.Sort = SortSpec{Field: FieldName}
	u.SetView(v)
	press(u, "right")
	if u.page != 1 {
		t.Fatalf("page %d after right", u.page)
	}
	press(u, "I")
	if u.page != 0 {
		t.Errorf("page %d after I, want 0", u.page)
	}
}

func TestSortInvertReportsViewChange(t *testing.T) {
	u := capacityUI(t, 2)
	v := u.view
	v.Sort = SortSpec{Field: FieldPrice, Descending: true}
	u.SetView(v)
	var got []ViewState
	u.OnViewChange(func(s ViewState) { got = append(got, s) })

	press(u, "I")
	if len(got) != 1 {
		t.Fatalf("view changes: %d, want 1", len(got))
	}
	if got[0].Sort != (SortSpec{Field: FieldPrice}) {
		t.Errorf("saved sort %+v", got[0].Sort)
	}
}

func TestSortInvertLowercaseIIsNotBound(t *testing.T) {
	u := capacityUI(t, 2)
	before := u.view
	var changes int
	u.OnViewChange(func(ViewState) { changes++ })
	press(u, "i")
	if u.view.Sort != before.Sort || changes != 0 {
		t.Errorf("lowercase i changed the view: %+v, %d changes", u.view.Sort, changes)
	}
}

// sort-invert R2: I does nothing on Recommendations.
func TestSortInvertNoOpOnRecommendations(t *testing.T) {
	u := recUI(t)
	pressAll(u, "tab")
	if !u.onRecommendations() {
		t.Fatal("tab did not open Recommendations")
	}
	before := u.view.Sort
	var changes int
	u.OnViewChange(func(ViewState) { changes++ })
	pressAll(u, "I")
	if u.view.Sort != before {
		t.Errorf("sort changed on Recommendations: %+v → %+v", before, u.view.Sort)
	}
	if changes != 0 {
		t.Errorf("%d view changes on Recommendations", changes)
	}
	if !u.onRecommendations() {
		t.Error("I left the Recommendations screen")
	}
}

// sort-invert R3: with a panel open, I is the panel's key.
func TestSortInvertNoOpWithPanelOpen(t *testing.T) {
	for _, open := range []string{"s", "f"} {
		t.Run(open, func(t *testing.T) {
			u := capacityUI(t, 3)
			before := u.view.Sort
			press(u, open)
			if u.panel == panelNone {
				t.Fatalf("%s did not open a panel", open)
			}
			press(u, "I")
			if u.view.Sort != before {
				t.Errorf("I flipped the sort with the panel open: %+v", u.view.Sort)
			}
		})
	}
}

// sort-invert R3: in the search, command line or view-name prompt, I is text.
func TestSortInvertTypedAsText(t *testing.T) {
	cases := []struct {
		name string
		open []string
		text func(u *UIModel) string
	}{
		{"search", []string{"/"}, func(u *UIModel) string { return u.view.Search }},
		{"command line", []string{":"}, func(u *UIModel) string { return u.command }},
		{"view name", []string{"v", "enter"}, func(u *UIModel) string { return u.viewsPanel.name }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			u := capacityUI(t, 3)
			u.SetViews(memViews{})
			before := u.view.Sort
			press(u, c.open...)
			if c.name == "view name" && !u.viewsPanel.naming {
				t.Fatal("the view-name prompt did not open")
			}
			press(u, "I")
			if got := c.text(u); got != "I" {
				t.Errorf("typed text %q, want %q", got, "I")
			}
			if u.view.Sort != before {
				t.Errorf("sort changed while typing: %+v", u.view.Sort)
			}
		})
	}
}

func TestSortInvertKeepsGroupOrder(t *testing.T) {
	u := capacityUI(t, 4)
	v := u.view
	v.SetGroupBy(FieldNodeGroup)
	v.Sort = SortSpec{Field: FieldName}
	u.SetView(v)

	for i, wantNodes := range []string{"node-00,node-02", "node-02,node-00"} {
		if i > 0 {
			press(u, "I")
		}
		out := u.View()
		if got := screenOrder(t, out, "default  2 nodes", "gpu      2 nodes"); got[0] != "default  2 nodes" {
			t.Errorf("descending=%v: group order %v", u.view.Sort.Descending, got)
		}
		if got := screenOrder(t, out, "node-00", "node-02"); strings.Join(got, ",") != wantNodes {
			t.Errorf("descending=%v: nodes in group %v, want %s", u.view.Sort.Descending, got, wantNodes)
		}
	}
}

func TestSortInvertMissingValuesStayLast(t *testing.T) {
	names := []string{"node-ta", "node-tb", "node-tc", "node-zx", "node-zy"}
	for _, desc := range []bool{false, true} {
		u := teamUI(t)
		v := u.view
		v.Sort = SortSpec{Field: LabelField(teamLabel), Descending: !desc}
		u.SetView(v)
		press(u, "I")
		if u.view.Sort.Descending != desc {
			t.Fatalf("I did not flip to descending=%v", desc)
		}
		got := screenOrder(t, u.View(), names...)
		if tail := strings.Join(got[3:], ","); tail != "node-zx,node-zy" {
			t.Errorf("descending=%v: nodes without the label are not last by name: %v", desc, got)
		}
		valueOrder := map[bool]string{false: "node-ta", true: "node-tb"}[desc]
		if got[0] != valueOrder {
			t.Errorf("descending=%v: first node %s, want %s (%v)", desc, got[0], valueOrder, got)
		}

		// same order as the same sort chosen another way (Sort panel / saved view)
		ref := teamUI(t)
		rv := ref.view
		rv.Sort = u.view.Sort
		ref.SetView(rv)
		if want := screenOrder(t, ref.View(), names...); strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("descending=%v: order after I %v differs from the same sort set directly %v", desc, got, want)
		}
	}
}

// sort-invert R4: the help and the footer list I.
func TestSortInvertHelpAndFooter(t *testing.T) {
	style, err := ParseStyle("#04B575,#FFFF00,#FF0000")
	if err != nil {
		t.Fatal(err)
	}
	u := NewUIModel(style) // no window size: width 0
	if !strings.Contains(u.helpText(), "s: sort • I: flip ↑/↓") {
		t.Errorf("footer does not show I right after s: %q", u.helpText())
	}

	idx := -1
	for i, k := range helpKeys {
		if k[0] == "I" {
			idx = i
		}
	}
	if idx < 1 || helpKeys[idx-1][0] != "s" {
		t.Errorf("help does not list I right after s (index %d)", idx)
	}
	for _, k := range headerKeys {
		if k[0] == "I" {
			t.Error("I added to the header key box")
		}
	}
}
