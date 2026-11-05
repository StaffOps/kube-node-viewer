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
	"math"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Tests for specs/monthly-price-columns: PRICE/M and IDLE/M on every node
// row (R1–R6), written against the requirements and the design test plan.

// monthlyNode is one node of monthlyUI and what its price columns must show.
type monthlyNode struct {
	name   string
	labels map[string]string
	// price is the hourly price; NaN for unknown
	price      float64
	equivalent bool
	wantPriceH string
	wantPriceM string
	wantIdleM  string
}

// No node has pods, so a node with a real price is idle for its full price.
var monthlyNodes = []monthlyNode{
	{"node-real", map[string]string{"karpenter.sh/capacity-type": "on-demand"},
		0.154, false, "$0.1540", "$112.42", "$112.42"},
	// interactive-tui R8: a reserved node's price is the list price marked ~
	{"node-resv", map[string]string{"karpenter.sh/capacity-type": "reserved"},
		0.1008, true, "~$0.1008", "~$73.58", "-"},
	{"node-unkn", map[string]string{"karpenter.sh/capacity-type": "on-demand"},
		math.NaN(), false, "-", "-", "-"},
	{"node-big", map[string]string{"karpenter.sh/capacity-type": "on-demand", LabelPrefix + "instance-price": "32"},
		32, false, "$32.0000", "$23,360.00", "$23,360.00"},
	{"node-farg", map[string]string{"eks.amazonaws.com/compute-type": "fargate"},
		0.05, false, "$0.0500", "$36.50", "-"},
}

func monthlyUI(t *testing.T) *UIModel {
	t.Helper()
	u := newTestUI(t)
	u.Update(tea.WindowSizeMsg{Width: 240, Height: 60})
	for _, d := range monthlyNodes {
		n := NewNode(&v1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: d.name, CreationTimestamp: metav1.Now(), Labels: d.labels},
			Spec:       v1.NodeSpec{ProviderID: "aws:///us-east-1a/i-" + d.name},
			Status: v1.NodeStatus{Allocatable: v1.ResourceList{
				v1.ResourceCPU: resource.MustParse("2"), v1.ResourcePods: resource.MustParse("10"),
			}},
		})
		u.cluster.AddNode(n).Show()
		if d.equivalent {
			n.SetEquivalentPrice(d.price)
		} else {
			n.SetPrice(d.price)
		}
	}
	u.Refresh()
	return u
}

func nodeByName(t *testing.T, u *UIModel, name string) *Node {
	t.Helper()
	for _, n := range u.stats.Nodes {
		if n.Name() == name {
			return n
		}
	}
	t.Fatalf("no node %s", name)
	return nil
}

// titleLine is the node list's column title line.
func titleLine(t *testing.T, out string) string {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "NAME") && strings.Contains(line, "PODS") {
			return line
		}
	}
	t.Fatalf("no title line:\n%s", out)
	return ""
}

// titles are the column titles after PODS, in screen order.
func titles(t *testing.T, out string) []string {
	t.Helper()
	fields := strings.Fields(titleLine(t, out))
	at := slices.Index(fields, "PODS")
	if at < 0 {
		t.Fatalf("no PODS title: %v", fields)
	}
	return fields[at+1:]
}

// columnValues reads the column with the given title off the screen, for each
// node row in screen order, keyed by the node name it starts with. Positions
// are in runes: the bars are one cell per rune.
func columnValues(t *testing.T, out, title string, names []string) (order []string, values map[string]string) {
	t.Helper()
	head := []rune(titleLine(t, out))
	start := strings.Index(string(head), title)
	if start < 0 {
		t.Fatalf("no %s column in %q", title, string(head))
	}
	start = len([]rune(string(head)[:start]))
	end := start + len([]rune(title))
	// the column ends where the next title starts
	for end < len(head) && head[end] != ' ' {
		end++
	}
	for end < len(head) && head[end] == ' ' {
		end++
	}
	if end == len(head) {
		end = 1 << 30
	}
	values = map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimLeft(line, " ▌▶>│")
		for _, name := range names {
			if !strings.HasPrefix(trimmed, name+" ") {
				continue
			}
			r := []rune(line)
			// the DaemonSets footer line also starts with the node name;
			// only a row as wide as the column is a node row
			if len(r) <= start || strings.HasPrefix(trimmed, name+" · ") {
				continue
			}
			// a value can start a cell before the title when a wider value
			// right-aligns it; take the whole field around the title start
			lo := start
			for lo > 0 && r[lo-1] != ' ' {
				lo--
			}
			hi := min(end, len(r))
			values[name] = strings.Fields(string(r[lo:hi]) + " ")[0]
			order = append(order, name)
		}
	}
	return order, values
}

