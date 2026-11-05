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
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

var (
	poolField = LabelField("karpenter.sh/nodepool")
	zoneField = LabelField("topology.kubernetes.io/zone")
	archField = LabelField("kubernetes.io/arch")
)

// focusUI has six nodes over two nodepools, two zones and two archs, one of
// them with no nodepool label (the <none> group), grouped by the fields and
// sorted by name. saves counts the views passed to the save callback.
func focusUI(t *testing.T, groupBy ...Field) (u *UIModel, saves *[]ViewState) {
	t.Helper()
	u = newTestUI(t)
	for _, n := range []struct{ name, pool, zone, arch string }{
		{"a1", "default", "us-east-1a", "amd64"},
		{"a2", "default", "us-east-1a", "arm64"},
		{"b1", "default", "us-east-1b", "amd64"},
		{"g1", "gpu", "us-east-1a", "amd64"},
		{"g2", "gpu", "us-east-1b", "amd64"},
		{"n1", "", "us-east-1a", "amd64"},
	} {
		labels := map[string]string{"topology.kubernetes.io/zone": n.zone, "kubernetes.io/arch": n.arch}
		if n.pool != "" {
			labels["karpenter.sh/nodepool"] = n.pool
		}
		labeledNode(u, n.name, labels)
	}
	u.Refresh()
	v := u.view
	v.SetGroupBy(groupBy...)
	v.Sort = SortSpec{Field: FieldName}
	u.SetView(v)
	saves = &[]ViewState{}
	u.OnViewChange(func(s ViewState) { *saves = append(*saves, s) })
	u.View()
	return u, saves
}

// cursorTo moves the cursor down until it is on the row with the key.
func cursorTo(t *testing.T, u *UIModel, key string) {
	t.Helper()
	u.View()
	for i := 0; i < 200 && u.cursorKey != key; i++ {
		press(u, "down")
		u.View()
	}
	if u.cursorKey != key {
		t.Fatalf("cursor never reached %q (at %q):\n%s", key, u.cursorKey, screen(u))
	}
}

// listed tells which of the node names are on screen.
func listed(u *UIModel, names ...string) map[string]bool {
	out := screen(u)
	got := map[string]bool{}
	for _, n := range names {
		for _, line := range strings.Split(out, "\n") {
			if strings.Contains(" "+line+" ", " "+n+" ") {
				got[n] = true
			}
		}
	}
	return got
}

func wantListed(t *testing.T, u *UIModel, want ...string) {
	t.Helper()
	all := []string{"a1", "a2", "b1", "g1", "g2", "n1"}
	got := listed(u, all...)
	for _, n := range all {
		if got[n] != contains(want, n) {
			t.Errorf("node %s listed=%v, want %v:\n%s", n, got[n], contains(want, n), screen(u))
		}
	}
}

func contains(list []string, s string) bool {
	for _, l := range list {
		if l == s {
			return true
		}
	}
	return false
}

// R1, R2, R6: enter on a heading lists only that group, the header and the
// summary count it, the View line shows it, and nothing is saved.
func TestFocusShowsOnlyTheGroup(t *testing.T) {
	u, saves := focusUI(t, poolField)
	before := u.view
	cursorTo(t, u, "g:gpu")
	press(u, "enter")
	wantListed(t, u, "g1", "g2")
	out := screen(u)
	for _, want := range []string{"2 of 6 shown", "(filtered: 2 of 6 nodes)", "Focus: karpenter.sh/nodepool=gpu", "esc: back"} {
		if !strings.Contains(out, want) {
			t.Errorf("screen lacks %q:\n%s", want, out)
		}
	}
	if len(*saves) != 0 {
		t.Errorf("the focus was saved: %+v", *saves)
	}
	if !reflect.DeepEqual(u.view, before) {
		t.Errorf("a focused frame changed the view:\n%+v\n%+v", u.view, before)
	}
}

