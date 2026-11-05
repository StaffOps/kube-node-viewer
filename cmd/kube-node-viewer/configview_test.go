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

package main

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/staffops/kube-node-viewer/pkg/model"
	"github.com/staffops/kube-node-viewer/pkg/state"
)

// Tests for config-views R1-R6 and R8, written against
// specs/config-views/requirements.md and the design's test plan.

// oneView is a config file with a single view named "a"; each line is a key
// of the view, written as it sits under the item (four spaces in).
func oneView(lines ...string) string {
	var b strings.Builder
	b.WriteString("views:\n  - name: a\n")
	for _, l := range lines {
		for _, sub := range strings.Split(l, "\n") {
			b.WriteString("    " + sub + "\n")
		}
	}
	return b.String()
}

// loadOK loads the body and fails the test on an error.
func loadOK(t *testing.T, body string) settings {
	t.Helper()
	s, err := loadSettings(writeSettings(t, body), nil)
	if err != nil {
		t.Fatalf("loadSettings: %v\n%s", err, body)
	}
	return s
}

// onlyView loads a file with one view and returns it.
func onlyView(t *testing.T, body string) model.ViewState {
	t.Helper()
	s := loadOK(t, body)
	if len(s.views) != 1 {
		t.Fatalf("%d views, want 1", len(s.views))
	}
	return s.views[0].view
}

// config-views R1: a key left out of a view takes the built-in default.
func TestConfigViewKeyLeftOutTakesTheBuiltInDefault(t *testing.T) {
	got := onlyView(t, oneView())
	if !reflect.DeepEqual(got, model.DefaultViewState()) {
		t.Errorf("view with only a name:\n got %+v\nwant %+v", got, model.DefaultViewState())
	}
}

// config-views R1: each key of a view read from config.yaml.
func TestConfigViewKeys(t *testing.T) {
	gpu := "nvidia.com/gpu"
	tests := []struct {
		name  string
		lines []string
		check func(t *testing.T, v model.ViewState)
	}{
		{
			name:  "columns in screen order",
			lines: []string{"columns:\n  - zone\n  - price\n  - instance-type"},
			check: func(t *testing.T, v model.ViewState) {
				want := []model.Field{model.FieldZone, model.FieldPrice, model.FieldInstanceType}
				if !reflect.DeepEqual(v.Columns, want) {
					t.Errorf("columns %v, want %v", v.Columns, want)
				}
			},
		},
		{
			name:  "columns as a flow list",
			lines: []string{"columns: [price, zone]"},
			check: func(t *testing.T, v model.ViewState) {
				if want := []model.Field{model.FieldPrice, model.FieldZone}; !reflect.DeepEqual(v.Columns, want) {
					t.Errorf("columns %v, want %v", v.Columns, want)
				}
			},
		},
		{
			name:  "group-by one level",
			lines: []string{"group-by:\n  - zone"},
			check: func(t *testing.T, v model.ViewState) {
				if want := []model.Field{model.FieldZone}; !reflect.DeepEqual(v.GroupBy, want) {
					t.Errorf("group-by %v, want %v", v.GroupBy, want)
				}
			},
		},
		{
			name:  "group-by three levels, top first",
			lines: []string{"group-by:\n  - nodegroup\n  - zone\n  - karpenter.sh/nodepool"},
			check: func(t *testing.T, v model.ViewState) {
				want := []model.Field{model.FieldNodeGroup, model.FieldZone, model.LabelField(pool)}
				if !reflect.DeepEqual(v.GroupBy, want) {
					t.Errorf("group-by %v, want %v", v.GroupBy, want)
				}
			},
		},
		{
			name:  "resources in full",
			lines: []string{"resources:\n  - cpu\n  - memory\n  - " + gpu},
			check: func(t *testing.T, v model.ViewState) {
				if want := []string{"cpu", "memory", gpu}; !reflect.DeepEqual(v.Resources, want) {
					t.Errorf("resources %v, want %v", v.Resources, want)
				}
			},
		},
		{
			name: "filters keep and exclude, turned on",
			lines: []string{"filters:\n" +
				"  - field: capacity\n    values:\n      - Spot\n      - OnDemand\n" +
				"  - field: zone\n    exclude:\n      - us-east-1c\n" +
				"  - field: karpenter.sh/nodepool\n    values:\n      - default"},
			check: func(t *testing.T, v model.ViewState) {
				want := []model.Filter{
					{Field: model.FieldCapacity, Values: []string{"Spot", "OnDemand"}, Enabled: true},
					{Field: model.FieldZone, Excluded: []string{"us-east-1c"}, Enabled: true},
					{Field: model.LabelField(pool), Values: []string{"default"}, Enabled: true},
				}
				if !reflect.DeepEqual(v.Filters, want) {
					t.Errorf("filters\n got %+v\nwant %+v", v.Filters, want)
				}
			},
		},
		{
			name:  "groups-only true folds to the groups",
			lines: []string{"group-by: [zone]", "groups-only: true"},
			check: func(t *testing.T, v model.ViewState) {
				if v.ShowNodes {
					t.Error("groups-only: true shows the node list")
				}
			},
		},
		{
			name:  "groups-only false keeps the node list",
			lines: []string{"group-by: [zone]", "groups-only: false"},
			check: func(t *testing.T, v model.ViewState) {
				if !v.ShowNodes {
					t.Error("groups-only: false hides the node list")
				}
			},
		},
		{
			name:  "groups-only false without group-by is fine",
			lines: []string{"groups-only: false"},
			check: func(t *testing.T, v model.ViewState) {
				if !v.ShowNodes {
					t.Error("node list hidden")
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, onlyView(t, oneView(tt.lines...)))
		})
	}
}

