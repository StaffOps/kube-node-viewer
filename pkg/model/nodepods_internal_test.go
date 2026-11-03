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

// Tests of the Pods tab of the node details popup, written against the
// acceptance criteria of specs/node-pods/requirements.md (R1-R9).

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const podsNodeName = "node-a"

// podSpec is a pod of the Pods tab tests. Empty requests are not set.
type podSpec struct {
	ns, name             string
	cpu, mem, gpu        string
	ownerKind, ownerName string
	node                 string
}

func (s podSpec) object() *v1.Pod {
	req := v1.ResourceList{}
	for res, q := range map[v1.ResourceName]string{v1.ResourceCPU: s.cpu, v1.ResourceMemory: s.mem, "nvidia.com/gpu": s.gpu} {
		if q != "" {
			req[res] = resource.MustParse(q)
		}
	}
	var refs []metav1.OwnerReference
	if s.ownerKind != "" {
		yes := true
		refs = []metav1.OwnerReference{{Kind: s.ownerKind, Name: s.ownerName, Controller: &yes}}
	}
	node := s.node
	if node == "" {
		node = podsNodeName
	}
	ns := s.ns
	if ns == "" {
		ns = "default"
	}
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: s.name, OwnerReferences: refs},
		Spec:       v1.PodSpec{NodeName: node, Containers: []v1.Container{{Name: "c", Resources: v1.ResourceRequirements{Requests: req}}}},
		Status:     v1.PodStatus{Phase: v1.PodRunning},
	}
}

// fourCPU16Gi is the allocatable of the default test node: 10% CPU is 400m,
// 25% memory is 4Gi.
var fourCPU16Gi = v1.ResourceList{
	v1.ResourceCPU: resource.MustParse("4"), v1.ResourceMemory: resource.MustParse("16Gi"), v1.ResourcePods: resource.MustParse("110"),
}

// podsNode adds node-a with the allocatable and extra labels, priced $0.40/h.
func podsNode(u *UIModel, alloc v1.ResourceList, labels map[string]string) *Node {
	l := map[string]string{v1.LabelInstanceTypeStable: "m6i.2xlarge", "karpenter.sh/capacity-type": "on-demand"}
	for k, v := range labels {
		l[k] = v
	}
	n := NewNode(&v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: podsNodeName, CreationTimestamp: metav1.Now(), Labels: l},
		Spec:       v1.NodeSpec{ProviderID: "aws:///us-east-1a/i-node-a"},
		Status:     v1.NodeStatus{Allocatable: alloc, Capacity: alloc},
	})
	u.cluster.AddNode(n).Show()
	n.SetPrice(0.40)
	return n
}

func addPods(u *UIModel, pods ...podSpec) {
	for _, p := range pods {
		u.cluster.AddPod(NewPod(p.object()))
	}
	u.Refresh()
}

// podsUI opens the details of node-a on a window of the width and moves to
// the Pods tab.
func podsUI(t *testing.T, width int, alloc v1.ResourceList, pods ...podSpec) (*UIModel, *Node) {
	t.Helper()
	u := newTestUI(t)
	n := podsNode(u, alloc, nil)
	addPods(u, pods...)
	openPods(t, u, width)
	return u, n
}

func openPods(t *testing.T, u *UIModel, width int) {
	t.Helper()
	u.Refresh()
	u.Update(tea.WindowSizeMsg{Width: width, Height: 60})
	u.View()
	press(u, "enter")
	if u.panel != panelNode {
		t.Fatalf("enter did not open the node details (panel %v)", u.panel)
	}
	press(u, "tab")
}

// box is the popup as drawn, without styles: the inner text of every line,
// title and borders included as drawn.
func box(u *UIModel) []string {
	u.View()
	p, _ := u.panelView()
	var out []string
	for _, l := range u.popup(p, 60) {
		out = append(out, ansi.Strip(l))
	}
	return out
}

// shown is the text of the popup, one string.
func shown(u *UIModel) string { return strings.Join(box(u), "\n") }

// podRowsShown are the rendered pod rows, without the box border, in order.
func podRowsShown(u *UIModel) []string {
	u.View()
	p, _ := u.panelView()
	var out []string
	for _, r := range p.rows {
		out = append(out, ansi.Strip(r))
	}
	return out
}

// rowOf is the rendered row whose POD column contains the name.
func podRowOf(t *testing.T, u *UIModel, name string) string {
	t.Helper()
	for _, r := range podRowsShown(u) {
		if strings.Contains(r, name) {
			return r
		}
	}
	t.Fatalf("no row for %q in:\n%s", name, shown(u))
	return ""
}

// cells are the last n whitespace-separated columns of a row.
func cells(row string, n int) []string {
	f := strings.Fields(row)
	if len(f) < n {
		return f
	}
	return f[len(f)-n:]
}

// names are the namespace/name (or label) of the rendered rows, in order.
func names(u *UIModel) []string {
	var out []string
	for _, r := range podRowsShown(u) {
		r = strings.TrimLeft(r, " ▸▾├└─")
		if i := strings.Index(r, "  "); i >= 0 {
			r = r[:i]
		}
		out = append(out, r)
	}
	return out
}

