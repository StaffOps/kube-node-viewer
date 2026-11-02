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
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newTestUI(t *testing.T) *UIModel {
	t.Helper()
	style, err := ParseStyle("#04B575,#FFFF00,#FF0000")
	if err != nil {
		t.Fatal(err)
	}
	u := NewUIModel(style)
	u.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	return u
}

func addTestNode(u *UIModel, name string) {
	n := NewNode(&v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name, CreationTimestamp: metav1.Now()},
		Spec:       v1.NodeSpec{ProviderID: "aws:///us-east-1a/i-" + name},
		Status: v1.NodeStatus{Allocatable: v1.ResourceList{
			v1.ResourceCPU: resource.MustParse("2"), v1.ResourcePods: resource.MustParse("10"),
		}},
	})
	u.cluster.AddNode(n).Show()
}

// run applies a message and any command it returns, as the Bubble Tea loop does
// for the commands this model issues (a snapshot or nothing).
func run(u *UIModel, msg tea.Msg) {
	_, cmd := u.Update(msg)
	if cmd != nil {
		if next := cmd(); next != nil {
			if _, isTick := next.(tickMsg); !isTick {
				u.Update(next)
			}
		}
	}
}

func TestChangedMsgRefreshesSnapshot(t *testing.T) {
	u := newTestUI(t)
	addTestNode(u, "node-a")
	if strings.Contains(u.View(), "node-a") {
		t.Fatal("node shown before any change message")
	}
	run(u, ChangedMsg{Gen: u.cluster.Generation()})
	if !strings.Contains(u.View(), "node-a") {
		t.Error("node missing after a change message")
	}
}

func TestChangedMsgFromOldGenerationIsDropped(t *testing.T) {
	u := newTestUI(t)
	oldGen := u.gen
	// what a context switch does: new data under a new generation
	u.gen = u.cluster.Reset()
	addTestNode(u, "node-b")
	run(u, ChangedMsg{Gen: oldGen})
	if strings.Contains(u.View(), "node-b") {
		t.Error("a change from an older generation refreshed the view")
	}
}

// R11: keys and frames never wait for the cluster lock, which informer
// handlers hold while they apply events.
func TestKeysDoNotWaitForClusterLock(t *testing.T) {
	u := newTestUI(t)
	addTestNode(u, "node-a")
	u.Refresh()

	u.cluster.mu.Lock()
	defer u.cluster.mu.Unlock()
	done := make(chan struct{})
	go func() {
		defer close(done)
		u.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
		u.Update(tea.KeyMsg{Type: tea.KeyEnter})
		u.Update(ChangedMsg{Gen: u.gen})
		_ = u.View()
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("a key waited for the cluster lock")
	}
	if u.panel != panelFilters || u.filters.step != stepField {
		t.Error("the Filters panel did not open")
	}
}

func TestNodeListToggleReportsViewChange(t *testing.T) {
	u := newTestUI(t)
	v := DefaultViewState()
	v.SetGroupBy(FieldZone)
	u.SetView(v)
	var got []ViewState
	u.OnViewChange(func(s ViewState) { got = append(got, s) })

	u.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if len(got) != 1 || got[0].ShowNodes {
		t.Fatalf("view changes: %+v", got)
	}
}

func TestFilteredOutNodesLeaveTheList(t *testing.T) {
	u := newTestUI(t)
	addTestNode(u, "keep")
	addTestNode(u, "drop")
	v := DefaultViewState()
	v.Search = "keep"
	u.SetView(v)
	u.Refresh()
	out := u.View()
	if !strings.Contains(out, "keep") || strings.Contains(out, "drop") {
		t.Errorf("view:\n%s", out)
	}
}
