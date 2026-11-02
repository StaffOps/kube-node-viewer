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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// demoNode is one node of the made-up cluster the README screenshots show.
type demoNode struct {
	name, itype, zone, pool, mng, capacity string
	cpu, mem                               string // allocatable
	cpuUse, memUse                         float64
	pods                                   int
	price                                  float64
	age                                    time.Duration
	cordoned                               bool
}

var demoNodes = []demoNode{
	{"ip-10-0-11-24.ec2.internal", "m6i.xlarge", "us-east-1a", "default", "", "on-demand", "3920m", "14Gi", 0.91, 0.78, 38, 0.192, 52 * time.Hour, false},
	{"ip-10-0-11-87.ec2.internal", "m6i.xlarge", "us-east-1a", "default", "", "on-demand", "3920m", "14Gi", 0.84, 0.66, 31, 0.192, 49 * time.Hour, false},
	{"ip-10-0-12-15.ec2.internal", "m6i.xlarge", "us-east-1b", "default", "", "on-demand", "3920m", "14Gi", 0.62, 0.71, 27, 0.192, 30 * time.Hour, false},
	{"ip-10-0-13-40.ec2.internal", "m6i.2xlarge", "us-east-1c", "default", "", "spot", "7910m", "29Gi", 0.73, 0.58, 44, 0.137, 6 * time.Hour, false},
	{"ip-10-0-11-93.ec2.internal", "c7g.large", "us-east-1a", "spot-arm", "", "spot", "1930m", "3Gi", 0.88, 0.69, 14, 0.031, 3 * time.Hour, false},
	{"ip-10-0-12-61.ec2.internal", "c7g.large", "us-east-1b", "spot-arm", "", "spot", "1930m", "3Gi", 0.47, 0.35, 9, 0.029, 2 * time.Hour, false},
	{"ip-10-0-13-12.ec2.internal", "c7g.xlarge", "us-east-1c", "spot-arm", "", "spot", "3920m", "7Gi", 0.95, 0.82, 22, 0.058, 90 * time.Minute, false},
	{"ip-10-0-12-33.ec2.internal", "g5.xlarge", "us-east-1b", "gpu", "", "reserved", "3920m", "14Gi", 0.55, 0.61, 6, 1.006, 14 * 24 * time.Hour, false},
	{"ip-10-0-11-70.ec2.internal", "t3.large", "us-east-1a", "", "system-ng", "", "1930m", "7Gi", 0.68, 0.52, 17, 0.0832, 40 * 24 * time.Hour, false},
	{"ip-10-0-12-98.ec2.internal", "t3.large", "us-east-1b", "", "system-ng", "", "1930m", "7Gi", 0.39, 0.44, 12, 0.0832, 40 * 24 * time.Hour, true},
}

// demoSwitcher lists made-up contexts for the context popup.
type demoSwitcher struct{}

func (demoSwitcher) Contexts() ([]string, string, error) {
	return []string{"demo-dev", "demo-prod", "demo-staging", "kind-local"}, "demo-prod", nil
}

func (demoSwitcher) Switch(string) (Session, error) { return Session{}, fmt.Errorf("demo") }

func demoUI(t *testing.T) *UIModel {
	u := newTestUI(t)
	// the bar detects colours from the terminal, and tests have none
	u.progress = newProgress(u.style, progress.WithColorProfile(termenv.TrueColor))
	u.SetClusterInfo(ClusterInfo{Context: "demo-prod", Server: "https://0A1B2C3D4E5F.gr7.us-east-1.eks.amazonaws.com", Region: "us-east-1"})
	u.SetSwitcher(demoSwitcher{})
	for i, d := range demoNodes {
		labels := map[string]string{
			v1.LabelTopologyZone:       d.zone,
			v1.LabelTopologyRegion:     "us-east-1",
			v1.LabelInstanceTypeStable: d.itype,
		}
		if d.pool != "" {
			labels["karpenter.sh/nodepool"] = d.pool
			labels["karpenter.sh/capacity-type"] = d.capacity
		} else {
			labels["eks.amazonaws.com/nodegroup"] = d.mng
			labels["eks.amazonaws.com/capacityType"] = "ON_DEMAND"
		}
		allocCPU, allocMem := resource.MustParse(d.cpu), resource.MustParse(d.mem)
		n := NewNode(&v1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: d.name, CreationTimestamp: metav1.NewTime(time.Now().Add(-d.age)), Labels: labels},
			Spec:       v1.NodeSpec{ProviderID: fmt.Sprintf("aws:///%s/i-0demo%02d", d.zone, i), Unschedulable: d.cordoned},
			Status: v1.NodeStatus{
				Allocatable: v1.ResourceList{v1.ResourceCPU: allocCPU, v1.ResourceMemory: allocMem, v1.ResourcePods: resource.MustParse("58")},
				Addresses:   []v1.NodeAddress{{Type: v1.NodeInternalIP, Address: fmt.Sprintf("10.0.%d.%d", 11+i%3, 10+i*7)}},
				Conditions:  []v1.NodeCondition{{Type: v1.NodeReady, Status: v1.ConditionTrue}},
			},
		})
		u.cluster.AddNode(n).Show()
		if d.capacity == "reserved" {
			n.SetEquivalentPrice(d.price)
		} else {
			n.SetPrice(d.price)
		}
		// split the node's requests across its pods
		cpuPer := int64(float64(allocCPU.MilliValue()) * d.cpuUse / float64(d.pods))
		memPer := int64(float64(allocMem.Value()) * d.memUse / float64(d.pods))
		for p := 0; p < d.pods; p++ {
			pod := &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: fmt.Sprintf("%s-pod-%d", d.name, p)},
				Spec: v1.PodSpec{NodeName: d.name, Containers: []v1.Container{{Name: "app", Resources: v1.ResourceRequirements{Requests: v1.ResourceList{
					v1.ResourceCPU:    *resource.NewMilliQuantity(cpuPer, resource.DecimalSI),
					v1.ResourceMemory: *resource.NewQuantity(memPer, resource.BinarySI),
				}}}}},
				Status: v1.PodStatus{Phase: v1.PodRunning},
			}
			u.cluster.AddPod(NewPod(pod))
		}
	}
	v := DefaultViewState()
	v.Resources = []string{"cpu", "memory"}
	u.SetView(v)
	u.Refresh()
	return u
}