// cursorRow is the text of the row under the cursor, "" for none.
func cursorRow(t *testing.T, u *UIModel) string {
	t.Helper()
	u.View()
	p, _ := u.panelView()
	if p.cursor < 0 {
		return ""
	}
	if p.cursor >= len(p.rows) {
		t.Fatalf("cursor %d past %d rows", p.cursor, len(p.rows))
	}
	return ansi.Strip(p.rows[p.cursor])
}

func TestNodePodsShareAndCost(t *testing.T) {
	gi := func(f float64) string { return fmt.Sprintf("%dMi", int(f*1024)) }
	tests := []struct {
		name  string
		alloc v1.ResourceList
		pod   podSpec
		want  []string // CPU CPU% MEMORY MEM% SHARE COST/H
	}{
		{"the requirements example: 10% cpu, 25% memory", fourCPU16Gi,
			podSpec{name: "a", cpu: "400m", mem: "4Gi"}, []string{"0.40", "10%", "4.0Gi", "25%", "25%", "$0.1000"}},
		{"cpu larger than memory", fourCPU16Gi,
			podSpec{name: "a", cpu: "2", mem: gi(1)}, []string{"2.00", "50%", "1.0Gi", "6%", "50%", "$0.2000"}},
		{"memory allocatable missing: memory percent out of the share",
			v1.ResourceList{v1.ResourceCPU: resource.MustParse("4")},
			podSpec{name: "a", cpu: "1", mem: "8Gi"}, []string{"1.00", "25%", "8.0Gi", "-", "25%", "$0.1000"}},
		{"cpu allocatable zero: cpu percent out of the share",
			v1.ResourceList{v1.ResourceCPU: resource.MustParse("0"), v1.ResourceMemory: resource.MustParse("16Gi")},
			podSpec{name: "a", cpu: "3", mem: "3Gi"}, []string{"3.00", "-", "3.0Gi", "19%", "19%", "$0.0750"}},
		{"both allocatable missing: share and cost -", v1.ResourceList{},
			podSpec{name: "a", cpu: "1", mem: "1Gi"}, []string{"1.00", "-", "1.0Gi", "-", "-", "-"}},
		{"both allocatable zero: share and cost -",
			v1.ResourceList{v1.ResourceCPU: resource.MustParse("0"), v1.ResourceMemory: resource.MustParse("0")},
			podSpec{name: "a", cpu: "1", mem: "1Gi"}, []string{"1.00", "-", "1.0Gi", "-", "-", "-"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, _ := podsUI(t, 160, tt.alloc, tt.pod)
			if got := cells(podRowOf(t, u, "default/a"), 6); !slices.Equal(got, tt.want) {
				t.Errorf("cells %q, want %q", got, tt.want)
			}
		})
	}
}

// R3: on a $1000/month node, 10% cpu and 25% memory cost $250/month.
func TestNodePodsShareIsTheLargerPercent(t *testing.T) {
	r := newPodRow("k", "a",
		v1.ResourceList{v1.ResourceCPU: resource.MustParse("400m"), v1.ResourceMemory: resource.MustParse("4Gi")},
		fourCPU16Gi, podPrice{price: 1000.0 / 730, known: true})
	if !r.shareOK || math.Abs(r.share-0.25) > 1e-9 {
		t.Errorf("share %v (%v), want 0.25", r.share, r.shareOK)
	}
	if !r.costOK || math.Abs(r.cost*730-250) > 1e-6 {
		t.Errorf("monthly cost %v, want 250", r.cost*730)
	}
}

func TestNodePodsFargateShareAgainstAllocatable(t *testing.T) {
	u := newTestUI(t)
	n := podsNode(u, v1.ResourceList{v1.ResourceCPU: resource.MustParse("2"), v1.ResourceMemory: resource.MustParse("4Gi")},
		map[string]string{"eks.amazonaws.com/compute-type": "fargate"})
	addPods(u, podSpec{name: "f", cpu: "1", mem: "1Gi"})
	openPods(t, u, 160)
	alloc := n.Allocatable()
	cpu, mem := alloc[v1.ResourceCPU], alloc[v1.ResourceMemory]
	want := podPercent(max(1/cpu.AsApproximateFloat64(), float64(1<<30)/mem.AsApproximateFloat64()))
	if got := cells(podRowOf(t, u, "default/f"), 6)[4]; got != want {
		t.Errorf("fargate share %q, want %q against the node's allocatable", got, want)
	}
}

