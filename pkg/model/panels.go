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
	"sort"
	"strings"
)

type panelKind int

const (
	panelNone panelKind = iota
	panelHelp
	panelFilters
)

// panelKey handles a key while a panel is open; esc closes or steps back.
func (u *UIModel) panelKey(key string) {
	switch u.panel {
	case panelHelp:
		if key == "esc" || key == "?" {
			u.panel = panelNone
		}
	case panelFilters:
		u.filtersKey(key)
	}
}

// helpKeys lists every key, for the help panel.
var helpKeys = [][2]string{
	{"f", "filters: add, edit, turn on/off, remove"},
	{"n", "show or hide the node list (with a group-by)"},
	{"←/→ h/l", "previous / next page"},
	{"home", "first page"},
	{"ctrl+e", "compact or expand the header"},
	{"?", "this help"},
	{"q, ctrl+c", "quit"},
	{"esc", "close a panel or go back a step"},
}

func (u *UIModel) helpLines() []string {
	lines := []string{titleStyle("Keys"), ""}
	for _, k := range helpKeys {
		lines = append(lines, fmt.Sprintf("  %s  %s", headerKey(fmt.Sprintf("%-10s", k[0])), k[1]))
	}
	return append(lines, "", titleStyle("Colours"), "",
		"  NODEGROUP  "+managedNodeGroupStyle("name")+" is an EKS managed node group; plain is a Karpenter NodePool")
}

type filterStep int

const (
	stepList filterStep = iota
	stepField
	stepValues
)

// filtersPanel is the Filters panel (R3): the list of filters, then a field
// picker and a value picker to add or edit one.
type filtersPanel struct {
	step   filterStep
	cursor int
	// editing is the index of the filter being edited, -1 for a new one
	editing  int
	field    Field
	fields   []Field
	values   []ValueCount
	selected map[string]bool
}

func (p *filtersPanel) hint() string {
	switch p.step {
	case stepField:
		return "↑/↓: move • enter: choose • esc: back"
	case stepValues:
		return "↑/↓: move • space: select • enter: apply • esc: back"
	}
	return "↑/↓: move • space: on/off • d: remove • enter: edit/add • esc: close"
}

func (u *UIModel) openFilters() {
	u.panel = panelFilters
	u.filters = filtersPanel{editing: -1}
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
	switch p.step {
	case stepList:
		filters := u.view.Filters
		var moved bool
		if p.cursor, moved = moveCursor(p.cursor, len(filters)+1, key); moved {
			return
		}
		switch key {
		case "esc":
			u.panel = panelNone
		case " ":
			if p.cursor < len(filters) {
				changed := append([]Filter(nil), filters...)
				changed[p.cursor].Enabled = !changed[p.cursor].Enabled
				u.setFilters(changed)
			}
		case "d", "delete", "backspace":
			if p.cursor < len(filters) {
				changed := append(append([]Filter(nil), filters[:p.cursor]...), filters[p.cursor+1:]...)
				u.setFilters(changed)
				p.cursor = min(p.cursor, len(changed))
			}
		case "enter":
			if p.cursor == len(filters) {
				p.step, p.cursor, p.editing = stepField, 0, -1
				p.fields = filterFieldChoices(u.stats.Nodes)
				return
			}
			p.editing = p.cursor
			u.openValues(filters[p.cursor].Field)
		}
	case stepField:
		var moved bool
		if p.cursor, moved = moveCursor(p.cursor, len(p.fields), key); moved {
			return
		}
		switch key {
		case "esc":
			p.step, p.cursor = stepList, len(u.view.Filters)
		case "enter":
			if len(p.fields) == 0 {
				return
			}
			field := p.fields[p.cursor]
			p.editing = -1
			// a field has one filter: adding it again edits that one
			for i, f := range u.view.Filters {
				if f.Field == field {
					p.editing = i
				}
			}
			u.openValues(field)
		}
	case stepValues:
		var moved bool
		if p.cursor, moved = moveCursor(p.cursor, len(p.values), key); moved {
			return
		}
		switch key {
		case "esc":
			p.step, p.cursor = stepList, max(p.editing, 0)
		case " ":
			if len(p.values) > 0 {
				v := p.values[p.cursor].Value
				p.selected[v] = !p.selected[v]
			}
		case "enter":
			u.applyValues()
		}
	}
}