// TestScreenshots writes the screens the README shows, with colours, as ANSI
// text files in $KNV_SCREENSHOTS. It only runs when that is set:
//
//	KNV_SCREENSHOTS=/tmp/shots go test ./pkg/model -run TestScreenshots
//
// The cluster is made up, so no real account or cluster shows in the docs.
func TestScreenshots(t *testing.T) {
	dir := os.Getenv("KNV_SCREENSHOTS")
	if dir == "" {
		t.Skip("set KNV_SCREENSHOTS to a directory to write the README screenshots")
	}
	lipgloss.SetColorProfile(termenv.TrueColor)
	lipgloss.SetHasDarkBackground(true)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	scenes := []struct {
		name          string
		width, height int
		keys          func(u *UIModel)
	}{
		{"main", 168, 38, func(u *UIModel) {}},
		{"groups-folded", 168, 34, func(u *UIModel) {
			v := u.view
			v.SetGroupBy(FieldNodeGroup, FieldZone)
			u.SetView(v)
			press(u, "n")
		}},
		{"groups-details", 168, 30, func(u *UIModel) {
			v := u.view
			v.SetGroupBy(FieldNodeGroup, FieldZone)
			u.SetView(v)
			press(u, "n", "n", "d")
		}},
		{"filters", 168, 38, func(u *UIModel) { press(u, "f", "enter", "down", "down", " ") }},
		{"group-by", 168, 38, func(u *UIModel) { press(u, "g", "down", " ", "down", "down", " ") }},
		{"columns", 168, 38, func(u *UIModel) { press(u, "o") }},
		{"contexts", 168, 38, func(u *UIModel) { apply(u, press(u, "c")) }},
		// tall enough to start with the full panel, which ctrl+e compacts
		{"header-compact", 168, 30, func(u *UIModel) { u.Update(tea.KeyMsg{Type: tea.KeyCtrlE}) }},
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// one session, step by step, for the README animation: each frame is the
	// screen after its key, with a caption naming the key
	demo := demoUI(t)
	demo.Update(tea.WindowSizeMsg{Width: 150, Height: 34})
	steps := []struct {
		caption string
		keys    func(u *UIModel)
	}{
		{"kube-node-viewer: every node, cpu and memory requests against allocatable", func(u *UIModel) {}},
		{"g  opens Group by", func(u *UIModel) { press(u, "g") }},
		{"space  ticks nodegroup as level 1", func(u *UIModel) { press(u, "down", " ") }},
		{"space  ticks zone as level 2, a sub-group of nodegroup", func(u *UIModel) { press(u, "down", "down", " ") }},
		{"esc  nodes nest under nodegroup › zone, with subtotals", func(u *UIModel) { press(u, "esc") }},
		{"enter  folds the selected group on its own (↑/↓ select)", func(u *UIModel) { press(u, "enter") }},
		{"enter  unfolds it", func(u *UIModel) { press(u, "enter") }},
		{"n  folds the nodes: zones carry the usage bars", func(u *UIModel) { press(u, "n") }},
		{"n  folds the zones: node groups carry the bars", func(u *UIModel) { press(u, "n") }},
		{"d  details: each node group's zones as one-line rows", func(u *UIModel) { press(u, "d") }},
		{"ctrl+e  compacts the header", func(u *UIModel) { demo.Update(tea.KeyMsg{Type: tea.KeyCtrlE}) }},
		{"ctrl+e  expands it again", func(u *UIModel) { demo.Update(tea.KeyMsg{Type: tea.KeyCtrlE}) }},
		{"n  unfolds everything", func(u *UIModel) { press(u, "n") }},
	}
	for i, st := range steps {
		st.keys(demo)
		demo.dirty = true
		base := filepath.Join(dir, fmt.Sprintf("demo-%02d", i))
		if err := os.WriteFile(base+".ansi", []byte(demo.View()), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(base+".caption", []byte(st.caption), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	for _, s := range scenes {
		u := demoUI(t)
		u.Update(tea.WindowSizeMsg{Width: s.width, Height: s.height})
		s.keys(u)
		u.dirty = true
		frame := u.View()
		if err := os.WriteFile(filepath.Join(dir, s.name+".ansi"), []byte(frame), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