func TestConfigViewGroupsOnlyYAMLBooleans(t *testing.T) {
	for word, groupsOnly := range map[string]bool{
		"true": true, "false": false, "yes": true, "no": false, "on": true, "off": false,
	} {
		t.Run(word, func(t *testing.T) {
			v := onlyView(t, oneView("group-by: [zone]", "groups-only: "+word))
			if v.ShowNodes == groupsOnly {
				t.Errorf("groups-only: %s gives ShowNodes %v", word, v.ShowNodes)
			}
		})
	}
}

func TestConfigViewSortOrder(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  model.SortSpec
	}{
		{"no sort no order", nil, model.DefaultViewState().Sort},
		{"creation descending first", []string{"sort: creation"}, model.SortSpec{Field: model.FieldCreation, Descending: true}},
		{"price descending first", []string{"sort: price"}, model.SortSpec{Field: model.FieldPrice, Descending: true}},
		{"idle descending first", []string{"sort: idle"}, model.SortSpec{Field: model.FieldIdle, Descending: true}},
		{"pods descending first", []string{"sort: pods"}, model.SortSpec{Field: model.FieldPods, Descending: true}},
		{"usage descending first", []string{"sort: usage:cpu"}, model.SortSpec{Field: model.UsageField("cpu"), Descending: true}},
		{"usage of an extended resource", []string{"sort: usage:nvidia.com/gpu"}, model.SortSpec{Field: model.UsageField("nvidia.com/gpu"), Descending: true}},
		{"name ascending first", []string{"sort: name"}, model.SortSpec{Field: model.FieldName}},
		{"ip ascending first", []string{"sort: ip"}, model.SortSpec{Field: model.FieldIP}},
		{"label ascending first", []string{"sort: " + pool}, model.SortSpec{Field: model.LabelField(pool)}},
		{"price ascending", []string{"sort: price", "order: ascending"}, model.SortSpec{Field: model.FieldPrice}},
		{"name descending", []string{"sort: name", "order: descending"}, model.SortSpec{Field: model.FieldName, Descending: true}},
		{"order before sort in the file", []string{"order: ascending", "sort: pods"}, model.SortSpec{Field: model.FieldPods}},
		{"order without sort sets creation ascending", []string{"order: ascending"}, model.SortSpec{Field: model.FieldCreation}},
		{"order without sort sets creation descending", []string{"order: descending"}, model.SortSpec{Field: model.FieldCreation, Descending: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := onlyView(t, oneView(tt.lines...)).Sort; got != tt.want {
				t.Errorf("sort %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestConfigViewFieldNamesPerKey checks config-views R2: each key's built-in names, any
// other name a label, label: forcing a label.
func TestConfigViewFieldNamesPerKey(t *testing.T) {
	label := model.LabelField
	type keyOf func(v model.ViewState) model.Field
	column := func(v model.ViewState) model.Field { return v.Columns[0] }
	group := func(v model.ViewState) model.Field { return v.GroupBy[0] }
	filter := func(v model.ViewState) model.Field { return v.Filters[0].Field }
	sorted := func(v model.ViewState) model.Field { return v.Sort.Field }
	line := map[string]func(string) string{
		"columns":  func(n string) string { return "columns:\n  - " + n },
		"group-by": func(n string) string { return "group-by:\n  - " + n },
		"filters":  func(n string) string { return "filters:\n  - field: " + n + "\n    values:\n      - x" },
		"sort":     func(n string) string { return "sort: " + n },
	}
	read := map[string]keyOf{"columns": column, "group-by": group, "filters": filter, "sort": sorted}
	tests := []struct {
		key, name string
		want      model.Field
	}{}
	add := func(key string, want map[string]model.Field) {
		for n, f := range want {
			tests = append(tests, struct {
				key, name string
				want      model.Field
			}{key, n, f})
		}
	}
	derived := map[string]model.Field{
		"capacity": model.FieldCapacity, "nodegroup": model.FieldNodeGroup, "zone": model.FieldZone,
		"region": model.FieldRegion, "instance-type": model.FieldInstanceType, "status": model.FieldStatus,
	}
	common := map[string]model.Field{
		pool:         label(pool),
		"label:zone": label("zone"),
		"team":       label("team"),
		// a typo of a built-in is read as a label (README)
		"zon": label("zon"),
	}
	for _, key := range []string{"group-by", "filters"} {
		add(key, derived)
		add(key, common)
	}
	add("sort", map[string]model.Field{
		"creation": model.FieldCreation, "name": model.FieldName, "ip": model.FieldIP, "price": model.FieldPrice,
		"idle": model.FieldIdle, "pods": model.FieldPods, "usage:cpu": model.UsageField("cpu"),
		"usage:memory": model.UsageField("memory"),
	})
	add("sort", common)
	add("columns", map[string]model.Field{
		"instance-type": model.FieldInstanceType, "price": model.FieldPrice, "capacity": model.FieldCapacity,
		"nodegroup": model.FieldNodeGroup, "zone": model.FieldZone, "status": model.FieldStatus,
		"idle": model.FieldIdle, "region": model.FieldRegion, "age": model.FieldAge, "ip": model.FieldIP,
	})
	add("columns", common)
	for _, tt := range tests {
		t.Run(tt.key+"/"+tt.name, func(t *testing.T) {
			v := onlyView(t, oneView(line[tt.key](tt.name)))
			if got := read[tt.key](v); got != tt.want {
				t.Errorf("%s: %s read as %q, want %q", tt.key, tt.name, got, tt.want)
			}
		})
	}
}

// TestConfigViewErrors checks every config-views R6 case: the start fails with the file
// and the path of the problem.
func TestConfigViewErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
		// want is the start of the message after "config file <path>: "
		want string
	}{
		// unknown keys, compared exactly
		{"unknown top key", "vews: []\n", `unknown field "vews"`},
		{"top key case", "Views: []\n", `unknown field "Views"`},
		{"default-view case", "Default-View: a\n", `unknown field "Default-View"`},
		{"unknown view key", oneView("colums: [price]"), `views[0]: unknown field "colums"`},
		{"view key case", "views:\n  - Name: a\n", `views[0]: unknown field "Name"`},
		{"unknown filter key", oneView("filters:\n  - field: zone\n    valeus: [a]"), `views[0].filters[0]: unknown field "valeus"`},
		{"filter key case", oneView("filters:\n  - Field: zone\n    values: [a]"), `views[0].filters[0]: unknown field "Field"`},

		// wrong types
		{"views not a list", "views: cost\n", "views: cost is not a list"},
		{"view not a map", "views:\n  - cost\n", "views[0] is not a map"},
		{"name yes", "views:\n  - name: yes\n", "views[0].name: true is not a string; quote it"},
		{"name number", "views:\n  - name: 123\n", "views[0].name: 123 is not a string; quote it"},
		{"name tilde", "views:\n  - name: ~\n", "views[0].name: null is not a string; quote it"},
		{"column yes", oneView("columns: [yes]"), "views[0].columns[0]: true is not a string; quote it"},
		{"group-by number", oneView("group-by:\n  - zone\n  - 123"), "views[0].group-by[1]: 123 is not a string; quote it"},
		{"sort number", oneView("sort: 123"), "views[0].sort: 123 is not a string; quote it"},
		{"order yes", oneView("order: yes"), "views[0].order: true is not a string; quote it"},
		{"resource tilde", oneView("resources:\n  - cpu\n  - ~"), "views[0].resources[1]: null is not a string; quote it"},
		{"resources a string", oneView("resources: cpu"), "views[0].resources: cpu is not a list"},
		{"columns a map", oneView("columns:\n  price: 1"), "views[0].columns: "},
		{"filters not a list", oneView("filters: zone"), "views[0].filters: zone is not a list"},
		{"filter not a map", oneView("filters: [zone]"), "views[0].filters[0] is not a map"},
		{"filter field number", oneView("filters:\n  - field: 1\n    values: [a]"), "views[0].filters[0].field: 1 is not a string; quote it"},
		{"filter value number", oneView("filters:\n  - field: zone\n    values: [123]"), "views[0].filters[0].values[0]: 123 is not a string; quote it"},
		{"filter exclude yes", oneView("filters:\n  - field: zone\n    exclude: [a, no]"), "views[0].filters[0].exclude[1]: false is not a string; quote it"},
		{"groups-only a word", oneView("group-by: [zone]", "groups-only: maybe"), "views[0].groups-only: maybe is not true or false"},
		{"groups-only quoted", oneView("group-by: [zone]", `groups-only: "true"`), "views[0].groups-only: true is not true or false"},
		{"default-view number", "default-view: 123\n", "default-view: 123 is not a string; quote it"},
		{"start-with yes", "start-with: yes\n", "start-with: true is not a string; quote it"},

		// names
		{"missing name", "views:\n  - sort: price\n", "views[0]: needs a name"},
		{"empty name", "views:\n  - name: \"\"\n", "views[0].name: empty"},
		{"blank name", "views:\n  - name: \"  \"\n", "views[0].name: empty"},
		{"repeated name", "views:\n  - name: a\n  - name: b\n  - name: a\n", `views[2].name: "a" is repeated`},
		{"second view without name", "views:\n  - name: a\n  - columns: [price]\n", "views[1]: needs a name"},

		// empty lists, repeats, levels
		{"empty columns", oneView("columns: []"), "views[0].columns: empty list"},
		{"null columns", oneView("columns:"), "views[0].columns: empty list"},
		{"empty group-by", oneView("group-by: []"), "views[0].group-by: empty list"},
		{"empty resources", oneView("resources: []"), "views[0].resources: empty list"},
		{"empty filters", oneView("filters: []"), "views[0].filters: empty list"},
		{"empty values", oneView("filters:\n  - field: zone\n    values: []"), "views[0].filters[0].values: empty list"},
		{"empty exclude", oneView("filters:\n  - field: zone\n    exclude: []"), "views[0].filters[0].exclude: empty list"},
		{"repeated column", oneView("columns: [price, zone, price]"), "views[0].columns[2]: price is repeated"},
		{"repeated column as label", oneView("columns: [team, label:team]"), "views[0].columns[1]: "},
		{"repeated group-by", oneView("group-by: [zone, zone]"), "views[0].group-by[1]: zone is repeated"},
		{"repeated resource", oneView("resources: [cpu, memory, cpu]"), "views[0].resources[2]: cpu is repeated"},
		{"repeated value", oneView("filters:\n  - field: zone\n    values: [a, b, a]"), "views[0].filters[0].values[2]: a is repeated"},
		{"repeated exclude", oneView("filters:\n  - field: zone\n    exclude: [a, a]"), "views[0].filters[0].exclude[1]: a is repeated"},
		{"four group-by levels", oneView("group-by: [zone, region, capacity, status]"), "views[0].group-by: "},
		{"empty item", oneView(`columns: [price, ""]`), "views[0].columns[1]: empty"},

		// built-in names in a key that does not list them
		{"group-by price", oneView("group-by: [price]"), "views[0].group-by[0]: "},
		{"group-by pods", oneView("group-by: [zone, pods]"), "views[0].group-by[1]: "},
		{"group-by usage", oneView("group-by: [usage:cpu]"), "views[0].group-by[0]: "},
		{"filter on price", oneView("filters:\n  - field: price\n    values: [a]"), "views[0].filters[0].field: "},
		{"filter on age", oneView("filters:\n  - field: age\n    values: [a]"), "views[0].filters[0].field: "},
		{"padded name", "views:\n  - name: \" a \"\n", "views[0].name: "},
		{"sort age", oneView("sort: age"), "views[0].sort: "},
		{"column name", oneView("columns: [name]"), "views[0].columns[0]: "},
		{"column creation", oneView("columns: [price, creation]"), "views[0].columns[1]: "},
		{"column pods", oneView("columns: [pods]"), "views[0].columns[0]: "},
		{"column usage", oneView("columns: [usage:cpu]"), "views[0].columns[0]: "},
		{"label prefix without key", oneView("columns: [\"label:\"]"), "views[0].columns[0]: "},
		{"usage without resource", oneView("sort: \"usage:\""), "views[0].sort: "},

		// order and start-with values
		{"order up", oneView("order: up"), `views[0].order: "up"`},
		{"order case", oneView("order: Descending"), `views[0].order: "Descending"`},
		{"start-with first", "views:\n  - name: a\ndefault-view: a\nstart-with: first\n", `start-with: "first"`},

		// filters
		{"filter without field", oneView("filters:\n  - values: [a]"), "views[0].filters[0]: needs field"},
		{"filter with both", oneView("filters:\n  - field: zone\n    values: [a]\n    exclude: [b]"), "views[0].filters[0]: "},
		{"filter with neither", "views:\n  - name: a\n  - name: b\n    filters:\n      - field: zone\n", "views[1].filters[0]: needs values or exclude"},
		{"second filter on a field", oneView("filters:\n  - field: zone\n    values: [a]\n  - field: zone\n    exclude: [b]"), "views[0].filters[1].field: "},
		{"second filter on a label, once with label:", oneView("filters:\n  - field: team\n    values: [a]\n  - field: label:team\n    values: [b]"), "views[0].filters[1].field: "},

		// groups-only
		{"groups-only without group-by", oneView("groups-only: true"), "views[0].groups-only: "},
		{"groups-only yes without group-by", oneView("groups-only: yes"), "views[0].groups-only: "},

		// start view
		{"default-view naming nothing", "views:\n  - name: a\ndefault-view: b\n", `default-view: no view is named "b"`},
		{"default-view without views", "default-view: cost\n", "default-view: "},
		{"default-view empty", "views:\n  - name: a\ndefault-view: \"\"\n", "default-view: empty"},
		{"start-with default-view without default-view", "views:\n  - name: a\nstart-with: default-view\n", "start-with: "},
		{"start-with default-view without anything", "start-with: default-view\n", "start-with: "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeSettings(t, tt.body)
			_, err := loadSettings(path, nil)
			if err == nil {
				t.Fatalf("no error for\n%s", tt.body)
			}
			prefix := "config file " + path + ": " + tt.want
			if !strings.HasPrefix(err.Error(), prefix) {
				t.Errorf("error\n %q\nwant it to start with\n %q", err, prefix)
			}
		})
	}
}

// TestConfigViewErrorSuggestsLabelPrefix: a built-in in the wrong key says
// how to name the label of that name.
func TestConfigViewErrorSuggestsLabelPrefix(t *testing.T) {
	_, err := loadSettings(writeSettings(t, oneView("group-by: [price]")), nil)
	if err == nil || !strings.Contains(err.Error(), "label:price") {
		t.Errorf("error %v does not name label:price", err)
	}
	if v := onlyView(t, oneView("group-by: [label:price]")); !reflect.DeepEqual(v.GroupBy, []model.Field{model.LabelField("price")}) {
		t.Errorf("label:price grouped by %v", v.GroupBy)
	}
}

func TestConfigViewErrorsDoNotBreakTheLabelLists(t *testing.T) {
	// a valid file with both parts keeps the label lists working
	s := loadOK(t, "labels:\n  allow:\n    - team\nviews:\n  - name: a\n")
	if got := shown(s.policy); !reflect.DeepEqual(got, []string{"team"}) {
		t.Errorf("shows %v", got)
	}
	// the label-list messages are unchanged
	_, err := loadSettings(writeSettings(t, "labels:\n  allow:\n    - yes\nviews:\n  - name: a\n"), nil)
	if err == nil || !strings.Contains(err.Error(), "labels.allow[0]: true is not a string; quote it") {
		t.Errorf("label error %v", err)
	}
}

// config-views R4, R5: default-view and start-with read and checked.
func TestConfigViewStartSettings(t *testing.T) {
	two := "views:\n  - name: a\n    sort: price\n  - name: b\n    sort: name\n"
	tests := []struct {
		name     string
		body     string
		lastView bool
		// def is the default view's sort field, "" for none
		def model.Field
	}{
		{"nothing", "", false, ""},
		{"views only", two, false, ""},
		{"default-view only", two + "default-view: b\n", false, model.FieldName},
		{"default-view and start-with default-view", two + "default-view: a\nstart-with: default-view\n", false, model.FieldPrice},
		{"last-view with default-view", two + "default-view: a\nstart-with: last-view\n", true, model.FieldPrice},
		{"last-view without default-view", two + "start-with: last-view\n", true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := loadOK(t, tt.body)
			if s.lastView != tt.lastView {
				t.Errorf("lastView %v, want %v", s.lastView, tt.lastView)
			}
			switch {
			case tt.def == "" && s.defaultView != nil:
				t.Errorf("default view %+v, want none", *s.defaultView)
			case tt.def != "" && s.defaultView == nil:
				t.Error("no default view")
			case tt.def != "" && s.defaultView.Sort.Field != tt.def:
				t.Errorf("default view sorts on %s, want %s", s.defaultView.Sort.Field, tt.def)
			}
		})
	}
}

// config-views R1: views keep the file's order.
func TestConfigViewsKeepFileOrder(t *testing.T) {
	s := loadOK(t, "views:\n  - name: zeta\n  - name: alpha\n  - name: mid\n")
	var names []string
	for _, v := range s.views {
		names = append(names, v.name)
	}
	if want := []string{"zeta", "alpha", "mid"}; !reflect.DeepEqual(names, want) {
		t.Errorf("views %v, want %v", names, want)
	}
}

// --- R4 / R5: the start view ---

// startView is a view nothing else in these tests produces.
func startView() model.ViewState {
	v := model.DefaultViewState()
	v.SetGroupBy(model.FieldNodeGroup, model.FieldZone)
	v.Sort = model.SortSpec{Field: model.FieldPrice, Descending: true}
	v.Resources = []string{"cpu", "memory"}
	v.Columns = []model.Field{model.FieldInstanceType, model.FieldPrice}
	return v
}

// savedWithUI is a saved view with a header size, a screen and folded groups.
func savedWithUI() *model.ViewState {
	v := model.DefaultViewState()
	v.SetGroupBy(model.LabelField("team"))
	v.Resources = []string{"memory"}
	v.Header = model.HeaderCompact
	v.Screen = model.ScreenRecommendations
	v.Collapsed = []string{"red"}
	return &v
}

// config-views R4, R5: the start view against flags and saved state.
func TestResolveViewWithStartView(t *testing.T) {
	cfg := configFile{"resources": "memory", "group-by": pool, "node-sort": "name", "extra-labels": "team", "groups-only": "true"}
	t.Run("replaces defaults and the config file", func(t *testing.T) {
		start := startView()
		got, err := resolveView(cfg, nil, &start, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, startView()) {
			t.Errorf("view\n got %+v\nwant %+v", got, startView())
		}
	})
	t.Run("keeps header, screen and folded groups of the saved view only", func(t *testing.T) {
		start := startView()
		got, err := resolveView(cfg, savedWithUI(), &start, nil)
		if err != nil {
			t.Fatal(err)
		}
		want := startView()
		want.Header, want.Screen, want.Collapsed = model.HeaderCompact, model.ScreenRecommendations, []string{"red"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("view\n got %+v\nwant %+v", got, want)
		}
	})
	t.Run("explicit flags on top", func(t *testing.T) {
		start := startView()
		got, err := resolveView(cfg, savedWithUI(), &start, map[string]string{"resources": "nvidia.com/gpu", "group-by": "team"})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got.Resources, []string{"nvidia.com/gpu"}) || !reflect.DeepEqual(got.GroupBy, []model.Field{model.LabelField("team")}) {
			t.Errorf("flags lost: resources %v, group-by %v", got.Resources, got.GroupBy)
		}
		if !reflect.DeepEqual(got.Columns, startView().Columns) {
			t.Errorf("columns %v, want the start view's", got.Columns)
		}
	})
	t.Run("the result does not share the start view", func(t *testing.T) {
		start := startView()
		got, err := resolveView(nil, nil, &start, nil)
		if err != nil {
			t.Fatal(err)
		}
		got.Columns[0] = model.FieldZone
		got.GroupBy[0] = model.FieldStatus
		got.Resources[0] = "x"
		if !reflect.DeepEqual(start, startView()) {
			t.Errorf("start view changed through the result: %+v", start)
		}
	})
	t.Run("without a start view, as before", func(t *testing.T) {
		got, err := resolveView(cfg, savedWithUI(), nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, *savedWithUI()) {
			t.Errorf("saved view not used: %+v", got)
		}
	})
}

