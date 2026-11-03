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
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Tests for label-visibility R1-R6 and its counter-examples, written against
// specs/label-visibility/requirements.md.

// labelPanels are the four panels that list node labels, by their key.
var labelPanels = []struct {
	name, key string
}{
	{"Filters", "f"}, {"Group by", "g"}, {"Sort", "s"}, {"Columns", "o"},
}

// labeledNode adds a node with the given labels.
func labeledNode(u *UIModel, name string, labels map[string]string) {
	n := NewNode(&v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name, CreationTimestamp: metav1.Now(), Labels: labels},
		Spec:       v1.NodeSpec{ProviderID: "aws:///us-east-1a/i-" + name},
		Status: v1.NodeStatus{Allocatable: v1.ResourceList{
			v1.ResourceCPU: resource.MustParse("2"), v1.ResourcePods: resource.MustParse("10"),
		}},
	})
	u.cluster.AddNode(n).Show()
}

// policyUI has three nodes labeled team, env and kubernetes.io/os, with the
// lists given.
func policyUI(t *testing.T, allow, deny []string) *UIModel {
	t.Helper()
	u := newTestUI(t)
	p, err := NewLabelPolicy(allow, deny)
	if err != nil {
		t.Fatal(err)
	}
	u.SetLabelPolicy(p)
	for i, team := range []string{"red", "blue", "green"} {
		labeledNode(u, fmt.Sprintf("node-%d", i), map[string]string{
			"team": team, "env": []string{"prod", "dev", "stage"}[i], "kubernetes.io/os": "linux",
		})
	}
	u.Refresh()
	return u
}

// openPanel is the open panel's content, as drawn in the box.
func openPanel(t *testing.T, u *UIModel) panelView {
	t.Helper()
	p, ok := u.panelView()
	if !ok {
		t.Fatalf("no panel open (panel %v)", u.panel)
	}
	return p
}

// lists reports whether a row of the panel names the field: a word of the
// row, colours removed, is the field's name.
func lists(p panelView, name string) bool {
	for _, r := range p.rows {
		if slices.Contains(strings.Fields(ansi.Strip(r)), name) {
			return true
		}
	}
	return false
}

// rowOf is the index of the row naming the field, -1 when none does.
func rowOf(p panelView, name string) int {
	for i, r := range p.rows {
		if slices.Contains(strings.Fields(ansi.Strip(r)), name) {
			return i
		}
	}
	return -1
}

// moveTo presses down until the cursor is on the row naming the field.
func moveTo(t *testing.T, u *UIModel, name string) {
	t.Helper()
	at := rowOf(openPanel(t, u), name)
	if at < 0 {
		t.Fatalf("no row %q", name)
	}
	for openPanel(t, u).cursor < at {
		before := openPanel(t, u).cursor
		press(u, "down")
		if openPanel(t, u).cursor == before {
			t.Fatalf("cursor stuck at %d before %q", before, name)
		}
	}
}

func TestPanelsListOnlyAllowedLabels(t *testing.T) {
	for _, pn := range labelPanels {
		t.Run(pn.name, func(t *testing.T) {
			u := policyUI(t, []string{"team"}, nil)
			press(u, pn.key)
			p := openPanel(t, u)
			if !lists(p, "team") {
				t.Errorf("team not listed (R1):\n%s", strings.Join(p.rows, "\n"))
			}
			for _, hidden := range []string{"env", "kubernetes.io/os"} {
				if lists(p, hidden) {
					t.Errorf("%s listed (R1)", hidden)
				}
			}
		})
	}
}

// TestPanelsKeepDerivedAndComputedFields checks that every non-label row a
// panel lists with no lists is still listed with lists that would match it,
// and that no panel gains rows it did not have (counter-examples).
func TestPanelsKeepDerivedAndComputedFields(t *testing.T) {
	nonLabel := func(p panelView, all []string) []string {
		var out []string
		for _, r := range p.rows {
			name := strings.Fields(ansi.Strip(r))
			if len(name) == 0 {
				continue
			}
			if !slices.ContainsFunc(all, func(l string) bool { return slices.Contains(name, l) }) {
				out = append(out, ansi.Strip(r))
			}
		}
		return out
	}
	labels := []string{"team", "env", "kubernetes.io/os"}
	for _, pn := range labelPanels {
		t.Run(pn.name, func(t *testing.T) {
			base := policyUI(t, nil, nil)
			press(base, pn.key)
			want := nonLabel(openPanel(t, base), labels)
			if len(want) == 0 {
				t.Fatal("no derived rows to compare")
			}
			for _, lists := range []struct {
				name        string
				allow, deny []string
			}{
				{"allow none", []string{"nothing"}, nil},
				{"deny all", nil, []string{"*"}},
				{"deny derived names", nil, []string{"capacity", "nodegroup", "zone", "region", "instance-type", "status", "kube-node-viewer/*"}},
			} {
				u := policyUI(t, lists.allow, lists.deny)
				press(u, pn.key)
				got := nonLabel(openPanel(t, u), labels)
				if !slices.Equal(got, want) {
					t.Errorf("%s: non-label rows changed\n got %q\nwant %q", lists.name, got, want)
				}
				// show-all does not add computed labels to panels without them
				press(u, "a")
				got = nonLabel(openPanel(t, u), labels)
				if !slices.Equal(got, want) {
					t.Errorf("%s, show-all: non-label rows changed\n got %q\nwant %q", lists.name, got, want)
				}
			}
		})
	}
}

