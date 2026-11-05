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
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/staffops/kube-node-viewer/pkg/model"
)

// configView is a view of config.yaml (config-views R1).
type configView struct {
	name string
	view model.ViewState
}

// viewItemKeys are the keys of a view item.
var viewItemKeys = []string{"name", "columns", "group-by", "sort", "order", "resources", "filters", "groups-only"}

// Prefixes a field name may carry in config.yaml.
const (
	labelPrefix = "label:"
	usagePrefix = "usage:"
)

// fieldKind is a key that names fields, with the built-in names it accepts
// (config-views R2, D3). The names come from the panels that set the same
// thing, so the two stay in step.
type fieldKind struct {
	key      string
	builtins []model.Field
	// usage accepts usage:<resource>
	usage bool
	// aliases are other names for built-in fields, such as the column
	// titles price/h and price/m
	aliases map[string]model.Field
}

// columnAliases are the column titles as written on screen, lower-cased, so
// a config names PRICE/H and PRICE/M the way it sees them.
var columnAliases = map[string]model.Field{
	"price/h": model.FieldPrice,
	"price/m": model.FieldPriceMonthly,
	"idle/h":  model.FieldIdle,
	"idle/m":  model.FieldIdleMonthly,
}

var (
	groupFields  = fieldKind{key: "group-by", builtins: model.FilterFields}
	filterFields = fieldKind{key: "filter", builtins: model.FilterFields}
	sortFields   = fieldKind{key: "sort", builtins: append(model.SortFields(), model.FilterFields...), usage: true}
	columnFields = fieldKind{key: "column", builtins: model.ColumnFields(), aliases: columnAliases}
)

// isBuiltin tells whether name is a built-in field of any key.
func isBuiltin(name string) bool {
	for _, k := range []fieldKind{groupFields, sortFields, columnFields} {
		if _, alias := k.aliases[name]; alias || slices.Contains(k.builtins, model.Field(name)) {
			return true
		}
	}
	return false
}

// field maps a name written in config.yaml to a field: a built-in name of
// the key, label:<key> for a node label, usage:<resource> for sort, a
// built-in name of another key is an error, and anything else is a label.
// Label keys and resource names must be Kubernetes qualified names
// (fix-config-errors R1).
func (k fieldKind) field(path string, value any) (model.Field, error) {
	if hint, ok := k.spacedPrefix(value); ok {
		return "", fmt.Errorf("%s: write %s with no space after the colon", path, hint)
	}
	name, err := str(path, value)
	if err != nil {
		return "", err
	}
	if key, ok := strings.CutPrefix(name, labelPrefix); ok {
		if strings.TrimSpace(key) == "" {
			return "", fmt.Errorf("%s: %q names no label", path, name)
		}
		if err := qualifiedName(path, name, key, "label key"); err != nil {
			return "", err
		}
		return model.LabelField(key), nil
	}
	if res, ok := strings.CutPrefix(name, usagePrefix); ok && k.usage {
		if strings.TrimSpace(res) == "" {
			return "", fmt.Errorf("%s: %q names no resource", path, name)
		}
		if err := qualifiedName(path, name, res, "resource name"); err != nil {
			return "", err
		}
		return model.UsageField(v1.ResourceName(res)), nil
	}
	if slices.Contains(k.builtins, model.Field(name)) {
		return model.Field(name), nil
	}
	if f, ok := k.aliases[name]; ok {
		return f, nil
	}
	if strings.HasPrefix(name, usagePrefix) {
		return "", fmt.Errorf("%s: %s is not a %s field; usage:<resource> is a sort", path, name, k.key)
	}
	if hint, ok := k.sibling(name); ok {
		return "", fmt.Errorf("%s: %s is not a %s field; %s", path, name, k.key, hint)
	}
	if want, ok := k.caseOf(name); ok {
		return "", fmt.Errorf("%s: %s is not a %s field; %s", path, name, k.key, orLabel("write "+want, name))
	}
	if want, ok := k.prefixCase(name); ok {
		return "", fmt.Errorf("%s: write %s", path, want)
	}
	if want, ok := computedOf(name); ok {
		return "", fmt.Errorf("%s: %s is not a %s field; %s", path, name, k.key, orLabel("write "+want+" for the computed label", name))
	}
	if isBuiltin(name) || isBuiltin(strings.ToLower(name)) {
		return "", fmt.Errorf("%s: %s is not a %s field; write label:%s for a node label", path, name, k.key, name)
	}
	if err := qualifiedName(path, name, name, "label key"); err != nil {
		return "", err
	}
	return model.LabelField(name), nil
}