// R2: the listed pods are what the node's totals count.
func TestNodePodsListWhatTheNodeTotalsCount(t *testing.T) {
	u, n := podsUI(t, 160, fourCPU16Gi,
		podSpec{name: "web-1", cpu: "250m", mem: "512Mi"},
		podSpec{name: "web-2", cpu: "1500m"},
		podSpec{name: "cache", mem: "3Gi"},
		podSpec{name: "gpu", gpu: "1"},
		podSpec{ns: "kube-system", name: "aws-node-x", cpu: "50m", mem: "128Mi", ownerKind: "DaemonSet", ownerName: "aws-node"},
		podSpec{ns: "kube-system", name: "proxy-x", cpu: "100m", mem: "64Mi", ownerKind: "DaemonSet", ownerName: "kube-proxy"},
		podSpec{name: "elsewhere", cpu: "3", mem: "3Gi", node: "node-b"},
	)
	// the sum of the requested quantities of the listed rows is the node's
	// requested totals
	used := n.Used()
	for _, res := range []v1.ResourceName{v1.ResourceCPU, v1.ResourceMemory} {
		var sum resource.Quantity
		for _, p := range n.PodsCopy() {
			q := p.Requested()[res]
			sum.Add(q)
		}
		var rowSum resource.Quantity
		for _, r := range u.nodePodRows(n) {
			if res == v1.ResourceCPU {
				rowSum.Add(r.cpu)
			} else {
				rowSum.Add(r.mem)
			}
		}
		want := used[res]
		if sum.Cmp(want) != 0 || rowSum.Cmp(want) != 0 {
			t.Errorf("%s: pods sum %s, rows sum %s, node requested %s", res, sum.String(), rowSum.String(), want.String())
		}
	}
	got := names(u)
	for _, want := range []string{"default/web-1", "default/web-2", "default/cache", "default/gpu", "kube-system/aws-node-x ds", "kube-system/proxy-x ds"} {
		if !slices.Contains(got, want) {
			t.Errorf("rows %q lack %q", got, want)
		}
	}
	if len(got) != 6 || strings.Contains(shown(u), "elsewhere") {
		t.Errorf("rows %q: a pod of another node, or a row too many", got)
	}
}

func TestNodePodsFollowTheLiveNode(t *testing.T) {
	u, _ := podsUI(t, 160, fourCPU16Gi, podSpec{name: "first", cpu: "1"})
	addPods(u, podSpec{name: "late", cpu: "500m"})
	if !strings.Contains(shown(u), "default/late") {
		t.Errorf("a pod bound while the tab is open is not listed:\n%s", shown(u))
	}
	u.cluster.DeletePod("default", "first")
	u.Refresh()
	out := shown(u)
	if strings.Contains(out, "default/first") || !strings.Contains(out, "1 pod ·") {
		t.Errorf("a removed pod is still listed, or the node line did not follow:\n%s", out)
	}
}

// R2: the fixed node line and column header.
func TestNodePodsNodeLineAndHeader(t *testing.T) {
	u, _ := podsUI(t, 160, fourCPU16Gi,
		podSpec{name: "a", cpu: "2", mem: "4Gi"}, podSpec{name: "b", cpu: "1", mem: "4Gi"})
	lines := box(u)
	if len(lines) < 5 {
		t.Fatalf("box too short:\n%s", strings.Join(lines, "\n"))
	}
	if !strings.Contains(lines[0], "╭─ node-a") {
		t.Errorf("title %q, want the node name", lines[0])
	}
	if !strings.Contains(lines[1], "Details  [Pods]") || !strings.Contains(lines[1], "sort: cost ↓") {
		t.Errorf("tab line %q", lines[1])
	}
	for _, want := range []string{"m6i.2xlarge", "$0.4000/h", "2 pods", "cpu 3.00/4.00 75%", "mem 8.0/16.0Gi 50%"} {
		if !strings.Contains(lines[2], want) {
			t.Errorf("node line %q lacks %q", lines[2], want)
		}
	}
	if got := strings.Fields(lines[3]); !slices.Equal(got, []string{"│", "POD", "CPU", "CPU%", "MEMORY", "MEM%", "SHARE", "COST/H", "│"}) {
		t.Errorf("header %q", lines[3])
	}
}

func TestNodePodsPrices(t *testing.T) {
	tests := []struct {
		name         string
		labels       map[string]string
		price        func(n *Node)
		wantCost     string
		wantNodeLine string
	}{
		{"unknown price", nil, func(n *Node) { n.SetPrice(math.NaN()) }, "-", "m6i.2xlarge - ·"},
		{"reserved node: on-demand equivalent", map[string]string{"karpenter.sh/capacity-type": "reserved"},
			func(n *Node) { n.SetEquivalentPrice(0.40) }, "~$0.1000", "~$0.4000/h"},
		// the controller sets an override-label price with SetPrice, also on
		// a reserved node: it counts as real
		{"override label on a reserved node", map[string]string{"karpenter.sh/capacity-type": "reserved", LabelPrefix + "instance-price": "0.40"},
			func(n *Node) { n.SetPrice(0.40) }, "$0.1000", "m6i.2xlarge $0.4000/h"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := newTestUI(t)
			n := podsNode(u, fourCPU16Gi, tt.labels)
			tt.price(n)
			addPods(u, podSpec{name: "a", cpu: "400m", mem: "4Gi"})
			openPods(t, u, 160)
			c := cells(podRowOf(t, u, "default/a"), 6)
			if c[5] != tt.wantCost || c[4] != "25%" {
				t.Errorf("share %q cost %q, want 25%% %q", c[4], c[5], tt.wantCost)
			}
			if out := shown(u); !strings.Contains(out, tt.wantNodeLine) {
				t.Errorf("node line lacks %q:\n%s", tt.wantNodeLine, out)
			}
		})
	}
}

func TestNodePodsUnknownPriceCostSortIsByShare(t *testing.T) {
	u := newTestUI(t)
	n := podsNode(u, fourCPU16Gi, nil)
	n.SetPrice(math.NaN())
	addPods(u, podSpec{name: "small", cpu: "100m"}, podSpec{name: "big", mem: "8Gi"}, podSpec{name: "mid", cpu: "1"})
	openPods(t, u, 160)
	if got := names(u); !slices.Equal(got, []string{"default/big", "default/mid", "default/small"}) {
		t.Errorf("cost sort without a price: %q, want by share", got)
	}
}

