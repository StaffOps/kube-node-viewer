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
	u.SetClusterInfo(ClusterInfo{Context: "qrvey-test-upgrade", Server: "https://87F7FBEF13B2DF60B170A02F69970772.gr7.us-east-1.eks.amazonaws.com", Region: "us-east-1"})
	for i := 0; i < 8; i++ {
		labels := map[string]string{
			"karpenter.sh/capacity-type": "on-demand",
			v1.LabelTopologyZone:         "us-east-1a",
			v1.LabelInstanceTypeStable:   "t4g.xlarge",
		}
		if i%3 == 0 {
			labels["eks.amazonaws.com/nodegroup"] = "karpenter-20261031152303179400000003d"
		} else {
			labels["karpenter.sh/nodepool"] = "qrvey-ondemand-dependencies-critical-arm"
		}
		n := NewNode(&v1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("ip-10-110-%d-111.ec2.internal", 30+i), CreationTimestamp: metav1.Now(), Labels: labels},
			Spec:       v1.NodeSpec{ProviderID: fmt.Sprintf("aws:///us-east-1a/i-%d", i)},
			Status: v1.NodeStatus{
				Allocatable: v1.ResourceList{v1.ResourceCPU: resource.MustParse("4"), v1.ResourceMemory: resource.MustParse("16Gi"), v1.ResourcePods: resource.MustParse("58")},
				Addresses:   []v1.NodeAddress{{Type: v1.NodeInternalIP, Address: fmt.Sprintf("10.110.%d.111", 30+i)}},
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
