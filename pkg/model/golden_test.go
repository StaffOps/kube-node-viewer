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
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"
	"github.com/muesli/termenv"

	"github.com/staffops/kube-node-viewer/pkg/recommend"
)

var updateGolden = flag.Bool("update", false, "rewrite the golden screens in testdata/screens")

// goldenScenes are the main screens of the made-up cluster of the README
// screenshots, rendered as plain text.
var goldenScenes = []struct {
	name          string
	width, height int
	keys          func(u *UIModel)
}{
	{"main", 168, 38, func(u *UIModel) {}},
	{"group-by", 168, 38, func(u *UIModel) {
		v := u.view
		v.SetGroupBy(FieldNodeGroup, FieldZone)
		u.SetView(v)
	}},
	{"groups-folded", 168, 34, func(u *UIModel) {
		v := u.view
		v.SetGroupBy(FieldNodeGroup, FieldZone)
		u.SetView(v)
		press(u, "n")
	}},
	{"filters", 168, 38, func(u *UIModel) { press(u, "f", "enter", "down", "down", " ") }},
	{"columns", 168, 38, func(u *UIModel) { press(u, "o") }},
	{"node-details", 168, 52, func(u *UIModel) {
		u.View()
		press(u, "down", "down", "enter")
	}},
	{"node-pods", 168, 52, func(u *UIModel) {
		u.View()
		press(u, "down", "down", "enter")
		pressAll(u, "tab")
	}},
	{"recommendations", 168, 40, func(u *UIModel) {
		// as production builds it (aks-recommendations R1): an Azure advisor
		// whose note must not show on an EKS-only screen
		u.advisor = AdvisorsByCloud{"aws": fakeAdvisor{}, "azure": fakeAzureAdvisor{notes: []string{"pricing disabled: Azure rows that need a price are not shown"}}}
		u.SetRecommendOptions(RecommendOptions{Headroom: 0.8, OldAge: 30 * 24 * time.Hour})
		pressAll(u, "tab")
	}},
	// aks-recommendations R1: the Recommendations screen of a made-up AKS
	// cluster, with agent-pool rows
	{"recommendations-aks", 168, 30, func(u *UIModel) {
		u.advisor = AdvisorsByCloud{"aws": fakeAdvisor{}, "azure": fakeAzureAdvisor{}}
		for _, n := range u.stats.Nodes {
			n.SetPrice(0.0918)
		}
		u.Refresh()
		pressAll(u, "tab")
	}},
	// recommendations R9: the math of an AWS Graviton row, unchanged by the
	// Azure rules (the made-up cluster has no rightsize row)
	{"recommendation-why", 168, 40, func(u *UIModel) {
		u.advisor = AdvisorsByCloud{"aws": fakeAdvisor{}, "azure": fakeAzureAdvisor{}}
		u.SetRecommendOptions(RecommendOptions{Headroom: 0.8, OldAge: 30 * 24 * time.Hour})
		pressAll(u, "tab")
		for i, r := range u.recs.Recs {
			if r.Kind == recommend.Graviton {
				u.recCursor = i
				break
			}
		}
		pressAll(u, "enter")
	}},
	// aks-recommendations R7, R8, R9, R3: priced Azure rows (rightsize, an
	// info generation row, arm) and the waiting note
	{"recommendations-aks-priced", 168, 30, func(u *UIModel) {
		u.advisor = AdvisorsByCloud{"aws": fakeAdvisor{}, "azure": fakePricedAzure{}}
		pressAll(u, "tab")
	}},
	// azure-live-skus R8: the math of an Azure rightsize row names the
	// source of the sizes
	{"recommendation-why-aks", 168, 40, func(u *UIModel) {
		u.advisor = AdvisorsByCloud{"aws": fakeAdvisor{}, "azure": fakePricedAzure{}}
		pressAll(u, "tab")
		for i, r := range u.recs.Recs {
			if r.Kind == recommend.Rightsize {
				u.recCursor = i
				break
			}
		}
		pressAll(u, "enter")
	}},
	{"help", 168, 60, func(u *UIModel) { press(u, "?") }},
	{"header-compact", 168, 30, func(u *UIModel) { u.Update(tea.KeyMsg{Type: tea.KeyCtrlE}) }},
}

