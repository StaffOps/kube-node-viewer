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

// Modified by the kube-node-viewer authors, 2026.
package main

import (
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/staffops/kube-node-viewer/pkg/model"
	"github.com/staffops/kube-node-viewer/pkg/state"
)

//go:generate cp -r ../../ATTRIBUTION.md ./
//go:embed ATTRIBUTION.md
var attribution string

func main() {
	flags, err := ParseFlags()
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		log.Fatalf("cannot parse flags: %v", err)
	}

	if flags.ShowAttribution {
		fmt.Println(attribution)
		os.Exit(0)
	}

	if flags.Version {
		info, ok := debug.ReadBuildInfo()
		version, commit := resolveVersion(version, commit, info, ok)
		fmt.Printf("kube-node-viewer version %s\n", version)
		fmt.Printf("commit: %s\n", commit)
		fmt.Printf("built at: %s\n", date)
		fmt.Printf("built by: %s\n", builtBy)
		os.Exit(0)
	}

	style, err := model.ParseStyle(flags.Style)
	if err != nil {
		log.Fatalf("creating style, %s", err)
	}
	nodeSelector, err := labels.Parse(flags.NodeSelector)
	if err != nil {
		log.Fatalf("parsing node selector: %s", err)
	}
	// a bad state file is reported here, before the TUI takes the screen
	states := state.Load(state.DefaultPath(homeDir), os.Stderr)
	// a failed save is told in the footer once the TUI runs
	var running atomic.Pointer[tea.Program]
	saver := state.NewSaver(states, stateSaveDelay, func(err error) {
		if p := running.Load(); p != nil {
			go p.Send(model.NoticeMsg{Text: "view state not saved: " + err.Error()})
		}
	})
	defer saver.Close()

	m := model.NewUIModel(style)
	m.DisablePricing = flags.DisablePricing
	// The program exists before the watches start: Send blocks until it runs.
	program := tea.NewProgram(m, tea.WithAltScreen())
	running.Store(program)
	conn := &sessions{
		flags:        flags,
		nodeSelector: nodeSelector,
		states:       states,
		saver:        saver,
		program:      program,
		explicit:     flags.Explicit,
	}
	defer conn.Close()
	first, err := conn.Switch(flags.Context)
	if err != nil {
		log.Fatalf("connecting: %s", err)
	}
	m.Connect(first)
	m.SetSwitcher(conn)
	m.SetViews(namedViews{states, saver})
	m.SetExportFallback(filepath.Dir(state.DefaultPath(homeDir)))

	// from here the TUI owns the terminal: client-go and pricing messages go
	// to a file, and the header shows the latest one
	logFile := filepath.Join(filepath.Dir(state.DefaultPath(homeDir)), "kube-node-viewer.log")
	logs, restoreLogs := captureLogs(logFile)
	m.SetIssues(logs.Last, logFile)
	_, err = program.Run()
	restoreLogs()
	if err != nil {
		conn.Close()
		saver.Close()
		log.Fatalf("error running tea: %s", err)
	}
}

// stateSaveDelay batches view changes made in quick succession into one write.
const stateSaveDelay = 500 * time.Millisecond

// changeInterval is the most often cluster changes redraw the screen.
const changeInterval = 100 * time.Millisecond

// labelFilter is a --group-by value: 'label' or 'label=a,b,c'.
type labelFilter struct {
	label  string
	values []string
}

func parseLabelFilter(s string) (labelFilter, error) {
	label, rawValues, _ := strings.Cut(s, "=")
	filter := labelFilter{label: strings.TrimSpace(label)}
	if filter.label == "" {
		return labelFilter{}, fmt.Errorf("parsing %q: no label name given", s)
	}
	for _, v := range strings.Split(rawValues, ",") {
		if v = strings.TrimSpace(v); v != "" {
			filter.values = append(filter.values, v)
		}
	}
	return filter, nil
}

// namedViews reads named views from the state file and saves them through the
// background writer (views-and-export R2).
type namedViews struct {
	store *state.Store
	saver *state.Saver
}

func (v namedViews) ViewNames() []string                      { return v.store.ViewNames() }
func (v namedViews) View(name string) (model.ViewState, bool) { return v.store.View(name) }
func (v namedViews) SaveView(name string, s model.ViewState)  { v.saver.SaveView(name, s) }
func (v namedViews) DeleteView(name string)                   { v.saver.DeleteView(name) }