func TestOnlyColumnsOffersComputedLabels(t *testing.T) {
	u := policyUI(t, nil, []string{"kube-node-viewer/*"})
	for _, pn := range labelPanels {
		press(u, pn.key)
		p := openPanel(t, u)
		got := lists(p, LabelPrefix+"node-age")
		if want := pn.key == "o"; got != want {
			t.Errorf("%s lists %snode-age: %v, want %v", pn.name, LabelPrefix, got, want)
		}
		press(u, "esc")
	}
}

func TestPrefixedRealLabelFollowsTheLists(t *testing.T) {
	for _, pn := range labelPanels {
		t.Run(pn.name, func(t *testing.T) {
			u := policyUI(t, []string{"team"}, nil)
			labeledNode(u, "priced", map[string]string{"team": "x", LabelPrefix + "instance-price": "0.5"})
			u.Refresh()
			press(u, pn.key)
			if lists(openPanel(t, u), LabelPrefix+"instance-price") {
				t.Error("a real kube-node-viewer/ label ignored the allow list (R1)")
			}
			press(u, "a")
			if !lists(openPanel(t, u), LabelPrefix+"instance-price") {
				t.Error("show-all did not list the prefixed real label")
			}
		})
	}
}

func TestPanelsLeaveOutDeniedLabels(t *testing.T) {
	tests := []struct {
		name         string
		allow, deny  []string
		shown, gone  []string
		extraLabels  map[string]string
		wantHiddenOf int
	}{
		{
			name:  "deny wins over allow",
			allow: []string{"app.example.com/*"}, deny: []string{"app.example.com/internal-*"},
			extraLabels: map[string]string{"app.example.com/owner": "me", "app.example.com/internal-id": "1"},
			shown:       []string{"app.example.com/owner"},
			gone:        []string{"app.example.com/internal-id", "team", "env"},
		},
		{
			name: "deny only", deny: []string{"env"},
			shown: []string{"team", "kubernetes.io/os"},
			gone:  []string{"env"},
		},
	}
	for _, tt := range tests {
		for _, pn := range labelPanels {
			t.Run(tt.name+"/"+pn.name, func(t *testing.T) {
				u := policyUI(t, tt.allow, tt.deny)
				if tt.extraLabels != nil {
					labeledNode(u, "extra", tt.extraLabels)
					u.Refresh()
				}
				press(u, pn.key)
				p := openPanel(t, u)
				for _, k := range tt.shown {
					if !lists(p, k) {
						t.Errorf("%s not listed (R2)", k)
					}
				}
				for _, k := range tt.gone {
					if lists(p, k) {
						t.Errorf("%s listed (R2)", k)
					}
				}
			})
		}
	}
}

func TestNoListsListEveryLabel(t *testing.T) {
	for _, lists2 := range []struct {
		name        string
		allow, deny []string
		zero        bool
	}{
		{name: "no policy set", zero: true},
		{name: "empty lists", allow: []string{}, deny: []string{}},
	} {
		for _, pn := range labelPanels {
			t.Run(lists2.name+"/"+pn.name, func(t *testing.T) {
				var u *UIModel
				if lists2.zero {
					u = newTestUI(t)
					labeledNode(u, "n", map[string]string{"team": "a", "env": "b", "kubernetes.io/os": "linux"})
					u.Refresh()
				} else {
					u = policyUI(t, lists2.allow, lists2.deny)
				}
				press(u, pn.key)
				p := openPanel(t, u)
				for _, k := range []string{"team", "env", "kubernetes.io/os"} {
					if !lists(p, k) {
						t.Errorf("%s not listed (R3)", k)
					}
				}
				if p.status != "" {
					t.Errorf("status %q with nothing hidden (R6)", p.status)
				}
				before := slices.Clone(p.rows)
				press(u, "a")
				p = openPanel(t, u)
				if p.status != "" || !slices.Equal(p.rows, before) {
					t.Errorf("a did something with nothing hidden (R6): status %q", p.status)
				}
				if u.panel == panelNone {
					t.Error("a closed the panel")
				}
			})
		}
	}
}

