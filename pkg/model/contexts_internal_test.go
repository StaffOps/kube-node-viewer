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
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type fakeSwitcher struct {
	sessions map[string]Session
	saved    map[string][]ViewState
}

func (f *fakeSwitcher) Contexts() ([]string, string, error) {
	return []string{"a", "b", "broken"}, "a", nil
}

func (f *fakeSwitcher) Switch(name string) (Session, error) {
	s, ok := f.sessions[name]
	if !ok {
		return Session{}, errors.New("unreachable")
	}
	s.Save = func(v ViewState) { f.saved[name] = append(f.saved[name], v) }
	return s, nil
}

func storeWith(names ...string) *Cluster {
	c := NewCluster()
	for _, name := range names {
		addNodeTo(c, name)
	}
	return c
}

func addNodeTo(c *Cluster, name string) {
	c.AddNode(NewNode(&v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name, CreationTimestamp: metav1.Now()},
		Spec:       v1.NodeSpec{ProviderID: "aws:///us-east-1a/i-" + name},
		Status:     v1.NodeStatus{Allocatable: v1.ResourceList{v1.ResourceCPU: resource.MustParse("2")}},
	})).Show()
}

// apply runs a key and the command it returns, feeding the result back, as
// the Bubble Tea loop does.
func apply(u *UIModel, cmd tea.Cmd) {
	for cmd != nil {
		msg := cmd()
		if msg == nil {
			return
		}
		_, cmd = u.Update(msg)
	}
}

func switchUI(t *testing.T) (*UIModel, *fakeSwitcher, *Cluster) {
	u := newTestUI(t)
	old := storeWith("old-node")
	viewB := DefaultViewState()
	viewB.SetGroupBy(FieldZone)
	f := &fakeSwitcher{
		sessions: map[string]Session{
			"a": {Cluster: old, Info: ClusterInfo{Context: "a"}, View: DefaultViewState()},
			"b": {Cluster: storeWith("new-node"), Info: ClusterInfo{Context: "b", Region: "eu-west-1"}, View: viewB},
		},
		saved: map[string][]ViewState{},
	}
	first, _ := f.Switch("a")
	u.Connect(first)
	u.SetSwitcher(f)
	u.Refresh()
	return u, f, old
}

func TestContextSwitch(t *testing.T) {
	u, f, old := switchUI(t)
	oldGen := u.gen

	apply(u, press(u, "c"))
	out := u.View()
	if !strings.Contains(out, "a  (current)") || !strings.Contains(out, "broken") {
		t.Fatalf("context list:\n%s", out)
	}
	apply(u, press(u, "down", "enter"))

	if u.panel != panelNone || u.info.Context != "b" || u.cluster == old {
		t.Fatalf("not switched: panel %v context %q", u.panel, u.info.Context)
	}
	if len(u.view.GroupBy) != 1 {
		t.Error("cluster b's view state not loaded")
	}
	out = u.View()
	if !strings.Contains(out, "new-node") || strings.Contains(out, "old-node") {
		t.Errorf("after switch:\n%s", out)
	}

	// R10: late events from the old cluster never show
	addNodeTo(old, "late-node")
	run(u, ChangedMsg{Gen: oldGen})
	run(u, ChangedMsg{Gen: u.gen})
	if strings.Contains(u.View(), "late-node") {
		t.Error("a late event from the previous cluster appeared")
	}

	// view changes now save under cluster b
	press(u, "r")
	if len(f.saved["b"]) != 1 || len(f.saved["a"]) != 0 {
		t.Errorf("saved a=%d b=%d", len(f.saved["a"]), len(f.saved["b"]))
	}
}

func TestContextSwitchErrorStaysOnCluster(t *testing.T) {
	u, _, old := switchUI(t)
	apply(u, press(u, "c"))
	apply(u, press(u, "down", "down", "enter"))
	if u.cluster != old || u.panel != panelContexts {
		t.Fatal("a failed switch left the cluster")
	}
	if out := u.View(); !strings.Contains(out, "broken: unreachable") {
		t.Errorf("error not shown:\n%s", out)
	}
}

func TestContextPickerOnCurrentJustCloses(t *testing.T) {
	u, _, old := switchUI(t)
	apply(u, press(u, "c"))
	apply(u, press(u, "enter"))
	if u.panel != panelNone || u.cluster != old {
		t.Error("picking the current context reconnected")
	}
}
