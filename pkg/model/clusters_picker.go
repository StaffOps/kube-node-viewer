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
	"slices"
	"sort"
)

// clusterPicker is the Clusters screen's context list (c): contexts under
// their guessed cloud, ticks pending until enter (clusters-screen R8).
type clusterPicker struct {
	rows   []pickerRow
	ticked map[string]bool
	cursor int
}

type pickerRow struct {
	cloud string
	// name is empty on a cloud's line
	name    string
	missing bool
}

// pickerClouds is the order of the picker's cloud lines.
var pickerClouds = []string{"aws", "azure", "gce", "unknown", "missing"}

func (u *UIModel) openClusterPicker() {
	if u.fl.fleet == nil {
		return
	}
	p := clusterPicker{ticked: map[string]bool{}}
	byCloud := map[string][]pickerRow{}
	known := map[string]bool{}
	for _, info := range u.fl.infos {
		known[info.Name] = true
		cloud := info.Cloud
		if cloud == "" {
			cloud = "unknown"
		}
		byCloud[cloud] = append(byCloud[cloud], pickerRow{cloud: cloud, name: info.Name})
	}
	for _, name := range u.fl.chosen {
		if !known[name] {
			byCloud["missing"] = append(byCloud["missing"], pickerRow{cloud: "missing", name: name, missing: true})
		}
	}
	for _, cloud := range pickerClouds {
		rows := byCloud[cloud]
		if len(rows) == 0 {
			continue
		}
		sort.Slice(rows, func(a, b int) bool { return rows[a].name < rows[b].name })
		p.rows = append(p.rows, pickerRow{cloud: cloud})
		p.rows = append(p.rows, rows...)
	}
	if u.fl.saved {
		for _, name := range u.fl.chosen {
			p.ticked[name] = true
		}
	} else {
		// first use: the context on screen and its cloud's contexts, or the
		// scope's cloud when one is given (clusters-screen R8, D5)
		cloud := u.contextInfo(u.info.Context).Cloud
		if u.fl.scope != "" {
			cloud = u.fl.scope
		}
		p.ticked[u.info.Context] = true
		for _, info := range u.fl.infos {
			if cloud != "" && info.Cloud == cloud {
				p.ticked[info.Name] = true
			}
		}
	}
	p.cursor = max(slices.IndexFunc(p.rows, func(r pickerRow) bool { return r.name == u.info.Context }), 0)
	u.fl.picker = p
	u.panel = panelClusters
}

// cloudTicks lists the contexts on a cloud's line.
func (p *clusterPicker) cloudNames(cloud string) []string {
	var out []string
	for _, r := range p.rows {
		if r.cloud == cloud && r.name != "" {
			out = append(out, r.name)
		}
	}
	return out
}

func (u *UIModel) clusterPickerKey(key string) {
	p := &u.fl.picker
	switch key {
	case "esc":
		// pending ticks are dropped; nothing is saved (clusters-screen R8)
		u.panel = panelNone
		return
	case "enter":
		var chosen []string
		for _, r := range p.rows {
			if r.name != "" && p.ticked[r.name] {
				chosen = append(chosen, r.name)
			}
		}
		u.fl.chosen, u.fl.saved = chosen, true
		if u.fl.store != nil {
			u.fl.store.SaveFleet(chosen, u.fl.scope)
		}
		u.panel = panelNone
		u.fleetSync()
		return
	case " ":
		if len(p.rows) == 0 {
			return
		}
		r := p.rows[p.cursor]
		if r.name != "" {
			p.ticked[r.name] = !p.ticked[r.name]
			return
		}
		names := p.cloudNames(r.cloud)
		all := true
		for _, n := range names {
			all = all && p.ticked[n]
		}
		for _, n := range names {
			p.ticked[n] = !all
		}
		return
	}
	p.cursor, _ = moveCursor(p.cursor, len(p.rows), key)
}

func (u *UIModel) clusterPickerPanel() panelView {
	p := &u.fl.picker
	// a context with the server of the one on screen, or of an earlier one
	// by name, is the same cluster (clusters-screen R17)
	sameAs := map[string]string{}
	first := map[string]string{}
	if s := normalizeServer(u.info.Server); s != "" {
		first[s] = u.info.Context
	}
	infos := slices.Clone(u.fl.infos)
	sort.Slice(infos, func(a, b int) bool { return infos[a].Name < infos[b].Name })
	for _, info := range infos {
		s := normalizeServer(info.Server)
		if s == "" {
			continue
		}
		if owner, ok := first[s]; ok && owner != info.Name {
			sameAs[info.Name] = owner
		} else if !ok {
			first[s] = info.Name
		}
	}
	rows := make([]string, 0, len(p.rows))
	for _, r := range p.rows {
		if r.name == "" {
			rows = append(rows, topGroupStyle(r.cloud))
			continue
		}
		box := "[ ] "
		if p.ticked[r.name] {
			box = "[x] "
		}
		line := "  " + box + r.name
		if r.name == u.info.Context {
			line += helpStyle("  (on screen)")
		}
		if owner, ok := sameAs[r.name]; ok {
			line += helpStyle("  same cluster as " + shortContext(owner))
		}
		if r.missing {
			line += helpStyle("  missing")
		}
		rows = append(rows, line)
	}
	if len(rows) == 0 {
		rows = []string{"  no contexts in the kubeconfig"}
	}
	return panelView{title: "Clusters", rows: rows, cursor: p.cursor}
}