func monthlyNames() []string {
	names := make([]string, len(monthlyNodes))
	for i, d := range monthlyNodes {
		names[i] = d.name
	}
	return names
}

// monthly-price-columns R1, R2 and the counter-example on PRICE/H: the cells of each column.
func TestMonthlyCells(t *testing.T) {
	u := monthlyUI(t)
	now := time.Now()
	for _, d := range monthlyNodes {
		t.Run(d.name, func(t *testing.T) {
			n := nodeByName(t, u, d.name)
			for _, c := range []struct {
				field Field
				want  string
			}{
				{FieldPrice, d.wantPriceH},
				{FieldPriceMonthly, d.wantPriceM},
				{FieldIdleMonthly, d.wantIdleM},
			} {
				if got := u.cell(n, c.field, now); got != c.want {
					t.Errorf("%s: %q, want %q", columnTitle(c.field), got, c.want)
				}
			}
		})
	}
}

// R1, R2 on screen: each value sits in its own column of the node's row.
func TestMonthlyColumnsOnScreen(t *testing.T) {
	u := monthlyUI(t)
	out := u.View()
	names := monthlyNames()
	for _, c := range []struct {
		title string
		want  func(monthlyNode) string
	}{
		{"PRICE/H", func(d monthlyNode) string { return d.wantPriceH }},
		{"PRICE/M", func(d monthlyNode) string { return d.wantPriceM }},
		{"IDLE/M", func(d monthlyNode) string { return d.wantIdleM }},
	} {
		_, values := columnValues(t, out, c.title, names)
		for _, d := range monthlyNodes {
			if got := values[d.name]; got != c.want(d) {
				t.Errorf("%s of %s on screen: %q, want %q\n%s", c.title, d.name, got, c.want(d), out)
			}
		}
	}
}

// R2: IDLE/M is IDLE/H × 730 with two decimals and separators, on nodes
// that have pods; no idle cost shows "-".
func TestIdleMonthlyIsIdleHourlyTimes730(t *testing.T) {
	u := demoUI(t)
	printer := message.NewPrinter(language.English)
	now := time.Now()
	var priced, unpriced int
	for _, n := range u.stats.Nodes {
		got := u.cell(n, FieldIdleMonthly, now)
		idle, ok := n.IdleHourly()
		if !ok {
			unpriced++
			if got != "-" {
				t.Errorf("%s (%s) has no idle cost but IDLE/M %q", n.Name(), n.CapacityType(), got)
			}
			continue
		}
		priced++
		if want := printer.Sprintf("$%0.2f", idle*730); got != want {
			t.Errorf("%s: IDLE/M %q, want %q (idle/h %v)", n.Name(), got, want, idle)
		}
		if hourly := u.cell(n, FieldIdle, now); hourly == "-" {
			t.Errorf("%s: IDLE/H %q while IDLE/M %q", n.Name(), hourly, got)
		}
	}
	if priced == 0 || unpriced == 0 {
		t.Fatalf("demo cluster lacks a case: %d with idle, %d without", priced, unpriced)
	}
}

// The month factor is 730 hours, as everywhere else.
func TestMonthFactor(t *testing.T) {
	if avgHoursPerMonth != 730 {
		t.Errorf("avgHoursPerMonth %v, want 730", avgHoursPerMonth)
	}
}

func TestMonthlyColumnTitles(t *testing.T) {
	for f, want := range map[Field]string{
		FieldPrice: "PRICE/H", FieldPriceMonthly: "PRICE/M", FieldIdle: "IDLE/H", FieldIdleMonthly: "IDLE/M",
	} {
		if got := columnTitle(f); got != want {
			t.Errorf("columnTitle(%q) = %q, want %q", f, got, want)
		}
	}
}