// --- R4 / R5 through sessions: first connection, c and back ---

const testKubeconfig = `apiVersion: v1
kind: Config
clusters:
- name: one
  cluster:
    server: https://127.0.0.1:1
- name: two
  cluster:
    server: https://127.0.0.1:2
contexts:
- name: a
  context:
    cluster: one
    user: u
- name: b
  context:
    cluster: two
    user: u
current-context: a
users:
- name: u
  user:
    token: not-a-token
`

// Cluster keys of the two contexts of testKubeconfig.
const (
	keyA = "a@https://127.0.0.1:1"
	keyB = "b@https://127.0.0.1:2"
)

// runOf is one run of the viewer: sessions over a kubeconfig whose servers
// listen nowhere, with pricing off, sharing a state store across runs.
type runOf struct {
	t      *testing.T
	conn   *sessions
	states *state.Store
}

func newRun(t *testing.T, states *state.Store, cfg configFile, explicit map[string]string, start *model.ViewState, lastView bool) *runOf {
	t.Helper()
	path := filepath.Join(t.TempDir(), "kubeconfig")
	if err := os.WriteFile(path, []byte(testKubeconfig), 0o600); err != nil {
		t.Fatal(err)
	}
	saver := state.NewSaver(states, time.Hour, func(error) {})
	conn := &sessions{
		flags:        Flags{Kubeconfig: path, DisablePricing: true, Config: cfg},
		nodeSelector: labels.Everything(),
		states:       states,
		saver:        saver,
		program:      tea.NewProgram(nil),
		explicit:     explicit,
		starts:       newStartViews(start, lastView),
	}
	t.Cleanup(func() {
		conn.Close()
		saver.Close()
	})
	return &runOf{t: t, conn: conn, states: states}
}

