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
	"strings"
	"time"
	"unicode/utf8"
)

// Views keeps the named views, shared by every cluster (views-and-export R2).
// Saves are written in the background; a failure comes back as a NoticeMsg.
type Views interface {
	ViewNames() []string
	View(name string) (ViewState, bool)
	SaveView(name string, v ViewState)
	DeleteView(name string)
}

// ReadOnlyViews is implemented by Views that hold views the TUI cannot change,
// such as the views of config.yaml (config-views R3). They are listed with
// a marker and can be applied, but not deleted or saved over.
type ReadOnlyViews interface {
	ReadOnly(name string) bool
}

// readOnlyMarker follows a read-only view's name in the Views panel.
const readOnlyMarker = " (config)"

// NoticeMsg puts a short message in the footer for noticeDuration.
type NoticeMsg struct {
	Text string
}

// noticeDuration is how long a footer notice stays.
const noticeDuration = 5 * time.Second

// SetViews enables the Views popup.
func (u *UIModel) SetViews(v Views) {
	u.views = v
}

func (u *UIModel) notify(text string) {
	u.notice, u.noticeUntil = text, time.Now().Add(noticeDuration)
	u.dirty = true
}

// currentNotice is the footer notice while it lasts.
func (u *UIModel) currentNotice() string {
	if u.notice == "" || time.Now().After(u.noticeUntil) {
		return ""
	}
	return u.notice
}

// viewsPanel is the Views popup: the names, then a row to save the current
// view under a typed name.
type viewsPanel struct {
	cursor int
	naming bool
	name   string
}

func (p *viewsPanel) hint() string {
	if p.naming {
		return "type a name • enter: save • esc: cancel"
	}
	return "↑/↓: move • enter: apply / save • d: delete • esc: close"
}

func (u *UIModel) openViews() {
	u.panel = panelViews
	u.viewsPanel = viewsPanel{}
}

func (u *UIModel) viewNames() []string {
	if u.views == nil {
		return nil
	}
	return u.views.ViewNames()
}

// readOnlyView tells whether the named view comes from config.yaml.
func (u *UIModel) readOnlyView(name string) bool {
	ro, ok := u.views.(ReadOnlyViews)
	return ok && ro.ReadOnly(name)
}

func (u *UIModel) viewsKey(key string) {
	p := &u.viewsPanel
	if p.naming {
		u.nameKey(key)
		return
	}
	names := u.viewNames()
	var moved bool
	if p.cursor, moved = moveCursor(p.cursor, len(names)+1, key); moved {
		return
	}
	switch key {
	case "esc":
		u.panel = panelNone
	case "enter", " ":
		if p.cursor == len(names) {
			if u.views == nil {
				u.notify("named views are not available")
				return
			}
			p.naming, p.name = true, ""
			return
		}
		u.applyView(names[p.cursor])
	case "d", "delete", "backspace":
		if p.cursor < len(names) {
			if u.readOnlyView(names[p.cursor]) {
				u.notify("view " + names[p.cursor] + " is defined in config.yaml; remove it there")
				return
			}
			u.views.DeleteView(names[p.cursor])
			u.notify("deleted view " + names[p.cursor])
			p.cursor = min(p.cursor, len(names)-1)
		}
	}
}

// nameKey edits the name of a view being saved: printable keys are text.
func (u *UIModel) nameKey(key string) {
	p := &u.viewsPanel
	switch key {
	case "enter":
		name := strings.TrimSpace(p.name)
		if name == "" {
			u.notify("a view needs a name")
			return
		}
		if u.readOnlyView(name) {
			u.notify("view " + name + " is defined in config.yaml; pick another name")
			return
		}
		u.views.SaveView(name, u.view.forSaving())
		u.notify("saved view " + name)
		p.naming = false
		p.cursor = max(indexOf(u.viewNames(), name), 0)
	case "esc":
		p.naming = false
	case "backspace":
		if p.name != "" {
			_, size := utf8.DecodeLastRuneInString(p.name)
			p.name = p.name[:len(p.name)-size]
		}
	case "ctrl+u":
		p.name = ""
	default:
		if key == " " || utf8.RuneCountInString(key) == 1 {
			p.name += key
		}
	}
}

func indexOf(names []string, name string) int {
	for i, n := range names {
		if n == name {
			return i
		}
	}
	return -1
}

// forSaving is the view as a named view keeps it: no search text, no groups
// folded one by one (their paths are one cluster's values), no header size.
func (s ViewState) forSaving() ViewState {
	s.Search, s.Collapsed, s.Header, s.Screen = "", nil, "", ""
	return s
}

// applyView loads a named view through the same path as a cluster's view, so
// resources and the r cycle follow, keeps what a named view doesn't carry,
// and saves the result as the cluster's view.
func (u *UIModel) applyView(name string) {
	v, ok := u.views.View(name)
	if !ok {
		return
	}
	v.Search, v.Collapsed, v.Header, v.Screen = u.view.Search, u.view.Collapsed, u.view.Header, u.view.Screen
	u.SetView(v)
	if u.onViewChange != nil {
		u.onViewChange(u.view)
	}
	u.panel = panelNone
	u.notify("applied view " + name)
}

func (u *UIModel) viewsPanelView() panelView {
	p := &u.viewsPanel
	names := u.viewNames()
	rows := make([]string, 0, len(names)+1)
	for _, name := range names {
		if u.readOnlyView(name) {
			name += readOnlyMarker
		}
		rows = append(rows, name)
	}
	if p.naming {
		rows = append(rows, "name: "+p.name+"█")
	} else {
		rows = append(rows, "+ save current view as…")
	}
	title := "Views"
	if len(names) == 0 && !p.naming {
		title = "Views (none saved yet)"
	}
	return panelView{title: title, rows: rows, cursor: p.cursor}
}
