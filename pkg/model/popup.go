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
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var (
	popupBorder   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#5f5f87", Dark: "#9e9ec8"}).Render
	popupSelected = lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "#1c1c1c"}).
			Background(lipgloss.AdaptiveColor{Light: "#5f5fd7", Dark: "#a8a8f0"}).Render
)

// popupMinWidth keeps short lists from drawing a sliver of a box.
const popupMinWidth = 32

// panelView is what a panel shows: a title, its rows and the selected row,
// -1 for none.
type panelView struct {
	title  string
	rows   []string
	cursor int
	// hint is the panel's keys, shown at the bottom of the box
	hint string
}

func (u *UIModel) panelView() (panelView, bool) {
	p, ok := u.panelContent()
	if ok {
		p.hint = u.helpText()
	}
	return p, ok
}

func (u *UIModel) panelContent() (panelView, bool) {
	switch u.panel {
	case panelHelp:
		return u.helpPanel(), true
	case panelFilters:
		return u.filtersPanelView(), true
	case panelGroupBy:
		return u.groupByPanel(), true
	case panelSort:
		return u.sortPanel(), true
	case panelColumns:
		return u.columnsPanel(), true
	case panelContexts:
		return u.contextsPanel(), true
	}
	return panelView{}, false
}

// popup draws the panel in a box with its title in the top border, at most
// height lines, scrolled to keep the selected row in view. The selected row is
// highlighted across the box.
func (u *UIModel) popup(p panelView, height int) []string {
	inner := popupMinWidth
	for _, r := range p.rows {
		inner = max(inner, textWidth(r))
	}
	inner = max(inner, textWidth(p.title)+2, textWidth(p.hint))
	if u.width > 0 {
		inner = min(inner, u.width-6)
	}
	inner = max(inner, 1)

	visible := max(height-2, 1)
	if p.hint != "" {
		visible = max(height-4, 1)
	}
	start, end := window(len(p.rows), max(p.cursor, 0), visible)

	title := " " + titleStyle(ansi.Truncate(p.title, inner-1, "…")) + " "
	top := popupBorder("╭─") + title + popupBorder(strings.Repeat("─", max(inner-textWidth(p.title)-1, 0))+"╮")

	scroll := ""
	if len(p.rows) > visible {
		scroll = fmt.Sprintf(" %d-%d of %d ", start+1, end, len(p.rows))
	}
	bottom := popupBorder("╰" + strings.Repeat("─", max(inner+2-len(scroll)-1, 0)) + scroll + "─╯")

	lines := []string{top}
	for i := start; i < end; i++ {
		row := ansi.Truncate(p.rows[i], inner, "…")
		if i == p.cursor {
			row = popupSelected(pad(ansi.Strip(row), inner))
		} else {
			row = pad(row, inner)
		}
		lines = append(lines, popupBorder("│")+" "+row+"\x1b[0m "+popupBorder("│"))
	}
	if p.hint != "" {
		lines = append(lines,
			popupBorder("├"+strings.Repeat("─", inner+2)+"┤"),
			popupBorder("│")+" "+pad(keyHint(ansi.Truncate(p.hint, inner, "…")), inner)+"\x1b[0m "+popupBorder("│"))
	}
	return append(lines, bottom)
}

// keyHint shows "key: action • key: action" with the keys in bold and the
// actions dimmed.
func keyHint(hint string) string {
	parts := strings.Split(hint, " • ")
	for i, part := range parts {
		if key, action, ok := strings.Cut(part, ": "); ok {
			parts[i] = headerKey(key) + headerAction(": "+action)
		} else {
			parts[i] = helpStyle(part)
		}
	}
	return strings.Join(parts, helpStyle(" • "))
}

// overlay draws the box over the body, centred, one line below the body's
// top. The body grows with blank lines when the box is taller than it, up to
// height.
func (u *UIModel) overlay(body, box []string, height int) []string {
	for len(body) < min(len(box)+1, height) {
		body = append(body, "")
	}
	boxW := 0
	for _, l := range box {
		boxW = max(boxW, lipgloss.Width(l))
	}
	x := 2
	if u.width > 0 {
		x = max((u.width-boxW)/2, 0)
	}
	y := min(1, max(len(body)-len(box), 0))
	for i, line := range box {
		row := y + i
		if row >= len(body) {
			break
		}
		bg := body[row]
		left := pad(ansi.Truncate(bg, x, ""), x)
		right := ansi.TruncateLeft(bg, x+boxW, "")
		body[row] = left + "\x1b[0m" + line + "\x1b[0m" + right
	}
	return body
}