func TestHiddenLabelInUseStaysListedAndApplied(t *testing.T) {
	deny := []string{"env"}
	env := LabelField("env")
	tests := []struct {
		name  string
		panel string
		set   func(*ViewState)
		check func(t *testing.T, u *UIModel, p panelView)
	}{
		{
			name: "group-by from saved state", panel: "g",
			set: func(v *ViewState) { v.SetGroupBy(env) },
			check: func(t *testing.T, u *UIModel, p panelView) {
				if at := rowOf(p, "env"); at < 0 || !strings.HasPrefix(ansi.Strip(p.rows[at]), "[1] env") {
					t.Errorf("env not listed as level [1]:\n%s", strings.Join(p.rows, "\n"))
				}
				if !slices.Equal(u.view.GroupBy, []Field{env}) {
					t.Errorf("group-by %v", u.view.GroupBy)
				}
			},
		},
		{
			name: "column from --extra-labels", panel: "o",
			set: func(v *ViewState) { v.Columns = append(v.Columns, env) },
			check: func(t *testing.T, u *UIModel, p panelView) {
				if !lists(p, "env") {
					t.Error("Columns does not list the env column")
				}
			},
		},
		{
			name: "filter", panel: "f",
			set: func(v *ViewState) { v.Filters = []Filter{{Field: env, Values: []string{"prod"}, Enabled: true}} },
			check: func(t *testing.T, u *UIModel, p panelView) {
				if !lists(p, "env") {
					t.Error("Filters does not list the env filter")
				}
			},
		},
		{
			name: "disabled filter", panel: "f",
			set: func(v *ViewState) { v.Filters = []Filter{{Field: env, Values: []string{"prod"}}} },
			check: func(t *testing.T, u *UIModel, p panelView) {
				if !lists(p, "env") {
					t.Error("Filters does not list the disabled env filter")
				}
			},
		},
		{
			name: "sort", panel: "s",
			set: func(v *ViewState) { v.Sort = SortSpec{Field: env} },
			check: func(t *testing.T, u *UIModel, p panelView) {
				if !lists(p, "env") {
					t.Error("Sort does not list the env sort")
				}
				if ansi.Strip(p.rows[p.cursor]) == "" || rowOf(p, "env") != p.cursor {
					t.Errorf("cursor not on the current sort env: row %d", p.cursor)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := policyUI(t, nil, deny)
			v := u.view
			tt.set(&v)
			u.SetView(v)
			u.Refresh()
			press(u, tt.panel)
			p := openPanel(t, u)
			tt.check(t, u, p)
			// the view uses env, so it is not counted as hidden
			if p.status != "" {
				t.Errorf("status %q: the in-use label was counted as hidden", p.status)
			}
		})
	}
}

func TestHiddenInUseLabelIsStillApplied(t *testing.T) {
	env := LabelField("env")
	t.Run("filter", func(t *testing.T) {
		u := policyUI(t, nil, []string{"env"})
		v := u.view
		v.Filters = []Filter{{Field: env, Values: []string{"prod"}, Enabled: true}}
		u.SetView(v)
		u.Refresh()
		out := u.View()
		if !strings.Contains(out, "node-0") || strings.Contains(out, "node-1") {
			t.Errorf("env filter not applied:\n%s", out)
		}
	})
	t.Run("column", func(t *testing.T) {
		u := policyUI(t, nil, []string{"env"})
		v := u.view
		v.Columns = append(v.Columns, env)
		u.SetView(v)
		u.Refresh()
		if out := u.View(); !strings.Contains(out, "stage") || !strings.Contains(out, "prod") {
			t.Errorf("env column not shown:\n%s", out)
		}
	})
	t.Run("sort", func(t *testing.T) {
		u := policyUI(t, nil, []string{"env"})
		v := u.view
		v.Sort = SortSpec{Field: env}
		u.SetView(v)
		u.Refresh()
		out := u.View()
		// dev (node-1) < prod (node-0) < stage (node-2)
		if strings.Index(out, "node-1") >= strings.Index(out, "node-0") || strings.Index(out, "node-0") >= strings.Index(out, "node-2") {
			t.Errorf("not sorted by env:\n%s", out)
		}
	})
	t.Run("group-by", func(t *testing.T) {
		u := policyUI(t, nil, []string{"env"})
		v := u.view
		v.SetGroupBy(env)
		u.SetView(v)
		u.Refresh()
		if out := u.View(); !strings.Contains(out, "stage") {
			t.Errorf("not grouped by env:\n%s", out)
		}
	})
}

func TestUntickedHiddenLabelStaysUntilThePanelCloses(t *testing.T) {
	env := LabelField("env")
	t.Run("group-by", func(t *testing.T) {
		u := policyUI(t, nil, []string{"env"})
		v := u.view
		v.SetGroupBy(env)
		u.SetView(v)
		u.Refresh()
		press(u, "g")
		moveTo(t, u, "env")
		press(u, " ")
		if len(u.view.GroupBy) != 0 {
			t.Fatalf("space did not untick env: %v", u.view.GroupBy)
		}
		if !lists(openPanel(t, u), "env") {
			t.Fatal("the unticked env row vanished before the panel closed (R5)")
		}
		// undo: tick it again
		moveTo(t, u, "env")
		press(u, " ")
		if !slices.Equal(u.view.GroupBy, []Field{env}) {
			t.Errorf("could not tick env again: %v", u.view.GroupBy)
		}
		press(u, " ", "esc", "g")
		if lists(openPanel(t, u), "env") {
			t.Error("env still listed after it left the view and the panel reopened (R5)")
		}
		if s := openPanel(t, u).status; s != "1 labels hidden by config • a: show all" {
			t.Errorf("status %q", s)
		}
	})
	t.Run("columns", func(t *testing.T) {
		u := policyUI(t, nil, []string{"env"})
		v := u.view
		v.Columns = append(v.Columns, env)
		u.SetView(v)
		u.Refresh()
		press(u, "o")
		moveTo(t, u, "env")
		press(u, " ")
		if slices.Contains(u.view.Columns, env) {
			t.Fatal("space did not hide the env column")
		}
		if !lists(openPanel(t, u), "env") {
			t.Fatal("the hidden env column vanished from Columns before it closed (R5)")
		}
		press(u, "esc", "o")
		if lists(openPanel(t, u), "env") {
			t.Error("env still listed after it left the view and the panel reopened (R5)")
		}
	})
	t.Run("filters", func(t *testing.T) {
		u := policyUI(t, nil, []string{"env"})
		v := u.view
		v.Filters = []Filter{{Field: env, Values: []string{"prod"}, Enabled: true}}
		u.SetView(v)
		u.Refresh()
		press(u, "f")
		moveTo(t, u, "env")
		press(u, "d")
		if len(u.view.Filters) != 0 {
			t.Fatalf("d did not clear the filter: %v", u.view.Filters)
		}
		if !lists(openPanel(t, u), "env") {
			t.Fatal("the cleared env filter vanished before the panel closed (R5)")
		}
		press(u, "esc", "f")
		if lists(openPanel(t, u), "env") {
			t.Error("env still listed after it left the view and the panel reopened (R5)")
		}
	})
}

func TestStatusLineCountsHiddenLabels(t *testing.T) {
	tests := []struct {
		name        string
		allow, deny []string
		view        func(*ViewState)
		want        string
	}{
		{name: "allow one of three", allow: []string{"team"}, want: "2 labels hidden by config • a: show all"},
		{name: "deny one", deny: []string{"env"}, want: "1 labels hidden by config • a: show all"},
		{name: "deny all", deny: []string{"*"}, want: "3 labels hidden by config • a: show all"},
		{name: "in-use not counted", allow: []string{"team"}, view: func(v *ViewState) { v.SetGroupBy(LabelField("env")) },
			want: "1 labels hidden by config • a: show all"},
		{name: "nothing hidden", allow: []string{"*"}, want: ""},
		{name: "list matches no key on the nodes", deny: []string{"absent"}, want: ""},
	}
	for _, tt := range tests {
		for _, pn := range labelPanels {
			t.Run(tt.name+"/"+pn.name, func(t *testing.T) {
				u := policyUI(t, tt.allow, tt.deny)
				// a second node with the same keys: counted once
				labeledNode(u, "dup", map[string]string{"team": "red", "env": "prod", "kubernetes.io/os": "linux"})
				if tt.view != nil {
					v := u.view
					tt.view(&v)
					u.SetView(v)
				}
				u.Refresh()
				press(u, pn.key)
				if got := openPanel(t, u).status; got != tt.want {
					t.Errorf("status %q, want %q", got, tt.want)
				}
				if tt.want != "" && !strings.Contains(u.View(), tt.want) {
					t.Errorf("status line not drawn:\n%s", u.View())
				}
			})
		}
	}
}

func TestShowAllToggles(t *testing.T) {
	for _, pn := range labelPanels {
		t.Run(pn.name, func(t *testing.T) {
			u := policyUI(t, []string{"team"}, nil)
			press(u, pn.key)
			press(u, "a")
			p := openPanel(t, u)
			if p.status != "showing all labels • a: hide" {
				t.Errorf("after a: status %q", p.status)
			}
			for _, k := range []string{"team", "env", "kubernetes.io/os"} {
				if !lists(p, k) {
					t.Errorf("after a: %s not listed (R6)", k)
				}
			}
			if !strings.Contains(u.View(), "showing all labels • a: hide") {
				t.Error("after a: status line not drawn")
			}
			press(u, "a")
			p = openPanel(t, u)
			if p.status != "2 labels hidden by config • a: show all" {
				t.Errorf("after a a: status %q", p.status)
			}
			if lists(p, "env") {
				t.Error("after a a: env still listed")
			}
			// show-all lasts until the panel closes
			press(u, "a", "esc", pn.key)
			p = openPanel(t, u)
			if lists(p, "env") || p.status != "2 labels hidden by config • a: show all" {
				t.Errorf("reopened panel kept show-all: status %q", p.status)
			}
			if u.view.Search != "" {
				t.Error("a was typed into the search")
			}
		})
	}
}

func TestShowAllDoesNotChangeTheView(t *testing.T) {
	u := policyUI(t, []string{"team"}, nil)
	var saved int
	u.OnViewChange(func(ViewState) { saved++ })
	before := u.view
	for _, pn := range labelPanels {
		press(u, pn.key, "a", "a", "a", "esc")
	}
	if saved != 0 {
		t.Errorf("a saved the view %d times", saved)
	}
	if fmt.Sprint(before) != fmt.Sprint(u.view) {
		t.Errorf("a changed the view: %v → %v", before, u.view)
	}
}

func TestShowAllKeepsTheCursorOnItsField(t *testing.T) {
	for _, pn := range labelPanels {
		t.Run(pn.name, func(t *testing.T) {
			// allow the last label in natural order so that hidden ones come
			// before it: the row index changes, the field must not
			u := policyUI(t, []string{"team"}, nil)
			press(u, pn.key)
			moveTo(t, u, "team")
			press(u, "a")
			p := openPanel(t, u)
			if rowOf(p, "team") != p.cursor {
				t.Errorf("after a the cursor is on %q, want team", ansi.Strip(p.rows[p.cursor]))
			}
			press(u, "a")
			p = openPanel(t, u)
			if rowOf(p, "team") != p.cursor {
				t.Errorf("after a a the cursor is on %q, want team", ansi.Strip(p.rows[p.cursor]))
			}
		})
	}
}

// TestShowAllOffWithTheCursorOnAHiddenRow checks that the cursor stays on a
// row when the field it was on is hidden again.
func TestShowAllOffWithTheCursorOnAHiddenRow(t *testing.T) {
	for _, pn := range labelPanels {
		t.Run(pn.name, func(t *testing.T) {
			u := policyUI(t, []string{"team"}, nil)
			press(u, pn.key, "a")
			moveTo(t, u, "kubernetes.io/os")
			press(u, "a")
			p := openPanel(t, u)
			if lists(p, "kubernetes.io/os") {
				t.Fatal("kubernetes.io/os still listed after a off")
			}
			if p.cursor < 0 || p.cursor >= len(p.rows) {
				t.Errorf("cursor %d out of %d rows", p.cursor, len(p.rows))
			}
			// the panel still works from there
			press(u, "down", "a")
			if openPanel(t, u).status != "showing all labels • a: hide" {
				t.Error("a did not turn show-all on again")
			}
		})
	}
}

func TestShowAllOffClosesAHiddenOpenFilterField(t *testing.T) {
	u := policyUI(t, []string{"team"}, nil)
	press(u, "f", "a")
	moveTo(t, u, "env")
	press(u, "enter")
	if u.filters.open != LabelField("env") {
		t.Fatalf("enter did not open env: %q", u.filters.open)
	}
	if !lists(openPanel(t, u), "prod") {
		t.Fatal("env values not listed")
	}
	press(u, "a")
	p := openPanel(t, u)
	if u.filters.open != "" || lists(p, "prod") {
		t.Errorf("a off left the hidden env field open: %q", u.filters.open)
	}
	if lists(p, "env") {
		t.Error("env still listed after a off")
	}
	if p.cursor < 0 || p.cursor >= len(p.rows) {
		t.Errorf("cursor %d out of %d rows", p.cursor, len(p.rows))
	}

	// a field that stays visible stays open
	press(u, "a")
	moveTo(t, u, "team")
	press(u, "enter")
	press(u, "a")
	if u.filters.open != LabelField("team") {
		t.Errorf("a off closed the visible team field: %q", u.filters.open)
	}
}

func TestStatusFollowsTheCurrentNodes(t *testing.T) {
	for _, pn := range labelPanels {
		t.Run(pn.name, func(t *testing.T) {
			u := newTestUI(t)
			p, _ := NewLabelPolicy([]string{"team"}, nil)
			u.SetLabelPolicy(p)
			u.Refresh()
			press(u, pn.key)
			if s := openPanel(t, u).status; s != "" {
				t.Errorf("no nodes: status %q", s)
			}
			press(u, "a")
			if s := openPanel(t, u).status; s != "" {
				t.Errorf("no nodes, a: status %q", s)
			}
			labeledNode(u, "n1", map[string]string{"team": "a", "env": "x"})
			u.Refresh()
			if s := openPanel(t, u).status; s != "1 labels hidden by config • a: show all" {
				t.Errorf("one node: status %q", s)
			}
			labeledNode(u, "n2", map[string]string{"env": "y", "kubernetes.io/os": "linux"})
			u.Refresh()
			if s := openPanel(t, u).status; s != "2 labels hidden by config • a: show all" {
				t.Errorf("two nodes: status %q", s)
			}
		})
	}
}

func TestShortWindowShowsARowAndTheStatus(t *testing.T) {
	for _, pn := range labelPanels {
		t.Run(pn.name, func(t *testing.T) {
			u := policyUI(t, []string{"team"}, nil)
			u.Update(tea.WindowSizeMsg{Width: 120, Height: 10})
			press(u, pn.key)
			out := ansi.Strip(u.View())
			if !strings.Contains(out, "2 labels hidden by config") {
				t.Errorf("status line not shown on a 10-line window:\n%s", out)
			}
			var row bool
			for _, line := range strings.Split(out, "\n") {
				if strings.Contains(line, "│") {
					for _, name := range []string{"capacity", "creation", "name", "instance-type", "team", "price"} {
						if slices.Contains(strings.Fields(strings.Trim(line, "│ ")), name) {
							row = true
						}
					}
				}
			}
			if !row {
				t.Errorf("no panel row on a 10-line window:\n%s", out)
			}
			if n := len(strings.Split(u.View(), "\n")); n > 10 {
				t.Errorf("view is %d lines on a 10-line window", n)
			}
		})
	}
}

func TestNodeDetailsShowEveryLabel(t *testing.T) {
	u := policyUI(t, []string{"team"}, []string{"env"})
	press(u, "enter")
	if u.panel != panelNode {
		t.Fatalf("enter did not open the node details: panel %v", u.panel)
	}
	u.Update(tea.WindowSizeMsg{Width: 160, Height: 90})
	out := ansi.Strip(u.View())
	for _, k := range []string{"env", "kubernetes.io/os"} {
		if !strings.Contains(out, k) {
			t.Errorf("node details hide the label %s:\n%s", k, out)
		}
	}
}

func TestSearchStillMatchesHiddenLabelValues(t *testing.T) {
	u := policyUI(t, []string{"team"}, []string{"env"})
	press(u, "/", "s", "t", "a", "g", "e", "enter")
	out := u.View()
	if !strings.Contains(out, "node-2") || strings.Contains(out, "node-0") || strings.Contains(out, "node-1") {
		t.Errorf("search on a hidden label's value did not keep only node-2:\n%s", out)
	}
}

// --- T8 follow-ups: short and narrow popups, lists that follow the nodes ---

// Line kinds of a drawn popup box.
const (
	kTop    = "top"
	kRow    = "row"
	kSep    = "separator"
	kStatus = "status"
	kHint   = "hint"
	kBottom = "bottom"
)

// dropOrder is what a box of the given height shows when it has more rows
// than fit, a status line and a hint: the rows shrink to one, then the
// separator goes, then the status line, then the hint; below three lines the
// top border goes, so the bottom border is the last line left (T8, R6).
func dropOrder(height int, status bool) []string {
	full := []string{kTop, kRow, kSep, kStatus, kHint, kBottom}
	if !status {
		full = []string{kTop, kRow, kSep, kHint, kBottom}
	}
	switch {
	case height >= len(full):
		rows := make([]string, height-len(full)+1)
		for i := range rows {
			rows[i] = kRow
		}
		return slices.Concat([]string{kTop}, rows, full[2:])
	case height == 1:
		return []string{kBottom}
	case height == 2:
		return []string{kRow, kBottom}
	case height == 3:
		return []string{kTop, kRow, kBottom}
	}
	// 4 lines and up to one less than full: drop the separator, then the
	// status line
	out := []string{kTop, kRow}
	if status && height >= 5 {
		out = append(out, kStatus)
	}
	return append(out, kHint, kBottom)
}

// boxText is a line of the box, borders and padding removed.
func boxText(line string) string {
	return strings.TrimSpace(strings.Trim(strings.TrimSpace(line), "│"))
}

// kindOf classifies a stripped box line. hintWord is the first word of the
// panel's hint; status lines start with a count or "all labels" / "showing".
func kindOf(line, hintWord string, isStatus func(string) bool) string {
	switch {
	case strings.Contains(line, "╭"):
		return kTop
	case strings.Contains(line, "╰"):
		return kBottom
	case strings.Contains(line, "├"):
		return kSep
	}
	text := boxText(line)
	switch {
	case isStatus(text):
		return kStatus
	case strings.HasPrefix(text, hintWord):
		return kHint
	}
	return kRow
}

func TestPopupDropOrder(t *testing.T) {
	rows := make([]string, 10)
	for i := range rows {
		rows[i] = fmt.Sprintf("row%d", i)
	}
	const status, short = "2 labels hidden by config • a: show all", "2 hidden • a: all"
	isStatus := func(s string) bool { return strings.HasPrefix(s, "2 ") }
	for _, withStatus := range []bool{true, false} {
		for _, width := range []int{70, 20} {
			for height := 1; height <= 9; height++ {
				t.Run(fmt.Sprintf("status=%v/w%d/h%d", withStatus, width, height), func(t *testing.T) {
					u := newTestUI(t)
					u.width = width
					p := panelView{title: "Sort by", rows: rows, cursor: 0, hint: "esc: close"}
					if withStatus {
						p.status, p.statusShort = status, short
					}
					lines := u.popup(p, height)
					if len(lines) > height {
						t.Fatalf("%d lines for a height of %d", len(lines), height)
					}
					var got []string
					for _, l := range lines {
						got = append(got, kindOf(ansi.Strip(l), "esc:", isStatus))
					}
					if want := dropOrder(height, withStatus); !slices.Equal(got, want) {
						t.Errorf("lines %v, want %v\n%s", got, want, ansi.Strip(strings.Join(lines, "\n")))
					}
				})
			}
		}
	}
}

func TestPopupStatusTextByWidth(t *testing.T) {
	const status, short = "2 labels hidden by config • a: show all", "2 hidden • a: all"
	tests := []struct {
		width int
		want  string // the status line's text starts with it
		not   string // and does not contain it
	}{
		{width: 70, want: status},
		{width: 40, want: short, not: "labels hidden"},
		{width: 20, want: "2 hidden", not: "labels hidden"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprint(tt.width), func(t *testing.T) {
			u := newTestUI(t)
			u.width = tt.width
			p := panelView{title: "Columns", rows: []string{"a", "b"}, cursor: 0, hint: "esc: close", status: status, statusShort: short}
			var line string
			for _, l := range u.popup(p, 10) {
				if s := boxText(ansi.Strip(l)); strings.HasPrefix(s, "2 ") {
					line = s
				}
			}
			if !strings.HasPrefix(line, tt.want) {
				t.Errorf("status line %q, want it to start with %q", line, tt.want)
			}
			if tt.not != "" && strings.Contains(line, tt.not) {
				t.Errorf("status line %q contains %q on a %d-wide window", line, tt.not, tt.width)
			}
		})
	}
	t.Run("no short text: the full one is truncated", func(t *testing.T) {
		u := newTestUI(t)
		u.width = 20
		p := panelView{title: "T", rows: []string{"a"}, hint: "esc: close", status: status}
		var found bool
		for _, l := range u.popup(p, 10) {
			if s := boxText(ansi.Strip(l)); strings.HasPrefix(s, "2 labels") {
				found = true
				if textWidth(l) > 20 {
					t.Errorf("status line %q is wider than the window", s)
				}
			}
		}
		if !found {
			t.Error("status line missing")
		}
	})
}

// windowBox is the popup box cut out of a stripped view: its lines from the
// top border to the bottom border.
func windowBox(t *testing.T, out string) []string {
	t.Helper()
	lines := strings.Split(out, "\n")
	first, x, w := -1, 0, 0
	for i, l := range lines {
		r := []rune(l)
		if at := slices.Index(r, '╭'); at >= 0 {
			first, x = i, at
			w = slices.Index(r, '╮') - at + 1
			break
		}
	}
	var box []string
	if first < 0 {
		// the top border is dropped below three lines; cut from the left
		// edge of the bottom border instead
		for i, l := range lines {
			if at := slices.Index([]rune(l), '╰'); at >= 0 {
				first, x = i, at
				w = slices.Index([]rune(l), '╯') - at + 1
				for first > 0 && slices.Contains([]rune(lines[first-1]), '│') {
					first--
				}
				break
			}
		}
	}
	if first < 0 || w <= 0 {
		t.Fatalf("no box in the view:\n%s", out)
	}
	for _, l := range lines[first:] {
		r := []rune(l)
		if len(r) < x+w {
			t.Fatalf("box line %q shorter than the box:\n%s", l, out)
		}
		line := string(r[x : x+w])
		box = append(box, line)
		if strings.Contains(line, "╰") {
			return box
		}
	}
	t.Fatalf("box has no bottom border:\n%s", out)
	return nil
}

// TestShortWindowsDropLinesInOrder opens each panel on windows 6 to 12 lines
// tall, 70 and 20 columns wide, with labels hidden so it has a status line.
func TestShortWindowsDropLinesInOrder(t *testing.T) {
	for _, pn := range labelPanels {
		for _, width := range []int{70, 20} {
			for height := 6; height <= 12; height++ {
				for _, showAll := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/w%d/h%d/showAll=%v", pn.name, width, height, showAll), func(t *testing.T) {
						u := policyUI(t, []string{"team"}, nil)
						u.Update(tea.WindowSizeMsg{Width: width, Height: height})
						press(u, pn.key)
						if showAll {
							press(u, "a")
						}
						hintWord := strings.Fields(openPanel(t, u).hint)[0]
						isStatus := func(s string) bool {
							return strings.HasPrefix(s, "2 ") || strings.HasPrefix(s, "all labels") || strings.HasPrefix(s, "showing all")
						}
						out := ansi.Strip(u.View())
						box := windowBox(t, out)
						var got []string
						var statusText string
						for _, l := range box {
							k := kindOf(l, hintWord, isStatus)
							got = append(got, k)
							if k == kStatus {
								statusText = boxText(l)
							}
						}
						if want := dropOrder(len(box), true); !slices.Equal(got, want) {
							t.Errorf("box lines %v, want %v for a %d-line box:\n%s", got, want, len(box), out)
						}
						// the window heights at which each line appears
						for _, c := range []struct {
							kind string
							from int
						}{{kRow, 6}, {kBottom, 6}, {kHint, 9}, {kStatus, 10}, {kSep, 11}} {
							if has := slices.Contains(got, c.kind); has != (height >= c.from) {
								t.Errorf("%s shown: %v on a %d-line window, want %v:\n%s", c.kind, has, height, height >= c.from, out)
							}
						}
						if height >= 8 {
							if n := len(strings.Split(u.View(), "\n")); n > height {
								t.Errorf("view is %d lines on a %d-line window", n, height)
							}
						}
						if statusText == "" {
							return
						}
						full, short := "2 labels hidden by config", "2 hidden"
						if showAll {
							full, short = "showing all labels • a: hide", "all labels"
						}
						switch width {
						case 70:
							if !strings.HasPrefix(statusText, full) {
								t.Errorf("status %q on a 70-wide window, want the full text %q", statusText, full)
							}
						case 20:
							if !strings.HasPrefix(statusText, short) || strings.HasPrefix(statusText, full) {
								t.Errorf("status %q on a 20-wide window, want the short text %q", statusText, short)
							}
						}
					})
				}
			}
		}
	}
}