// R1: the <none> group focuses like any other.
func TestFocusOnTheNoneGroup(t *testing.T) {
	u, _ := focusUI(t, poolField)
	cursorTo(t, u, "g:"+UngroupedValue)
	press(u, "enter")
	wantListed(t, u, "n1")
}

// R1, R7: a list folded with n lists the focused group's nodes; n and d do
// nothing while focused; esc brings the folded list back as it was.
func TestFocusOnAFoldedList(t *testing.T) {
	u, saves := focusUI(t, poolField, zoneField)
	press(u, "n") // nodes hidden
	press(u, "n") // zones hidden, nodepools carry the bars
	press(u, "d") // the zones as one-line rows
	folded := u.view
	*saves = nil
	wantListed(t, u)
	cursorTo(t, u, "g:gpu")
	press(u, "enter")
	wantListed(t, u, "g1", "g2")
	press(u, "n", "d")
	if !reflect.DeepEqual(u.view, folded) || len(*saves) != 0 {
		t.Errorf("n or d changed the view while focused: %+v saves %d", u.view, len(*saves))
	}
	if help := u.helpText(); strings.Contains(help, "n: fold") || strings.Contains(help, "d: details") {
		t.Errorf("key line offers n or d while focused: %q", help)
	}
	press(u, "esc")
	wantListed(t, u)
	if u.cursorKey != "g:gpu" {
		t.Errorf("cursor after esc: %q", u.cursorKey)
	}
	press(u, "n")
	if len(*saves) != 1 {
		t.Error("n does not fold again after leaving the focus")
	}
}

// R1, R5: a group folded on its own shows its nodes once focused and is
// folded again after esc; space on the focused heading does nothing.
func TestFocusUnfoldsAFoldedGroup(t *testing.T) {
	u, saves := focusUI(t, poolField)
	cursorTo(t, u, "g:gpu")
	press(u, " ")
	if !u.view.IsCollapsed([]string{"gpu"}) || len(*saves) != 1 {
		t.Fatalf("space did not fold gpu and save it: %+v", *saves)
	}
	wantListed(t, u, "a1", "a2", "b1", "n1")
	press(u, "enter")
	wantListed(t, u, "g1", "g2")
	// space on the focused heading, with a pending move onto it
	press(u, "down", "up", " ")
	if len(*saves) != 1 || !u.view.IsCollapsed([]string{"gpu"}) {
		t.Errorf("space on the focused heading saved a fold: %d saves", len(*saves))
	}
	press(u, "esc")
	if out := screen(u); !strings.Contains(out, "▸ gpu") {
		t.Errorf("gpu not folded again after esc:\n%s", out)
	}
}

// R3, R4: two levels, a sub-group focus, enter on an ancestor, one esc per
// enter with the cursor back on the heading.
func TestFocusTwoLevels(t *testing.T) {
	u, _ := focusUI(t, poolField, zoneField)
	cursorTo(t, u, "g:default")
	press(u, "enter")
	wantListed(t, u, "a1", "a2", "b1")
	cursorTo(t, u, "g:default › us-east-1a")
	press(u, "enter")
	wantListed(t, u, "a1", "a2")
	if out := screen(u); !strings.Contains(out, "Focus: karpenter.sh/nodepool=default › topology.kubernetes.io/zone=us-east-1a") {
		t.Errorf("View line:\n%s", out)
	}
	// enter on the ancestor heading changes nothing
	u.cursorKey, u.cursorDelta = "g:default", 0
	u.View()
	press(u, "enter")
	if len(u.focusLevels()) != 2 {
		t.Errorf("enter on an ancestor changed the focus: %+v", u.focusLevels())
	}
	press(u, "esc")
	wantListed(t, u, "a1", "a2", "b1")
	if u.View(); u.cursorKey != "g:default › us-east-1a" {
		t.Errorf("cursor after the first esc: %q", u.cursorKey)
	}
	press(u, "esc")
	wantListed(t, u, "a1", "a2", "b1", "g1", "g2", "n1")
	if u.View(); u.cursorKey != "g:default" {
		t.Errorf("cursor after the second esc: %q", u.cursorKey)
	}
	// with no focus and no search, esc does nothing
	press(u, "esc")
	if u.focused() || u.cursorKey != "g:default" {
		t.Error("esc with nothing to undo changed something")
	}
}