func (r *runOf) open(name string) model.Session {
	r.t.Helper()
	s, err := r.conn.Switch(name)
	if err != nil {
		r.t.Fatalf("Switch(%s): %v", name, err)
	}
	return s
}

func newStates(t *testing.T) *state.Store {
	return state.Load(filepath.Join(t.TempDir(), "state.json"), io.Discard)
}

// config-views R4: start-with default-view on first connection, c and back.
func TestSessionsDefaultViewMode(t *testing.T) {
	states := newStates(t)
	states.Put(keyA, *savedWithUI())
	start := startView()
	explicit := map[string]string{"resources": "nvidia.com/gpu"}
	run := newRun(t, states, configFile{"resources": "memory"}, explicit, &start, false)

	// first cluster: the default view, the saved header/screen/folds, the flags
	first := run.open("a")
	want := startView()
	want.Header, want.Screen, want.Collapsed = model.HeaderCompact, model.ScreenRecommendations, []string{"red"}
	want.Resources = []string{"nvidia.com/gpu"}
	if !reflect.DeepEqual(first.View, want) {
		t.Fatalf("first connection\n got %+v\nwant %+v", first.View, want)
	}

	// a change in the TUI is saved per cluster
	changed := first.View
	changed.SetGroupBy(model.FieldStatus)
	first.Save(changed)
	if got, ok := states.Get(keyA); !ok || !reflect.DeepEqual(got.GroupBy, []model.Field{model.FieldStatus}) {
		t.Errorf("change not saved for the cluster: %+v", got)
	}

	// c to another cluster: its first connection, the default view, no flags
	other := run.open("b")
	if !reflect.DeepEqual(other.View, startView()) {
		t.Errorf("second cluster\n got %+v\nwant %+v (flags apply to the first cluster only)", other.View, startView())
	}

	// c back: as it was left in this run, not the default view again
	back := run.open("a")
	if !reflect.DeepEqual(back.View, changed) {
		t.Errorf("back to the first cluster\n got %+v\nwant %+v", back.View, changed)
	}

	// and back to b, unchanged: still what it showed
	if again := run.open("b"); !reflect.DeepEqual(again.View, startView()) {
		t.Errorf("back to b\n got %+v\nwant %+v", again.View, startView())
	}

	// the next run opens the default view again, header etc. from the save
	next := newRun(t, states, nil, nil, &start, false)
	got := next.open("a").View
	want = startView()
	want.Header, want.Screen, want.Collapsed = changed.Header, changed.Screen, changed.Collapsed
	if !reflect.DeepEqual(got, want) {
		t.Errorf("next run\n got %+v\nwant %+v", got, want)
	}
}