// R3: the defaults and their order, in the state and on screen.
func TestDefaultColumnsOrder(t *testing.T) {
	want := []Field{FieldInstanceType, FieldPrice, FieldPriceMonthly, FieldIdleMonthly, FieldCapacity, FieldNodeGroup, FieldZone, FieldStatus}
	if got := DefaultViewState().Columns; !slices.Equal(got, want) {
		t.Errorf("default columns %v, want %v", got, want)
	}
	u := monthlyUI(t)
	got := titles(t, u.View())
	wantTitles := []string{"INSTANCE", "PRICE/H", "PRICE/M", "IDLE/M", "CAPACITY", "NODEGROUP", "ZONE", "STATUS"}
	if !slices.Equal(got, wantTitles) {
		t.Errorf("titles on screen %v, want %v", got, wantTitles)
	}
}

// oldDefaults are the columns a cluster saved before this feature.
var oldDefaults = []Field{FieldInstanceType, FieldPrice, FieldCapacity, FieldNodeGroup, FieldZone, FieldStatus}

// R3: a saved view keeps its columns, no migration.
func TestSavedColumnsAreUntouched(t *testing.T) {
	u := monthlyUI(t)
	v := DefaultViewState()
	v.Columns = slices.Clone(oldDefaults)
	u.SetView(v)
	if !slices.Equal(u.view.Columns, oldDefaults) {
		t.Fatalf("columns after SetView %v, want %v", u.view.Columns, oldDefaults)
	}
	got := titles(t, u.View())
	if want := []string{"INSTANCE", "PRICE/H", "CAPACITY", "NODEGROUP", "ZONE", "STATUS"}; !slices.Equal(got, want) {
		t.Errorf("titles of an old saved view %v, want %v", got, want)
	}
	// a saved view that lists the new columns shows them, where it put them
	v.Columns = []Field{FieldIdleMonthly, FieldStatus, FieldPriceMonthly}
	u.SetView(v)
	if got := titles(t, u.View()); !slices.Equal(got, []string{"IDLE/M", "STATUS", "PRICE/M"}) {
		t.Errorf("titles of a saved view with monthly columns: %v", got)
	}
}

// R3: the Columns panel of an old saved view lists the shown columns first,
// then offers PRICE/M, IDLE/M and IDLE/H.
func TestColumnsPanelOfAnOldSavedView(t *testing.T) {
	u := monthlyUI(t)
	v := DefaultViewState()
	v.Columns = slices.Clone(oldDefaults)
	u.SetView(v)
	press(u, "o")
	fields := u.columns.fields
	if !slices.Equal(fields[:len(oldDefaults)], oldDefaults) {
		t.Errorf("panel starts %v, want the shown columns %v", fields[:len(oldDefaults)], oldDefaults)
	}
	for _, f := range []Field{FieldPriceMonthly, FieldIdleMonthly, FieldIdle} {
		if i := slices.Index(fields, f); i < len(oldDefaults) {
			t.Errorf("%s at %d, want offered after the shown columns", f, i)
		}
	}
	rows := strings.Join(u.columnsPanel().rows, "\n")
	for _, title := range []string{"PRICE/M", "IDLE/M", "IDLE/H"} {
		if !strings.Contains(rows, title) {
			t.Errorf("Columns panel does not list %s:\n%s", title, rows)
		}
	}
}

// pressOn moves the Columns panel cursor to the field and presses key.
func pressOn(t *testing.T, u *UIModel, f Field, key string) {
	t.Helper()
	at := slices.Index(u.columns.fields, f)
	if at < 0 {
		t.Fatalf("%s not in the Columns panel: %v", f, u.columns.fields)
	}
	// the cursor wraps, so going down always reaches it
	for i := 0; i < len(u.columns.fields) && u.columns.cursor != at; i++ {
		press(u, "down")
	}
	if u.columns.cursor != at {
		t.Fatalf("cursor at %d, want %d", u.columns.cursor, at)
	}
	press(u, key)
}

