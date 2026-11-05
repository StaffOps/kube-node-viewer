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
	"reflect"
	"strings"
	"testing"

	"github.com/staffops/kube-node-viewer/pkg/model"
)

// Tests for specs/fix-config-errors: each case asserts the whole message
// after the file path.

type errCase struct{ name, body, want string }

func checkErrors(t *testing.T, tests []errCase) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeSettings(t, tt.body)
			_, err := loadSettings(path, nil)
			if err == nil {
				t.Fatalf("no error for\n%s", tt.body)
			}
			if got := strings.TrimPrefix(err.Error(), "config file "+path+": "); got != tt.want {
				t.Errorf("error\n %q\nwant\n %q", got, tt.want)
			}
		})
	}
}

// R1, R2: label keys and resource names are qualified names; padded values
// and patterns are refused.
func TestConfigViewNamesAreKubernetesNames(t *testing.T) {
	checkErrors(t, []errCase{
		{"padded label after label:", oneView(`columns: ["label: zone"]`), `views[0].columns[0]: "label: zone" has leading or trailing whitespace`},
		{"padded bare label", oneView(`group-by: ["zone "]`), `views[0].group-by[0]: "zone " has leading or trailing whitespace`},
		{"padded built-in", oneView(`sort: " price"`), `views[0].sort: " price" has leading or trailing whitespace`},
		{"tab", oneView("columns: [\"label:\\tzone\"]"), "views[0].columns[0]: \"label:\\tzone\" has leading or trailing whitespace"},
		{"padded filter field", oneView("filters:\n  - field: \"team \"\n    values: [a]"), `views[0].filters[0].field: "team " has leading or trailing whitespace`},
		{"padded resource", oneView(`resources: ["cpu "]`), `views[0].resources[0]: "cpu " has leading or trailing whitespace`},
		{"usage in resources", oneView("resources: [usage:cpu]"), `views[0].resources[0]: "usage:cpu" is not a valid resource name`},
		{"label in resources", oneView("resources: [label:zone]"), `views[0].resources[0]: "label:zone" is not a valid resource name`},
		{"padded usage", oneView(`sort: "usage: cpu"`), `views[0].sort: "usage: cpu" has leading or trailing whitespace`},
		{"comma in a label", oneView(`group-by: ["zone,region"]`), `views[0].group-by[0]: "zone,region" is not a valid label key`},
		{"inner space", oneView(`columns: ["label:a b"]`), `views[0].columns[0]: "label:a b" is not a valid label key`},
		{"padded filter value", oneView("filters:\n  - field: zone\n    values: [\"a \"]"), `views[0].filters[0].values[0]: "a " has leading or trailing whitespace`},
		{"padded excluded value", oneView("filters:\n  - field: zone\n    exclude: [\" a\"]"), `views[0].filters[0].exclude[0]: " a" has leading or trailing whitespace`},
		{"padded allow pattern", "labels:\n  allow: [\"team \"]\n", `labels.allow[0]: "team " has leading or trailing whitespace`},
		{"padded deny pattern", "labels:\n  deny: [\" team\"]\n", `labels.deny[0]: " team" has leading or trailing whitespace`},
	})
}