func TestSessionsDefaultViewModeBackWithoutChanges(t *testing.T) {
	// a cluster opened with the default view and left alone comes back as
	// it was, even though nothing was saved for it
	states := newStates(t)
	start := startView()
	run := newRun(t, states, nil, map[string]string{"resources": "memory"}, &start, false)
	first := run.open("a").View
	run.open("b")
	if back := run.open("a").View; !reflect.DeepEqual(back, first) {
		t.Errorf("back\n got %+v\nwant %+v", back, first)
	}
}

// config-views R5: start-with last-view opens each cluster's saved view.
func TestSessionsLastViewMode(t *testing.T) {
	states := newStates(t)
	states.Put(keyA, *savedWithUI())
	start := startView()
	run := newRun(t, states, configFile{"resources": "memory"}, nil, &start, true)

	if got := run.open("a").View; !reflect.DeepEqual(got, *savedWithUI()) {
		t.Errorf("saved cluster\n got %+v\nwant its saved view %+v", got, *savedWithUI())
	}
	second := run.open("b")
	if !reflect.DeepEqual(second.View, startView()) {
		t.Errorf("cluster without a saved view\n got %+v\nwant the default view", second.View)
	}
	changed := second.View
	changed.Resources = []string{"pods"}
	second.Save(changed)
	run.open("a")
	if back := run.open("b").View; !reflect.DeepEqual(back, changed) {
		t.Errorf("back to b\n got %+v\nwant %+v", back, changed)
	}
	// next run: b has a saved view now
	next := newRun(t, states, nil, nil, &start, true)
	if got := next.open("b").View; !reflect.DeepEqual(got.Resources, []string{"pods"}) {
		t.Errorf("next run b: %+v", got)
	}
}

