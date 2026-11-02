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
		"g5.xlarge · OnDemand · $1.0060/h",
		"Node group   gpu",
		"Zone         us-east-1b (us-east-1)",
		"Kubelet      v1.35.9",
		"Provider ID  aws:///us-east-1b/i-0gpu",
		"Resources  requested / allocatable / capacity",
		"memory          -",
		"nvidia.com/gpu  - / 1 / 1",
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
	if !strings.Contains(out, "╭─ ip-10-0-11-24.ec2.internal") || !strings.Contains(out, "pods    38 (66%) / 58 / 58") {
		t.Fatalf("details of the second node:\n%s", out)
	}
	if !strings.Contains(out, "cpu     3534m (90%) / 3920m / 4") || !strings.Contains(out, "memory  10.9Gi (78%) / 14.0Gi / 15.8Gi") {
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

func TestEnterOnAGroupStillFolds(t *testing.T) {
	u := detailsUI(t)
	v := u.view
	v.SetGroupBy(FieldCapacity)
	u.SetView(v)
	press(u, "enter")
	if u.panel != panelNone {
		t.Errorf("enter on a group heading opened panel %v", u.panel)
	}
}