func TestNodePodsPricingDisabled(t *testing.T) {
	u := newTestUI(t)
	u.DisablePricing = true
	podsNode(u, fourCPU16Gi, nil)
	addPods(u, podSpec{name: "a", cpu: "400m", mem: "4Gi"}, podSpec{name: "b", cpu: "1", mem: "1Gi"})
	openPods(t, u, 160)
	out := shown(u)
	if strings.Contains(out, "COST/H") || strings.Contains(out, "$") {
		t.Errorf("a price or the COST/H column with pricing disabled:\n%s", out)
	}
	if got := cells(podRowOf(t, u, "default/a"), 5); got[4] != "25%" {
		t.Errorf("last column %q, want the share", got)
	}
	var seen []string
	for range 5 {
		seen = append(seen, sortShown(t, u))
		press(u, "s")
	}
	if want := []string{"sort: cpu ↓", "sort: memory ↓", "sort: name ↑", "sort: cpu ↓", "sort: memory ↓"}; !slices.Equal(seen, want) {
		t.Errorf("sorts %q, want %q (default cpu, cost skipped)", seen, want)
	}
}

func sortShown(t *testing.T, u *UIModel) string {
	t.Helper()
	tabLine := box(u)[1]
	i := strings.Index(tabLine, "sort: ")
	if i < 0 {
		t.Fatalf("no sort on the tab line %q", tabLine)
	}
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(tabLine[i:]), "│"))
}

func TestNodePodsNoRequests(t *testing.T) {
	u, _ := podsUI(t, 160, fourCPU16Gi,
		podSpec{name: "mem-only", mem: "3Gi"},
		podSpec{name: "gpu-only", gpu: "1"},
		podSpec{name: "nothing"},
		podSpec{name: "zero", cpu: "0", mem: "0"},
	)
	if got := cells(podRowOf(t, u, "default/mem-only"), 6); !slices.Equal(got, []string{"-", "-", "3.0Gi", "19%", "19%", "$0.0750"}) ||
		strings.Contains(podRowOf(t, u, "default/mem-only"), "no requests") {
		t.Errorf("memory-only pod: %q", got)
	}
	for _, name := range []string{"default/gpu-only", "default/nothing", "default/zero"} {
		if r := podRowOf(t, u, name); !strings.Contains(r, "no requests") || strings.Contains(r, "%") {
			t.Errorf("%s: %q, want no requests", name, r)
		}
	}
}

// R5: pods with no requests go after every pod with requests, in every sort
// and direction, the name sort included.
func TestNodePodsNoRequestsSortLast(t *testing.T) {
	u, _ := podsUI(t, 160, fourCPU16Gi,
		podSpec{name: "aaa-gpu", gpu: "1"},
		podSpec{name: "zzz-none"},
		podSpec{name: "mmm", cpu: "1", mem: "1Gi"},
		podSpec{name: "bbb", cpu: "100m"},
		podSpec{name: "yyy", mem: "8Gi"},
		podSpec{ns: "kube-system", name: "ds-none", ownerKind: "DaemonSet", ownerName: "quiet"},
	)
	noReq := map[string]bool{"default/aaa-gpu": true, "default/zzz-none": true, "kube-system/ds-none ds": true}
	for range 4 { // cost, cpu, memory, name
		for range 2 { // both directions
			got := names(u)
			sort := sortShown(t, u)
			for i, name := range got {
				if !noReq[name] && i > 0 && noReq[got[i-1]] {
					t.Errorf("%s: %q after a pod with no requests in %q", sort, name, got)
				}
			}
			if len(got) != 6 {
				t.Errorf("%s: rows %q", sort, got)
			}
			press(u, "S")
		}
		press(u, "s")
	}
}