// config-views R5: last-view with no default-view.
func TestSessionsLastViewModeWithoutDefaultView(t *testing.T) {
	states := newStates(t)
	run := newRun(t, states, configFile{"resources": "memory"}, nil, nil, true)
	got := run.open("a").View
	if !reflect.DeepEqual(got.Resources, []string{"memory"}) {
		t.Errorf("no saved view, no default view: resources %v, want the config file's", got.Resources)
	}
}

func TestSessionsWithoutConfigViewsBehaveAsBefore(t *testing.T) {
	states := newStates(t)
	states.Put(keyA, *savedWithUI())
	run := newRun(t, states, configFile{"resources": "memory"}, map[string]string{"node-sort": "name"}, nil, false)
	got := run.open("a").View
	want := *savedWithUI()
	want.Sort = model.SortSpec{Field: model.LabelField("name")}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("saved cluster\n got %+v\nwant %+v", got, want)
	}
	if got := run.open("b").View; !reflect.DeepEqual(got.Resources, []string{"memory"}) || got.Sort != model.DefaultViewState().Sort {
		t.Errorf("unsaved cluster: %+v (flags must not reach it)", got)
	}
	// back to a reads the saved view again, as before
	if got := run.open("a").View; !reflect.DeepEqual(got.GroupBy, savedWithUI().GroupBy) || got.Sort != model.DefaultViewState().Sort {
		t.Errorf("back to a: %+v", got)
	}
}