// R3: one enter that adds two levels is undone by one esc.
func TestFocusOneEscPerEnter(t *testing.T) {
	u, _ := focusUI(t, poolField, zoneField)
	cursorTo(t, u, "g:default › us-east-1b")
	press(u, "enter")
	wantListed(t, u, "b1")
	press(u, "esc")
	if u.focused() {
		t.Errorf("one esc left %+v", u.focusLevels())
	}
}

// R3: a cursor move pending when esc arrives does not move the cursor off
// the heading left.
func TestFocusEscWithAPendingMove(t *testing.T) {
	u, _ := focusUI(t, poolField)
	cursorTo(t, u, "g:gpu")
	press(u, "enter")
	press(u, "down", "esc")
	if u.View(); u.cursorKey != "g:gpu" {
		t.Errorf("cursor after down+esc: %q", u.cursorKey)
	}
}

// R3: the heading left on a later page brings that page back.
func TestFocusEscFollowsTheHeadingsPage(t *testing.T) {
	u := capacityUI(t, 40)
	u.Update(tea.WindowSizeMsg{Width: 160, Height: 20})
	v := u.view
	v.SetGroupBy(poolField)
	v.Sort = SortSpec{Field: FieldName}
	u.SetView(v)
	cursorTo(t, u, "g:gpu")
	page := u.page
	if page == 0 {
		t.Fatal("test needs gpu on a later page")
	}
	press(u, "enter")
	// R1: a focus over more than one page pages as before
	if out := screen(u); !strings.Contains(out, "•") {
		t.Errorf("no page dots for a focus over two pages:\n%s", out)
	}
	press(u, "right")
	if out := screen(u); !strings.Contains(out, "gpu (cont.)") {
		t.Errorf("continuing page does not repeat the group:\n%s", out)
	}
	press(u, "esc")
	u.View()
	if u.cursorKey != "g:gpu" || u.page != page {
		t.Errorf("after esc: cursor %q page %d, want g:gpu page %d", u.cursorKey, u.page, page)
	}
	if !strings.Contains(selectedLine(t, u), "gpu") {
		t.Errorf("selected line: %q", selectedLine(t, u))
	}
}

// R1: three levels; a level-2 group folded on its own stays folded inside a
// level-1 focus.
func TestFocusKeepsFoldedSubGroups(t *testing.T) {
	u, _ := focusUI(t, poolField, zoneField, archField)
	cursorTo(t, u, "g:default › us-east-1a")
	press(u, " ")
	u.cursorKey = ""
	cursorTo(t, u, "g:default")
	press(u, "enter")
	wantListed(t, u, "b1")
	if out := screen(u); !strings.Contains(out, "▸ us-east-1a") {
		t.Errorf("folded sub-group unfolded by the focus:\n%s", out)
	}
	// R5: space on a sub-group inside the focus folds it
	cursorTo(t, u, "g:default › us-east-1b")
	press(u, " ")
	wantListed(t, u)
}

// R4, R6: the focus survives a group-by change and enter adds the new
// field's level; esc undoes that step only.
func TestFocusAcrossAGroupByChange(t *testing.T) {
	u, _ := focusUI(t, poolField)
	cursorTo(t, u, "g:gpu")
	press(u, "enter")
	v := u.view
	v.SetGroupBy(zoneField)
	u.setView(v)
	wantListed(t, u, "g1", "g2")
	if !strings.Contains(screen(u), "Focus: karpenter.sh/nodepool=gpu") {
		t.Errorf("focus not shown after the group-by change:\n%s", screen(u))
	}
	u.cursorKey = ""
	cursorTo(t, u, "g:us-east-1a")
	press(u, "enter")
	wantListed(t, u, "g1")
	press(u, "esc")
	wantListed(t, u, "g1", "g2")
	// the heading left (g:gpu) does not exist under zone: first row
	press(u, "esc")
	if u.View(); u.focused() || u.page != 0 || u.cursorKey != "g:us-east-1a" {
		t.Errorf("after the last esc: focused=%v page %d cursor %q", u.focused(), u.page, u.cursorKey)
	}
}

