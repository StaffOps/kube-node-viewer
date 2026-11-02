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
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/staffops/kube-node-viewer/pkg/recommend"
)

type fakeAdvisor struct{}

var fakeSpecs = map[string]recommend.Spec{
	"m6i.xlarge": {VCPU: 4, MemGiB: 16, Arch: "x86_64", Current: true}, "m6i.large": {VCPU: 2, MemGiB: 8, Arch: "x86_64", Current: true},
	"m6i.2xlarge": {VCPU: 8, MemGiB: 32, Arch: "x86_64", Current: true},
	"m7g.xlarge":  {VCPU: 4, MemGiB: 16, Arch: "arm64", Current: true}, "m7g.2xlarge": {VCPU: 8, MemGiB: 32, Arch: "arm64", Current: true},
	"t3.large": {VCPU: 2, MemGiB: 8, Arch: "x86_64", Current: true}, "t4g.large": {VCPU: 2, MemGiB: 8, Arch: "arm64", Current: true},
}
var fakePrices = map[string]float64{
	"m6i.xlarge": 0.192, "m6i.large": 0.096, "m6i.2xlarge": 0.384, "m7g.xlarge": 0.1632, "m7g.2xlarge": 0.3264,
	"t3.large": 0.0832, "t4g.large": 0.0672,
}

func (fakeAdvisor) Spec(t string) (recommend.Spec, bool)  { s, ok := fakeSpecs[t]; return s, ok }
func (fakeAdvisor) OnDemand(t string) (float64, bool)     { p, ok := fakePrices[t]; return p, ok }
func (fakeAdvisor) SpotIn(string, string) (float64, bool) { return 0, false }
func (fakeAdvisor) Types() []string {
	var out []string
	for t := range fakeSpecs {
		out = append(out, t)
	}
	return out
}

// run applies a command and everything it leads to, batches included, as
// the Bubble Tea loop does; ticks are skipped.
func runAll(u *UIModel, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	switch msg := cmd().(type) {
	case nil, tickMsg:
	case tea.BatchMsg:
		for _, c := range msg {
			runAll(u, c)
		}
	default:
		_, next := u.Update(msg)
		runAll(u, next)
	}
}

func recUI(t *testing.T) *UIModel {
	u := demoUI(t)
	u.advisor = fakeAdvisor{}
	u.SetRecommendOptions(RecommendOptions{Headroom: 0.8, OldAge: 30 * 24 * time.Hour})
	u.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	return u
}

func pressAll(u *UIModel, keys ...string) {
	for _, k := range keys {
		runAll(u, press(u, k))
	}
}

func TestCommandLineSwitchesScreens(t *testing.T) {
	u := recUI(t)
	var saved []ViewState
	u.OnViewChange(func(v ViewState) { saved = append(saved, v) })

	pressAll(u, ":", "r", "e", "c")
	if !strings.Contains(u.View(), ":rec█") {
		t.Fatalf("command line not shown:\n%s", u.View())
	}
	pressAll(u, "enter")
	out := u.View()
	if !u.onRecommendations() || !strings.Contains(out, "Recommendations") || !strings.Contains(out, "Savings:") {
		t.Fatalf("not on Recommendations:\n%s", out)
	}
	if len(saved) == 0 || saved[len(saved)-1].Screen != ScreenRecommendations {
		t.Error("screen not saved with the cluster's view")
	}
	if (ViewState{Screen: ScreenRecommendations}).forSaving().Screen != "" {
		t.Error("named views keep the screen")
	}
	pressAll(u, "tab")
	if u.onRecommendations() {
		t.Error("tab did not switch back")
	}
}

func TestCommandLineTextCompletionAndErrors(t *testing.T) {
	u := recUI(t)
	pressAll(u, ":", "q")
	if !u.commanding {
		t.Fatal("q left the command line")
	}
	pressAll(u, "backspace", "o", "v", "e", "r", "tab")
	if u.command != "overview" {
		t.Errorf("completion: %q", u.command)
	}
	pressAll(u, "ctrl+u")
	for _, r := range "foo" {
		pressAll(u, string(r))
	}
	pressAll(u, "enter")
	if !strings.Contains(u.View(), `unknown command "foo"`) {
		t.Errorf("unknown command not reported:\n%s", u.View())
	}
}

func TestRecommendationsScreen(t *testing.T) {
	u := recUI(t)
	pressAll(u, "tab")
	out := u.View()
	for _, want := range []string{"KIND", "graviton", "SAVES/MO"} {
		if !strings.Contains(out, want) {
			t.Errorf("screen misses %q:\n%s", want, out)
		}
	}
	// spot rows never come before others
	if strings.Contains(out, "spot  ") && strings.Index(out, "spot  ") < strings.Index(out, "graviton") {
		t.Error("spot listed before graviton")
	}
	// Overview-only keys do nothing; enter opens the math
	pressAll(u, "g", "s", "n")
	if u.panel != panelNone {
		t.Fatal("an Overview key acted on Recommendations")
	}
	pressAll(u, "enter")
	if u.panel != panelRecMath || !strings.Contains(u.View(), "Why") {
		t.Errorf("math popup:\n%s", u.View())
	}
	pressAll(u, "esc")
	if u.panel != panelNone {
		t.Error("esc did not close the math")
	}
}

func TestRecommendationsWithoutAdvisor(t *testing.T) {
	u := demoUI(t)
	u.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	pressAll(u, "tab")
	if !strings.Contains(u.View(), "not available") {
		t.Errorf("no advisor:\n%s", u.View())
	}
}