// spacedPrefix tells whether value is what YAML makes of "label: zone"
// (a space after the colon): a map of one key, label (or usage for sort),
// to a non-empty string, and the name the user meant (fix-config-errors R3).
func (k fieldKind) spacedPrefix(value any) (string, bool) {
	m, ok := value.(map[string]any)
	if !ok || len(m) != 1 {
		return "", false
	}
	for key, v := range m {
		s, ok := v.(string)
		if !ok || strings.TrimSpace(s) == "" || (key != "label" && (key != "usage" || !k.usage)) {
			return "", false
		}
		// only a name the hint makes valid; label: a b stays a map error
		if len(validation.IsQualifiedName(strings.TrimSpace(s))) > 0 {
			return "", false
		}
		return key + ":" + strings.TrimSpace(s), true
	}
	return "", false
}

// siblings are built-in names refused by a key that has another name for
// the same thing (fix-config-errors R5).
var siblings = map[string]map[string]string{
	"sort": {
		"price/h":     "write sort: price, which orders PRICE/H and PRICE/M alike",
		"price/m":     "write sort: price, which orders PRICE/H and PRICE/M alike",
		"price-month": "write sort: price, which orders PRICE/H and PRICE/M alike",
		"idle/h":      "write sort: idle, which orders IDLE/H and IDLE/M alike",
		"idle/m":      "write sort: idle, which orders IDLE/H and IDLE/M alike",
		"idle-month":  "write sort: idle, which orders IDLE/H and IDLE/M alike",
		"age":         "write sort: creation, which orders by the node's age",
	},
	"column": {
		"creation": "the column age shows the node's creation time",
	},
}

func (k fieldKind) sibling(name string) (string, bool) {
	// the titles as written on screen too (PRICE/M)
	hint, ok := siblings[k.key][strings.ToLower(name)]
	return hint, ok
}

// screenTitles are column titles that are not a field name or alias once
// lower-cased (pkg/model columnTitle).
var screenTitles = map[string]model.Field{
	"instance": model.FieldInstanceType,
}

// caseOf tells whether name is a built-in name, alias or screen title of the
// key in another case (INSTANCE, Zone, PRICE/M), and the name to write
// instead: read as a label it would show a column that looks like the
// built-in one and is empty (fix-config-case R1).
func (k fieldKind) caseOf(name string) (string, bool) {
	lower := strings.ToLower(name)
	// a title is no field name in any case: instance shows as INSTANCE too
	if f, ok := screenTitles[lower]; ok && slices.Contains(k.builtins, f) {
		return string(f), true
	}
	if lower == name {
		return "", false
	}
	if _, ok := k.aliases[lower]; ok || slices.Contains(k.builtins, model.Field(lower)) {
		return lower, true
	}
	return "", false
}

// prefixCase tells whether name starts with label: (or usage: in sort) in
// another case, and how to write it (Label:zone → label:zone).
func (k fieldKind) prefixCase(name string) (string, bool) {
	for _, prefix := range []string{labelPrefix, usagePrefix} {
		if prefix == usagePrefix && !k.usage {
			continue
		}
		if len(name) > len(prefix) && strings.EqualFold(name[:len(prefix)], prefix) && name[:len(prefix)] != prefix {
			return prefix + name[len(prefix):], true
		}
	}
	return "", false
}

// computedUsage matches the computed usage labels, lower-cased.
var computedUsage = regexp.MustCompile(`^node-.+-usage$`)

// computedOf tells whether name is a computed label (pkg/model
// ComputeLabel) written without its kube-node-viewer/ prefix or in another
// case, which would show a column titled like the computed one and empty,
// and the key to write (fix-config-case R1).
func computedOf(name string) (string, bool) {
	key := strings.ToLower(name)
	key = strings.TrimPrefix(key, model.LabelPrefix)
	if key != "node-age" && !computedUsage.MatchString(key) {
		return "", false
	}
	want := model.LabelPrefix + key
	return want, want != name
}