// R3, D4: esc clears a kept search first, then leaves the focus.
func TestFocusEscClearsTheSearchFirst(t *testing.T) {
	u, _ := focusUI(t, poolField)
	cursorTo(t, u, "g:gpu")
	press(u, "enter", "/", "g", "2", "enter")
	wantListed(t, u, "g2")
	press(u, "esc")
	if u.view.Search != "" || !u.focused() {
		t.Fatalf("first esc: search %q focused %v", u.view.Search, u.focused())
	}
	wantListed(t, u, "g1", "g2")
	press(u, "esc")
	if u.focused() {
		t.Error("second esc did not leave the focus")
	}
}

// R6: a new cluster or a named view shows every group again.
func TestSetViewClearsTheFocus(t *testing.T) {
	u, _ := focusUI(t, poolField)
	cursorTo(t, u, "g:gpu")
	press(u, "enter")
	u.SetView(u.ViewState())
	if u.focused() {
		t.Error("SetView kept the focus")
	}
	wantListed(t, u, "a1", "a2", "b1", "g1", "g2", "n1")
}

// R6: the two empty messages.
func TestFocusEmptyMessages(t *testing.T) {
	u, _ := focusUI(t, poolField)
	cursorTo(t, u, "g:gpu")
	press(u, "enter")
	v := u.view
	v.Filters = []Filter{{Field: poolField, Values: []string{"default"}, Enabled: true}}
	u.setView(v)
	if out := screen(u); !strings.Contains(out, "No nodes in the focused group.") {
		t.Errorf("focus left nothing:\n%s", out)
	}
	u.view.Search = "zzz"
	u.dirty = true
	if out := screen(u); !strings.Contains(out, "No nodes match the filters or search.") {
		t.Errorf("filters and search left nothing:\n%s", out)
	}
	press(u, "esc", "esc")
	if u.focused() {
		t.Error("esc did not leave an empty focus")
	}
}

// R1: enter on a node inside the focus opens its details.
func TestFocusEnterOnANodeOpensDetails(t *testing.T) {
	u, _ := focusUI(t, poolField)
	cursorTo(t, u, "g:gpu")
	press(u, "enter")
	cursorTo(t, u, "n:g1")
	press(u, "enter")
	if u.panel != panelNode || u.detailsNode != "g1" {
		t.Errorf("enter on a focused node: panel %v node %q", u.panel, u.detailsNode)
	}
}

// R2: the key line.
func TestFocusKeyLine(t *testing.T) {
	u, _ := focusUI(t, poolField)
	if help := u.helpText(); !strings.Contains(help, "enter: focus • space: fold • n: fold") {
		t.Errorf("grouped key line: %q", help)
	}
	cursorTo(t, u, "g:gpu")
	press(u, "enter")
	help := u.helpText()
	if !strings.HasPrefix(help, "esc: back • enter: focus • space: fold • ") || strings.Contains(help, "n: fold") {
		t.Errorf("focused key line: %q", help)
	}
	v := u.view
	v.SetGroupBy()
	u.setView(v)
	if help := u.helpText(); !strings.HasPrefix(help, "esc: back • f: filter") || strings.Contains(help, "enter: focus") {
		t.Errorf("focused with no group-by: %q", help)
	}
	wantListed(t, u, "g1", "g2")
}