// openValues starts the value picker for the field, with the values present
// among all watched nodes and their node counts. Values the filter already
// has stay listed even when no node has them now.
func (u *UIModel) openValues(field Field) {
	p := &u.filters
	p.step, p.cursor, p.field = stepValues, 0, field
	p.values = ValueCounts(u.stats.Nodes, field)
	p.selected = map[string]bool{}
	if p.editing >= 0 {
		listed := map[string]bool{}
		for _, v := range p.values {
			listed[v.Value] = true
		}
		for _, v := range u.view.Filters[p.editing].Values {
			p.selected[v] = true
			if !listed[v] {
				p.values = append(p.values, ValueCount{Value: v})
			}
		}
	}
}

// applyValues saves the picked values into the filter being edited or a new
// one; picking none removes the filter.
func (u *UIModel) applyValues() {
	p := &u.filters
	var values []string
	for _, v := range p.values {
		if p.selected[v.Value] {
			values = append(values, v.Value)
		}
	}
	filters := append([]Filter(nil), u.view.Filters...)
	switch {
	case p.editing >= 0 && len(values) == 0:
		filters = append(filters[:p.editing], filters[p.editing+1:]...)
		p.cursor = min(p.editing, len(filters))
	case p.editing >= 0:
		filters[p.editing] = Filter{Field: p.field, Values: values, Enabled: true}
		p.cursor = p.editing
	case len(values) > 0:
		filters = append(filters, Filter{Field: p.field, Values: values, Enabled: true})
		p.cursor = len(filters) - 1
	default:
		p.cursor = len(filters)
	}
	p.step, p.editing = stepList, -1
	u.setFilters(filters)
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

func marker(selected bool) string {
	if selected {
		return "> "
	}
	return "  "
}

func check(on bool) string {
	if on {
		return "[x]"
	}
	return "[ ]"
}

// filterLines draws the Filters panel in at most height lines.
func (u *UIModel) filterLines(height int) []string {
	p := &u.filters
	var title string
	var rows []string
	switch p.step {
	case stepList:
		title = "Filters"
		nameW := 0
		for _, f := range u.view.Filters {
			nameW = max(nameW, len(fieldName(f.Field)))
		}
		for i, f := range u.view.Filters {
			state := ""
			if !f.Enabled {
				state = helpStyle("  (off)")
			}
			rows = append(rows, fmt.Sprintf("%s%s %-*s  %s%s", marker(i == p.cursor), check(f.Enabled), nameW, fieldName(f.Field), strings.Join(f.Values, ", "), state))
		}
		rows = append(rows, marker(p.cursor == len(u.view.Filters))+"+ add filter")
	case stepField:
		title = "Add filter: pick a field"
		for i, f := range p.fields {
			rows = append(rows, marker(i == p.cursor)+fieldName(f))
		}
	case stepValues:
		title = fieldName(p.field) + ": pick values (nodes with each)"
		valueW := 0
		for _, v := range p.values {
			valueW = max(valueW, len(v.Value))
		}
		for i, v := range p.values {
			rows = append(rows, fmt.Sprintf("%s%s %-*s  %d", marker(i == p.cursor), check(p.selected[v.Value]), valueW, v.Value, v.Count))
		}
		if len(p.values) == 0 {
			rows = append(rows, helpStyle("  no node has this field"))
		}
	}
	start, end := window(len(rows), p.cursor, height-2)
	lines := []string{titleStyle(title), ""}
	return append(lines, rows[start:end]...)
}