// storedListPanels are the panels that keep their list between frames.
var storedListPanels = []struct {
	name, key, anchor string
}{{"Sort", "s", "zone"}, {"Columns", "o", "ZONE"}}

func TestSortAndColumnsListLabelsOfNodesThatArriveLater(t *testing.T) {
	for _, pn := range storedListPanels {
		t.Run(pn.name, func(t *testing.T) {
			u := newTestUI(t)
			p, err := NewLabelPolicy([]string{"team", "app"}, nil)
			if err != nil {
				t.Fatal(err)
			}
			u.SetLabelPolicy(p)
			u.Refresh()
			press(u, pn.key)
			moveTo(t, u, pn.anchor)

			labeledNode(u, "n1", map[string]string{"team": "a", "env": "x"})
			u.Refresh()
			got := openPanel(t, u)
			if !lists(got, "team") {
				t.Fatalf("team not listed after its node arrived:\n%s", ansi.Strip(strings.Join(got.rows, "\n")))
			}
			if lists(got, "env") {
				t.Error("env listed: it is not allowed")
			}
			if rowOf(got, pn.anchor) != got.cursor {
				t.Errorf("cursor moved off %s to %q", pn.anchor, ansi.Strip(got.rows[got.cursor]))
			}
			if got.status != "1 labels hidden by config • a: show all" {
				t.Errorf("status %q", got.status)
			}

			// a label sorted before team arrives while the cursor is on team:
			// the row index changes, the field must not
			moveTo(t, u, "team")
			at := openPanel(t, u).cursor
			labeledNode(u, "n2", map[string]string{"app": "b"})
			u.Refresh()
			// the key acts on the field under the cursor without a frame
			// drawn in between
			switch pn.key {
			case "s":
				press(u, "enter")
				if u.view.Sort.Field != LabelField("team") {
					t.Errorf("enter sorted by %q, want team", u.view.Sort.Field)
				}
				// enter closes Sort; it reopens on the current sort
				press(u, "s")
			case "o":
				press(u, " ")
				if !slices.Contains(u.view.Columns, LabelField("team")) {
					t.Errorf("space did not show the team column: %v", u.view.Columns)
				}
			}
			got = openPanel(t, u)
			if !lists(got, "app") {
				t.Fatal("app not listed after its node arrived")
			}
			if rowOf(got, "team") != got.cursor {
				t.Errorf("cursor on %q, want team (was row %d)", ansi.Strip(got.rows[got.cursor]), at)
			}
		})
	}
}