// R3: space shows or hides each price column on its own.
func TestColumnsPanelTogglesEachPriceColumn(t *testing.T) {
	cases := []struct {
		name    string
		toggle  []Field
		shown   []string
		missing []string
	}{
		{"hourly price alone", []Field{FieldPriceMonthly}, []string{"PRICE/H"}, []string{"PRICE/M"}},
		{"monthly price alone", []Field{FieldPrice}, []string{"PRICE/M"}, []string{"PRICE/H"}},
		{"both prices", nil, []string{"PRICE/H", "PRICE/M"}, nil},
		{"hourly idle alone", []Field{FieldIdle, FieldIdleMonthly}, []string{"IDLE/H"}, []string{"IDLE/M"}},
		{"monthly idle alone", nil, []string{"IDLE/M"}, []string{"IDLE/H"}},
		{"both idles", []Field{FieldIdle}, []string{"IDLE/H", "IDLE/M"}, nil},
		{"no price column", []Field{FieldPrice, FieldPriceMonthly, FieldIdleMonthly}, nil, []string{"PRICE/H", "PRICE/M", "IDLE/H", "IDLE/M"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			u := monthlyUI(t)
			press(u, "o")
			for _, f := range c.toggle {
				pressOn(t, u, f, " ")
			}
			press(u, "esc")
			got := titles(t, u.View())
			for _, title := range c.shown {
				if !slices.Contains(got, title) {
					t.Errorf("%s not shown: %v", title, got)
				}
			}
			for _, title := range c.missing {
				if slices.Contains(got, title) {
					t.Errorf("%s still shown: %v", title, got)
				}
			}
			// the other columns stay put
			for _, title := range []string{"INSTANCE", "CAPACITY", "STATUS"} {
				if !slices.Contains(got, title) {
					t.Errorf("toggling price columns removed %s: %v", title, got)
				}
			}
		})
	}
}

// R3: K/J move a monthly column like any other.
func TestColumnsPanelMovesMonthlyColumns(t *testing.T) {
	u := monthlyUI(t)
	press(u, "o")
	pressOn(t, u, FieldPriceMonthly, "K")
	if got := u.view.Columns[:3]; !slices.Equal(got, []Field{FieldInstanceType, FieldPriceMonthly, FieldPrice}) {
		t.Errorf("after K on PRICE/M: %v", u.view.Columns)
	}
	pressOn(t, u, FieldIdleMonthly, "J")
	if i, j := slices.Index(u.view.Columns, FieldIdleMonthly), slices.Index(u.view.Columns, FieldCapacity); i != j+1 {
		t.Errorf("after J on IDLE/M: %v", u.view.Columns)
	}
}

// monthly-price-columns R5: with pricing disabled none of the four price columns is drawn, and no
// node row shows a price.
func TestDisablePricingLeavesOutAllPriceColumns(t *testing.T) {
	u := monthlyUI(t)
	u.DisablePricing = true
	v := u.view
	v.Columns = append(slices.Clone(DefaultColumns), FieldIdle)
	u.SetView(v)
	out := u.View()
	got := titles(t, out)
	for _, title := range []string{"PRICE/H", "PRICE/M", "IDLE/H", "IDLE/M"} {
		if slices.Contains(got, title) {
			t.Errorf("%s drawn with pricing disabled: %v", title, got)
		}
	}
	if want := []string{"INSTANCE", "CAPACITY", "NODEGROUP", "ZONE", "STATUS"}; !slices.Equal(got, want) {
		t.Errorf("titles with pricing disabled %v, want %v", got, want)
	}
	for _, line := range strings.Split(out, "\n") {
		for _, name := range monthlyNames() {
			if strings.Contains(line, name+" ") && strings.Contains(line, "$") {
				t.Errorf("row of %s shows a price: %q", name, line)
			}
		}
	}
	// the view keeps the columns, so enabling pricing brings them back
	if !slices.Contains(u.view.Columns, FieldPriceMonthly) {
		t.Error("DisablePricing removed PRICE/M from the view state")
	}
}

