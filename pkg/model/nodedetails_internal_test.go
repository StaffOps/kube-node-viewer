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
	"github.com/charmbracelet/x/ansi"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// detailsUI is the demo cluster plus a tainted node under memory pressure
// with a GPU, sorted by name so that node ("aaa-…") is the first row.
func detailsUI(t *testing.T) *UIModel {
	u := demoUI(t)
	n := NewNode(&v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "aaa-gpu-node", CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
			Labels: map[string]string{
				v1.LabelInstanceTypeStable: "g5.xlarge", v1.LabelTopologyZone: "us-east-1b", v1.LabelTopologyRegion: "us-east-1",
				"karpenter.sh/nodepool": "gpu", "karpenter.sh/capacity-type": "on-demand",
			},
		},
		Spec: v1.NodeSpec{
			ProviderID: "aws:///us-east-1b/i-0gpu",
			Taints:     []v1.Taint{{Key: "nvidia.com/gpu", Value: "present", Effect: v1.TaintEffectNoSchedule}},
		},
		Status: v1.NodeStatus{
			Capacity: v1.ResourceList{v1.ResourceCPU: resource.MustParse("4"), v1.ResourceMemory: resource.MustParse("16Gi"),
				v1.ResourcePods: resource.MustParse("58"), "nvidia.com/gpu": resource.MustParse("1"), "hugepages-2Mi": resource.MustParse("0")},
			Allocatable: v1.ResourceList{v1.ResourceCPU: resource.MustParse("3920m"), v1.ResourceMemory: resource.MustParse("14Gi"),
				v1.ResourcePods: resource.MustParse("58"), "nvidia.com/gpu": resource.MustParse("1")},
			Conditions: []v1.NodeCondition{
				{Type: v1.NodeReady, Status: v1.ConditionTrue, Reason: "KubeletReady"},
				{Type: v1.NodeMemoryPressure, Status: v1.ConditionTrue, Reason: "KubeletHasInsufficientMemory", Message: "available memory is low"},
			},
			NodeInfo: v1.NodeSystemInfo{KubeletVersion: "v1.35.9", OSImage: "Bottlerocket", KernelVersion: "6.1", Architecture: "amd64", ContainerRuntimeVersion: "containerd://2.1"},
		},
	})
	u.cluster.AddNode(n).Show()
	n.SetPrice(1.006)
	v := u.view
	v.Sort = SortSpec{Field: FieldName}
	u.SetView(v)
	u.Refresh()
	u.Update(tea.WindowSizeMsg{Width: 160, Height: 90})
	return u
}

