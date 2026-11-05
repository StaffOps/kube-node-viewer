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
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// realisticUI looks like the owner's cluster: long names, managed node
// groups, cpu and memory, prices.
func realisticUI(t *testing.T) *UIModel {
	u := newTestUI(t)
	u.SetClusterInfo(ClusterInfo{Context: "demo1-test-upgrade", Server: "https://ABCDEF0123456789ABCDEF0123456789.gr7.us-east-1.eks.amazonaws.com", Region: "us-east-1"})
	for i := 0; i < 8; i++ {
		labels := map[string]string{
			"karpenter.sh/capacity-type": "on-demand",
			v1.LabelTopologyZone:         "us-east-1a",
			v1.LabelInstanceTypeStable:   "t4g.xlarge",
		}
		if i%3 == 0 {
			labels["eks.amazonaws.com/nodegroup"] = "karpenter-20261031152303179400000003d"
		} else {
			labels["karpenter.sh/nodepool"] = "demo1-ondemand-dependencies-critical-arm"
		}
		n := NewNode(&v1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("ip-10-200-%d-111.ec2.internal", 30+i), CreationTimestamp: metav1.Now(), Labels: labels},
			Spec:       v1.NodeSpec{ProviderID: fmt.Sprintf("aws:///us-east-1a/i-%d", i)},
			Status: v1.NodeStatus{
				Allocatable: v1.ResourceList{v1.ResourceCPU: resource.MustParse("4"), v1.ResourceMemory: resource.MustParse("16Gi"), v1.ResourcePods: resource.MustParse("58")},
				Addresses:   []v1.NodeAddress{{Type: v1.NodeInternalIP, Address: fmt.Sprintf("10.200.%d.111", 30+i)}},
			},
		})
		u.cluster.AddNode(n).Show()
		n.SetPrice(0.1344)
	}
	v := DefaultViewState()
	v.Resources = []string{"cpu", "memory"}
	u.SetView(v)
	u.Refresh()
	return u
}

// Every line fits the window and the frame never has more lines than the
// window: a longer line wraps, the frame scrolls and leaves garbage behind.
func TestFrameFitsWindow(t *testing.T) {
	u := realisticUI(t)
	for _, grouped := range []bool{false, true} {
		if grouped {
			v := u.view
			v.SetGroupBy(FieldNodeGroup, FieldZone)
			u.SetView(v)
		}
		for width := 40; width <= 260; width += 7 {
			for _, height := range []int{10, 16, 24, 40} {
				u.Update(tea.WindowSizeMsg{Width: width, Height: height})
				frame := u.View()
				lines := strings.Split(frame, "\n")
				if len(lines) > height {
					t.Errorf("grouped=%v %dx%d: %d lines", grouped, width, height, len(lines))
				}
				for i, line := range lines {
					if w := lipgloss.Width(line); w > width {
						t.Errorf("grouped=%v %dx%d: line %d is %d wide: %q", grouped, width, height, i, w, line)
						break
					}
				}
			}
		}
	}
}

func TestPopupBordersLineUp(t *testing.T) {
	u := realisticUI(t)
	u.Update(tea.WindowSizeMsg{Width: 150, Height: 30})
	for _, key := range []string{"f", "g", "s", "o", "?"} {
		press(u, key)
		p, _ := u.panelView()
		box := u.popup(p, 20)
		want := lipgloss.Width(box[0])
		for i, line := range box {
			if w := lipgloss.Width(line); w != want {
				t.Errorf("%s: line %d is %d wide, top border %d", key, i, w, want)
			}
		}
		press(u, "esc", "esc")
	}
}

func TestNodeAndPodCountsOnlyInHeader(t *testing.T) {
	u := realisticUI(t)
	u.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	out := u.View()
	if strings.Count(out, "8 of 8 shown") != 1 || strings.Contains(out, "8 nodes (") || strings.Contains(out, "pods (0 pending") {
		t.Errorf("counts repeated below the header:\n%s", out)
	}
}

func TestHeaderShowsRecentIssueOnly(t *testing.T) {
	u := realisticUI(t)
	u.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	at := time.Now()
	u.SetIssues(func() (string, time.Time) { return "failed to list *v1.Node: connection refused", at }, "/tmp/knv.log")
	u.dirty = true
	if out := u.View(); !strings.Contains(out, "connection refused") || !strings.Contains(out, "/tmp/knv.log") {
		t.Errorf("recent issue not shown:\n%s", out)
	}
	at = time.Now().Add(-issueWindow - time.Second)
	u.dirty = true
	if strings.Contains(u.View(), "connection refused") {
		t.Error("an old issue is still shown")
	}
}

