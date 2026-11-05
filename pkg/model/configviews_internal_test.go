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
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// Tests for config-views R3 and R7 in the Views panel, written against
// specs/config-views/requirements.md.

// fixedViews are read-only views listed first, then state views whose names
// they do not have, as config-views R3 describes the panel.
type fixedViews struct {
	names   []string
	fixed   map[string]ViewState
	state   memViews
	saved   []string
	deleted []string
}

func newFixedViews(state memViews, fixed ...namedFixed) *fixedViews {
	f := &fixedViews{fixed: map[string]ViewState{}, state: state}
	for _, v := range fixed {
		f.names = append(f.names, v.name)
		f.fixed[v.name] = v.view
	}
	return f
}

type namedFixed struct {
	name string
	view ViewState
}

func (f *fixedViews) ViewNames() []string {
	names := slices.Clone(f.names)
	for _, n := range f.state.ViewNames() {
		if !f.ReadOnly(n) {
			names = append(names, n)
		}
	}
	return names
}

func (f *fixedViews) View(name string) (ViewState, bool) {
	if v, ok := f.fixed[name]; ok {
		return v, true
	}
	return f.state.View(name)
}

func (f *fixedViews) SaveView(name string, v ViewState) {
	f.saved = append(f.saved, name)
	f.state.SaveView(name, v)
}

func (f *fixedViews) DeleteView(name string) {
	f.deleted = append(f.deleted, name)
	f.state.DeleteView(name)
}

func (f *fixedViews) ReadOnly(name string) bool {
	_, ok := f.fixed[name]
	return ok
}

// viewRows are the Views panel's rows without colours.
func viewRows(t *testing.T, u *UIModel) []string {
	t.Helper()
	var rows []string
	for _, r := range openPanel(t, u).rows {
		rows = append(rows, strings.TrimSpace(ansi.Strip(r)))
	}
	return rows
}

// costView groups by capacity and sorts by price.
func costView() ViewState {
	v := DefaultViewState()
	v.SetGroupBy(FieldCapacity)
	v.Sort = SortSpec{Field: FieldPrice, Descending: true}
	v.Resources = []string{"cpu", "memory"}
	return v
}

func spotView() ViewState {
	v := DefaultViewState()
	v.Filters = []Filter{{Field: FieldCapacity, Values: []string{"Spot"}, Enabled: true}}
	return v
}

func TestViewsPanelListsConfigViewsFirstWithMarker(t *testing.T) {
	u := capacityUI(t, 2)
	state := memViews{"aaa": DefaultViewState(), "cost": DefaultViewState(), "my-saved-view": DefaultViewState()}
	u.SetViews(newFixedViews(state, namedFixed{"spot", spotView()}, namedFixed{"cost", costView()}))
	press(u, "v")
	want := []string{"spot (config)", "cost (config)", "aaa", "my-saved-view", "+ save current view as…"}
	if got := viewRows(t, u); !reflect.DeepEqual(got, want) {
		t.Errorf("rows\n got %q\nwant %q", got, want)
	}
}

func TestViewsPanelWithoutReadOnlyViewsHasNoMarker(t *testing.T) {
	u := capacityUI(t, 2)
	u.SetViews(memViews{"cost": costView()})
	press(u, "v")
	if got := viewRows(t, u); !reflect.DeepEqual(got, []string{"cost", "+ save current view as…"}) {
		t.Errorf("rows %q", got)
	}
}

func TestEnterAppliesAConfigView(t *testing.T) {
	u := capacityUI(t, 4)
	views := newFixedViews(memViews{"cost": DefaultViewState()}, namedFixed{"spot", spotView()}, namedFixed{"cost", costView()})
	u.SetViews(views)
	var reported []ViewState
	u.OnViewChange(func(v ViewState) { reported = append(reported, v) })

	v := u.view
	v.Search = "node"
	v.Header = HeaderCompact
	v.Collapsed = []string{"Spot"}
	u.SetView(v)

	// the second row is the config view cost, not the hidden state view
	press(u, "v", "down", "enter")
	if !reflect.DeepEqual(u.view.GroupBy, []Field{FieldCapacity}) || u.view.Sort.Field != FieldPrice ||
		!reflect.DeepEqual(u.view.Resources, []string{"cpu", "memory"}) {
		t.Errorf("config view not applied: %+v", u.view)
	}
	if u.view.Search != "node" || u.view.Header != HeaderCompact || !reflect.DeepEqual(u.view.Collapsed, []string{"Spot"}) {
		t.Errorf("applying lost search, header or folds: %+v", u.view)
	}
	if len(reported) == 0 || reported[len(reported)-1].Sort.Field != FieldPrice {
		t.Error("the applied config view was not saved as the cluster's view")
	}
	if len(views.saved) != 0 || len(views.deleted) != 0 {
		t.Errorf("applying wrote named views: saved %v deleted %v", views.saved, views.deleted)
	}
	if u.panel != panelNone {
		t.Error("panel still open after applying")
	}

	// the first row, a config view with a filter, applies the same way
	press(u, "v", "enter")
	if len(u.view.Filters) != 1 || !u.view.Filters[0].Enabled || u.view.Filters[0].Field != FieldCapacity {
		t.Errorf("spot filters %+v", u.view.Filters)
	}
}

