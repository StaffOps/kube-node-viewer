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

	tea "github.com/charmbracelet/bubbletea"
)

// Switcher lists kubeconfig contexts and connects to one (R10). Both run off
// the UI goroutine, inside a tea.Cmd: they read files and reach the API
// server.
type Switcher interface {
	Contexts() (names []string, current string, err error)
	Switch(context string) (Session, error)
}

// Session is a connected cluster: its own store, fed by its own watches, the
// view state to show it with, and where to save view changes.
type Session struct {
	Cluster *Cluster
	Info    ClusterInfo
	View    ViewState
	Save    func(ViewState)
}

type contextsMsg struct {
	names   []string
	current string
	err     error
}

type switchedMsg struct {
	session Session
	err     error
	context string
}

// contextsPanel is the context picker (c).
type contextsPanel struct {
	names     []string
	current   string
	cursor    int
	loading   bool
	switching string
	err       error
}

func (p *contextsPanel) hint() string {
	if p.switching != "" {
		return "connecting… • esc: close"
	}
	return "↑/↓: move • enter: switch • esc: close"
}

// SetSwitcher enables the context picker.
func (u *UIModel) SetSwitcher(s Switcher) {
	u.switcher = s
}

// Connect shows a session: its store replaces the current one, and messages
// from the old store's generation are dropped from now on.
func (u *UIModel) Connect(s Session) {
	u.cluster = s.Cluster
	u.gen = s.Cluster.Generation()
	u.stats = Stats{}
	u.onViewChange = s.Save
	u.SetView(s.View)
	u.SetClusterInfo(s.Info)
}

func (u *UIModel) openContexts() tea.Cmd {
	u.panel = panelContexts
	u.contexts = contextsPanel{loading: true}
	switcher := u.switcher
	if switcher == nil {
		u.contexts = contextsPanel{err: fmt.Errorf("context switching is not available")}
		return nil
	}
	return func() tea.Msg {
		names, current, err := switcher.Contexts()
		return contextsMsg{names: names, current: current, err: err}
	}
}

func (u *UIModel) contextsLoaded(msg contextsMsg) {
	if u.panel != panelContexts {
		return
	}
	p := &u.contexts
	p.loading, p.names, p.err = false, msg.names, msg.err
	// the context on screen, which may not be the kubeconfig's current one
	p.current = u.info.Context
	if p.current == "" {
		p.current = msg.current
	}
	p.cursor = max(slices.Index(p.names, p.current), 0)
}

func (u *UIModel) contextsKey(key string) tea.Cmd {
	p := &u.contexts
	if key == "esc" {
		u.panel = panelNone
		return nil
	}
	if p.loading || p.switching != "" {
		return nil
	}
	var moved bool
	if p.cursor, moved = moveCursor(p.cursor, len(p.names), key); moved || key != "enter" || len(p.names) == 0 {
		return nil
	}
	name := p.names[p.cursor]
	if name == p.current {
		u.panel = panelNone
		return nil
	}
	p.switching, p.err = name, nil
	switcher := u.switcher
	return func() tea.Msg {
		s, err := switcher.Switch(name)
		return switchedMsg{session: s, err: err, context: name}
	}
}

func (u *UIModel) switched(msg switchedMsg) tea.Cmd {
	if msg.err != nil {
		u.contexts.switching = ""
		u.contexts.err = fmt.Errorf("%s: %w", msg.context, msg.err)
		return nil
	}
	if u.panel == panelContexts {
		u.panel = panelNone
	}
	u.contexts.switching = ""
	u.Connect(msg.session)
	return u.snapshotCmd()
}

func (u *UIModel) contextsPanel() panelView {
	p := &u.contexts
	var rows []string
	switch {
	case p.loading:
		rows = []string{"  reading kubeconfig…"}
	case p.switching != "":
		rows = []string{"  connecting to " + p.switching + "…"}
	default:
		for _, name := range p.names {
			current := ""
			if name == p.current {
				current = helpStyle("  (current)")
			}
			rows = append(rows, name+current)
		}
	}
	if p.err != nil {
		rows = append(rows, "", u.style.red("  "+p.err.Error()))
	}
	cursor := p.cursor
	if p.loading || p.switching != "" {
		cursor = -1
	}
	return panelView{title: "Context", rows: rows, cursor: cursor}
}
