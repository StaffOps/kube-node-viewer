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

import "strings"

// focusLevel narrows the node list to the nodes whose field has the value,
// as a group heading of that field shows them (group-focus).
type focusLevel struct {
	Field Field
	Value string
}

// focusStep is what one enter on a heading added to the focus, and the
// heading it came from, so esc undoes one enter (R3).
type focusStep struct {
	levels  []focusLevel
	heading string
}

// focused tells whether a group is focused.
func (u *UIModel) focused() bool { return len(u.focus) > 0 }

// focusLevels is every level of the focus, in the order they were added.
func (u *UIModel) focusLevels() []focusLevel {
	var levels []focusLevel
	for _, s := range u.focus {
		levels = append(levels, s.levels...)
	}
	return levels
}

func (u *UIModel) focusHasField(f Field) bool {
	for _, l := range u.focusLevels() {
		if l.Field == f {
			return true
		}
	}
	return false
}

// focusNodes keeps the nodes in every focus level, in their order. It reads
// Field.Value, as groups are built from it, so <none> matches the nodes
// without a value.
func (u *UIModel) focusNodes(nodes []*Node) []*Node {
	levels := u.focusLevels()
	if len(levels) == 0 {
		return nodes
	}
	out := make([]*Node, 0, len(nodes))
	for _, n := range nodes {
		keep := true
		for _, l := range levels {
			if l.Field.Value(n) != l.Value {
				keep = false
				break
			}
		}
		if keep {
			out = append(out, n)
		}
	}
	return out
}

// headingLevels are the field and value of each level of a heading's path
// under the current group-by.
func (u *UIModel) headingLevels(path []string) []focusLevel {
	var levels []focusLevel
	for i, value := range path {
		if i >= len(u.view.GroupBy) {
			break
		}
		levels = append(levels, focusLevel{Field: u.view.GroupBy[i], Value: value})
	}
	return levels
}

// inFocus tells whether every level of the heading is already a focus level:
// the focused group's heading or one above it (R4, R5).
func (u *UIModel) inFocus(path []string) bool {
	if !u.focused() {
		return false
	}
	have := u.focusLevels()
	for _, l := range u.headingLevels(path) {
		found := false
		for _, h := range have {
			if h == l {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// enterFocus adds the heading's levels whose field is not in the focus yet,
// as one step (R1, R4). The cursor stays on the heading.
func (u *UIModel) enterFocus(path []string) {
	var step focusStep
	for _, l := range u.headingLevels(path) {
		if !u.focusHasField(l.Field) {
			step.levels = append(step.levels, l)
		}
	}
	if len(step.levels) == 0 {
		return
	}
	step.heading = groupCursorKey(path)
	u.focus = append(u.focus, step)
	u.cursorKey, u.cursorPath, u.cursorDelta = step.heading, path, 0
	u.page = 0
	u.dirty = true
}

// leaveFocus undoes the last enter and puts the cursor back on the heading
// it came from, on that heading's page (R3).
func (u *UIModel) leaveFocus() {
	if !u.focused() {
		return
	}
	step := u.focus[len(u.focus)-1]
	u.focus = u.focus[:len(u.focus)-1]
	u.cursorKey, u.cursorPath, u.cursorDelta = step.heading, nil, 0
	u.cursorFind = true
	u.page = 0
	u.dirty = true
}

// displayView is the view the focused list is drawn from: nodes shown, every
// level, and the headings in focus unfolded. u.view and its Collapsed are
// left alone, so nothing of it is saved (D3, D5).
func (u *UIModel) displayView() ViewState {
	v := u.view
	v.ShowNodes, v.GroupDepth, v.Details = true, 0, false
	v.Collapsed = nil
	for _, c := range u.view.Collapsed {
		if !u.inFocus(strings.Split(c, pathSeparator)) {
			v.Collapsed = append(v.Collapsed, c)
		}
	}
	return v
}

// focusSummary is the focus for the View line: field=value per level.
func (u *UIModel) focusSummary() string {
	var parts []string
	for _, l := range u.focusLevels() {
		parts = append(parts, fieldName(l.Field)+"="+l.Value)
	}
	return strings.Join(parts, pathSeparator)
}
