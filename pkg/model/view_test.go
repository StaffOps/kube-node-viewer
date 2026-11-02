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

package model_test

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/staffops/kube-node-viewer/pkg/model"
)

type viewNode struct {
	name, capacity, pool, zone, ip, itype string
	price                                 float64
	age                                   time.Duration
	cordoned                              bool
}

func (vn viewNode) build() *model.Node {
	n := testNode(vn.name)
	n.CreationTimestamp = metav1.NewTime(time.Now().Add(-vn.age))
	n.Labels = map[string]string{}
	set := func(k, v string) {
		if v != "" {
			n.Labels[k] = v
		}
	}
	set("karpenter.sh/capacity-type", vn.capacity)
	set("karpenter.sh/nodepool", vn.pool)
	set(v1.LabelTopologyZone, vn.zone)
	set(v1.LabelInstanceTypeStable, vn.itype)
	if vn.ip != "" {
		n.Status.Addresses = []v1.NodeAddress{{Type: v1.NodeInternalIP, Address: vn.ip}}
	}
	n.Spec.Unschedulable = vn.cordoned
	n.Status.Allocatable = v1.ResourceList{v1.ResourceCPU: resource.MustParse("4")}
	node := model.NewNode(n)
	node.SetPrice(vn.price)
	return node
}

func viewNodes() []*model.Node {
	specs := []viewNode{
		{name: "a", capacity: "spot", pool: "default", zone: "us-east-1a", ip: "10.0.1.5", itype: "m6i.large", price: 0.10, age: 3 * time.Hour},
		{name: "b", capacity: "on-demand", pool: "default", zone: "us-east-1b", ip: "10.0.2.7", itype: "m5.xlarge", price: 0.19, age: 2 * time.Hour},
		{name: "c", capacity: "spot", pool: "gpu", zone: "us-east-1a", ip: "10.0.1.9", itype: "g5.xlarge", price: 0.40, age: time.Hour, cordoned: true},
		{name: "d", capacity: "on-demand", zone: "us-east-1a", ip: "10.0.3.1", itype: "m5.large", price: math.NaN(), age: 4 * time.Hour},
	}
	var nodes []*model.Node
	for _, s := range specs {
		nodes = append(nodes, s.build())
	}
	return nodes
}

func names(nodes []*model.Node) []string {
	var out []string
	for _, n := range nodes {
		out = append(out, n.Name())
	}
	return out
}

func expectNames(t *testing.T, got []*model.Node, want ...string) {
	t.Helper()
	if !reflect.DeepEqual(names(got), want) {
		t.Errorf("got %v, want %v", names(got), want)
	}
}

func byName() model.SortSpec { return model.SortSpec{Field: model.FieldName} }

func TestFiltersAndAcrossFieldsOrWithin(t *testing.T) {
	s := model.ViewState{Sort: byName(), Filters: []model.Filter{
		{Field: model.FieldCapacity, Values: []string{"Spot", "OnDemand"}, Enabled: true},
		{Field: model.FieldZone, Values: []string{"us-east-1a"}, Enabled: true},
	}}
	expectNames(t, s.Select(viewNodes()), "a", "c", "d")

	s.Filters = append(s.Filters, model.Filter{Field: model.FieldNodeGroup, Values: []string{"default"}, Enabled: true})
	expectNames(t, s.Select(viewNodes()), "a")
}

func TestDisabledFilterDoesNotFilter(t *testing.T) {
	s := model.ViewState{Sort: byName(), Filters: []model.Filter{
		{Field: model.FieldCapacity, Values: []string{"Spot"}, Enabled: false},
	}}
	expectNames(t, s.Select(viewNodes()), "a", "b", "c", "d")
}

func TestFilterOnLabelAndMissingValue(t *testing.T) {
	s := model.ViewState{Sort: byName(), Filters: []model.Filter{
		{Field: model.LabelField("karpenter.sh/nodepool"), Values: []string{model.UngroupedValue}, Enabled: true},
	}}
	expectNames(t, s.Select(viewNodes()), "d")
}

