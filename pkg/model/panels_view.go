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
	"strings"
	"unicode/utf8"

	v1 "k8s.io/api/core/v1"
)

// --- Group by (g) -----------------------------------------------------------

// groupPanel edits the up to three group-by levels (R4): pick a field per
// level, remove a level, move it up or down.
type groupPanel struct {
	picking bool
	// level is the level being picked for; len(GroupBy) adds one
	level  int
	cursor int
	fields []Field
}

func (u *UIModel) openGroupBy() {
	u.panel = panelGroupBy
	u.group = groupPanel{}
}

func (p *groupPanel) hint() string {
	if p.picking {
		return "↑/↓: move • enter: choose • esc: back"
	}
	return "↑/↓: move • enter: change/add • d: remove • K/J: move level up/down • esc: close"
}

func (u *UIModel) groupByKey(key string) {
	p := &u.group
	levels := u.view.GroupBy
	if p.picking {
		var moved bool
		if p.cursor, moved = moveCursor(p.cursor, len(p.fields), key); moved {
			return
		}
		switch key {
		case "esc":
			p.picking, p.cursor = false, p.level
		case "enter":
			next := slices.Clone(levels)
			field := p.fields[p.cursor]
			if p.level < len(next) {
				next[p.level] = field
			} else {
				next = append(next, field)
			}
			// a field picked again moves to this level
			for i := len(next) - 1; i >= 0; i-- {
				if i != p.level && next[i] == field {
					next = slices.Delete(next, i, i+1)
				}
			}
			u.setGroupBy(next)
			p.picking, p.cursor = false, min(p.level, len(u.view.GroupBy)-1)
		}
		return
	}

	rows := len(levels)
	if rows < MaxGroupLevels {
		rows++ // "+ add level"
	}
	var moved bool
	if p.cursor, moved = moveCursor(p.cursor, rows, key); moved {
		return
	}
	switch key {
	case "esc":
		u.panel = panelNone
	case "enter":
		p.picking, p.level = true, p.cursor
		p.fields = filterFieldChoices(u.stats.Nodes)
		p.cursor = 0
		if p.level < len(levels) {
			p.cursor = max(slices.Index(p.fields, levels[p.level]), 0)
		}
	case "d", "delete", "backspace":
		if p.cursor < len(levels) {
			u.setGroupBy(slices.Delete(slices.Clone(levels), p.cursor, p.cursor+1))
			p.cursor = min(p.cursor, len(u.view.GroupBy))
		}
	case "K", "J":
		to := p.cursor - 1
		if key == "J" {
			to = p.cursor + 1
		}
		if p.cursor < len(levels) && to >= 0 && to < len(levels) {
			next := slices.Clone(levels)
			next[p.cursor], next[to] = next[to], next[p.cursor]
			u.setGroupBy(next)
			p.cursor = to
		}
	}
}

func (u *UIModel) setGroupBy(fields []Field) {
	v := u.view
	v.SetGroupBy(fields...)
	u.setView(v)
}

func (u *UIModel) groupByLines(height int) []string {
	p := &u.group
	if p.picking {
		rows := make([]string, len(p.fields))
		for i, f := range p.fields {
			rows[i] = marker(i == p.cursor) + fieldName(f)
		}
		return panelLines(fmt.Sprintf("Group by, level %d: pick a field", p.level+1), rows, p.cursor, height)
	}
	var rows []string
	for i, f := range u.view.GroupBy {
		rows = append(rows, fmt.Sprintf("%s%d. %s", marker(i == p.cursor), i+1, fieldName(f)))
	}
	if len(u.view.GroupBy) < MaxGroupLevels {
		rows = append(rows, marker(p.cursor == len(u.view.GroupBy))+"+ add level")
	}
	return panelLines(fmt.Sprintf("Group by (up to %d levels)", MaxGroupLevels), rows, p.cursor, height)
}

// --- Sort (s) ---------------------------------------------------------------

type sortPanel struct {
	cursor int
	fields []Field
}

// sortChoices are the fields the node list sorts on (R5): creation, the
// columns, usage per resource, then every label.
func (u *UIModel) sortChoices() []Field {
	fields := []Field{FieldCreation, FieldName, FieldIP, FieldPrice, FieldPods}
	resources := []string{string(v1.ResourceCPU), string(v1.ResourceMemory)}
	for _, r := range u.view.Resources {
		if !slices.Contains(resources, r) {
			resources = append(resources, r)
		}
	}
	for _, r := range resources {
		fields = append(fields, UsageField(v1.ResourceName(r)))
	}
	return append(fields, filterFieldChoices(u.stats.Nodes)...)
}

// descendingFirst are the sorts that start biggest first: newest, priciest,
// fullest.
func descendingFirst(f Field) bool {
	switch f {
	case FieldCreation, FieldPrice, FieldPods:
		return true
	}
	return strings.HasPrefix(string(f), usageFieldPrefix)
}

func (u *UIModel) openSort() {
	u.panel = panelSort
	fields := u.sortChoices()
	u.sort = sortPanel{fields: fields, cursor: max(slices.Index(fields, u.view.Sort.Field), 0)}
}

func (p *sortPanel) hint() string {
	return "↑/↓: move • enter: sort by it • space: flip ↑/↓ • esc: close"
}

func (u *UIModel) sortKey(key string) {
	p := &u.sort
	var moved bool
	if p.cursor, moved = moveCursor(p.cursor, len(p.fields), key); moved {
		return
	}
	switch key {
	case "esc":
		u.panel = panelNone
	case " ":
		v := u.view
		v.Sort.Descending = !v.Sort.Descending
		u.setView(v)
	case "enter":
		field := p.fields[p.cursor]
		v := u.view
		if v.Sort.Field == field {
			v.Sort.Descending = !v.Sort.Descending
		} else {
			v.Sort = SortSpec{Field: field, Descending: descendingFirst(field)}
		}
		u.setView(v)
		u.panel = panelNone
	}
}