// R3, R4, R5: hints and values shown as written.
func TestConfigViewErrorHints(t *testing.T) {
	checkErrors(t, []errCase{
		{"space after label:", oneView("columns:\n  - label: zone"), "views[0].columns[0]: write label:zone with no space after the colon"},
		{"space after label: in group-by", oneView("group-by:\n  - label: team"), "views[0].group-by[0]: write label:team with no space after the colon"},
		{"space after label: in a filter", oneView("filters:\n  - field:\n      label: team\n    values: [a]"), "views[0].filters[0].field: write label:team with no space after the colon"},
		{"space after usage: in sort", oneView("sort:\n  usage: cpu"), "views[0].sort: write usage:cpu with no space after the colon"},
		{"usage: map in columns", oneView("columns:\n  - usage: cpu"), "views[0].columns[0]: {usage: cpu} is a map, not a string"},
		{"label: with no value", oneView("columns:\n  - label:"), "views[0].columns[0]: {label: null} is a map, not a string"},
		{"label: with an invalid key", oneView("columns:\n  - label: a b"), "views[0].columns[0]: {label: a b} is a map, not a string"},
		{"upper-case sort title", oneView("sort: PRICE/M"), "views[0].sort: PRICE/M is not a sort field; write sort: price, which orders PRICE/H and PRICE/M alike"},
		{"two keys", oneView("columns:\n  - {label: a, b: c}"), "views[0].columns[0]: {b: c, label: a} is a map, not a string"},
		{"list as a column", oneView("columns:\n  - [a, b]"), "views[0].columns[0]: [a, b] is a list, not a string"},
		{"map instead of a list", "views: {a: b}\n", "views: {a: b} is not a list"},
		{"map of columns", oneView("columns:\n  price: 1"), "views[0].columns: {price: 1} is not a list"},
		{"big number", oneView("columns: [1000000]"), "views[0].columns[0]: 1000000 is not a string; quote it"},
		{"list as a pattern", "labels:\n  allow: [[a]]\n", "labels.allow[0]: [a] is a list, not a string"},
		{"sort price/m", oneView("sort: price/m"), "views[0].sort: price/m is not a sort field; write sort: price, which orders PRICE/H and PRICE/M alike"},
		{"sort price/h", oneView("sort: price/h"), "views[0].sort: price/h is not a sort field; write sort: price, which orders PRICE/H and PRICE/M alike"},
		{"sort price-month", oneView("sort: price-month"), "views[0].sort: price-month is not a sort field; write sort: price, which orders PRICE/H and PRICE/M alike"},
		{"sort idle/m", oneView("sort: idle/m"), "views[0].sort: idle/m is not a sort field; write sort: idle, which orders IDLE/H and IDLE/M alike"},
		{"sort idle-month", oneView("sort: idle-month"), "views[0].sort: idle-month is not a sort field; write sort: idle, which orders IDLE/H and IDLE/M alike"},
		{"sort age", oneView("sort: age"), "views[0].sort: age is not a sort field; write sort: creation, which orders by the node's age"},
		{"column creation", oneView("columns: [creation]"), "views[0].columns[0]: creation is not a column field; the column age shows the node's creation time"},
	})
}

// Unchanged behavior: the messages that were right stay as they were.
func TestConfigViewErrorsKeepTheirText(t *testing.T) {
	checkErrors(t, []errCase{
		{"yes", oneView("columns: [yes]"), "views[0].columns[0]: true is not a string; quote it"},
		{"number", oneView("columns: [123]"), "views[0].columns[0]: 123 is not a string; quote it"},
		{"tilde", oneView("columns: [~]"), "views[0].columns[0]: null is not a string; quote it"},
		{"blank", oneView(`columns: ["  "]`), "views[0].columns[0]: empty"},
		{"label: alone", oneView(`columns: ["label:"]`), `views[0].columns[0]: "label:" names no label`},
		{"label: and spaces", oneView(`columns: ["label:  "]`), `views[0].columns[0]: "label:  " names no label`},
		{"usage: alone", oneView(`sort: "usage:"`), `views[0].sort: "usage:" names no resource`},
		{"built-in of another key", oneView("group-by: [price]"), "views[0].group-by[0]: price is not a group-by field; write label:price for a node label"},
		{"order", oneView(`order: " descending"`), `views[0].order: " descending" is not ascending or descending`},
		{"start-with", "start-with: \"last-view \"\n", `start-with: "last-view " is not default-view or last-view`},
		{"default-view", "views:\n  - name: a\ndefault-view: \" a\"\n", `default-view: no view is named " a"`},
		{"view name", "views:\n  - name: \"a \"\n", `views[0].name: "a " has leading or trailing spaces`},
	})
	// an inline "sort: label: zone" is a YAML syntax error, from the parser
	_, err := loadSettings(writeSettings(t, oneView("sort: label: zone")), nil)
	if err == nil || !strings.Contains(err.Error(), "mapping values are not allowed") {
		t.Errorf("inline label: zone: %v", err)
	}
}

// Unchanged behavior: names that are valid Kubernetes names still load.
func TestConfigViewValidNamesStillLoad(t *testing.T) {
	v := onlyView(t, oneView(
		`columns: ["label:nvidia.com/gpu.product", team, price/m]`,
		"resources: [cpu, nvidia.com/gpu, hugepages-2Mi, ephemeral-storage, pods]",
		"sort: usage:hugepages-2Mi",
		"group-by: [MyLabel, example.com/MyLabel]",
		"filters:\n  - field: zone\n    values: [\"a b\", \"<none>\"]",
	))
	if want := []model.Field{model.LabelField("nvidia.com/gpu.product"), model.LabelField("team"), model.FieldPriceMonthly}; !reflect.DeepEqual(v.Columns, want) {
		t.Errorf("columns %v, want %v", v.Columns, want)
	}
	if want := []string{"cpu", "nvidia.com/gpu", "hugepages-2Mi", "ephemeral-storage", "pods"}; !reflect.DeepEqual(v.Resources, want) {
		t.Errorf("resources %v, want %v", v.Resources, want)
	}
	if v.Sort.Field != model.UsageField("hugepages-2Mi") {
		t.Errorf("sort %v", v.Sort.Field)
	}
	if want := []model.Field{model.LabelField("MyLabel"), model.LabelField("example.com/MyLabel")}; !reflect.DeepEqual(v.GroupBy, want) {
		t.Errorf("group-by %v, want %v", v.GroupBy, want)
	}
	if want := []string{"a b", "<none>"}; !reflect.DeepEqual(v.Filters[0].Values, want) {
		t.Errorf("filter values %v, want %v", v.Filters[0].Values, want)
	}
}