func TestNodePodsDaemonSets(t *testing.T) {
	ds := func(ns, pod, set, cpu, mem string) podSpec {
		return podSpec{ns: ns, name: pod, cpu: cpu, mem: mem, ownerKind: "DaemonSet", ownerName: set}
	}
	u, _ := podsUI(t, 160, fourCPU16Gi,
		podSpec{name: "app", cpu: "200m", mem: "1Gi"},
		// cpu-heavy and memory-heavy: the Total's share comes from the summed
		// requests (35%), not from adding the rows' shares (about 60%)
		ds("kube-system", "cpu-hog-1", "cpu-hog", "1100m", ""),
		ds("kube-system", "mem-hog-1", "mem-hog", "", "4Gi"),
		// two pods of one DaemonSet on the node are one row
		ds("kube-system", "twice-1", "twice", "100m", "64Mi"),
		ds("kube-system", "twice-2", "twice", "100m", "64Mi"),
		// the same name in another namespace is another DaemonSet
		ds("logging", "twice-3", "twice", "100m", "64Mi"),
		// a DaemonSet with no name recorded is marked like the others
		ds("kube-system", "nameless", "", "50m", "32Mi"),
	)
	// the all-pods view lists every pod, DaemonSet pods marked ds (R6)
	got := names(u)
	for _, want := range []string{"default/app", "kube-system/cpu-hog-1 ds", "kube-system/mem-hog-1 ds", "kube-system/twice-1 ds",
		"kube-system/twice-2 ds", "logging/twice-3 ds", "kube-system/nameless ds"} {
		if !slices.Contains(got, want) {
			t.Errorf("all-pods rows %q lack %q", got, want)
		}
	}
	if len(got) != 7 || !strings.Contains(box(u)[1], "view: all pods") {
		t.Errorf("all-pods view %q:\n%s", got, shown(u))
	}

	// d switches to the DaemonSets view: header, Total, one row per
	// namespace/name, the nameless pods under <unnamed> (R9)
	press(u, "d")
	lines := box(u)
	if !strings.Contains(lines[1], "view: daemonsets") || !strings.Contains(lines[3], "DAEMONSET") {
		t.Fatalf("DaemonSets view:\n%s", strings.Join(lines, "\n"))
	}
	total := lines[4]
	if !strings.Contains(total, "Total (6 pods, 4 sets)") {
		t.Errorf("Total line %q, want 6 pods and 4 sets", total)
	}
	// 1.45 cores of 4 is 36%; 4Gi+224Mi of 16Gi is 26%: share 36%
	if got := cells(strings.TrimSuffix(strings.TrimSpace(total), "│"), 6); !slices.Equal(got, []string{"1.45", "36%", "4.2Gi", "26%", "36%", "$0.1450"}) {
		t.Errorf("Total cells %q", got)
	}
	got = names(u)
	if len(got) != 5 {
		t.Fatalf("DaemonSet rows %q", got)
	}
	for _, want := range []string{"kube-system/cpu-hog", "kube-system/mem-hog", "kube-system/twice", "logging/twice", "<unnamed>"} {
		if !slices.Contains(got, want) {
			t.Errorf("DaemonSet rows lack %q: %q", want, got)
		}
	}
	twice := cells(podRowOf(t, u, "kube-system/twice"), 6)
	if twice[0] != "0.20" || twice[2] != "128Mi" {
		t.Errorf("a DaemonSet row does not sum its pods: %q", twice)
	}
	if mem := cells(podRowOf(t, u, "kube-system/mem-hog"), 6); mem[0] != "-" || mem[4] != "25%" {
		t.Errorf("memory-only DaemonSet row %q", mem)
	}
	press(u, "d")
	if len(names(u)) != 7 {
		t.Errorf("d did not switch back to all pods: %q", names(u))
	}
}

func TestNodePodsNoDaemonSetRow(t *testing.T) {
	u, _ := podsUI(t, 160, fourCPU16Gi,
		podSpec{name: "a", cpu: "1"},
		podSpec{ns: "kube-system", name: "static", cpu: "100m", ownerKind: "Node", ownerName: podsNodeName})
	if out := shown(u); strings.Contains(out, "DaemonSets") || !strings.Contains(out, "kube-system/static") ||
		strings.Contains(podRowOf(t, u, "kube-system/static"), " ds") {
		t.Errorf("a DaemonSets row with no DaemonSet pods, or the static pod missing or marked ds:\n%s", out)
	}
	press(u, "d")
	if out := shown(u); !strings.Contains(out, "no DaemonSets") || cursorRow(t, u) != "" {
		t.Errorf("the DaemonSets view with no DaemonSet pods:\n%s", out)
	}
}

// R7: cycle, directions, flip and the sort shown on the tab line.
func TestNodePodsSortCycle(t *testing.T) {
	u, _ := podsUI(t, 160, fourCPU16Gi,
		podSpec{name: "c-cpu", cpu: "2", mem: "1Gi"},     // share 50%
		podSpec{name: "a-mem", cpu: "100m", mem: "12Gi"}, // share 75%
		podSpec{name: "b-small", cpu: "200m", mem: "2Gi"},
	)
	steps := []struct {
		key, sort string
		order     []string
	}{
		{"", "sort: cost ↓", []string{"default/a-mem", "default/c-cpu", "default/b-small"}},
		{"S", "sort: cost ↑", []string{"default/b-small", "default/c-cpu", "default/a-mem"}},
		{"s", "sort: cpu ↓", []string{"default/c-cpu", "default/b-small", "default/a-mem"}},
		{"S", "sort: cpu ↑", []string{"default/a-mem", "default/b-small", "default/c-cpu"}},
		{"s", "sort: memory ↓", []string{"default/a-mem", "default/b-small", "default/c-cpu"}},
		{"s", "sort: name ↑", []string{"default/a-mem", "default/b-small", "default/c-cpu"}},
		{"S", "sort: name ↓", []string{"default/c-cpu", "default/b-small", "default/a-mem"}},
		{"s", "sort: cost ↓", []string{"default/a-mem", "default/c-cpu", "default/b-small"}},
	}
	for _, st := range steps {
		if st.key != "" {
			press(u, st.key)
		}
		if got := sortShown(t, u); got != st.sort {
			t.Errorf("after %q: %q, want %q", st.key, got, st.sort)
		}
		if got := names(u); !slices.Equal(got, st.order) {
			t.Errorf("%s: %q, want %q", st.sort, got, st.order)
		}
	}
}

func TestNodePodsSortTiesOnName(t *testing.T) {
	u, _ := podsUI(t, 160, fourCPU16Gi,
		podSpec{ns: "b", name: "x", cpu: "1", mem: "1Gi"},
		podSpec{ns: "a", name: "y", cpu: "1", mem: "1Gi"},
		podSpec{ns: "a", name: "x", cpu: "1", mem: "1Gi"},
	)
	want := []string{"a/x", "a/y", "b/x"}
	for range 3 { // cost, cpu, memory
		for range 2 {
			if got := names(u); !slices.Equal(got, want) {
				t.Errorf("%s: ties %q, want %q", sortShown(t, u), got, want)
			}
			press(u, "S")
		}
		press(u, "s")
	}
}