// --- R3: config views next to the state's named views ---

// stateViews is the state file's named views, in memory, sorted by name.
type stateViews map[string]model.ViewState

func (m stateViews) ViewNames() []string {
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	slices.Sort(names)
	return names
}
func (m stateViews) View(name string) (model.ViewState, bool) { v, ok := m[name]; return v, ok }
func (m stateViews) SaveView(name string, v model.ViewState)  { m[name] = v }
func (m stateViews) DeleteView(name string)                   { delete(m, name) }

// config-views R3: config views listed next to the state's named views.
func TestCombinedViews(t *testing.T) {
	cfg := loadOK(t, "views:\n  - name: spot\n    sort: name\n  - name: cost\n    sort: price\n").views
	stateView := model.DefaultViewState()
	stateView.Resources = []string{"memory"}
	st := stateViews{"cost": stateView, "mine": stateView, "aaa": stateView}
	c := combinedViews{config: cfg, state: st}

	if got, want := c.ViewNames(), []string{"spot", "cost", "aaa", "mine"}; !reflect.DeepEqual(got, want) {
		t.Errorf("names %v, want %v (config first in file order, clash hidden)", got, want)
	}
	if v, ok := c.View("cost"); !ok || v.Sort.Field != model.FieldPrice {
		t.Errorf("cost is the state view, not the config view: %+v", v)
	}
	if v, ok := c.View("mine"); !ok || !reflect.DeepEqual(v.Resources, []string{"memory"}) {
		t.Errorf("state view: %+v %v", v, ok)
	}
	if _, ok := c.View("none"); ok {
		t.Error("an unknown name has a view")
	}
	for name, ro := range map[string]bool{"spot": true, "cost": true, "mine": false, "none": false} {
		if c.ReadOnly(name) != ro {
			t.Errorf("ReadOnly(%s) = %v", name, !ro)
		}
	}
	var _ model.ReadOnlyViews = c

	// save and delete never reach the state file for a config name
	c.SaveView("spot", stateView)
	c.DeleteView("cost")
	if _, ok := st["spot"]; ok {
		t.Error("a save under a config name reached the state file")
	}
	if _, ok := st["cost"]; !ok {
		t.Error("the hidden state view was deleted; it must stay in the file")
	}
	// they do for a state name
	c.SaveView("new", stateView)
	c.DeleteView("aaa")
	if _, ok := st["new"]; !ok {
		t.Error("save of a state name lost")
	}
	if _, ok := st["aaa"]; ok {
		t.Error("delete of a state name lost")
	}

	// a change to an applied config view never reaches the config view
	v, _ := c.View("spot")
	v.Columns[0] = model.FieldZone
	v.Resources[0] = "x"
	if again, _ := c.View("spot"); !reflect.DeepEqual(again, cfg[0].view) || again.Columns[0] == model.FieldZone {
		t.Errorf("config view changed through a copy: %+v", again)
	}
}