// monthly-price-columns R6: at 120 columns, cpu only, PRICE/M, CAPACITY and STATUS stay, and any
// dropped column is less important than every kept one.
func TestNarrowWindowKeepsTheMostImportantColumns(t *testing.T) {
	order := []string{"STATUS", "PRICE/M", "CAPACITY", "PRICE/H", "IDLE/M", "INSTANCE", "IDLE/H", "NODEGROUP", "ZONE", "IP", "AGE", "REGION"}
	for _, width := range []int{120, 100, 90} {
		t.Run(strconv.Itoa(width), func(t *testing.T) {
			u := realisticUI(t)
			v := u.view
			v.Resources = []string{"cpu"}
			v.Columns = append(slices.Clone(DefaultColumns), FieldIdle, FieldIP, FieldAge, FieldRegion)
			u.SetView(v)
			u.Update(tea.WindowSizeMsg{Width: width, Height: 40})
			got := titles(t, u.View())
			t.Logf("at %d: %v", width, got)
			if width == 120 {
				for _, title := range []string{"PRICE/M", "CAPACITY", "STATUS"} {
					if !slices.Contains(got, title) {
						t.Errorf("%s dropped at 120 columns: %v", title, got)
					}
				}
			}
			if !slices.Contains(got, "STATUS") {
				t.Errorf("STATUS dropped at %d: %v", width, got)
			}
			for i, kept := range order {
				if !slices.Contains(got, kept) {
					continue
				}
				for _, more := range order[:i] {
					if !slices.Contains(got, more) {
						t.Errorf("at %d %s kept but the more important %s dropped: %v", width, kept, more, got)
					}
				}
			}
		})
	}
}

// monthly-price-columns R4: the Sort panel keeps price and idle and has no monthly sort.
func TestSortPanelHasNoMonthlyFields(t *testing.T) {
	u := monthlyUI(t)
	press(u, "s")
	fields := u.sort.fields
	for _, f := range []Field{FieldPrice, FieldIdle} {
		if !slices.Contains(fields, f) {
			t.Errorf("Sort panel lacks %s: %v", f, fields)
		}
	}
	for _, f := range []Field{FieldPriceMonthly, FieldIdleMonthly, "price-month", "idle-month"} {
		if slices.Contains(fields, f) {
			t.Errorf("Sort panel offers %s", f)
		}
	}
	rows := u.sortPanel().rows
	for _, want := range []string{"price", "idle"} {
		if !slices.Contains(rows, want) {
			t.Errorf("Sort panel rows lack %q: %v", want, rows)
		}
	}
	for _, r := range rows {
		if strings.Contains(strings.ToLower(r), "month") || strings.Contains(r, "/M") {
			t.Errorf("Sort panel row %q is a monthly sort", r)
		}
	}
}

// dollars parses a cell as a number; ok is false for "-".
func dollars(t *testing.T, cell string) (float64, bool) {
	t.Helper()
	if cell == "-" {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimLeft(cell, "~$"), ",", ""), 64)
	if err != nil {
		t.Fatalf("cell %q: %v", cell, err)
	}
	return v, true
}

// R4: sorting by price or idle orders the monthly columns too, with the
// nodes that have no value last.
func TestSortingByPriceOrdersMonthlyColumns(t *testing.T) {
	cases := []struct {
		sort  Field
		title string
	}{
		{FieldPrice, "PRICE/M"},
		{FieldIdle, "IDLE/M"},
	}
	for _, c := range cases {
		for _, desc := range []bool{true, false} {
			t.Run(c.title+map[bool]string{true: " desc", false: " asc"}[desc], func(t *testing.T) {
				u := monthlyUI(t)
				v := u.view
				v.Sort = SortSpec{Field: c.sort, Descending: desc}
				u.SetView(v)
				out := u.View()
				order, values := columnValues(t, out, c.title, monthlyNames())
				if len(order) != len(monthlyNodes) {
					t.Fatalf("rows on screen %v:\n%s", order, out)
				}
				var prev float64
				seenNone, first := false, true
				for _, name := range order {
					val, ok := dollars(t, values[name])
					if !ok {
						seenNone = true
						continue
					}
					if seenNone {
						t.Errorf("%s with %s after a node without one: %v", name, values[name], order)
					}
					if !first && ((desc && val > prev) || (!desc && val < prev)) {
						t.Errorf("%s out of order at %s (%s): %v %v", c.title, name, values[name], order, values)
					}
					prev, first = val, false
				}
			})
		}
	}
}