func (u *UIModel) sortLines(height int) []string {
	p := &u.sort
	rows := make([]string, len(p.fields))
	for i, f := range p.fields {
		current := ""
		if f == u.view.Sort.Field {
			current = "  ↑"
			if u.view.Sort.Descending {
				current = "  ↓"
			}
		}
		rows[i] = marker(i == p.cursor) + fieldName(f) + current
	}
	return panelLines("Sort by", rows, p.cursor, height)
}

// --- Columns (o) ------------------------------------------------------------

type columnsPanel struct {
	cursor int
	fields []Field
}

// columnChoices are the shown columns in order, then the fixed columns not
// shown, then every label.
func (u *UIModel) columnChoices() []Field {
	fields := slices.Clone(u.view.Columns)
	add := func(f Field) {
		if !slices.Contains(fields, f) {
			fields = append(fields, f)
		}
	}
	for _, f := range DefaultColumns {
		add(f)
	}
	add(FieldRegion)
	add(FieldAge)
	add(LabelField(LabelPrefix + "node-age"))
	for _, r := range []v1.ResourceName{v1.ResourceCPU, v1.ResourceMemory} {
		add(LabelField(LabelPrefix + "node-" + string(r) + "-usage"))
	}
	for _, f := range filterFieldChoices(u.stats.Nodes) {
		if f.Label() != "" {
			add(f)
		}
	}
	return fields
}

func (u *UIModel) openColumns() {
	u.panel = panelColumns
	u.columns = columnsPanel{fields: u.columnChoices()}
}

func (p *columnsPanel) hint() string {
	return "↑/↓: move • space: show/hide • K/J: move column left/right • esc: close"
}

func (u *UIModel) columnsKey(key string) {
	p := &u.columns
	var moved bool
	if p.cursor, moved = moveCursor(p.cursor, len(p.fields), key); moved {
		return
	}
	field := p.fields[p.cursor]
	shown := u.view.Columns
	at := slices.Index(shown, field)
	switch key {
	case "esc":
		u.panel = panelNone
	case " ":
		v := u.view
		if at >= 0 {
			v.Columns = slices.Delete(slices.Clone(shown), at, at+1)
		} else {
			v.Columns = append(slices.Clone(shown), field)
		}
		u.setView(v)
		u.reorderColumnChoices()
	case "K", "J":
		to := at - 1
		if key == "J" {
			to = at + 1
		}
		if at >= 0 && to >= 0 && to < len(shown) {
			v := u.view
			v.Columns = slices.Clone(shown)
			v.Columns[at], v.Columns[to] = v.Columns[to], v.Columns[at]
			u.setView(v)
			u.reorderColumnChoices()
		}
	}
}

// reorderColumnChoices keeps the list in screen order after a change, with
// the cursor on the same column.
func (u *UIModel) reorderColumnChoices() {
	p := &u.columns
	field := p.fields[p.cursor]
	p.fields = u.columnChoices()
	p.cursor = max(slices.Index(p.fields, field), 0)
}

func (u *UIModel) columnLines(height int) []string {
	p := &u.columns
	rows := make([]string, len(p.fields))
	for i, f := range p.fields {
		name := fieldName(f)
		if f.Label() == "" {
			name = columnTitle(f)
		}
		rows[i] = marker(i == p.cursor) + check(slices.Contains(u.view.Columns, f)) + " " + name
	}
	return panelLines("Columns (shown first, in screen order)", rows, p.cursor, height)
}

// --- Search (/) -------------------------------------------------------------

// searchKey edits the search text (R6): it filters on every keystroke; enter
// keeps it, esc clears it. The text is not saved with the view.
func (u *UIModel) searchKey(key string) {
	switch key {
	case "enter":
		u.searching = false
	case "esc":
		u.searching = false
		u.view.Search = ""
	case "backspace":
		if s := u.view.Search; s != "" {
			_, size := utf8.DecodeLastRuneInString(s)
			u.view.Search = s[:len(s)-size]
		}
	case "ctrl+u":
		u.view.Search = ""
	default:
		// typed text, a space included; other named keys are ignored
		if key == " " || utf8.RuneCountInString(key) == 1 {
			u.view.Search += key
		}
	}
	u.page = 0
}

// --- Resources (r) ----------------------------------------------------------

// resourceCycle is what r steps through (R9): cpu, memory, both, then any
// other list the view started with.
func (u *UIModel) resourceCycle() [][]string {
	cycle := [][]string{{"cpu"}, {"memory"}, {"cpu", "memory"}}
	if u.customResources != nil {
		cycle = append(cycle, u.customResources)
	}
	return cycle
}

func (u *UIModel) cycleResources() {
	cycle := u.resourceCycle()
	next := cycle[0]
	for i, r := range cycle {
		if slices.Equal(r, u.view.Resources) {
			next = cycle[(i+1)%len(cycle)]
		}
	}
	v := u.view
	v.Resources = slices.Clone(next)
	u.setView(v)
	// snapshots already sum every resource; only the list shown changes
	u.SetResources(v.Resources)
}

// panelLines draws a panel's title and the rows that fit around the cursor.
func panelLines(title string, rows []string, cursor, height int) []string {
	start, end := window(len(rows), cursor, height-2)
	return append([]string{titleStyle(title), ""}, rows[start:end]...)
}