// config-views R3: the Views panel lists the combined views.
func TestCombinedViewsIsWhatTheViewsPanelUses(t *testing.T) {
	// the UI model applies a config view through combinedViews
	cfg := loadOK(t, "views:\n  - name: spot\n    group-by: [capacity]\n").views
	u := model.NewUIModel(mustStyle(t))
	u.SetViews(combinedViews{config: cfg, state: stateViews{}})
	u.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	if !strings.Contains(u.View(), "spot (config)") {
		t.Errorf("Views panel does not list spot (config):\n%s", u.View())
	}
}

func mustStyle(t *testing.T) *model.Style {
	t.Helper()
	s, err := model.ParseStyle("#04B575,#FFFF00,#FF0000")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// --- R8: the docs (docs/views.md, linked from the README) ---

// viewsDoc is docs/views.md, where the config views are documented.
func viewsDoc(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "views.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// readmeSection is the README section under the heading, up to the next one
// of the same level.
func readmeSection(t *testing.T, heading string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	start := strings.Index(text, "\n"+heading+"\n")
	if start < 0 {
		t.Fatalf("README has no %q section", heading)
	}
	rest := text[start+len(heading)+2:]
	if end := strings.Index(rest, "\n## "); end >= 0 {
		rest = rest[:end]
	}
	return rest
}

var yamlBlock = regexp.MustCompile("(?s)```yaml\n(.*?)```")

// config-views R8: the README's examples decode.
func TestReadmeConfigViewsExamplesDecode(t *testing.T) {
	readme := readmeSection(t, "## Config views")
	if !strings.Contains(readme, "(docs/views.md)") {
		t.Error("README Config views does not link docs/views.md")
	}
	section := viewsDoc(t)
	blocks := yamlBlock.FindAllStringSubmatch(section, -1)
	if len(blocks) == 0 {
		t.Fatal("no yaml example in docs/views.md")
	}
	// the README's short example must load too
	for _, b := range yamlBlock.FindAllStringSubmatch(readme, -1) {
		loadOK(t, b[1])
	}
	flow := regexp.MustCompile(`(?m)^\s*(-\s*)?[\w.-]+:\s*\[`)
	for i, b := range blocks {
		t.Run(strings.TrimSpace(strings.SplitN(b[1], "\n", 2)[0])+"#"+string(rune('0'+i)), func(t *testing.T) {
			if loc := flow.FindString(b[1]); loc != "" {
				t.Errorf("flow list in a README example (R8 wants block lists): %q", loc)
			}
			s := loadOK(t, b[1])
			if len(s.views) == 0 {
				t.Error("example defines no view")
			}
		})
	}
	// the first example is the full one: check what it says it does
	s := loadOK(t, blocks[0][1])
	if len(s.views) != 2 || s.views[0].name != "cost" || s.views[1].name != "spot" {
		t.Fatalf("views %+v", s.views)
	}
	if s.defaultView == nil || s.defaultView.Sort.Field != model.FieldPrice || s.lastView {
		t.Errorf("start view %+v, lastView %v", s.defaultView, s.lastView)
	}
	spot := s.views[1].view
	if spot.ShowNodes || len(spot.Filters) != 2 || !spot.Filters[0].Enabled {
		t.Errorf("spot %+v", spot)
	}
}

// config-views R8: the README documents keys, names and start settings.
func TestReadmeConfigViewsDocumentsTheContract(t *testing.T) {
	// words joined by single spaces, so re-wrapping the doc changes nothing
	section := strings.Join(strings.Fields(viewsDoc(t)), " ")
	for _, s := range []string{
		"default-view", "start-with", "last-view", "groups-only", "filters", "exclude", "order",
		"label:zone", "usage:", "--group-by zone", "(config)",
		"leading or trailing whitespace", "sort: price",
		// fix-config-case R1, R2
		"another case", "label:INSTANCE", "kube-node-viewer/node-age", "written twice",
	} {
		if !strings.Contains(section, s) {
			t.Errorf("docs/views.md does not mention %q", s)
		}
	}
	for _, key := range viewItemKeys {
		if !strings.Contains(section, "`"+key+"`") {
			t.Errorf("docs/views.md does not document the key %q", key)
		}
	}
}

func TestConfigViewColumnTitlesAndDerivedSort(t *testing.T) {
	cases := []struct {
		name string
		body string
		want func(model.ViewState) bool
	}{
		{"price/h and price/m", "columns: [price/h, price/m, idle/h, idle/m]", func(v model.ViewState) bool {
			return slices.Equal(v.Columns, []model.Field{model.FieldPrice, model.FieldPriceMonthly, model.FieldIdle, model.FieldIdleMonthly})
		}},
		{"field names still accepted", "columns: [price, price-month, idle, idle-month]", func(v model.ViewState) bool {
			return slices.Equal(v.Columns, []model.Field{model.FieldPrice, model.FieldPriceMonthly, model.FieldIdle, model.FieldIdleMonthly})
		}},
		{"sort on a derived field", "sort: zone", func(v model.ViewState) bool {
			return v.Sort.Field == model.FieldZone && !v.Sort.Descending
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if v := onlyView(t, oneView(c.body)); !c.want(v) {
				t.Errorf("view %+v", v)
			}
		})
	}
	// an alias and its field name are the same column
	if _, err := loadSettings(writeSettings(t, oneView("columns: [price/m, price-month]")), nil); err == nil || !strings.Contains(err.Error(), "repeated") {
		t.Errorf("alias repeat: %v", err)
	}
}