// R7, R9: DaemonSet pods sort among the pods; the DaemonSet rows follow the
// same sort under a Total line that does not move.
func TestNodePodsDaemonSetRowsSort(t *testing.T) {
	ds := func(pod, set, cpu string) podSpec {
		return podSpec{ns: "kube-system", name: pod, cpu: cpu, ownerKind: "DaemonSet", ownerName: set}
	}
	u, _ := podsUI(t, 160, fourCPU16Gi,
		podSpec{name: "big", cpu: "2"},
		podSpec{name: "small", cpu: "100m"},
		ds("a-1", "a", "50m"), ds("b-1", "b", "250m"), ds("c-1", "c", "300m"),
	)
	press(u, "s") // cpu ↓
	if got := names(u); !slices.Equal(got, []string{"default/big", "kube-system/c-1 ds", "kube-system/b-1 ds", "default/small", "kube-system/a-1 ds"}) {
		t.Errorf("cpu ↓: %q, want DaemonSet pods among the pods", got)
	}
	press(u, "d")
	if got := names(u); !slices.Equal(got, []string{"kube-system/c", "kube-system/b", "kube-system/a"}) {
		t.Errorf("cpu ↓ DaemonSets: %q", got)
	}
	totalBefore := box(u)[4]
	press(u, "down") // on kube-system/b
	press(u, "S")
	if got := names(u); !slices.Equal(got, []string{"kube-system/a", "kube-system/b", "kube-system/c"}) {
		t.Errorf("cpu ↑ DaemonSets: %q", got)
	}
	if got := cursorRow(t, u); !strings.Contains(got, "kube-system/b") {
		t.Errorf("the cursor did not stay on its DaemonSet: %q", got)
	}
	if box(u)[4] != totalBefore || !strings.Contains(totalBefore, "Total (3 pods, 3 sets)") {
		t.Errorf("Total line moved or wrong: %q → %q", totalBefore, box(u)[4])
	}
}

func manyPods(n int) []podSpec {
	var pods []podSpec
	for i := range n {
		pods = append(pods, podSpec{name: fmt.Sprintf("pod-%02d", i), cpu: fmt.Sprintf("%dm", 10+i)})
	}
	return pods
}

// R8: the cursor keys.
func TestNodePodsKeys(t *testing.T) {
	u, _ := podsUI(t, 160, fourCPU16Gi, append(manyPods(30),
		podSpec{ns: "kube-system", name: "ds-1", cpu: "1", ownerKind: "DaemonSet", ownerName: "ds"})...)
	press(u, "s", "s", "s") // name ↑: default/pod-00… then kube-system/ds-1
	rows := names(u)
	at := func(want int) {
		t.Helper()
		if got := cursorRow(t, u); !strings.Contains(got, rows[want]) {
			t.Errorf("cursor on %q, want %q", got, rows[want])
		}
	}
	at(0)
	press(u, "down")
	at(1)
	press(u, "j", "j")
	at(3)
	press(u, "k")
	at(2)
	press(u, "up", "up", "up")
	at(0)
	press(u, "pgdown")
	if i := slices.IndexFunc(rows, func(r string) bool { return strings.Contains(cursorRow(t, u), r) }); i < 2 {
		t.Errorf("pgdown moved to row %d", i)
	}
	press(u, "pgdown", "pgdown", "pgdown", "pgdown")
	at(len(rows) - 1)
	press(u, "pgup")
	if strings.Contains(cursorRow(t, u), rows[len(rows)-1]) || strings.Contains(cursorRow(t, u), rows[len(rows)-2]) {
		t.Errorf("pgup moved one row or none")
	}
	press(u, "home")
	at(0)

	// enter and space on a pod row do nothing and do not close the popup
	press(u, "down")
	before := names(u)
	press(u, "enter", " ")
	if u.panel != panelNode || u.detailsTab != detailsTabPods || !slices.Equal(names(u), before) {
		t.Errorf("enter/space on a pod row: panel %v tab %v rows %q", u.panel, u.detailsTab, names(u))
	}
	// nor on a DaemonSet pod, nor in the DaemonSets view
	press(u, "home", "up")
	for range 31 {
		press(u, "down")
	}
	at(len(rows) - 1)
	press(u, "enter", " ")
	if !slices.Equal(names(u), before) || u.panel != panelNode {
		t.Errorf("enter/space on a DaemonSet pod: %q", names(u))
	}
	// d puts the cursor on the first row of the other view
	press(u, "d")
	if got := cursorRow(t, u); !strings.Contains(got, "kube-system/ds") {
		t.Errorf("cursor %q after d, want the first DaemonSet row", got)
	}
	press(u, "enter", " ", "pgdown", "home")
	if got := names(u); !slices.Equal(got, []string{"kube-system/ds"}) || u.panel != panelNode {
		t.Errorf("keys in the DaemonSets view: %q", got)
	}
	press(u, "d")
	at(0)
	press(u, "esc")
	if u.panel != panelNone {
		t.Errorf("esc did not close the popup")
	}
}

