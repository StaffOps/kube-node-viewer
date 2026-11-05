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
	"sort"
	"strings"
	"testing"
)

type memViews map[string]ViewState

func (m memViews) ViewNames() []string {
	var names []string
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
func (m memViews) View(name string) (ViewState, bool) { v, ok := m[name]; return v, ok }
func (m memViews) SaveView(name string, v ViewState)  { m[name] = v }
func (m memViews) DeleteView(name string)             { delete(m, name) }

func typeText(u *UIModel, text string) {
	for _, r := range text {
		press(u, string(r))
	}
}

// views-and-export R2: save the current view under a name, apply it.
func TestSaveAndApplyNamedView(t *testing.T) {
	u := capacityUI(t, 4)
	views := memViews{}
	u.SetViews(views)
	var saved []ViewState
	u.OnViewChange(func(v ViewState) { saved = append(saved, v) })

	v := u.view
	v.SetGroupBy(FieldCapacity)
	v.Search = "node"
	v.Collapsed = []string{"Spot"}
	u.SetView(v)

	// v → "+ save current view as…" (the only row) → type a name with q and d in it
	press(u, "v", "enter")
	typeText(u, "quick d")
	if quits(press(u, "enter")) {
		t.Fatal("enter quit")
	}
	got, ok := views["quick d"]
	if !ok {
		t.Fatalf("not saved: %v", views)
	}
	if got.Search != "" || got.Collapsed != nil || !reflect.DeepEqual(got.GroupBy, []Field{FieldCapacity}) {
		t.Errorf("saved view carries the wrong fields: %+v", got)
	}

	// change the view, then apply the saved one
	v = u.view
	v.GroupBy = nil
	v.Resources = []string{"memory"}
	u.SetView(v)
	press(u, "esc", "v", "enter")
	if !reflect.DeepEqual(u.view.GroupBy, []Field{FieldCapacity}) || u.view.Search != "node" {
		t.Errorf("applied view: %+v", u.view)
	}
	if len(saved) == 0 || !reflect.DeepEqual(saved[len(saved)-1].GroupBy, []Field{FieldCapacity}) {
		t.Error("the applied view was not saved as the cluster's view")
	}
}

func TestApplyingAViewSetsItsResources(t *testing.T) {
	u := capacityUI(t, 2)
	views := memViews{"gpu": {Resources: []string{"cpu", "nvidia.com/gpu"}, ShowNodes: true}}
	u.SetViews(views)
	press(u, "v", "enter")
	if !reflect.DeepEqual(u.view.Resources, []string{"cpu", "nvidia.com/gpu"}) || len(u.cluster.resources) != 2 {
		t.Fatalf("resources %v / %v", u.view.Resources, u.cluster.resources)
	}
	if !strings.Contains(u.View(), "nvidia.com/gpu") {
		t.Error("the view's resources are not drawn")
	}
	// the custom list is the fourth step of r
	press(u, "r", "r", "r", "r")
	if !reflect.DeepEqual(u.view.Resources, []string{"cpu", "nvidia.com/gpu"}) {
		t.Errorf("r cycle lost the view's list: %v", u.view.Resources)
	}
}

// views-and-export R2: d deletes a view; an empty name is refused.
func TestEmptyNameIsRefusedAndDeleteWorks(t *testing.T) {
	u := capacityUI(t, 2)
	views := memViews{"old": DefaultViewState()}
	u.SetViews(views)
	press(u, "v", "down", "enter", " ", " ", "enter")
	if len(views) != 1 || !strings.Contains(u.View(), "a view needs a name") {
		t.Errorf("empty name: %v", views)
	}
	press(u, "esc", "up", "d")
	if len(views) != 0 {
		t.Errorf("d did not delete: %v", views)
	}
}

func TestSavingUnderAnExistingNameReplacesIt(t *testing.T) {
	u := capacityUI(t, 2)
	views := memViews{"x": DefaultViewState()}
	u.SetViews(views)
	v := u.view
	v.Sort = SortSpec{Field: FieldPrice, Descending: true}
	u.SetView(v)
	press(u, "v", "down", "enter", "x", "enter")
	if views["x"].Sort.Field != FieldPrice {
		t.Errorf("not replaced: %+v", views["x"])
	}
}