// orLabel adds the label: way to the hint when name is a valid label key.
func orLabel(hint, name string) string {
	if len(validation.IsQualifiedName(name)) == 0 {
		hint += ", or label:" + name + " for a node label"
	}
	return hint
}

// qualifiedName checks a label key or resource name, part of the item
// written as raw: whitespace around it, or not a qualified name at all.
func qualifiedName(path, raw, name, what string) error {
	if len(validation.IsQualifiedName(name)) == 0 {
		return nil
	}
	if trimmed := strings.TrimSpace(name); trimmed != name && len(validation.IsQualifiedName(trimmed)) == 0 {
		return fmt.Errorf("%s: %q has leading or trailing whitespace", path, raw)
	}
	return fmt.Errorf("%s: %q is not a valid %s", path, raw, what)
}

// fieldList reads a non-empty list of fields without repeats.
func (k fieldKind) fieldList(path string, value any) ([]model.Field, error) {
	items, err := nonEmptyList(path, value)
	if err != nil {
		return nil, err
	}
	var out []model.Field
	for i, item := range items {
		itemPath := fmt.Sprintf("%s[%d]", path, i)
		f, err := k.field(itemPath, item)
		if err != nil {
			return nil, err
		}
		if slices.Contains(out, f) {
			return nil, fmt.Errorf("%s: %s is repeated", itemPath, describeItem(item))
		}
		out = append(out, f)
	}
	return out, nil
}

// nonEmptyList is a list with at least one item.
func nonEmptyList(path string, value any) ([]any, error) {
	items, err := list(path, value)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("%s: empty list", path)
	}
	return items, nil
}

// stringList reads a non-empty list of strings without repeats.
func stringList(path string, value any, checks ...func(itemPath, s string) error) ([]string, error) {
	items, err := nonEmptyList(path, value)
	if err != nil {
		return nil, err
	}
	var out []string
	for i, item := range items {
		itemPath := fmt.Sprintf("%s[%d]", path, i)
		s, err := str(itemPath, item)
		if err != nil {
			return nil, err
		}
		if err := unpadded(itemPath, s); err != nil {
			return nil, err
		}
		if slices.Contains(out, s) {
			return nil, fmt.Errorf("%s: %s is repeated", itemPath, s)
		}
		for _, check := range checks {
			if err := check(itemPath, s); err != nil {
				return nil, err
			}
		}
		out = append(out, s)
	}
	return out, nil
}