func TestDeleteRefusedOnAConfigView(t *testing.T) {
	u := capacityUI(t, 2)
	views := newFixedViews(memViews{"mine": DefaultViewState()}, namedFixed{"spot", spotView()})
	u.SetViews(views)
	press(u, "v", "d")
	if len(views.deleted) != 0 {
		t.Errorf("d deleted %v", views.deleted)
	}
	if n := u.currentNotice(); !strings.Contains(n, "config.yaml") || !strings.Contains(n, "spot") {
		t.Errorf("notice %q does not say spot is defined in config.yaml", n)
	}
	if got := viewRows(t, u); !reflect.DeepEqual(got, []string{"spot (config)", "mine", "+ save current view as…"}) {
		t.Errorf("rows after d: %q", got)
	}
	// d on a state view still deletes it
	press(u, "down", "d")
	if !reflect.DeepEqual(views.deleted, []string{"mine"}) {
		t.Errorf("deleted %v, want [mine]", views.deleted)
	}
}

func TestSaveUnderAConfigViewNameRefused(t *testing.T) {
	for _, name := range []string{"spot", "  spot "} {
		t.Run(name, func(t *testing.T) {
			u := capacityUI(t, 2)
			views := newFixedViews(memViews{}, namedFixed{"spot", spotView()})
			u.SetViews(views)
			press(u, "v", "down", "enter")
			typeText(u, name)
			press(u, "enter")
			if len(views.saved) != 0 {
				t.Errorf("saved %v under a config name", views.saved)
			}
			if n := u.currentNotice(); !strings.Contains(n, "config.yaml") {
				t.Errorf("notice %q does not name config.yaml", n)
			}
			if got, _ := views.View("spot"); !reflect.DeepEqual(got, spotView()) {
				t.Errorf("config view changed: %+v", got)
			}
			// another name saves
			press(u, "ctrl+u")
			typeText(u, "spot-2")
			press(u, "enter")
			if !reflect.DeepEqual(views.saved, []string{"spot-2"}) {
				t.Errorf("saved %v, want [spot-2]", views.saved)
			}
		})
	}
}

// TestConfigViewWithHiddenLabelsStaysListed checks config-views R7: a config view that
// uses labels the label lists hide is applied, and the panels list them.
func TestConfigViewWithHiddenLabelsStaysListed(t *testing.T) {
	v := DefaultViewState()
	v.SetGroupBy(LabelField("env"))
	v.Sort = SortSpec{Field: LabelField("kubernetes.io/os")}
	v.Columns = append(slices.Clone(DefaultColumns), LabelField("kubernetes.io/os"))
	v.Filters = []Filter{{Field: LabelField("env"), Values: []string{"prod", "dev"}, Enabled: true}}
	used := map[string][]string{
		"f": {"env"},
		"g": {"env"},
		"s": {"kubernetes.io/os"},
		"o": {"kubernetes.io/os"},
	}
	for _, pn := range labelPanels {
		t.Run(pn.name, func(t *testing.T) {
			u := policyUI(t, []string{"team"}, nil)
			u.SetViews(newFixedViews(memViews{}, namedFixed{"ops", v}))
			press(u, "v", "enter")
			if !reflect.DeepEqual(u.view.GroupBy, []Field{LabelField("env")}) {
				t.Fatalf("view not applied: %+v", u.view)
			}
			press(u, pn.key)
			p := openPanel(t, u)
			for _, label := range used[pn.key] {
				if !lists(p, label) {
					t.Errorf("%s not listed while the view uses it (R7):\n%s", label, strings.Join(p.rows, "\n"))
				}
			}
			if !lists(p, "team") {
				t.Error("the allowed label is gone")
			}
		})
	}
}