// R8: Recommendations ignore the focus (rows, counts, e, esc).
func TestFocusOnRecommendations(t *testing.T) {
	u, _ := focusUI(t, poolField)
	cursorTo(t, u, "g:gpu")
	press(u, "enter", "tab")
	if !u.onRecommendations() {
		t.Fatal("tab did not switch to Recommendations")
	}
	if out := screen(u); !strings.Contains(out, "6 of 6 shown") {
		t.Errorf("Recommendations counts the focus:\n%s", out)
	}
	if doc := u.exportDoc(time.Now()); len(doc.Nodes) != 6 || doc.Focus != nil {
		t.Errorf("export on Recommendations: %d nodes, focus %+v", len(doc.Nodes), doc.Focus)
	}
	press(u, "esc", "tab")
	if !u.focused() {
		t.Fatal("esc on Recommendations left the focus")
	}
	wantListed(t, u, "g1", "g2")
}

// R8: e on Overview exports the focused nodes and the focus.
func TestFocusExport(t *testing.T) {
	u, _ := focusUI(t, poolField)
	raw, _ := json.Marshal(u.exportDoc(time.Now()))
	if strings.Contains(string(raw), `"focus"`) {
		t.Errorf("focus key without a focus: %s", raw)
	}
	cursorTo(t, u, "g:gpu")
	press(u, "enter")
	doc := u.exportDoc(time.Now())
	var names []string
	for _, n := range doc.Nodes {
		names = append(names, n.Name)
	}
	if strings.Join(names, ",") != "g1,g2" {
		t.Errorf("exported %v", names)
	}
	if want := []exportFocus{{Field: "karpenter.sh/nodepool", Value: "gpu"}}; !reflect.DeepEqual(doc.Focus, want) {
		t.Errorf("focus %+v, want %+v", doc.Focus, want)
	}
	raw, _ = json.Marshal(doc)
	if !strings.Contains(string(raw), `"focus":[{"field":"karpenter.sh/nodepool","value":"gpu"}]`) {
		t.Errorf("JSON: %s", raw)
	}
}

// R1, R2, R6 and the counter-example on order: the pod count follows the
// focus, the View line has no Focus without one, the nodes keep their order,
// and the focus stays across a sort change.
func TestFocusPodsOrderAndSort(t *testing.T) {
	u, _ := focusUI(t, poolField)
	if out := screen(u); strings.Contains(out, "Focus:") {
		t.Errorf("View line shows a focus with none:\n%s", out)
	}
	addPods(u,
		podSpec{ns: "default", name: "p1", cpu: "100m", node: "g1"},
		podSpec{ns: "default", name: "p2", cpu: "100m", node: "g2"},
		podSpec{ns: "default", name: "p3", cpu: "100m", node: "a1"},
	)
	cursorTo(t, u, "g:gpu")
	press(u, "enter")
	out := screen(u)
	if !strings.Contains(out, "2 of 6 shown · 2 pods") {
		t.Errorf("pod count under the focus:\n%s", out)
	}
	if strings.Index(out, " g1 ") > strings.Index(out, " g2 ") {
		t.Errorf("nodes reordered by the focus:\n%s", out)
	}
	press(u, "I")
	out = screen(u)
	if !u.focused() || strings.Index(out, " g2 ") > strings.Index(out, " g1 ") {
		t.Errorf("sort flip under the focus:\n%s", out)
	}
	wantListed(t, u, "g1", "g2")
}

// R5: space on a node does nothing.
func TestFocusSpaceOnANode(t *testing.T) {
	u, saves := focusUI(t, poolField)
	cursorTo(t, u, "n:g1")
	press(u, " ")
	if len(*saves) != 0 || len(u.view.Collapsed) != 0 {
		t.Errorf("space on a node folded or saved: %+v", *saves)
	}
}

// R6: a named view saved while focused does not bring the focus back.
func TestFocusNotInANamedView(t *testing.T) {
	u, _ := focusUI(t, poolField)
	cursorTo(t, u, "g:gpu")
	press(u, "enter")
	saved := u.view.forSaving()
	u.SetView(saved)
	if u.focused() {
		t.Error("applying a view saved while focused focused again")
	}
}