// resourceList reads resources: a string list of qualified names
// (fix-config-errors R1).
func resourceList(path string, value any) ([]string, error) {
	out, err := stringList(path, value)
	if err != nil {
		return nil, err
	}
	for i, r := range out {
		if err := qualifiedName(fmt.Sprintf("%s[%d]", path, i), r, r, "resource name"); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// readViews checks the views list: items with a name each, no name twice.
func readViews(value any) ([]configView, error) {
	items, err := list("views", value)
	if err != nil {
		return nil, err
	}
	var out []configView
	for i, item := range items {
		path := fmt.Sprintf("views[%d]", i)
		v, err := readView(path, item)
		if err != nil {
			return nil, err
		}
		if slices.ContainsFunc(out, func(c configView) bool { return c.name == v.name }) {
			return nil, fmt.Errorf("%s.name: %q is repeated", path, v.name)
		}
		out = append(out, v)
	}
	return out, nil
}

// readView turns a view item into a view state: the built-in defaults, with
// each key the item has applied (config-views R1).
func readView(path string, value any) (configView, error) {
	item, err := mapping(path, value, viewItemKeys...)
	if err != nil {
		return configView{}, err
	}
	raw, ok := item["name"]
	if !ok {
		return configView{}, fmt.Errorf("%s: needs a name", path)
	}
	name, err := str(path+".name", raw)
	if err != nil {
		return configView{}, err
	}
	if strings.TrimSpace(name) != name {
		return configView{}, fmt.Errorf("%s.name: %q has leading or trailing spaces", path, name)
	}
	view := model.DefaultViewState()
	if raw, ok := item["columns"]; ok {
		if view.Columns, err = columnFields.fieldList(path+".columns", raw); err != nil {
			return configView{}, err
		}
	}
	if raw, ok := item["group-by"]; ok {
		levels, err := groupFields.fieldList(path+".group-by", raw)
		if err != nil {
			return configView{}, err
		}
		if len(levels) > model.MaxGroupLevels {
			return configView{}, fmt.Errorf("%s.group-by: %d levels, at most %d", path, len(levels), model.MaxGroupLevels)
		}
		view.SetGroupBy(levels...)
	}
	if view.Sort, err = readSort(path, item); err != nil {
		return configView{}, err
	}
	if raw, ok := item["resources"]; ok {
		if view.Resources, err = resourceList(path+".resources", raw); err != nil {
			return configView{}, err
		}
	}
	if raw, ok := item["filters"]; ok {
		if view.Filters, err = readFilters(path+".filters", raw); err != nil {
			return configView{}, err
		}
	}
	if raw, ok := item["groups-only"]; ok {
		groupsOnly, err := boolean(path+".groups-only", raw)
		if err != nil {
			return configView{}, err
		}
		if groupsOnly && len(view.GroupBy) == 0 {
			return configView{}, fmt.Errorf("%s.groups-only: needs group-by", path)
		}
		view.ShowNodes = !groupsOnly
	}
	return configView{name: name, view: view}, nil
}

// Values of a view's order.
const (
	orderAscending  = "ascending"
	orderDescending = "descending"
)

// readSort reads sort and order. Without order the direction is the one the
// Sort panel starts with for the field; order alone sets the direction of
// the creation sort.
func readSort(path string, item map[string]any) (model.SortSpec, error) {
	spec := model.DefaultViewState().Sort
	if raw, ok := item["sort"]; ok {
		field, err := sortFields.field(path+".sort", raw)
		if err != nil {
			return model.SortSpec{}, err
		}
		spec = model.SortSpec{Field: field, Descending: model.DescendingFirst(field)}
	}
	if raw, ok := item["order"]; ok {
		order, err := str(path+".order", raw)
		if err != nil {
			return model.SortSpec{}, err
		}
		switch order {
		case orderAscending:
			spec.Descending = false
		case orderDescending:
			spec.Descending = true
		default:
			return model.SortSpec{}, fmt.Errorf("%s.order: %q is not %s or %s", path, order, orderAscending, orderDescending)
		}
	}
	return spec, nil
}

// readFilters reads the filters: each on its own field, with either values
// to keep or values to exclude, turned on.
func readFilters(path string, value any) ([]model.Filter, error) {
	items, err := nonEmptyList(path, value)
	if err != nil {
		return nil, err
	}
	var out []model.Filter
	for i, raw := range items {
		itemPath := fmt.Sprintf("%s[%d]", path, i)
		item, err := mapping(itemPath, raw, "field", "values", "exclude")
		if err != nil {
			return nil, err
		}
		rawField, ok := item["field"]
		if !ok {
			return nil, fmt.Errorf("%s: needs field", itemPath)
		}
		field, err := filterFields.field(itemPath+".field", rawField)
		if err != nil {
			return nil, err
		}
		if slices.ContainsFunc(out, func(f model.Filter) bool { return f.Field == field }) {
			return nil, fmt.Errorf("%s.field: a second filter on %s", itemPath, describeItem(rawField))
		}
		rawValues, hasValues := item["values"]
		rawExclude, hasExclude := item["exclude"]
		filter := model.Filter{Field: field, Enabled: true}
		var checks []func(string, string) error
		if values := model.ClosedValues(field); values != nil {
			checks = append(checks, func(p, v string) error { return closedValue(p, string(field), values, v) })
		}
		switch {
		case hasValues && hasExclude:
			return nil, fmt.Errorf("%s: values and exclude together; use one", itemPath)
		case hasValues:
			if filter.Values, err = stringList(itemPath+".values", rawValues, checks...); err != nil {
				return nil, err
			}
		case hasExclude:
			if filter.Excluded, err = stringList(itemPath+".exclude", rawExclude, checks...); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("%s: needs values or exclude", itemPath)
		}
		out = append(out, filter)
	}
	return out, nil
}

// valueSpellings maps label spellings of capacity and status values, compared
// as fold does, to the value to write (fix-config-filter-values R1).
var valueSpellings = map[string]string{
	"capacityblock":      string(model.CapacityReserved),
	"regular":            string(model.CapacityOnDemand),
	"schedulingdisabled": model.StatusCordoned,
}

// fold lower-cases s and drops "-", "_" and spaces, so on-demand, ON_DEMAND
// and OnDemand compare equal.
func fold(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '-' || r == '_' || r == ' ' {
			return -1
		}
		return unicode.ToLower(r)
	}, s)
}

