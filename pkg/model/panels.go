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
	"fmt"
	"slices"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type panelKind int

const (
	panelNone panelKind = iota
	panelHelp
	panelFilters
	panelGroupBy
	panelSort
	panelColumns
	panelContexts
)

// panelKey handles a key while a panel is open; esc closes or steps back.
func (u *UIModel) panelKey(key string) tea.Cmd {
	switch u.panel {
	case panelContexts:
		return u.contextsKey(key)
	case panelHelp:
		if key == "esc" || key == "?" {
			u.panel = panelNone
		}
	case panelFilters:
		u.filtersKey(key)
	case panelGroupBy:
		u.groupByKey(key)
	case panelSort:
		u.sortKey(key)
	case panelColumns:
		u.columnsKey(key)
	}
	return nil
}

// helpKeys lists every key, for the help panel.
var helpKeys = [][2]string{
	{"f", "filters: open a field, tick values; space turns a filter on/off; x keeps or excludes"},
	{"g", "group by: tick up to 3 fields, K/J to reorder levels"},
	{"s", "sort: any column, usage or label; flip ↑/↓"},
	{"o", "columns: show, hide, reorder"},
	{"/", "search name, IP, instance type, labels; esc clears"},
	{"r", "resources: cpu → memory → cpu+memory"},
	{"c", "switch to another kubeconfig context"},
	{"d", "folded: list the next group level as one-line rows"},
	{"↑/↓ k/j", "select a group or node (marked in the left margin)"},
	{"enter", "fold or unfold the selected group on its own"},
	{"n", "fold one level per press: nodes, then each sub-group level; then unfold all"},
	{"←/→ h/l", "previous / next page"},
	{"home", "first page"},
	{"ctrl+e", "compact or expand the header"},
	{"?", "this help"},
	{"q, ctrl+c", "quit"},
	{"esc", "close a panel or go back a step"},
}

func (u *UIModel) helpPanel() panelView {
	var lines []string
	for _, k := range helpKeys {
		lines = append(lines, fmt.Sprintf("%s  %s", headerKey(fmt.Sprintf("%-10s", k[0])), k[1]))
	}
	lines = append(lines, "", titleStyle("Colours"),
		"NODEGROUP   "+managedNodeGroupStyle("name")+" is an EKS managed node group; plain is a Karpenter NodePool")
	return panelView{title: "Keys", rows: lines, cursor: -1}
}

// filtersPanel is the Filters panel (R3): every field is listed, those with a
// filter first; opening a field lists its values, with node counts, to tick.
// Each tick filters right away.
type filtersPanel struct {
	cursor int
	// open is the field whose values are listed under it, empty for none
	open Field
}

// filterRow is a field, or one of the open field's values.
type filterRow struct {
	field Field
	value string
	count int
	// isValue tells a value row from its field's row
	isValue bool
}

func (p *filtersPanel) hint() string {
	if p.open != "" {
		return "↑/↓: move • space/enter: tick value • x: keep/exclude • ←: close field • d: clear field • esc: close field"
	}
	return "↑/↓: move • enter/→: open field • space: filter on/off • x: keep/exclude • d: clear field • esc: close"
}

func (u *UIModel) openFilters() {
	u.panel = panelFilters
	u.filters = filtersPanel{}
}

// filterIndex is the position of the field's filter, -1 when it has none.
func (u *UIModel) filterIndex(field Field) int {
	return slices.IndexFunc(u.view.Filters, func(f Filter) bool { return f.Field == field })
}

// filterRows lists the fields with a filter in filter order, then the other
// fields and labels, with the open field's values under it.
func (u *UIModel) filterRows() []filterRow {
	var fields []Field
	for _, f := range u.view.Filters {
		fields = append(fields, f.Field)
	}
	for _, f := range filterFieldChoices(u.stats.Nodes) {
		if !slices.Contains(fields, f) {
			fields = append(fields, f)
		}
	}
	var rows []filterRow
	for _, f := range fields {
		rows = append(rows, filterRow{field: f})
		if f != u.filters.open {
			continue
		}
		values := ValueCounts(u.stats.Nodes, f)
		listed := map[string]bool{}
		for _, v := range values {
			listed[v.Value] = true
		}
		// values the filter has stay listed even when no node has them now
		if i := u.filterIndex(f); i >= 0 {
			for _, v := range u.view.Filters[i].Ticked() {
				if !listed[v] {
					values = append(values, ValueCount{Value: v})
				}
			}
		}
		for _, v := range values {
			rows = append(rows, filterRow{field: f, value: v.Value, count: v.Count, isValue: true})
		}
	}
	return rows
}

// moveCursor handles up and down, wrapping at both ends.
func moveCursor(cursor, n int, key string) (int, bool) {
	if n == 0 {
		return 0, false
	}
	switch key {
	case "up", "k":
		return (cursor - 1 + n) % n, true
	case "down", "j":
		return (cursor + 1) % n, true
	}
	return cursor, false
}