// R8: the cursor never lands on the tab line, the node line or the header.
func TestNodePodsCursorNeverOnFixedLines(t *testing.T) {
	u, _ := podsUI(t, 160, fourCPU16Gi, manyPods(3)...)
	fixed := box(u)[1:4]
	for _, keys := range [][]string{{"up"}, {"k", "k", "k"}, {"home"}, {"pgup"}, {"down", "up", "up"}} {
		press(u, keys...)
		got := cursorRow(t, u)
		if !strings.HasPrefix(strings.TrimSpace(got), "default/pod-") {
			t.Errorf("after %q the cursor is on %q", keys, got)
		}
		for _, f := range fixed {
			if strings.Contains(f, strings.TrimSpace(got)) {
				t.Errorf("cursor row %q is a fixed line", got)
			}
		}
	}
}

// R8 and R2: only the pod rows scroll, and the cursor stays visible.
func TestNodePodsScrollKeepsFixedLinesAndCursor(t *testing.T) {
	u, _ := podsUI(t, 160, fourCPU16Gi, manyPods(40)...)
	u.Update(tea.WindowSizeMsg{Width: 160, Height: 20})
	press(u, "s", "s", "s") // name ↑
	for range 35 {
		press(u, "down")
	}
	out := ansi.Strip(u.View())
	for _, want := range []string{"Details  [Pods]", "m6i.2xlarge", "POD", "default/pod-35"} {
		if !strings.Contains(out, want) {
			t.Errorf("a short window lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "default/pod-00") {
		t.Errorf("the rows did not scroll:\n%s", out)
	}
}

func TestNodePodsCursorFallback(t *testing.T) {
	t.Run("pod deleted: nearest remaining row", func(t *testing.T) {
		u, _ := podsUI(t, 160, fourCPU16Gi, manyPods(5)...)
		press(u, "s", "s", "s", "down", "down") // name ↑, on pod-02
		u.cluster.DeletePod("default", "pod-02")
		u.Refresh()
		if got := cursorRow(t, u); !strings.Contains(got, "pod-01") && !strings.Contains(got, "pod-03") {
			t.Errorf("cursor on %q, want pod-01 or pod-03", got)
		}
	})
	t.Run("last pod deleted: the new last row", func(t *testing.T) {
		u, _ := podsUI(t, 160, fourCPU16Gi, manyPods(3)...)
		press(u, "s", "s", "s", "down", "down") // on pod-02
		u.cluster.DeletePod("default", "pod-02")
		u.Refresh()
		if got := cursorRow(t, u); !strings.Contains(got, "pod-01") {
			t.Errorf("cursor on %q, want pod-01", got)
		}
		press(u, "up")
		if got := cursorRow(t, u); !strings.Contains(got, "pod-00") {
			t.Errorf("up after the fallback: %q", got)
		}
	})
	t.Run("DaemonSet row gone in the DaemonSets view: nearest remaining row", func(t *testing.T) {
		u, _ := podsUI(t, 160, fourCPU16Gi,
			podSpec{name: "app", cpu: "3"},
			podSpec{ns: "kube-system", name: "a-1", cpu: "10m", ownerKind: "DaemonSet", ownerName: "a"},
			podSpec{ns: "kube-system", name: "b-1", cpu: "20m", ownerKind: "DaemonSet", ownerName: "b"})
		press(u, "d", "down") // cost ↓: b, a; on kube-system/a
		if got := cursorRow(t, u); !strings.Contains(got, "kube-system/a") {
			t.Fatalf("cursor on %q, want kube-system/a", got)
		}
		u.cluster.DeletePod("kube-system", "a-1")
		u.Refresh()
		if got := cursorRow(t, u); !strings.Contains(got, "kube-system/b") {
			t.Errorf("after its DaemonSet went the cursor is on %q, rows %q", got, names(u))
		}
	})
	t.Run("last DaemonSet pod deleted in the DaemonSets view", func(t *testing.T) {
		u, _ := podsUI(t, 160, fourCPU16Gi,
			podSpec{name: "app", cpu: "3"},
			podSpec{ns: "kube-system", name: "a-1", cpu: "10m", ownerKind: "DaemonSet", ownerName: "a"})
		press(u, "d") // on kube-system/a
		u.cluster.DeletePod("kube-system", "a-1")
		u.Refresh()
		if got := cursorRow(t, u); got != "" || u.panel != panelNode || !strings.Contains(shown(u), "no DaemonSets") {
			t.Errorf("cursor %q panel %v after the last DaemonSet pod went:\n%s", got, u.panel, shown(u))
		}
	})
}

func TestNodePodsNoPods(t *testing.T) {
	u, _ := podsUI(t, 160, fourCPU16Gi)
	lines := box(u)
	if !strings.Contains(lines[3], "POD") || !strings.Contains(lines[4], "no pods") {
		t.Errorf("no pods not under the header:\n%s", strings.Join(lines, "\n"))
	}
	before := shown(u)
	press(u, "down", "up", "j", "k", "pgdown", "pgup", "home", "enter", " ")
	if u.panel != panelNode || shown(u) != before || cursorRow(t, u) != "" {
		t.Errorf("cursor keys did something with no pods:\n%s", shown(u))
	}
}

func TestNodePodsNodeGone(t *testing.T) {
	u, n := podsUI(t, 160, fourCPU16Gi, podSpec{name: "a", cpu: "1"})
	u.cluster.DeleteNode(n.ProviderID())
	u.Refresh()
	press(u, "down", "enter", "s", "S", " ")
	if out := shown(u); !strings.Contains(out, "no longer in the cluster") || u.panel != panelNode {
		t.Errorf("a node that went away from the Pods tab:\n%s", out)
	}
}

func TestNodePodsResetOnReopen(t *testing.T) {
	u, _ := podsUI(t, 160, fourCPU16Gi, append(manyPods(3),
		podSpec{ns: "kube-system", name: "ds-1", cpu: "1", ownerKind: "DaemonSet", ownerName: "ds"})...)
	first := cursorRow(t, u)
	press(u, "s", "S", "d", "home", "down", "down")
	press(u, "esc", "enter")
	if u.detailsTab != detailsTabDetails || !strings.Contains(shown(u), "[Details]  Pods") {
		t.Errorf("reopened on tab %v:\n%s", u.detailsTab, shown(u))
	}
	press(u, "tab")
	if got := sortShown(t, u); got != "sort: cost ↓" {
		t.Errorf("sort %q after reopening", got)
	}
	if !strings.Contains(box(u)[1], "view: all pods") {
		t.Errorf("the DaemonSets view is still on")
	}
	if got := cursorRow(t, u); got != first {
		t.Errorf("cursor on %q after reopening, want the first row %q", got, first)
	}
}

func TestNodePodsReopenDefaultWithPricingDisabled(t *testing.T) {
	u := newTestUI(t)
	u.DisablePricing = true
	podsNode(u, fourCPU16Gi, nil)
	addPods(u, podSpec{name: "a", cpu: "1"})
	openPods(t, u, 160)
	press(u, "s", "esc", "enter", "tab")
	if got := sortShown(t, u); got != "sort: cpu ↓" {
		t.Errorf("reopened sort %q, want cpu", got)
	}
}

// R2: narrow windows cut the name, then drop the percents; SHARE and COST/H
// stay. No line runs past the window.
func TestNodePodsNarrowWindows(t *testing.T) {
	long := "a-very-long-deployment-name-that-will-never-fit-7d9c8b6f5-x2kq"
	tests := []struct {
		width    int
		percents bool
	}{
		{80, true},
		{56, false},
		// the narrowest window that holds a 12-column name and every column
		// but the percents: 6 for the box, 2 marker, 12 name, 22, 10 cost
		{52, false},
		{40, false},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("width=%d", tt.width), func(t *testing.T) {
			u, _ := podsUI(t, tt.width, fourCPU16Gi, podSpec{name: long, cpu: "400m", mem: "4Gi"})
			lines := box(u)
			for _, l := range lines {
				if w := textWidth(l); w > tt.width {
					t.Errorf("line of %d columns on a %d-column window: %q", w, tt.width, l)
				}
			}
			header := lines[3]
			for _, col := range []string{"SHARE", "COST/H"} {
				if !strings.Contains(header, col) {
					t.Errorf("header %q lacks %s", header, col)
				}
			}
			if has := strings.Contains(header, "CPU%"); has != tt.percents {
				t.Errorf("header %q: percent columns %v, want %v", header, has, tt.percents)
			}
			row := podRowOf(t, u, "default/a-v")
			if strings.Contains(row, long) || !strings.Contains(row, "…") {
				t.Errorf("name not cut with …: %q", row)
			}
			name := strings.TrimSpace(row)
			name = name[:strings.Index(name, "…")+len("…")]
			if w := textWidth(name); w < podsMinPodW {
				t.Errorf("name cut to %d columns (%q), want at least %d", w, name, podsMinPodW)
			}
			if !strings.Contains(row, "25%") || !strings.Contains(row, "$0.1000") {
				t.Errorf("share or cost missing on the row %q", row)
			}
		})
	}
}

// Requests of any size fit their columns: CPU in 5 characters, MEMORY in 6.
func TestNodePodsValuesFitTheirColumns(t *testing.T) {
	big := v1.ResourceList{v1.ResourceCPU: resource.MustParse("2000"), v1.ResourceMemory: resource.MustParse("4Ti"), v1.ResourcePods: resource.MustParse("110")}
	tests := []struct {
		cpu, mem         string
		wantCPU, wantMem string
	}{
		{"1500", "300Gi", "1500", "300Gi"},
		{"128", "1536Mi", "128.0", "1.5Gi"},
		{"12", "100Gi", "12.00", "100Gi"},
		{"5m", "999Mi", "0.01", "999Mi"},
		{"1m", "512Ki", "0.00", "512Ki"},
	}
	for _, tt := range tests {
		t.Run(tt.cpu+"/"+tt.mem, func(t *testing.T) {
			u, _ := podsUI(t, 160, big, podSpec{name: "p", cpu: tt.cpu, mem: tt.mem})
			c := cells(podRowOf(t, u, "default/p"), 6)
			if c[0] != tt.wantCPU || c[2] != tt.wantMem {
				t.Errorf("cpu %q memory %q, want %q %q", c[0], c[2], tt.wantCPU, tt.wantMem)
			}
			if textWidth(c[0]) > podsCPUW || textWidth(c[2]) > podsMemW {
				t.Errorf("cells %q wider than their columns", c)
			}
		})
	}
}
