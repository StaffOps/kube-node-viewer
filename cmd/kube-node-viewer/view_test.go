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
	"testing"

	"github.com/staffops/kube-node-viewer/pkg/model"
)

const pool = "karpenter.sh/nodepool"

func resolve(t *testing.T, cfg configFile, saved *model.ViewState, explicit map[string]string) model.ViewState {
	t.Helper()
	v, err := resolveView(cfg, saved, explicit)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func flagsOnly(explicit map[string]string) func(t *testing.T) model.ViewState {
	return func(t *testing.T) model.ViewState { return resolve(t, nil, nil, explicit) }
}

// One test per row of the flag table (design D8, R15).

func TestFlagTableNodeSelectorIsNotViewState(t *testing.T) {
	got := flagsOnly(map[string]string{"node-selector": "team=a"})(t)
	if !reflect.DeepEqual(got, model.DefaultViewState()) {
		t.Errorf("--node-selector changed the view: %+v", got)
	}
}

func TestFlagTableGroupByKey(t *testing.T) {
	got := flagsOnly(map[string]string{"group-by": pool})(t)
	if !reflect.DeepEqual(got.GroupBy, []model.Field{model.LabelField(pool)}) || len(got.Filters) != 0 {
		t.Errorf("got group-by %v filters %v", got.GroupBy, got.Filters)
	}
}

func TestFlagTableGroupByKeyWithValues(t *testing.T) {
	got := flagsOnly(map[string]string{"group-by": pool + "=default, gpu"})(t)
	want := []model.Filter{{Field: model.LabelField(pool), Values: []string{"default", "gpu"}, Enabled: true}}
	if !reflect.DeepEqual(got.GroupBy, []model.Field{model.LabelField(pool)}) || !reflect.DeepEqual(got.Filters, want) {
		t.Errorf("got group-by %v filters %v", got.GroupBy, got.Filters)
	}
}

func TestFlagTableGroupsOnly(t *testing.T) {
	got := flagsOnly(map[string]string{"group-by": pool, "groups-only": "true"})(t)
	if got.ShowNodes {
		t.Error("--groups-only with --group-by kept the node list")
	}
	// R13: never hide both
	got = flagsOnly(map[string]string{"groups-only": "true"})(t)
	if !got.ShowNodes {
		t.Error("--groups-only without --group-by hid everything")
	}
}

func TestFlagTableNodeSort(t *testing.T) {
	cases := map[string]model.SortSpec{
		"creation":                            {Field: model.FieldCreation},
		"creation=dsc":                        {Field: model.FieldCreation, Descending: true},
		pool:                                  {Field: model.LabelField(pool)},
		pool + "=asc":                         {Field: model.LabelField(pool)},
		pool + "=dsc":                         {Field: model.LabelField(pool), Descending: true},
		"kube-node-viewer/node-cpu-usage=dsc": {Field: model.LabelField("kube-node-viewer/node-cpu-usage"), Descending: true},
	}
	for flag, want := range cases {
		if got := flagsOnly(map[string]string{"node-sort": flag})(t).Sort; got != want {
			t.Errorf("--node-sort %s: got %+v, want %+v", flag, got, want)
		}
	}
}

func TestFlagTableExtraLabels(t *testing.T) {
	got := flagsOnly(map[string]string{"extra-labels": "team, kube-node-viewer/node-age"})(t)
	want := append(append([]model.Field(nil), model.DefaultColumns...),
		model.LabelField("team"), model.LabelField("kube-node-viewer/node-age"))
	if !reflect.DeepEqual(got.Columns, want) {
		t.Errorf("got %v", got.Columns)
	}
}

func TestFlagTableResources(t *testing.T) {
	got := flagsOnly(map[string]string{"resources": "cpu,memory,nvidia.com/gpu"})(t)
	if !reflect.DeepEqual(got.Resources, []string{"cpu", "memory", "nvidia.com/gpu"}) {
		t.Errorf("got %v", got.Resources)
	}
	if _, err := resolveView(nil, nil, map[string]string{"resources": " , "}); err == nil {
		t.Error("empty --resources accepted")
	}
}

func TestFlagTableClusterAndRunOptionsAreNotViewState(t *testing.T) {
	got := flagsOnly(map[string]string{
		"context": "a", "kubeconfig": "/k", "style": "#000,#111,#222", "disable-pricing": "true",
	})(t)
	if !reflect.DeepEqual(got, model.DefaultViewState()) {
		t.Errorf("run options changed the view: %+v", got)
	}
}

// Precedence (R1, R2): explicit flag > saved state > config file > default.

func savedView() *model.ViewState {
	v := model.DefaultViewState()
	v.Sort = model.SortSpec{Field: model.FieldPrice, Descending: true}
	v.SetGroupBy(model.FieldZone)
	v.Resources = []string{"memory"}
	return &v
}

func TestConfigDoesNotOverrideSavedState(t *testing.T) {
	cfg := configFile{"node-sort": "creation=asc", "group-by": pool, "resources": "cpu"}
	got := resolve(t, cfg, savedView(), nil)
	if !reflect.DeepEqual(got, *savedView()) {
		t.Errorf("got %+v", got)
	}
}

func TestConfigAppliesWithoutSavedState(t *testing.T) {
	got := resolve(t, configFile{"group-by": pool}, nil, nil)
	if !reflect.DeepEqual(got.GroupBy, []model.Field{model.LabelField(pool)}) {
		t.Errorf("got %v", got.GroupBy)
	}
}

func TestExplicitFlagOverridesSavedState(t *testing.T) {
	saved := savedView()
	got := resolve(t, nil, saved, map[string]string{"node-sort": "creation"})
	if got.Sort != (model.SortSpec{Field: model.FieldCreation}) {
		t.Errorf("sort %+v", got.Sort)
	}
	if !reflect.DeepEqual(got.GroupBy, saved.GroupBy) || !reflect.DeepEqual(got.Resources, saved.Resources) {
		t.Error("an explicit flag reset parts of the saved state it doesn't set")
	}
	if !reflect.DeepEqual(*saved, *savedView()) {
		t.Error("resolving modified the saved state")
	}
}

func TestGroupByFlagReplacesItsOwnFilterOnly(t *testing.T) {
	saved := savedView()
	saved.Filters = []model.Filter{
		{Field: model.FieldCapacity, Values: []string{"Spot"}, Enabled: true},
		{Field: model.LabelField(pool), Values: []string{"old"}, Enabled: true},
	}
	got := resolve(t, nil, saved, map[string]string{"group-by": pool + "=new"})
	want := []model.Filter{
		{Field: model.FieldCapacity, Values: []string{"Spot"}, Enabled: true},
		{Field: model.LabelField(pool), Values: []string{"new"}, Enabled: true},
	}
	if !reflect.DeepEqual(got.Filters, want) {
		t.Errorf("got %v", got.Filters)
	}
	if saved.Filters[1].Values[0] != "old" {
		t.Error("resolving modified the saved filters")
	}
}
