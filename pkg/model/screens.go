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
	"unicode/utf8"
)

// Screens (recommendations R1, R2). The empty value is Overview.
const (
	ScreenOverview        = ""
	ScreenRecommendations = "recommendations"
)

// commands the : line knows, with their short forms.
var commands = []struct{ name, short, screen string }{
	{"overview", "ov", ScreenOverview},
	{"recommendations", "rec", ScreenRecommendations},
}

func (u *UIModel) onRecommendations() bool {
	return u.view.Screen == ScreenRecommendations
}

// setScreen switches screens and saves the choice with the cluster's view.
func (u *UIModel) setScreen(screen string) {
	if u.view.Screen == screen {
		return
	}
	v := u.view
	v.Screen = screen
	u.setView(v)
	u.recCursor = 0
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

// complete finishes the word when exactly one command starts with it.
func complete(prefix string) string {
	p := strings.TrimSpace(prefix)
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
	for _, c := range commands {
		if cmd == c.name || cmd == c.short {
			u.setScreen(c.screen)
			return
		}
	}
	u.notify(`unknown command "` + cmd + `" (try :overview, :recommendations)`)
}

// screenTitle names the screen for the header.
func (u *UIModel) screenTitle() string {
	if u.onRecommendations() {
		return "Recommendations"
	}
	return "Overview"
}
