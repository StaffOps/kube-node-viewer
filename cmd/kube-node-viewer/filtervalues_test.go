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

// filter is a view with one filter on field, its values under key.
func filter(field, key string, values ...string) string {
	return oneView("filters:\n  - field: " + field + "\n    " + key + ": [" + strings.Join(values, ", ") + "]")
}

// fix-config-filter-values R1: capacity and status values outside their lists
// are refused with a hint, item by item after the whitespace check.
func TestConfigFilterValuesOfClosedFields(t *testing.T) {
	const capacity = "views[0].filters[0].values[0]: "
	checkErrors(t, []errCase{
		{"spot", filter("capacity", "values", "spot"), capacity + `"spot" is not a capacity value; write Spot`},
		{"on-demand", filter("capacity", "values", "on-demand"), capacity + `"on-demand" is not a capacity value; write OnDemand`},
		{"ON_DEMAND", filter("capacity", "values", "ON_DEMAND"), capacity + `"ON_DEMAND" is not a capacity value; write OnDemand`},
		{"CAPACITY_BLOCK", filter("capacity", "values", "CAPACITY_BLOCK"), capacity + `"CAPACITY_BLOCK" is not a capacity value; write Reserved`},
		{"regular", filter("capacity", "values", "regular"), capacity + `"regular" is not a capacity value; write OnDemand`},
		{"Spot/Auto excluded", filter("capacity", "exclude", "Spot/Auto"), `views[0].filters[0].exclude[0]: "Spot/Auto" is not a capacity value; write Spot; Auto is not a capacity: filter on label:eks.amazonaws.com/compute-type`},
		{"Auto", filter("capacity", "values", "Auto"), capacity + `"Auto" is not a capacity value; Auto is not a capacity: filter on label:eks.amazonaws.com/compute-type`},
		{"Preemptible", filter("capacity", "values", "Preemptible"), capacity + `"Preemptible" is not a capacity value; use one of OnDemand, Spot, Reserved, Fargate, Unknown`},
		{"Ready/Cordoned", filter("status", "values", "Ready/Cordoned"), `views[0].filters[0].values[0]: "Ready/Cordoned" is not a status value; write one value per item: Ready, Cordoned`},
		{"ready", filter("status", "values", "ready"), `views[0].filters[0].values[0]: "ready" is not a status value; write Ready`},
		{"SchedulingDisabled", filter("status", "values", "SchedulingDisabled"), `views[0].filters[0].values[0]: "SchedulingDisabled" is not a status value; write Cordoned`},
		{"Cordon", filter("status", "values", "Cordon"), `views[0].filters[0].values[0]: "Cordon" is not a status value; use one of Ready, NotReady, Cordoned, Deleting, Launching`},
		{"second item", filter("capacity", "values", "Spot", "fargate"), `views[0].filters[0].values[1]: "fargate" is not a capacity value; write Fargate`},
		// the whitespace check of an item comes before its value check
		{"padded after a good one", filter("status", "values", "Ready", `" ready"`), `views[0].filters[0].values[1]: " ready" has leading or trailing whitespace`},
		{"hint before a later padded item", filter("capacity", "values", "spot", `" x"`), capacity + `"spot" is not a capacity value; write Spot`},
	})
}

// fix-config-filter-values R2: other fields keep their values as written, and
// every listed capacity and status value loads.
func TestConfigFilterValuesOfOpenFieldsLoad(t *testing.T) {
	for _, tt := range []struct {
		body  string
		field model.Field
		want  []string
	}{
		{filter("zone", "values", "US-EAST-1A"), model.FieldZone, []string{"US-EAST-1A"}},
		{filter("team", "values", "Payments", "payments"), model.LabelField("team"), []string{"Payments", "payments"}},
		{filter("instance-type", "exclude", "M5.Large"), model.FieldInstanceType, []string{"M5.Large"}},
		{filter("capacity", "values", model.CapacityValues...), model.FieldCapacity, model.CapacityValues},
		{filter("capacity", "exclude", model.CapacityValues...), model.FieldCapacity, model.CapacityValues},
		{filter("status", "values", model.StatusValues...), model.FieldStatus, model.StatusValues},
		{filter("status", "exclude", model.StatusValues...), model.FieldStatus, model.StatusValues},
	} {
		s, err := loadSettings(writeSettings(t, tt.body), nil)
		if err != nil {
			t.Errorf("%s: %v", tt.body, err)
			continue
		}
		f := s.views[0].view.Filters[0]
		if f.Field != tt.field || !reflect.DeepEqual(f.Ticked(), tt.want) {
			t.Errorf("%s: read as %s %v", tt.body, f.Field, f.Ticked())
		}
	}
}