// node-details R1: enter on a node opens its details.
func TestEnterOnANodeOpensItsDetails(t *testing.T) {
	u := detailsUI(t)
	if line := selectedLine(t, u); !strings.Contains(line, "aaa-gpu-node") {
		t.Fatalf("first selected row: %q", line)
	}
	press(u, "enter")
	if u.panel != panelNode {
		t.Fatalf("panel %v, want the node details", u.panel)
	}
	out := u.View()
	for _, want := range []string{
		"╭─ aaa-gpu-node",
		"g5.xlarge · OnDemand · $1.0060/h · $734.38/mo",
		"Node group   gpu",
		"Zone         us-east-1b (us-east-1)",
		"Kubelet      v1.35.9",
		"Provider ID  aws:///us-east-1b/i-0gpu",
		"Resources  requested / allocatable / capacity",
		"memory          -",
		"nvidia.com/gpu  - / 1",
		"MemoryPressure  True",
		"available memory is low",
		"nvidia.com/gpu=present:NoSchedule",
		"Labels (5)",
		"karpenter.sh/nodepool=gpu",
		"↑/↓: scroll",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("details lack %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "hugepages-2Mi") {
		t.Errorf("a resource the node has none of is listed:\n%s", out)
	}
	press(u, "esc")
	if u.panel != panelNone {
		t.Errorf("esc did not close the details")
	}
}

func TestDetailsShowRequestsAndScroll(t *testing.T) {
	u := detailsUI(t)
	u.View()
	press(u, "down", "enter") // ip-10-0-11-24: 38 pods, cpu and memory requested
	out := u.View()
	if !strings.Contains(out, "╭─ ip-10-0-11-24.ec2.internal") || !strings.Contains(out, "38 (66%) / 58") {
		t.Fatalf("details of the second node:\n%s", out)
	}
	if !strings.Contains(out, "cpu     3534m (90%) / 3920m / 4000m") || !strings.Contains(out, "memory  10.9Gi (78%) / 14.0Gi / 15.8Gi") {
		t.Errorf("cpu requests not shown:\n%s", out)
	}
	// a short screen scrolls; down moves the first row
	u.Update(tea.WindowSizeMsg{Width: 160, Height: 20})
	if !strings.Contains(u.View(), "╭─ ip-10-0-11-24") {
		t.Fatal("no popup")
	}
	before := u.View()
	press(u, "down", "down")
	if u.View() == before || u.detailsTop != 2 {
		t.Errorf("down did not scroll (top %d)", u.detailsTop)
	}
	press(u, "home")
	if u.detailsTop != 0 {
		t.Errorf("home: top %d", u.detailsTop)
	}
}

func TestDetailsOfANodeThatGoesAway(t *testing.T) {
	u := detailsUI(t)
	press(u, "enter")
	n, _ := u.cluster.GetNodeByName("aaa-gpu-node")
	u.cluster.DeleteNode(n.ProviderID())
	u.Refresh()
	if out := u.View(); !strings.Contains(out, "no longer in the cluster") || u.panel != panelNode {
		t.Errorf("a node that went away:\n%s", out)
	}
}

// enter on a group heading focuses the group (group-focus R1); it never
// opens the details popup.
func TestEnterOnAGroupFocusesIt(t *testing.T) {
	u := detailsUI(t)
	v := u.view
	v.SetGroupBy(FieldCapacity)
	u.SetView(v)
	press(u, "enter")
	if u.panel != panelNone {
		t.Errorf("enter on a group heading opened panel %v", u.panel)
	}
	if !u.focused() {
		t.Error("enter on a group heading did not focus it")
	}
}

// node-pods R1: tab switches Details ⇄ Pods, marked on a fixed line under
// the title, which stays the node name.
func TestNodeDetailsTabs(t *testing.T) {
	u := detailsUI(t)
	u.View()
	press(u, "enter")
	lines := box(u)
	if !strings.Contains(lines[0], "╭─ aaa-gpu-node") || !strings.Contains(lines[1], "[Details]  Pods") {
		t.Fatalf("details opened as:\n%s", strings.Join(lines, "\n"))
	}
	if !strings.Contains(shown(u), "Node group   gpu") {
		t.Errorf("Details content changed:\n%s", shown(u))
	}
	press(u, "tab")
	lines = box(u)
	if !strings.Contains(lines[0], "╭─ aaa-gpu-node") || !strings.Contains(lines[1], "Details  [Pods]") || strings.Contains(shown(u), "Node group") {
		t.Errorf("Pods tab:\n%s", strings.Join(lines, "\n"))
	}
	press(u, "tab")
	if lines = box(u); !strings.Contains(lines[1], "[Details]  Pods") || !strings.Contains(shown(u), "Node group   gpu") {
		t.Errorf("tab did not go back to Details:\n%s", strings.Join(lines, "\n"))
	}
	press(u, "enter")
	if u.panel != panelNone {
		t.Errorf("enter on Details no longer closes the popup")
	}
}

func TestNodeDetailsSpaceStillPages(t *testing.T) {
	u := detailsUI(t)
	u.View()
	press(u, "enter")
	u.Update(tea.WindowSizeMsg{Width: 160, Height: 20})
	before := u.View()
	press(u, " ")
	if u.panel != panelNode || u.detailsTop == 0 || u.View() == before {
		t.Errorf("space in Details did not page down (top %d)", u.detailsTop)
	}
}

// node-pods R1: the key line of each tab fits an 80-column window, and ?
// lists the keys.
func TestNodeDetailsKeyLines(t *testing.T) {
	u := detailsUI(t)
	u.View()
	press(u, "enter")
	u.Update(tea.WindowSizeMsg{Width: 80, Height: 60})
	for _, tab := range []struct {
		name string
		keys []string
	}{
		{"details", []string{"↑/↓", "tab", "esc"}},
		{"pods", []string{"↑/↓", "d: daemonsets", "s/S", "tab", "esc"}},
	} {
		hint := u.helpText()
		if w := textWidth(hint); w > 74 {
			t.Errorf("%s key line is %d columns: %q", tab.name, w, hint)
		}
		if !strings.Contains(shown(u), hint) {
			t.Errorf("%s key line %q not drawn whole on 80 columns:\n%s", tab.name, hint, shown(u))
		}
		for _, k := range tab.keys {
			if !strings.Contains(hint, k) {
				t.Errorf("%s key line %q lacks %q", tab.name, hint, k)
			}
		}
		press(u, "tab")
	}
	press(u, "esc", "?")
	help := ansi.Strip(u.View())
	for _, want := range []string{"Details ⇄ Pods", "all pods ⇄ DaemonSets", "sort by cost", "move by a page"} {
		if !strings.Contains(help, want) {
			t.Errorf("? lacks %q", want)
		}
	}
}

// node-pods R1: the Details scroll keys are unchanged by the tabs.
func TestNodeDetailsScrollKeysUnchanged(t *testing.T) {
	u := detailsUI(t)
	u.View()
	press(u, "enter")
	u.Update(tea.WindowSizeMsg{Width: 160, Height: 20})
	u.View()
	steps := []struct {
		key  string
		want int
	}{{"j", 1}, {"down", 2}, {"k", 1}, {"up", 0}, {"up", 0}, {"pgdown", 10}, {"pgup", 0}, {" ", 10}, {"home", 0}}
	for _, st := range steps {
		press(u, st.key)
		if u.detailsTop != st.want {
			t.Errorf("after %q top %d, want %d", st.key, u.detailsTop, st.want)
		}
	}
}

// The popup follows the live node: a changed value shows on the next frame;
// labels are sorted by key; enter closes it like esc.
// node-details R1
func TestDetailsAreLiveSortedAndEnterCloses(t *testing.T) {
	u := detailsUI(t)
	press(u, "enter")
	out := ansi.Strip(u.View())
	if !strings.Contains(out, "$1.0060/h") {
		t.Fatalf("price before the change:\n%s", out)
	}
	keys := []string{"karpenter.sh/capacity-type=", "karpenter.sh/nodepool=", "node.kubernetes.io/instance-type=",
		"topology.kubernetes.io/region=", "topology.kubernetes.io/zone="}
	last := -1
	for _, k := range keys {
		i := strings.Index(out, k)
		if i < 0 || i < last {
			t.Errorf("label %q missing or out of key order:\n%s", k, out)
		}
		last = i
	}

	n, _ := u.cluster.GetNodeByName("aaa-gpu-node")
	n.SetPrice(2)
	u.Refresh()
	if out := ansi.Strip(u.View()); !strings.Contains(out, "$2.0000/h") || u.panel != panelNode {
		t.Errorf("the popup did not follow the new price:\n%s", out)
	}

	press(u, "enter")
	if u.panel != panelNone {
		t.Errorf("enter did not close the details (panel %v)", u.panel)
	}
}