// spelledAs is the value of values that s means, ignoring case, "-", "_" and
// spaces, or through a known label spelling; "" when none.
func spelledAs(values []string, s string) string {
	f := fold(s)
	for _, v := range values {
		if fold(v) == f {
			return v
		}
	}
	if v, ok := valueSpellings[f]; ok && slices.Contains(values, v) {
		return v
	}
	return ""
}

// autoHint points an Auto value, from the CAPACITY column, at the label.
const autoHint = "Auto is not a capacity: filter on label:eks.amazonaws.com/compute-type"

// closedValue checks a filter value of a field with a fixed list of values
// (capacity, status): an exact match passes; otherwise the error says what to
// write (fix-config-filter-values R1).
func closedValue(path, field string, values []string, s string) error {
	if slices.Contains(values, s) {
		return nil
	}
	head := fmt.Sprintf("%s: %q is not a %s value; ", path, s, field)
	if want := spelledAs(values, s); want != "" {
		return errors.New(head + "write " + want)
	}
	if strings.Contains(s, "/") {
		var wants []string
		auto := false
		for _, part := range strings.Split(s, "/") {
			if field == string(model.FieldCapacity) && fold(part) == "auto" {
				auto = true
				continue
			}
			if want := spelledAs(values, part); want != "" {
				wants = append(wants, want)
			}
		}
		switch {
		case auto && len(wants) == 1:
			return errors.New(head + "write " + wants[0] + "; " + autoHint)
		case !auto && len(wants) > 1:
			return errors.New(head + "write one value per item: " + strings.Join(wants, ", "))
		}
	}
	if field == string(model.FieldCapacity) && fold(s) == "auto" {
		return errors.New(head + autoHint)
	}
	return errors.New(head + "use one of " + strings.Join(values, ", "))
}

// combinedViews lists the config views first, in file order, then the named
// views of the state file whose names no config view has (config-views R3,
// D4). Config views are read-only: the TUI never writes config.yaml.
type combinedViews struct {
	config []configView
	state  model.Views
}

func (c combinedViews) configView(name string) (model.ViewState, bool) {
	for _, v := range c.config {
		if v.name == name {
			return v.view, true
		}
	}
	return model.ViewState{}, false
}

func (c combinedViews) ViewNames() []string {
	names := make([]string, 0, len(c.config))
	for _, v := range c.config {
		names = append(names, v.name)
	}
	for _, name := range c.state.ViewNames() {
		if !c.ReadOnly(name) {
			names = append(names, name)
		}
	}
	return names
}

// View returns a copy, so a change to the applied view never reaches the
// config view.
func (c combinedViews) View(name string) (model.ViewState, bool) {
	if v, ok := c.configView(name); ok {
		return cloneView(v), true
	}
	return c.state.View(name)
}

// SaveView and DeleteView leave config views alone; the Views panel refuses
// both before they get here.
func (c combinedViews) SaveView(name string, v model.ViewState) {
	if !c.ReadOnly(name) {
		c.state.SaveView(name, v)
	}
}

func (c combinedViews) DeleteView(name string) {
	if !c.ReadOnly(name) {
		c.state.DeleteView(name)
	}
}

// ReadOnly tells whether the name is a config view's.
func (c combinedViews) ReadOnly(name string) bool {
	_, ok := c.configView(name)
	return ok
}

// cloneView copies the slices of a view state, so the copy can change
// without touching the original.
func cloneView(v model.ViewState) model.ViewState {
	v.Filters = slices.Clone(v.Filters)
	for i := range v.Filters {
		v.Filters[i].Values = slices.Clone(v.Filters[i].Values)
		v.Filters[i].Excluded = slices.Clone(v.Filters[i].Excluded)
	}
	v.GroupBy = slices.Clone(v.GroupBy)
	v.Resources = slices.Clone(v.Resources)
	v.Columns = slices.Clone(v.Columns)
	v.Collapsed = slices.Clone(v.Collapsed)
	return v
}