// goldenSetup builds the cluster of a scene that is not the README's made-up
// EKS cluster.
var goldenSetup = map[string]func(t *testing.T) *UIModel{"recommendations-aks": aksUI, "recommendations-aks-priced": aksPricedUI, "recommendation-why-aks": aksPricedUI}

// TestGoldenScreens compares each screen with testdata/screens/<name>.txt, so
// a layout change shows in the pull request as a text diff. After a change
// made on purpose, rewrite the files and commit them:
//
//	go test ./pkg/model -run TestGoldenScreens -update
func TestGoldenScreens(t *testing.T) {
	dir := filepath.Join("testdata", "screens")
	if *updateGolden {
		if os.Getenv("CI") != "" {
			t.Fatal("-update rewrites the expected screens; run it locally and commit the diff")
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// RUNEWIDTH_EASTASIAN=1 (or a CJK locale) widens the box-drawing
	// characters; the golden files are drawn with narrow ones. The ansi
	// package reads the variable once at start, so run again without it.
	if wide, err := strconv.ParseBool(os.Getenv("RUNEWIDTH_EASTASIAN")); err == nil && wide {
		args := []string{"-test.run=^TestGoldenScreens$"}
		if *updateGolden {
			args = append(args, "-update")
		}
		cmd := exec.Command(os.Args[0], args...) // #nosec G204 -- this test binary
		cmd.Env = append(os.Environ(), "RUNEWIDTH_EASTASIAN=0")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("with RUNEWIDTH_EASTASIAN=0: %v\n%s", err, out)
		}
		return
	}
	if c := runewidth.DefaultCondition; c.EastAsianWidth {
		c.EastAsianWidth = false
		c.CreateLUT()
		t.Cleanup(func() { c.EastAsianWidth = true; c.CreateLUT() })
	}
	// a golden file whose scene is gone would never be checked again
	files, err := filepath.Glob(filepath.Join(dir, "*.txt"))
	if err != nil {
		t.Fatal(err)
	}
	scenes := map[string]bool{}
	for _, s := range goldenScenes {
		scenes[s.name+".txt"] = true
	}
	for _, f := range files {
		if !scenes[filepath.Base(f)] {
			t.Errorf("%s has no scene in goldenScenes; delete it or add the scene", f)
		}
	}
	for _, s := range goldenScenes {
		t.Run(s.name, func(t *testing.T) {
			setup := goldenSetup[s.name]
			if setup == nil {
				setup = demoUI
			}
			u := setup(t)
			// plain text: no colour codes in the bars either
			u.progress = newProgress(u.style, progress.WithColorProfile(termenv.Ascii))
			u.Update(tea.WindowSizeMsg{Width: s.width, Height: s.height})
			s.keys(u)
			u.dirty = true
			got := screenText(u.View())
			file := filepath.Join(dir, s.name+".txt")
			if *updateGolden {
				if err := os.WriteFile(file, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("%v (run with -update to create it)", err)
			}
			if got != string(want) {
				t.Errorf("screen %s changed; if on purpose, run: go test ./pkg/model -run TestGoldenScreens -update\n%s",
					s.name, lineDiff(string(want), got))
			}
		})
	}
}

var ansiCode = regexp.MustCompile("\x1b\\[[0-9;]*m")

// screenText drops colour codes and the trailing spaces that only pad the
// terminal.
func screenText(s string) string {
	lines := strings.Split(ansiCode.ReplaceAllString(s, ""), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return strings.Join(lines, "\n") + "\n"
}

// lineDiff lists the lines that differ, numbered, as - want / + got.
func lineDiff(want, got string) string {
	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")
	var b strings.Builder
	for i := 0; i < len(w) || i < len(g); i++ {
		var wl, gl string
		if i < len(w) {
			wl = w[i]
		}
		if i < len(g) {
			gl = g[i]
		}
		if wl != gl {
			b.WriteString("line " + strconv.Itoa(i+1) + ":\n- " + wl + "\n+ " + gl + "\n")
		}
	}
	return b.String()
}