func TestDescribeItem(t *testing.T) {
	for _, tt := range []struct {
		in   any
		want string
	}{
		{nil, "null"},
		{true, "true"},
		{float64(1000000), "1000000"},
		{1.5, "1.5"},
		{float64(12345678901), "12345678901"},
		{1.5e300, "1.5e+300"},
		{"a", "a"},
		{[]any{"a", float64(2)}, "[a, 2]"},
		{map[string]any{"b": []any{float64(1), float64(2)}, "a": nil}, "{a: null, b: [1, 2]}"},
	} {
		if got := describeItem(tt.in); got != tt.want {
			t.Errorf("describeItem(%#v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// fix-config-case R1: a built-in name, alias or screen title in another case
// is refused with the name to write, not read as an empty label column.
func TestConfigViewBuiltinNamesInAnotherCase(t *testing.T) {
	checkErrors(t, []errCase{
		{"screen title INSTANCE", oneView("columns: [INSTANCE]"), "views[0].columns[0]: INSTANCE is not a column field; write instance-type, or label:INSTANCE for a node label"},
		{"STATUS column", oneView("columns: [zone, STATUS]"), "views[0].columns[1]: STATUS is not a column field; write status, or label:STATUS for a node label"},
		{"Zone group", oneView("group-by: [Zone]"), "views[0].group-by[0]: Zone is not a group-by field; write zone, or label:Zone for a node label"},
		{"Zone filter", oneView("filters:\n  - field: Zone\n    values: [a]"), "views[0].filters[0].field: Zone is not a filter field; write zone, or label:Zone for a node label"},
		{"NAME sort", oneView("sort: NAME"), "views[0].sort: NAME is not a sort field; write name, or label:NAME for a node label"},
		// fix-config-case R1: the title with a / cannot be a label, so only the alias
		{"upper-case title", oneView("columns: [PRICE/M]"), "views[0].columns[0]: PRICE/M is not a column field; write price/m"},
		{"IDLE/H column", oneView("columns: [IDLE/H]"), "views[0].columns[0]: IDLE/H is not a column field; write idle/h"},
		// fix-config-case R1: a built-in of another key in another case
		{"PRICE in group-by", oneView("group-by: [PRICE]"), "views[0].group-by[0]: PRICE is not a group-by field; write label:PRICE for a node label"},
		// unchanged: the sort hint still wins for the price titles
		{"upper-case sort title", oneView("sort: PRICE/M"), "views[0].sort: PRICE/M is not a sort field; write sort: price, which orders PRICE/H and PRICE/M alike"},
	})
}

// fix-config-case R2: YAML errors are one line with the line number, without
// the library's prefixes.
func TestConfigYAMLErrorsAreOneLine(t *testing.T) {
	checkErrors(t, []errCase{
		{"key written twice", "views:\n  - name: a\n    name: b\n    columns: [zone]\n", `line 3: key "name" already set in map`},
		{"two keys written twice", "views:\n  - name: a\n    name: b\n    sort: name\n    sort: ip\n", `line 3: key "name" already set in map; line 5: key "sort" already set in map`},
		{"syntax", "views: [a\n", "line 1: did not find expected ',' or ']'"},
	})
}

// fix-config-case R3: label keys that are not built-in names in any case
// still load as labels.
func TestConfigViewOtherUpperCaseLabelsStillLoad(t *testing.T) {
	path := writeSettings(t, oneView("group-by: [MyLabel, Team]", "columns: [instance-type, label:INSTANCE, Owner]"))
	s, err := loadSettings(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	v := s.views[0].view
	if want := []model.Field{model.LabelField("MyLabel"), model.LabelField("Team")}; !reflect.DeepEqual(v.GroupBy, want) {
		t.Errorf("group-by %v, want %v", v.GroupBy, want)
	}
	if want := []model.Field{model.FieldInstanceType, model.LabelField("INSTANCE"), model.LabelField("Owner")}; !reflect.DeepEqual(v.Columns, want) {
		t.Errorf("columns %v, want %v", v.Columns, want)
	}
}