func TestSortAndColumnsWithNoNodesTakeKeys(t *testing.T) {
	for _, pn := range storedListPanels {
		t.Run(pn.name, func(t *testing.T) {
			u := newTestUI(t)
			u.Refresh()
			press(u, pn.key, "down", "a", "a", " ")
			got := openPanel(t, u)
			if got.cursor < 0 || (len(got.rows) > 0 && got.cursor >= len(got.rows)) {
				t.Errorf("cursor %d out of %d rows", got.cursor, len(got.rows))
			}
		})
	}
}

// TestShowAllOffKeepsTheCursorNearItsPlace checks that when a hides the field
// under the cursor, the cursor stays about where it was, not on the first row.
func TestShowAllOffKeepsTheCursorNearItsPlace(t *testing.T) {
	for _, pn := range storedListPanels {
		t.Run(pn.name, func(t *testing.T) {
			u := policyUI(t, []string{"team"}, nil)
			press(u, pn.key, "a")
			moveTo(t, u, "kubernetes.io/os")
			before := openPanel(t, u)
			at := before.cursor
			press(u, "a")
			after := openPanel(t, u)
			if lists(after, "kubernetes.io/os") {
				t.Fatal("kubernetes.io/os still listed after a off")
			}
			removed := len(before.rows) - len(after.rows)
			if after.cursor < at-removed || after.cursor > at || after.cursor >= len(after.rows) {
				t.Errorf("cursor at row %d of %d, was %d with %d rows removed: not near its place",
					after.cursor, len(after.rows), at, removed)
			}
			if at > removed && after.cursor == 0 {
				t.Error("cursor jumped to the first row")
			}
		})
	}
}
