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
	"strings"
	"unicode/utf8"
)

// Screens (recommendations R1, R2). The empty value is Overview.
const (
	ScreenOverview        = ""
	ScreenRecommendations = "recommendations"
	// ScreenClusters is the Clusters screen (clusters-screen R1)
	ScreenClusters = "clusters"
)

// commands the : line knows, with their short forms.
var commands = []struct{ name, short, screen string }{
	{"overview", "ov", ScreenOverview},
	{"recommendations", "rec", ScreenRecommendations},
	{"clusters", "cl", ScreenClusters},
}

func (u *UIModel) onRecommendations() bool {
	return u.view.Screen == ScreenRecommendations
}

// setScreen switches screens and saves the choice with the cluster's view;
// entering or leaving the Clusters screen connects or closes its members
// (clusters-screen D3).
func (u *UIModel) setScreen(screen string) {
	if u.view.Screen == screen {
		return
	}
	v := u.view
	v.Screen = screen
	u.setView(v)
	u.recCursor = 0
	if screen == ScreenClusters {
		// the node details popup closes and no focus is drawn
		// (clusters-screen R13)
		if u.panel == panelNode {
			u.panel = panelNone
		}
		u.cursorKey, u.cursorPath, u.cursorDelta = "", nil, 0
	}
	u.fleetSync()
}

// knownScreen tells a screen this version shows; a saved one it does not
// know opens Overview (clusters-screen R1).
func knownScreen(screen string) bool {
	switch screen {
	case ScreenOverview, ScreenRecommendations, ScreenClusters:
		return true
	}
	return false
}

// toggleScreen is tab in the main view.
func (u *UIModel) toggleScreen() {
	if u.onRecommendations() {
		u.setScreen(ScreenOverview)
	} else {
		u.setScreen(ScreenRecommendations)
	}
}

// commandKey edits and runs the : command line; printable keys are text.
func (u *UIModel) commandKey(key string) {
	switch key {
	case "enter":
		u.commanding = false
		u.runCommand(strings.TrimSpace(u.command))
	case "esc":
		u.commanding = false
	case "tab":
		u.command = complete(u.command)
	case "backspace":
		if u.command != "" {
			_, size := utf8.DecodeLastRuneInString(u.command)
			u.command = u.command[:len(u.command)-size]
		}
	case "ctrl+u":
		u.command = ""
	default:
		if key == " " || utf8.RuneCountInString(key) == 1 {
			u.command += key
		}
	}
}

// complete finishes the word when exactly one command starts with it, or
// the cloud word after clusters (clusters-screen R14).
func complete(prefix string) string {
	p := strings.TrimSpace(prefix)
	if name, word, ok := strings.Cut(p, " "); ok {
		if name != "clusters" && name != "cl" {
			return prefix
		}
		word = strings.TrimSpace(word)
		match := ""
		for _, c := range ScopeClouds {
			if strings.HasPrefix(c, word) {
				if match != "" {
					return prefix
				}
				match = c
			}
		}
		if match == "" {
			return prefix
		}
		return name + " " + match
	}
	match := ""
	for _, c := range commands {
		if strings.HasPrefix(c.name, p) {
			if match != "" {
				return prefix
			}
			match = c.name
		}
	}
	if match == "" {
		return prefix
	}
	return match
}

func (u *UIModel) runCommand(cmd string) {
	if cmd == "" {
		return
	}
	fields := strings.Fields(cmd)
	name := fields[0]
	for _, c := range commands {
		if name != c.name && name != c.short {
			continue
		}
		if c.screen == ScreenClusters && len(fields) <= 2 {
			// a cloud word scopes the screen, none shows every cloud
			// (clusters-screen R14)
			scope := ""
			if len(fields) == 2 {
				scope = fields[1]
				if !slices.Contains(ScopeClouds, scope) {
					u.notify(`unknown cloud "` + scope + `" (aws, azure, gce)`)
					return
				}
			}
			u.setScope(scope)
			if u.onClusters() {
				u.fleetSync()
				return
			}
			u.setScreen(c.screen)
			return
		}
		if len(fields) == 1 {
			u.setScreen(c.screen)
			return
		}
	}
	u.notify(`unknown command "` + cmd + `" (try :overview, :recommendations, :clusters)`)
}

// screenTitle names the screen for the header.
func (u *UIModel) screenTitle() string {
	if u.onRecommendations() {
		return "Recommendations"
	}
	if u.onClusters() {
		return "Clusters"
	}
	return "Overview"
}