func TestStatusFilterMatchesAnyFlag(t *testing.T) {
	s := model.ViewState{Sort: byName(), Filters: []model.Filter{
		{Field: model.FieldStatus, Values: []string{model.StatusCordoned}, Enabled: true},
	}}
	expectNames(t, s.Select(viewNodes()), "c")
}

func TestSearch(t *testing.T) {
	cases := map[string][]string{
		"10.0.1":  {"a", "c"},
		"G5":      {"c"},
		"default": {"a", "b"},
		"nomatch": nil,
	}
	for text, want := range cases {
		s := model.ViewState{Sort: byName(), Search: text}
		if got := names(s.Select(viewNodes())); !reflect.DeepEqual(got, want) {
			t.Errorf("search %q: got %v, want %v", text, got, want)
		}
	}
}

func TestSort(t *testing.T) {
	cases := []struct {
		spec model.SortSpec
		want []string
	}{
		{model.SortSpec{Field: model.FieldCreation, Descending: true}, []string{"c", "b", "a", "d"}},
		{model.SortSpec{Field: model.FieldCreation}, []string{"d", "a", "b", "c"}},
		// an unknown price sorts last in either direction
		{model.SortSpec{Field: model.FieldPrice, Descending: true}, []string{"c", "b", "a", "d"}},
		{model.SortSpec{Field: model.FieldPrice}, []string{"a", "b", "c", "d"}},
		{model.SortSpec{Field: model.FieldIP}, []string{"a", "c", "b", "d"}},
		{model.SortSpec{Field: model.FieldZone, Descending: true}, []string{"b", "a", "c", "d"}},
		// nodes without the label go last
		{model.SortSpec{Field: model.LabelField("karpenter.sh/nodepool")}, []string{"a", "b", "c", "d"}},
		{model.SortSpec{Field: model.LabelField("karpenter.sh/nodepool"), Descending: true}, []string{"c", "a", "b", "d"}},
	}
	for _, c := range cases {
		s := model.ViewState{Sort: c.spec}
		if got := names(s.Select(viewNodes())); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%+v: got %v, want %v", c.spec, got, c.want)
		}
	}
}

func TestSelectLeavesInputAlone(t *testing.T) {
	nodes := viewNodes()
	before := names(nodes)
	model.ViewState{Sort: model.SortSpec{Field: model.FieldName, Descending: true}}.Select(nodes)
	if !reflect.DeepEqual(names(nodes), before) {
		t.Errorf("input reordered: %v", names(nodes))
	}
}

func TestSetGroupByKeepsThreeDistinct(t *testing.T) {
	var s model.ViewState
	s.SetGroupBy(model.FieldNodeGroup, model.FieldZone, model.FieldZone, model.FieldCapacity, model.FieldStatus)
	want := []model.Field{model.FieldNodeGroup, model.FieldZone, model.FieldCapacity}
	if !reflect.DeepEqual(s.GroupBy, want) {
		t.Errorf("got %v, want %v", s.GroupBy, want)
	}
}

func TestGroupsNested(t *testing.T) {
	s := model.ViewState{Sort: byName()}
	s.SetGroupBy(model.FieldNodeGroup, model.FieldZone)
	groups := s.Groups(s.Select(viewNodes()))

	var got []string
	for _, g := range groups {
		got = append(got, g.Name())
		for _, sub := range g.Groups {
			got = append(got, sub.Path[0]+">"+sub.Name()+":"+fmtNames(sub.Nodes))
		}
	}
	want := []string{
		"default", "default>us-east-1a:a", "default>us-east-1b:b",
		"gpu", "gpu>us-east-1a:c",
		model.UngroupedValue, model.UngroupedValue + ">us-east-1a:d",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}

	top := groups[0].Stats
	if top.NumNodes != 2 || top.PricedNodes != 2 || math.Abs(top.TotalPrice-0.29) > 1e-9 {
		t.Errorf("default subtotal: %+v", top)
	}
	if groups[0].Nodes != nil {
		t.Error("a group with sub-groups keeps nodes only in its leaves")
	}
}