func (u *UIModel) filtersKey(key string) {
	p := &u.filters
	rows := u.filterRows()
	var moved bool
	if p.cursor, moved = moveCursor(p.cursor, len(rows), key); moved || len(rows) == 0 {
		return
	}
	row := rows[min(p.cursor, len(rows)-1)]
	switch key {
	case "esc":
		if p.open == "" {
			u.panel = panelNone
			return
		}
		u.closeField()
	case "left", "h":
		if p.open != "" {
			u.closeField()
		}
	case "enter", "right", "l":
		if row.isValue {
			u.tickValue(row.field, row.value)
		} else if p.open == row.field {
			u.closeField()
		} else {
			p.open = row.field
			u.focusFilterRow(row.field, "")
		}
	case " ":
		if row.isValue {
			u.tickValue(row.field, row.value)
		} else if i := u.filterIndex(row.field); i >= 0 {
			filters := slices.Clone(u.view.Filters)
			filters[i].Enabled = !filters[i].Enabled
			u.setFilters(filters)
			u.focusFilterRow(row.field, "")
		}
	case "x":
		// switch the field's filter between keeping and excluding its values
		if i := u.filterIndex(row.field); i >= 0 {
			filters := slices.Clone(u.view.Filters)
			filters[i] = filters[i].Flipped()
			u.setFilters(filters)
			u.focusFilterRow(row.field, row.value)
		}
	case "d", "delete", "backspace":
		if i := u.filterIndex(row.field); i >= 0 {
			u.setFilters(slices.Delete(slices.Clone(u.view.Filters), i, i+1))
			u.focusFilterRow(row.field, row.value)
		}
	}
}

// closeField folds the open field's values, the cursor back on the field.
func (u *UIModel) closeField() {
	field := u.filters.open
	u.filters.open = ""
	u.focusFilterRow(field, "")
}

// tickValue adds the value to the field's filter, or removes it; a filter
// left without values goes away. Ticking turns the filter on.
func (u *UIModel) tickValue(field Field, value string) {
	filters := slices.Clone(u.view.Filters)
	i := u.filterIndex(field)
	if i < 0 {
		filters = append(filters, Filter{Field: field, Values: []string{value}, Enabled: true})
	} else {
		values := slices.Clone(filters[i].Ticked())
		if at := slices.Index(values, value); at >= 0 {
			values = slices.Delete(values, at, at+1)
		} else {
			values = append(values, value)
		}
		if len(values) == 0 {
			filters = slices.Delete(filters, i, i+1)
		} else {
			// ticking keeps the filter's mode, keeping or excluding
			filters[i] = filters[i].withTicked(values)
			filters[i].Enabled = true
		}
	}
	u.setFilters(filters)
	u.focusFilterRow(field, value)
}

// focusFilterRow puts the cursor on a field's row, or one of its values, as
// rows move when filters change.
func (u *UIModel) focusFilterRow(field Field, value string) {
	rows := u.filterRows()
	for i, r := range rows {
		if r.field == field && r.value == value {
			u.filters.cursor = i
			return
		}
	}
	u.filters.cursor = min(u.filters.cursor, max(len(rows)-1, 0))
}

func (u *UIModel) setFilters(filters []Filter) {
	v := u.view
	v.Filters = filters
	u.setView(v)
}

// filterFieldChoices are the derived fields, then every label key found on
// the nodes.
func filterFieldChoices(nodes []*Node) []Field {
	keys := map[string]struct{}{}
	for _, n := range nodes {
		n.addLabelKeys(keys)
	}
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Slice(sorted, func(a, b int) bool { return naturalLess(sorted[a], sorted[b]) })
	fields := append([]Field(nil), FilterFields...)
	for _, k := range sorted {
		fields = append(fields, LabelField(k))
	}
	return fields
}

// window is the range of n rows to show in height rows, keeping the cursor in
// view.
func window(n, cursor, height int) (start, end int) {
	if height < 1 {
		height = 1
	}
	if n <= height {
		return 0, n
	}
	start = min(max(cursor-height/2, 0), n-height)
	return start, start + height
}

func check(on bool) string {
	if on {
		return "[x]"
	}
	return "[ ]"
}

// filtersPanel lists the fields, and the open field's values, for the popup.
func (u *UIModel) filtersPanelView() panelView {
	p := &u.filters
	rows := u.filterRows()
	lines := make([]string, len(rows))
	for i, r := range rows {
		if r.isValue {
			selected := false
			if fi := u.filterIndex(r.field); fi >= 0 {
				selected = slices.Contains(u.view.Filters[fi].Ticked(), r.value)
			}
			lines[i] = fmt.Sprintf("    %s %s  %s", check(selected), r.value, helpStyle(fmt.Sprint(r.count)))
			continue
		}
		arrow := "▸"
		if r.field == p.open {
			arrow = "▾"
		}
		state := "[ ]"
		summary := ""
		if fi := u.filterIndex(r.field); fi >= 0 {
			f := u.view.Filters[fi]
			state = check(f.Enabled)
			summary = "  " + strings.Join(f.Ticked(), ", ")
			if f.Excludes() {
				summary = "  not " + strings.Join(f.Ticked(), ", ")
			}
			if !f.Enabled {
				summary += helpStyle("  (off)")
			}
		}
		lines[i] = fmt.Sprintf("%s %s %s%s", state, arrow, fieldName(r.field), summary)
	}
	return panelView{title: "Filters", rows: lines, cursor: p.cursor}
}
