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

// groupPanel lists every field, the group-by levels first in level order;
// ticking a field adds it as the next level, up to three (R4).
type groupPanel struct {
	cursor int
}

func (u *UIModel) openGroupBy() {
	u.panel = panelGroupBy
	u.group = groupPanel{}
}

func (p *groupPanel) hint() string {
	return "↑/↓: move • space/enter: add/remove level • K/J: move level up/down • esc: close"
}

// groupChoices are the levels in order, then every other field and label.
func (u *UIModel) groupChoices() []Field {
	fields := slices.Clone(u.view.GroupBy)
	for _, f := range filterFieldChoices(u.stats.Nodes) {
		if !slices.Contains(fields, f) {
			fields = append(fields, f)
		}
	}
	return fields
}

func (u *UIModel) groupByKey(key string) {
	p := &u.group
	fields := u.groupChoices()
	var moved bool
	if p.cursor, moved = moveCursor(p.cursor, len(fields), key); moved || len(fields) == 0 {
		return
	}
	field := fields[p.cursor]
	levels := u.view.GroupBy
	at := slices.Index(levels, field)
	switch key {
	case "esc":
		u.panel = panelNone
	case " ", "enter":
		switch {
		case at >= 0:
			u.setGroupBy(slices.Delete(slices.Clone(levels), at, at+1))
		case len(levels) < MaxGroupLevels:
			u.setGroupBy(append(slices.Clone(levels), field))
		default:
			return
		}
		p.cursor = max(slices.Index(u.groupChoices(), field), 0)
	case "K", "J":
		to := at - 1
		if key == "J" {
			to = at + 1
		}
		if at >= 0 && to >= 0 && to < len(levels) {
			next := slices.Clone(levels)
			next[at], next[to] = next[to], next[at]
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

func (u *UIModel) groupByPanel() panelView {
	fields := u.groupChoices()
	rows := make([]string, len(fields))
	for i, f := range fields {
		level := "[ ]"
		if at := slices.Index(u.view.GroupBy, f); at >= 0 {
			level = fmt.Sprintf("[%d]", at+1)
		}
		rows[i] = level + " " + fieldName(f)
	}
	title := fmt.Sprintf("Group by (up to %d levels)", MaxGroupLevels)
	if len(u.view.GroupBy) == MaxGroupLevels {
		title = "Group by (3 of 3 levels)"
	}
	return panelView{title: title, rows: rows, cursor: u.group.cursor}
}

// --- Sort (s) ---------------------------------------------------------------

type sortPanel struct {
	cursor int
	fields []Field
}

// sortChoices are the fields the node list sorts on (R5): creation, the
// columns, usage per resource, then every label.
func (u *UIModel) sortChoices() []Field {
	fields := []Field{FieldCreation, FieldName, FieldIP, FieldPrice, FieldIdle, FieldPods}
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
	case FieldCreation, FieldPrice, FieldIdle, FieldPods:
		return true
	}
	return strings.HasPrefix(string(f), usageFieldPrefix)
}

// flipSort flips the direction of the current sort from the main screen,
// the same change as space in the Sort panel.
func (u *UIModel) flipSort() {
	v := u.view
	v.Sort.Descending = !v.Sort.Descending
	u.setView(v)
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
		u.flipSort()
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

func (u *UIModel) sortPanel() panelView {
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
		rows[i] = fieldName(f) + current
	}
	return panelView{title: "Sort by", rows: rows, cursor: p.cursor}
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
	add(FieldIdle)
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

func (u *UIModel) columnsPanel() panelView {
	p := &u.columns
	rows := make([]string, len(p.fields))
	for i, f := range p.fields {
		name := fieldName(f)
		if f.Label() == "" {
			name = columnTitle(f)
		}
		rows[i] = check(slices.Contains(u.view.Columns, f)) + " " + name
	}
	return panelView{title: "Columns (shown first, in screen order)", rows: rows, cursor: p.cursor}
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