func fmtNames(nodes []*model.Node) string {
	out := ""
	for _, n := range nodes {
		out += n.Name()
	}
	return out
}

func TestNoGroupBy(t *testing.T) {
	if g := (model.ViewState{}).Groups(viewNodes()); g != nil {
		t.Errorf("got %d groups", len(g))
	}
}

func TestValueCounts(t *testing.T) {
	got := model.ValueCounts(viewNodes(), model.FieldCapacity)
	want := []model.ValueCount{{"OnDemand", 2}, {"Spot", 2}, {"Reserved", 0}, {"Fargate", 0}, {"Unknown", 0}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("capacity: got %v", got)
	}
	got = model.ValueCounts(viewNodes(), model.FieldNodeGroup)
	want = []model.ValueCount{{"default", 2}, {"gpu", 1}, {model.UngroupedValue, 1}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("nodegroup: got %v", got)
	}
}

func TestFitColumnsDropsLowestPriorityFirst(t *testing.T) {
	cols := []model.Field{model.FieldInstanceType, model.FieldCapacity, model.FieldNodeGroup, model.FieldZone, model.FieldIP, model.FieldPrice, model.FieldStatus}
	ten := func(model.Field) int { return 10 }

	shown, hidden := model.FitColumns(cols, 70, ten)
	if hidden != 0 || len(shown) != 7 {
		t.Errorf("all fit: shown %v hidden %d", shown, hidden)
	}
	shown, hidden = model.FitColumns(cols, 45, ten)
	want := []model.Field{model.FieldInstanceType, model.FieldCapacity, model.FieldPrice, model.FieldStatus}
	if hidden != 3 || !reflect.DeepEqual(shown, want) {
		t.Errorf("narrow: shown %v hidden %d", shown, hidden)
	}
}

func TestDefaultViewState(t *testing.T) {
	s := model.DefaultViewState()
	if !s.ShowNodes || s.Sort.Field != model.FieldCreation || !s.Sort.Descending || len(s.GroupBy) != 0 {
		t.Errorf("unexpected default: %+v", s)
	}
}

func TestExcludingFilter(t *testing.T) {
	s := model.ViewState{Sort: byName(), Filters: []model.Filter{
		{Field: model.FieldCapacity, Excluded: []string{"Spot"}, Enabled: true},
	}}
	expectNames(t, s.Select(viewNodes()), "b", "d")

	// any of the flags excluded drops the node: c is Ready and Cordoned
	s.Filters = []model.Filter{{Field: model.FieldStatus, Excluded: []string{model.StatusCordoned}, Enabled: true}}
	expectNames(t, s.Select(viewNodes()), "a", "b", "d")
}

func TestExcludedValuesAreNoFilterToOlderCode(t *testing.T) {
	// the filter as written now, read with the shape v0.8.x knows
	saved, err := json.Marshal(model.Filter{Field: model.FieldCapacity, Excluded: []string{"Spot"}, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	var old struct {
		Field   model.Field `json:"field"`
		Values  []string    `json:"values"`
		Enabled bool        `json:"enabled"`
	}
	if err := json.Unmarshal(saved, &old); err != nil {
		t.Fatal(err)
	}
	s := model.ViewState{Sort: byName(), Filters: []model.Filter{{Field: old.Field, Values: old.Values, Enabled: old.Enabled}}}
	expectNames(t, s.Select(viewNodes()), "a", "b", "c", "d")
}

func TestFlippedFilter(t *testing.T) {
	f := model.Filter{Field: model.FieldCapacity, Values: []string{"Spot"}, Enabled: true}
	ex := f.Flipped()
	if !ex.Excludes() || len(ex.Values) != 0 || ex.Ticked()[0] != "Spot" {
		t.Errorf("flipped: %+v", ex)
	}
	if back := ex.Flipped(); back.Excludes() || back.Values[0] != "Spot" {
		t.Errorf("flipped back: %+v", back)
	}
}
