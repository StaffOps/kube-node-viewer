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

	tea "github.com/charmbracelet/bubbletea"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// openDetails opens node-a's details on the Details tab, without `tab`.
func openDetails(t *testing.T, u *UIModel) string {
	t.Helper()
	u.Refresh()
	u.Update(tea.WindowSizeMsg{Width: 120, Height: 80})
	u.View()
	press(u, "enter")
	if u.panel != panelNode || u.detailsTab != detailsTabDetails {
		t.Fatalf("enter did not open the Details tab")
	}
	return shown(u)
}

// Owner 2026-11-03: when allocatable equals capacity (pods, usually), the
// row shows requested / allocatable only; when they differ, all three.
func TestDetailsTabCapacityOnlyWhenItDiffers(t *testing.T) {
	u := newTestUI(t)
	n := podsNode(u, fourCPU16Gi, nil)
	obj := n.Object().DeepCopy()
	obj.Status.Capacity = v1.ResourceList{
		v1.ResourceCPU: resource.MustParse("8"), v1.ResourceMemory: resource.MustParse("16Gi"), v1.ResourcePods: resource.MustParse("110"),
	}
	n.Update(obj)
	addPods(u, podSpec{name: "web", cpu: "1", mem: "1Gi"})
	out := openDetails(t, u)
	ends := func(suffix string) bool {
		for _, l := range strings.Split(out, "\n") {
			if strings.HasSuffix(strings.TrimRight(l, " │"), suffix) {
				return true
			}
		}
		return false
	}
	for _, want := range []string{"1000m (25%) / 4000m / 8000m", "1.0Gi (6%) / 16.0Gi"} {
		if !ends(want) {
			t.Errorf("no row ends with %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "/ 110 / 110") || strings.Contains(out, "pods  ") {
		t.Errorf("equal capacity repeated:\n%s", out)
	}
}

// Owner 2026-11-03: pods are not a resource row; they have their own section
// with the count and the max, before DaemonSets.
func TestDetailsTabPodsSection(t *testing.T) {
	u := newTestUI(t)
	podsNode(u, fourCPU16Gi, nil)
	addPods(u, podSpec{name: "web", cpu: "1", mem: "1Gi"}, podSpec{name: "api", cpu: "1", mem: "1Gi"})
	out := openDetails(t, u)
	res, pods, ds := strings.Index(out, "Resources"), strings.Index(out, "on the node / max"), strings.Index(out, "DaemonSets")
	if res < 0 || pods < res || ds < pods {
		t.Fatalf("want Resources, then Pods, then DaemonSets:\n%s", out)
	}
	if !strings.Contains(out[pods:ds], "2 (2%) / 110") {
		t.Errorf("Pods section lacks the count:\n%s", out)
	}
	for _, l := range strings.Split(out[res:pods], "\n") {
		if strings.Contains(strings.TrimLeft(l, " │"), "pods ") {
			t.Errorf("pods still listed in Resources: %q", l)
		}
	}
}

// node-pods R11 (owner 2026-11-03): CPU in millicores everywhere in the tab.
func TestDetailsTabCPUInMillicores(t *testing.T) {
	u := newTestUI(t)
	podsNode(u, fourCPU16Gi, nil)
	addPods(u, podSpec{name: "web", cpu: "1", mem: "1Gi"})
	out := openDetails(t, u)
	if !strings.Contains(out, "1000m (25%) / 4000m") {
		t.Errorf("cpu not in millicores:\n%s", out)
	}
}

// node-pods R11: on enter, the Details tab says what the DaemonSets request,
// absolute and as a percent of allocatable, per resource.
func TestDetailsTabShowsDaemonSets(t *testing.T) {
	u := newTestUI(t)
	podsNode(u, fourCPU16Gi, nil)
	addPods(u,
		podSpec{name: "web", cpu: "1", mem: "1Gi"},
		podSpec{ns: "kube-system", name: "proxy-x", cpu: "200m", mem: "1Gi", ownerKind: "DaemonSet", ownerName: "kube-proxy"},
		podSpec{ns: "kube-system", name: "agent-x", cpu: "200m", mem: "3Gi", ownerKind: "DaemonSet", ownerName: "agent"},
	)
	out := openDetails(t, u)
	// 400m of 4 CPU = 10%; 4Gi of 16Gi = 25%
	for _, want := range []string{
		"DaemonSets",
		"2 pods, 2 sets",
		"cpu     400m (10%)",
		"memory  4.0Gi (25%)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Details tab lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "of the node") {
		t.Errorf("Details tab section shows a share of the node:\n%s", out)
	}
	// it sits after Resources, before the rest
	if r, d := strings.Index(out, "Resources"), strings.Index(out, "2 pods, 2 sets"); r < 0 || d < r {
		t.Errorf("DaemonSets section is not after Resources:\n%s", out)
	}
}

func TestDetailsTabNoDaemonSets(t *testing.T) {
	u := newTestUI(t)
	podsNode(u, fourCPU16Gi, nil)
	addPods(u, podSpec{name: "web", cpu: "1", mem: "1Gi"})
	if out := openDetails(t, u); !strings.Contains(out, "none on this node") {
		t.Errorf("want 'none on this node':\n%s", out)
	}
}

// A resource with no allocatable shows - for its percent.
func TestDetailsTabDaemonSetsMissingAllocatable(t *testing.T) {
	u := newTestUI(t)
	podsNode(u, v1.ResourceList{v1.ResourceCPU: resource.MustParse("4"), v1.ResourcePods: resource.MustParse("110")}, nil)
	addPods(u, podSpec{ns: "kube-system", name: "ds-x", cpu: "400m", mem: "1Gi", ownerKind: "DaemonSet", ownerName: "agent"})
	out := openDetails(t, u)
	for _, want := range []string{"1 pod, 1 set", "cpu     400m (10%)", "memory  1.0Gi (-)"} {
		if !strings.Contains(out, want) {
			t.Errorf("Details tab lacks %q:\n%s", want, out)
		}
	}
}