// interactive-tui R8: totals leave reserved nodes out and say how many.
func TestSummaryCoversOnlyShownNodes(t *testing.T) {
	u := demoUI(t)
	u.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	all := u.View()
	if !strings.Contains(all, "$0.997/hour") || strings.Contains(all, "filtered:") {
		t.Fatalf("unfiltered summary:\n%s", all)
	}
	// spot-arm: three nodes at 0.031 + 0.029 + 0.058 = 0.118/hour
	v := u.view
	v.Filters = []Filter{{Field: FieldNodeGroup, Values: []string{"spot-arm"}, Enabled: true}}
	u.SetView(v)
	out := u.View()
	if !strings.Contains(out, "$0.118/hour") || !strings.Contains(out, "(filtered: 3 of 10 nodes)") {
		t.Errorf("filtered summary:\n%s", out)
	}
	// the box keeps the cluster's cost; the summary below it must not
	if below := out[strings.Index(out, "╰"):]; strings.Contains(below, "$0.997/hour") {
		t.Error("the summary still shows the cluster total under a filter")
	}
	// a reserved node filtered in is counted as not priced, not as cost
	v.Filters = []Filter{{Field: FieldCapacity, Values: []string{"Reserved"}, Enabled: true}}
	u.SetView(v)
	if out := u.View(); !strings.Contains(out, "$0.000/hour") || !strings.Contains(out, "(+1 reserved, not priced)") {
		t.Errorf("reserved only:\n%s", out)
	}
}

func TestHeaderKeepsTheClusterCost(t *testing.T) {
	u := demoUI(t)
	u.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	v := u.view
	v.Filters = []Filter{{Field: FieldNodeGroup, Values: []string{"spot-arm"}, Enabled: true}}
	u.SetView(v)
	out := u.View()
	header := out[:strings.Index(out, "╰")]
	if !strings.Contains(header, "3 of 10 shown · 45 pods · $728.102/month (+1 reserved, not priced) (global cluster)") {
		t.Errorf("cluster cost not after the counts under a filter:\n%s", header)
	}
	if !strings.Contains(out, "$0.118/hour") {
		t.Error("the summary lost the filtered cost")
	}
	u.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
	if compact := u.View(); !strings.Contains(compact[:strings.Index(compact, "╰")], "(global cluster)") {
		t.Errorf("compact header without the cluster cost:\n%s", compact)
	}
	u.DisablePricing = true
	u.dirty = true
	if strings.Contains(u.View(), "global cluster") {
		t.Error("cost shown with pricing disabled")
	}
}

// recommendations R4: the IDLE/H column, the idle sort and idle cost on
// group headings.
func TestIdleCost(t *testing.T) {
	u := demoUI(t)
	u.Update(tea.WindowSizeMsg{Width: 240, Height: 60})
	v := u.view
	v.Columns = append(v.Columns, FieldIdle)
	v.Sort = SortSpec{Field: FieldIdle, Descending: true}
	u.SetView(v)
	out := u.View()
	if !strings.Contains(out, "IDLE/H") {
		t.Fatalf("no IDLE/H column:\n%s", out)
	}
	// the reserved gpu node has no idle cost and sorts last
	nodes := u.view.Select(u.stats.Nodes)
	if last := nodes[len(nodes)-1]; last.CapacityType() != CapacityReserved {
		t.Errorf("last by idle: %s (%s)", last.Name(), last.CapacityType())
	}
	if _, ok := nodes[len(nodes)-1].IdleHourly(); ok {
		t.Error("a reserved node has an idle cost")
	}
	// m6i.xlarge at 84% cpu / 66% mem: 0.192 × 0.16
	for _, n := range nodes {
		if n.Name() == "ip-192-0-2-17.ec2.internal" {
			if idle, _ := n.IdleHourly(); idle < 0.029 || idle > 0.032 {
				t.Errorf("idle %v", idle)
			}
		}
	}
	v.SetGroupBy(FieldNodeGroup)
	u.SetView(v)
	if !strings.Contains(u.View(), "idle $") {
		t.Error("group headings without idle cost")
	}
}
