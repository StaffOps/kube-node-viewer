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
	"fmt"
	"strconv"
	"strings"

	"github.com/staffops/kube-node-viewer/pkg/model"
)

// viewKeys are the flags and config keys that set view state, in the order
// they apply: --groups-only depends on the group-by set before it.
var viewKeys = []string{"group-by", "groups-only", "node-sort", "extra-labels", "resources"}

// resolveView builds the starting view state (design D8): built-in defaults,
// then the config file, then the state saved for the cluster (which replaces
// both), then the flags given on the command line.
func resolveView(cfg configFile, saved *model.ViewState, explicit map[string]string) (model.ViewState, error) {
	view := model.DefaultViewState()
	if err := applyViewKeys(&view, cfg); err != nil {
		return model.ViewState{}, fmt.Errorf("config file: %w", err)
	}
	if saved != nil {
		view = *saved
	}
	if err := applyViewKeys(&view, explicit); err != nil {
		return model.ViewState{}, err
	}
	return view, nil
}

func applyViewKeys(view *model.ViewState, values map[string]string) error {
	for _, key := range viewKeys {
		if value, ok := values[key]; ok {
			if err := applyViewKey(view, key, value); err != nil {
				return fmt.Errorf("--%s: %w", key, err)
			}
		}
	}
	return nil
}

// applyViewKey is one row of the flag table.
func applyViewKey(view *model.ViewState, key, value string) error {
	switch key {
	case "group-by":
		if strings.TrimSpace(value) == "" {
			view.GroupBy = nil
			return nil
		}
		filter, err := parseLabelFilter(value)
		if err != nil {
			return err
		}
		field := model.LabelField(filter.label)
		view.SetGroupBy(field)
		// the values used to narrow the server-side selector; now they are an
		// in-memory filter the user can turn off
		var kept []model.Filter
		for _, f := range view.Filters {
			if f.Field != field {
				kept = append(kept, f)
			}
		}
		view.Filters = kept
		if len(filter.values) > 0 {
			view.Filters = append(view.Filters, model.Filter{Field: field, Values: filter.values, Enabled: true})
		}
	case "groups-only":
		groupsOnly, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		// the node list stays when there is no group summary to show instead
		view.ShowNodes = !groupsOnly || len(view.GroupBy) == 0
	case "node-sort":
		view.Sort = parseNodeSort(value)
	case "extra-labels":
		view.Columns = append([]model.Field(nil), model.DefaultColumns...)
		for _, label := range splitList(value) {
			view.Columns = append(view.Columns, model.LabelField(label))
		}
	case "resources":
		resources := splitList(value)
		if len(resources) == 0 {
			return fmt.Errorf("no resource given")
		}
		view.Resources = resources
	}
	return nil
}

// parseNodeSort reads 'creation' or a label name, ascending unless followed by
// '=dsc'.
func parseNodeSort(value string) model.SortSpec {
	name, order, _ := strings.Cut(strings.TrimSpace(value), "=")
	spec := model.SortSpec{Field: model.LabelField(name), Descending: order == "dsc"}
	if name == "creation" || name == "" {
		spec.Field = model.FieldCreation
	}
	return spec
}

func splitList(value string) []string {
	var out []string
	for _, v := range strings.Split(value, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}
