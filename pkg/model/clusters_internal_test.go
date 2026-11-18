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
	"errors"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Key-driven tests of the Clusters screen against a fake fleet.

// mnode is a made-up member node.
type mnode struct {
	name     string
	provider string
	price    float64 // 0: unknown
	pods     int
	group    string
}

func awsID(name string) string { return "aws:///us-east-1a/i-" + name }
func azID(name string) string {
	return "azure:///subscriptions/s/resourceGroups/example-rg/providers/Microsoft.Compute/virtualMachines/" + name
}
func otherID(name string) string { return "kind://docker/example/" + name }

// memberCluster is a member's store with these nodes, each with its pods
// requesting 100m of CPU.
func memberCluster(nodes ...mnode) *Cluster {
	c := NewCluster()
	for _, d := range nodes {
		group := d.group
		if group == "" {
			group = "pool"
		}
		n := NewNode(&v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:              d.name,
				CreationTimestamp: metav1.NewTime(time.Now().Add(-time.Hour)),
				Labels: map[string]string{
					"eks.amazonaws.com/nodegroup": group,
					v1.LabelTopologyZone:          "zone-1",
				},
			},
			Spec: v1.NodeSpec{ProviderID: d.provider},
			Status: v1.NodeStatus{
				Allocatable: v1.ResourceList{v1.ResourceCPU: resource.MustParse("2"), v1.ResourceMemory: resource.MustParse("4Gi"), v1.ResourcePods: resource.MustParse("20")},
				Conditions:  []v1.NodeCondition{{Type: v1.NodeReady, Status: v1.ConditionTrue}},
			},
		})
		c.AddNode(n).Show()
		if d.price > 0 {
			n.SetPrice(d.price)
		} else {
			n.SetPrice(math.NaN())
		}
		for p := 0; p < d.pods; p++ {
			c.AddPod(NewPod(&v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: fmt.Sprintf("%s-pod-%d", d.name, p)},
				Spec: v1.PodSpec{NodeName: d.name, Containers: []v1.Container{{Name: "app", Resources: v1.ResourceRequirements{Requests: v1.ResourceList{
					v1.ResourceCPU: resource.MustParse("100m"),
				}}}}},
				Status: v1.PodStatus{Phase: v1.PodRunning},
			}))
		}
	}
	return c
}

// fleetEnv is the made-up EKS cluster on screen (demo-prod) with a fake fleet.
type fleetEnv struct {
	u     *UIModel
	f     *fakeFleet
	store *fakeFleetStore
}

// newFleetEnv builds the UI with these extra kubeconfig contexts; chosen
// non-nil saves that choice, nil leaves none saved (first use).
func newFleetEnv(t *testing.T, height int, extra []ContextInfo, chosen []string) fleetEnv {
	u := demoUI(t)
	infos := append([]ContextInfo{{Name: "demo-prod", Server: u.info.Server, Cloud: "aws"}}, extra...)
	f := &fakeFleet{infos: infos}
	store := &fakeFleetStore{}
	if chosen != nil {
		store.contexts, store.ok = chosen, true
	}
	u.SetFleet(f, store)
	u.Update(tea.WindowSizeMsg{Width: 168, Height: height})
	return fleetEnv{u: u, f: f, store: store}
}

// command types a : command and runs it.
func command(u *UIModel, text string) {
	pressAll(u, ":")
	for _, r := range text {
		pressAll(u, string(r))
	}
	pressAll(u, "enter")
}

// connect tells the UI that a member connected; st is read on each snapshot.
func connect(u *UIModel, name string, c *Cluster, st *ConnStatus) {
	run(u, MemberReadyMsg{Context: name, Gen: c.Generation(), Cluster: c, Status: func() ConnStatus { return *st }})
}

// changed tells the UI that a member's store changed, and runs the snapshot.
func changed(u *UIModel, c *Cluster) {
	_, cmd := u.Update(MembersChangedMsg{Gens: []uint64{c.Generation()}})
	runAll(u, cmd)
}

func fscreen(u *UIModel) string {
	u.dirty = true
	return screenText(u.View())
}

// lineWith is the first line of out containing sub, "" when none does.
func lineWith(out, sub string) string {
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, sub) {
			return l
		}
	}
	return ""
}

// unfold puts the cursor on a section heading and presses space.
func unfold(u *UIModel, name string) {
	u.View()
	u.cursorKey, u.cursorDelta = "c:"+name, 0
	press(u, " ")
}

func cloudLine(out, cloud string) string {
	re := regexp.MustCompile(`(?m)^\s+` + regexp.QuoteMeta(cloud) + `\s+\d+ clusters?\s.*$`)
	return re.FindString(out)
}

// clusters-screen R1
func TestClustersCommandAndTab(t *testing.T) {
	e := newFleetEnv(t, 40, nil, []string{"demo-prod"})
	u := e.u
	for _, cmd := range []string{"clusters", "cl"} {
		command(u, cmd)
		if !u.onClusters() {
			t.Fatalf(":%s did not open Clusters", cmd)
		}
		if out := fscreen(u); !strings.Contains(out, "kube-node-viewer · Clusters") {
			t.Errorf(":%s header does not name Clusters:\n%s", cmd, out)
		}
		pressAll(u, "tab")
		if u.view.Screen != ScreenOverview {
			t.Errorf("tab from Clusters went to %q, want Overview", u.view.Screen)
		}
	}
	pressAll(u, "tab")
	if !u.onRecommendations() {
		t.Error("tab on Overview did not reach Recommendations")
	}
	pressAll(u, "tab")
	if u.view.Screen != ScreenOverview {
		t.Error("tab on Recommendations did not go back to Overview")
	}
	command(u, "cl")
	command(u, "overview")
	if u.view.Screen != ScreenOverview {
		t.Error(":overview did not leave Clusters")
	}
	command(u, "cl")
	command(u, "recommendations")
	if !u.onRecommendations() {
		t.Error(":recommendations did not leave Clusters")
	}
	command(u, "nope")
	if n := u.currentNotice(); !strings.Contains(n, ":clusters") {
		t.Errorf("unknown command notice does not name :clusters: %q", n)
	}
}

// clusters-screen R1
func TestClustersScreenSavedAndUnknownScreen(t *testing.T) {
	e := newFleetEnv(t, 40, nil, []string{"demo-prod"})
	var saved []ViewState
	e.u.OnViewChange(func(v ViewState) { saved = append(saved, v) })
	command(e.u, "cl")
	if len(saved) == 0 || saved[len(saved)-1].Screen != ScreenClusters {
		t.Error("the Clusters screen is not saved with the view")
	}
	v := e.u.view
	v.Screen = "nonsense"
	e.u.SetView(v)
	if e.u.view.Screen != ScreenOverview {
		t.Errorf("unknown saved screen opened %q, want Overview", e.u.view.Screen)
	}
}

// clusters-screen R1
func TestClustersHeaderSizes(t *testing.T) {
	e := newFleetEnv(t, 40, nil, []string{"demo-prod"})
	u := e.u
	command(u, "cl")
	u.View()

	full := screenText(u.clustersHeader())
	if u.headerSize() != headerFull {
		t.Fatalf("header size %v, want full", u.headerSize())
	}
	for _, want := range []string{"Scope:", "Clusters:", "Nodes:", "Cost:", "$728.102/month"} {
		if !strings.Contains(full, want) {
			t.Errorf("full header misses %q:\n%s", want, full)
		}
	}
	for _, not := range []string{"Context:", "Cluster:", "Region:", "pods"} {
		if strings.Contains(full, not) {
			t.Errorf("full header shows %q:\n%s", not, full)
		}
	}

	pressAll(u, "ctrl+e")
	if u.headerSize() != headerCompact {
		t.Fatalf("ctrl+e: header size %v, want compact", u.headerSize())
	}
	compact := screenText(u.clustersHeader())
	if !strings.Contains(compact, "Clusters · 1 cluster · 10 nodes · $728.102/month") || strings.Contains(compact, "pods") || strings.Contains(compact, "https://") {
		t.Errorf("compact header:\n%s", compact)
	}

	u.Update(tea.WindowSizeMsg{Width: 168, Height: 12})
	if u.headerSize() != headerLine {
		t.Fatalf("header size %v, want line", u.headerSize())
	}
	line := screenText(u.clustersHeader())
	if !strings.HasPrefix(strings.TrimSpace(line), "Clusters · 1 cluster · 10 nodes · $728.102/month") || strings.Contains(line, "pods") {
		t.Errorf("one-line header: %q", line)
	}
}

// clusters-screen R2, R8
func TestClustersFirstUseWantsNothingUntilEnter(t *testing.T) {
	e := newFleetEnv(t, 40, []ContextInfo{
		{Name: "other-aws", Server: "https://example-other.example.com", Cloud: "aws"},
		{Name: "aks", Server: "https://example-aks.example.com", Cloud: "azure"},
	}, nil)
	u := e.u
	command(u, "cl")
	if u.panel != panelClusters {
		t.Fatal("first use did not open the cluster picker")
	}
	for i, w := range e.f.wanted {
		if len(w) != 0 {
			t.Fatalf("SetWanted #%d before enter: %v", i, w)
		}
	}
	pressAll(u, "enter")
	if got := e.f.last(); !slices.Equal(got, []string{"other-aws"}) {
		t.Errorf("wanted after enter %v, want [other-aws] (not the context on screen, not the other cloud)", got)
	}
	if e.store.saves != 1 || !slices.Equal(e.store.contexts, []string{"demo-prod", "other-aws"}) {
		t.Errorf("saved %d times: %v", e.store.saves, e.store.contexts)
	}

	n := len(e.f.wanted)
	pressAll(u, "tab")
	if len(e.f.wanted) != n+1 || e.f.last() != nil {
		t.Errorf("leaving did not call SetWanted(nil): %v", e.f.wanted)
	}
}

// clusters-screen R2
func TestClustersConnectingUntilSynced(t *testing.T) {
	e := newFleetEnv(t, 40, []ContextInfo{{Name: "other-aws", Server: "https://example-other.example.com", Cloud: "aws"}}, []string{"demo-prod", "other-aws"})
	u := e.u
	command(u, "cl")
	if !slices.Equal(e.f.last(), []string{"other-aws"}) {
		t.Fatalf("wanted %v", e.f.last())
	}
	if l := lineWith(fscreen(u), "▸ other-aws"); !strings.Contains(l, "connecting…") {
		t.Errorf("before the first answer: %q", l)
	}
	c := memberCluster(mnode{name: "mem-a", provider: awsID("mem-a"), price: 0.1})
	st := &ConnStatus{}
	connect(u, "other-aws", c, st)
	if l := lineWith(fscreen(u), "▸ other-aws"); !strings.Contains(l, "connecting…") {
		t.Errorf("connected but nodes not synced: %q", l)
	}
	st.Synced = true
	changed(u, c)
	if l := lineWith(fscreen(u), "▸ other-aws"); strings.Contains(l, "connecting…") || !strings.Contains(l, "ok") || !strings.Contains(l, "1 nodes") {
		t.Errorf("after the node watch synced: %q", l)
	}
}

// clusters-screen R2, R8
func TestClustersMissingContext(t *testing.T) {
	e := newFleetEnv(t, 40, []ContextInfo{{Name: "other-aws", Server: "https://example-other.example.com", Cloud: "aws"}}, []string{"demo-prod", "gone", "other-aws"})
	u := e.u
	command(u, "cl")
	if slices.Contains(e.f.last(), "gone") {
		t.Errorf("a context missing from the kubeconfig is wanted: %v", e.f.last())
	}
	pressAll(u, "c")
	if l := lineWith(fscreen(u), "gone"); !strings.Contains(l, "[x] gone") || !strings.Contains(l, "missing") {
		t.Errorf("picker line for a missing context: %q", l)
	}
}

// clusters-screen R3, R15
func TestClustersPodFiguresPendingUntilPodsSynced(t *testing.T) {
	e := newFleetEnv(t, 50, []ContextInfo{{Name: "mem", Server: "https://example-mem.example.com", Cloud: "aws"}}, []string{"demo-prod", "mem"})
	u := e.u
	v := u.view
	v.SetGroupBy(FieldNodeGroup)
	u.SetView(v)
	command(u, "cl")
	c := memberCluster(
		mnode{name: "mem-a", provider: awsID("mem-a"), price: 0.5, pods: 4, group: "workers"},
		mnode{name: "mem-b", provider: awsID("mem-b"), price: 0.5, pods: 2, group: "workers"},
	)
	st := &ConnStatus{Synced: true}
	connect(u, "mem", c, st)

	out := fscreen(u)
	head := lineWith(out, "▸ mem ")
	if !strings.Contains(head, "idle …") || strings.Contains(head, "idle $") {
		t.Errorf("idle before the pods: %q", head)
	}
	if !strings.Contains(head, "/month") {
		t.Errorf("no cost from the nodes: %q", head)
	}

	unfold(u, "mem")
	if !slices.Equal(e.f.pods, []string{"mem"}) {
		t.Errorf("WatchPods calls %v", e.f.pods)
	}
	out = fscreen(u)
	group := lineWith(out, "workers ")
	if !strings.Contains(group, "… pods") || !strings.Contains(group, "idle …") || !strings.Contains(group, "cpu …") {
		t.Errorf("group line before the pods: %q\n%s", group, out)
	}
	if row := lineWith(out, "mem-a"); !strings.Contains(row, "…") {
		t.Errorf("node row before the pods: %q", row)
	}

	st.PodsSynced = true
	changed(u, c)
	out = fscreen(u)
	head = lineWith(out, "▾ mem ")
	if strings.Contains(head, "idle …") || !strings.Contains(head, "idle $") {
		t.Errorf("idle after the pods: %q", head)
	}
	group = lineWith(out, "workers ")
	if !strings.Contains(group, "6 pods") || strings.Contains(group, "idle …") {
		t.Errorf("group line after the pods: %q", group)
	}
}

// clusters-screen R15
func TestClustersPodSortsWaitForPods(t *testing.T) {
	e := newFleetEnv(t, 50, []ContextInfo{{Name: "mem", Server: "https://example-mem.example.com", Cloud: "aws"}}, []string{"demo-prod", "mem"})
	u := e.u
	command(u, "cl")
	c := memberCluster(
		mnode{name: "mem-a", provider: awsID("mem-a"), price: 0.5, pods: 0},
		mnode{name: "mem-b", provider: awsID("mem-b"), price: 0.5, pods: 5},
		mnode{name: "mem-c", provider: awsID("mem-c"), price: 0.5, pods: 2},
	)
	st := &ConnStatus{Synced: true}
	connect(u, "mem", c, st)
	unfold(u, "mem")

	order := func() []string {
		out := fscreen(u)
		names := []string{"mem-a", "mem-b", "mem-c"}
		slices.SortFunc(names, func(a, b string) int { return strings.Index(out, a) - strings.Index(out, b) })
		return names
	}
	sorts := []struct {
		field  Field
		synced []string
	}{
		{FieldPods, []string{"mem-b", "mem-c", "mem-a"}},
		{FieldIdle, []string{"mem-a", "mem-c", "mem-b"}},
		{UsageField(v1.ResourceCPU), []string{"mem-b", "mem-c", "mem-a"}},
		{LabelField("kube-node-viewer/node-cpu-usage"), []string{"mem-b", "mem-c", "mem-a"}},
	}
	for _, s := range sorts {
		st.PodsSynced = false
		changed(u, c)
		v := u.view
		v.Sort = SortSpec{Field: s.field, Descending: true}
		u.setView(v)
		if got := order(); !slices.Equal(got, []string{"mem-a", "mem-b", "mem-c"}) {
			t.Errorf("sort by %s before the pods: %v, want name order", s.field, got)
		}
		st.PodsSynced = true
		changed(u, c)
		if got := order(); !slices.Equal(got, s.synced) {
			t.Errorf("sort by %s after the pods: %v, want %v", s.field, got, s.synced)
		}
	}
}

// clusters-screen R15
func TestClustersUsageLabelFilterBeforePods(t *testing.T) {
	e := newFleetEnv(t, 50, []ContextInfo{{Name: "az", Server: "https://example-az.example.com", Cloud: "azure"}}, []string{"demo-prod", "az"})
	u := e.u
	command(u, "cl")
	c := memberCluster(mnode{name: "az-a", provider: azID("az-a"), price: 0.2, pods: 1})
	connect(u, "az", c, &ConnStatus{Synced: true})
	v := u.view
	v.Filters = []Filter{{Field: LabelField("kube-node-viewer/node-cpu-usage"), Values: []string{"5"}, Enabled: true}}
	u.setView(v)
	out := fscreen(u)
	head := lineWith(out, "▸ az ")
	if !strings.Contains(head, "0 nodes") || !strings.Contains(head, "pods not loaded") {
		t.Errorf("section with a usage-label filter before the pods: %q", head)
	}
	if l := cloudLine(out, "azure"); !strings.Contains(l, "1 cluster pods not loaded") {
		t.Errorf("azure total: %q\n%s", l, out)
	}
	if l := cloudLine(out, "aws"); strings.Contains(l, "pods not loaded") {
		t.Errorf("aws total (pods synced) says pods not loaded: %q", l)
	}
}

// clusters-screen R4, R12
func TestClustersCloudTotals(t *testing.T) {
	e := newFleetEnv(t, 60, []ContextInfo{
		{Name: "az", Server: "https://example-az.example.com", Cloud: "azure"},
		{Name: "cheap", Server: "https://example-cheap.example.com", Cloud: "aws"},
		{Name: "aaa-free", Server: "https://example-free.example.com", Cloud: "aws"},
		{Name: "pending-aws", Server: "https://example-pending.example.com", Cloud: "aws"},
		{Name: "mix", Server: "https://example-mix.example.com", Cloud: "aws"},
		{Name: "odd", Server: "https://example-odd.example.com"},
		{Name: "nowhere", Server: "https://example-nowhere.example.com"},
	}, []string{"demo-prod", "az", "cheap", "aaa-free", "pending-aws", "mix", "odd", "nowhere"})
	u := e.u
	command(u, "cl")
	synced := &ConnStatus{Synced: true, PodsSynced: true}
	connect(u, "az", memberCluster(mnode{name: "az-a", provider: azID("az-a"), price: 5}, mnode{name: "az-b", provider: azID("az-b"), price: 5}), synced)
	connect(u, "cheap", memberCluster(mnode{name: "cheap-a", provider: awsID("cheap-a"), price: 0.01}), synced)
	connect(u, "aaa-free", memberCluster(mnode{name: "free-a", provider: awsID("free-a")}), synced)
	connect(u, "mix", memberCluster(mnode{name: "mix-a", provider: awsID("mix-a"), price: 1}, mnode{name: "mix-b", provider: azID("mix-b"), price: 1}), synced)
	connect(u, "odd", memberCluster(mnode{name: "odd-a", provider: otherID("odd-a"), price: 1}), synced)

	out := fscreen(u)
	header := screenText(u.clustersHeader())
	if strings.Contains(header, "$") || strings.Contains(header, "Cost:") {
		t.Errorf("header with two clouds shows a cost:\n%s", header)
	}
	if !strings.Contains(header, "3 clusters of unknown or mixed cloud (1 not connected)") {
		t.Errorf("header does not count the clusters in no total:\n%s", header)
	}
	aws, azure := cloudLine(out, "aws"), cloudLine(out, "azure")
	if aws == "" || azure == "" {
		t.Fatalf("no line per cloud:\n%s", out)
	}
	if !strings.Contains(aws, ">= ") || !strings.Contains(aws, "1 cluster not connected") {
		t.Errorf("aws total with a cluster not connected: %q", aws)
	}
	if strings.Contains(azure, ">=") {
		t.Errorf("azure total is complete: %q", azure)
	}
	if strings.Index(out, azure) > strings.Index(out, aws) {
		t.Errorf("clouds not by cost (azure costs more):\n%s", out)
	}
	for _, bucket := range []string{"mixed", "other", "-"} {
		if l := cloudLine(out, bucket); l != "" {
			t.Errorf("bucket %s has a total: %q", bucket, l)
		}
	}
	want := []string{"▸ az ", "▸ demo-prod", "▸ cheap", "▸ aaa-free", "▸ mix ", "▸ odd ", "▸ nowhere"}
	last := -1
	for _, w := range want {
		i := strings.Index(out, w)
		if i < 0 {
			t.Fatalf("no heading %q:\n%s", w, out)
		}
		if i < last {
			t.Errorf("heading %q out of order (want %v):\n%s", w, want, out)
		}
		last = i
	}
	if l := lineWith(out, "▸ mix "); !strings.Contains(l, "mixed") {
		t.Errorf("a cluster of two schemes is not mixed: %q", l)
	}
	if l := lineWith(out, "▸ odd "); !strings.Contains(l, "other") {
		t.Errorf("an unknown scheme is not other: %q", l)
	}
}

// clusters-screen R4
func TestClustersOneCloudCostInHeader(t *testing.T) {
	e := newFleetEnv(t, 40, []ContextInfo{{Name: "aks", Server: "https://example-aks.example.com", Cloud: "azure"}}, []string{"demo-prod", "aks"})
	u := e.u
	command(u, "clusters aws")
	out := fscreen(u)
	header := screenText(u.clustersHeader())
	if !strings.Contains(header, "Cost:") || !strings.Contains(header, "$728.102/month") {
		t.Errorf("one cloud: the header has no cost:\n%s", header)
	}
	if cloudLine(out, "aws") != "" {
		t.Errorf("one cloud: a cloud line is shown:\n%s", out)
	}
}

// clusters-screen R5
func TestClustersFilterInsideEverySection(t *testing.T) {
	u := clustersUI(t)
	u.Update(tea.WindowSizeMsg{Width: 168, Height: 40})
	v := u.view
	v.Filters = []Filter{{Field: FieldNodeGroup, Values: []string{"workers"}, Enabled: true}}
	u.setView(v)
	out := fscreen(u)
	if l := lineWith(out, "▸ demo-prod"); !strings.Contains(l, "0 nodes") || !strings.Contains(l, "(filtered: 0 of 10 nodes)") {
		t.Errorf("demo-prod with a filter none of its nodes match: %q", l)
	}
	if l := lineWith(out, "▸ aks"); !strings.Contains(l, "1 nodes") || !strings.Contains(l, "(filtered: 1 of 2 nodes)") {
		t.Errorf("aks with the filter: %q", l)
	}
	if l := cloudLine(out, "azure"); !strings.Contains(l, "(filtered: 1 of 2 nodes)") {
		t.Errorf("azure total with the filter: %q", l)
	}
}

// clusters-screen R5
func TestClustersPanelsOfferEveryClustersValues(t *testing.T) {
	u := clustersUI(t)
	pressAll(u, "f")
	if u.panel != panelFilters {
		t.Fatal("f did not open Filters")
	}
	u.filters.open = FieldNodeGroup
	var values []string
	for _, r := range u.filterRows() {
		if r.isValue {
			values = append(values, r.value)
		}
	}
	for _, want := range []string{"workers", "system", "default", "spot-arm"} {
		if !slices.Contains(values, want) {
			t.Errorf("nodegroup values %v miss %q", values, want)
		}
	}
}

// clusterSwitcher records the contexts enter opens.
type clusterSwitcher struct {
	calls []string
}

func (s *clusterSwitcher) Contexts() ([]string, string, error) {
	return []string{"demo-prod", "other-aws", "broken"}, "demo-prod", nil
}

func (s *clusterSwitcher) Switch(name string) (Session, error) {
	s.calls = append(s.calls, name)
	if name == "broken" {
		return Session{}, errors.New("unreachable")
	}
	v := DefaultViewState()
	v.Screen = ScreenClusters
	return Session{Cluster: storeWith("x-node"), Info: ClusterInfo{Context: name}, View: v}, nil
}

// clusters-screen R6
func TestClustersEnterOpensClusterInOverview(t *testing.T) {
	e := newFleetEnv(t, 40, []ContextInfo{
		{Name: "other-aws", Server: "https://example-other.example.com", Cloud: "aws"},
		{Name: "broken", Server: "https://example-broken.example.com", Cloud: "aws"},
	}, []string{"demo-prod", "other-aws", "broken"})
	u := e.u
	sw := &clusterSwitcher{}
	u.SetSwitcher(sw)
	command(u, "cl")

	// a failed switch: footer notice, the screen stays
	u.View()
	u.cursorKey = "c:broken"
	cmd := press(u, "enter")
	if cmd == nil {
		t.Fatal("enter on a member returned no command")
	}
	run(u, cmd())
	if !u.onClusters() || !strings.Contains(u.currentNotice(), "broken") {
		t.Errorf("failed switch: screen %q notice %q", u.view.Screen, u.currentNotice())
	}

	// enter on the context on screen just shows Overview
	u.View()
	u.cursorKey = "c:demo-prod"
	if cmd := press(u, "enter"); cmd != nil || u.view.Screen != ScreenOverview || len(sw.calls) != 1 {
		t.Errorf("enter on the context on screen: cmd %v screen %q calls %v", cmd != nil, u.view.Screen, sw.calls)
	}

	command(u, "cl")
	u.View()
	u.cursorKey = "c:other-aws"
	cmd = press(u, "enter")
	if cmd == nil {
		t.Fatal("enter on a member returned no command")
	}
	if again := press(u, "enter"); again != nil {
		t.Error("a second enter while the switch is pending returned a command")
	}
	run(u, cmd())
	if !slices.Equal(sw.calls, []string{"broken", "other-aws"}) {
		t.Errorf("Switch calls %v", sw.calls)
	}
	if u.view.Screen != ScreenOverview || u.info.Context != "other-aws" {
		t.Errorf("after the switch: screen %q context %q", u.view.Screen, u.info.Context)
	}
	if e.f.last() != nil {
		t.Errorf("the fleet is not closed after the switch: %v", e.f.last())
	}
}

// clusters-screen R6
func TestClustersEnterOnNodeRowOpensItsCluster(t *testing.T) {
	e := newFleetEnv(t, 50, []ContextInfo{{Name: "other-aws", Server: "https://example-other.example.com", Cloud: "aws"}}, []string{"demo-prod", "other-aws"})
	u := e.u
	sw := &clusterSwitcher{}
	u.SetSwitcher(sw)
	command(u, "cl")
	c := memberCluster(mnode{name: "mem-a", provider: awsID("mem-a"), price: 0.1})
	connect(u, "other-aws", c, &ConnStatus{Synced: true})
	unfold(u, "other-aws")
	u.View()
	u.cursorKey = "n:other-aws/mem-a"
	cmd := press(u, "enter")
	if cmd == nil {
		t.Fatal("enter on a node row returned no command")
	}
	run(u, cmd())
	if u.info.Context != "other-aws" || u.view.Screen != ScreenOverview {
		t.Errorf("after enter on a node row: context %q screen %q", u.info.Context, u.view.Screen)
	}
}

// clusters-screen R7
func TestClustersMemberErrorsStayInTheirHeading(t *testing.T) {
	e := newFleetEnv(t, 40, []ContextInfo{
		{Name: "bad", Server: "https://example-bad.example.com", Cloud: "aws"},
		{Name: "flaky", Server: "https://example-flaky.example.com", Cloud: "aws"},
		{Name: "good", Server: "https://example-good.example.com", Cloud: "aws"},
	}, []string{"demo-prod", "bad", "flaky", "good"})
	u := e.u
	command(u, "cl")
	run(u, MemberReadyMsg{Context: "bad", Err: "Unauthorized: token expired"})
	flaky := &ConnStatus{Synced: true, Err: "pods is forbidden"}
	connect(u, "flaky", memberCluster(mnode{name: "flaky-a", provider: awsID("flaky-a"), price: 0.1}), flaky)
	connect(u, "good", memberCluster(mnode{name: "good-a", provider: awsID("good-a"), price: 0.1}), &ConnStatus{Synced: true})

	out := fscreen(u)
	if l := lineWith(out, "▸ bad"); !strings.Contains(l, "Unauthorized: token expired") {
		t.Errorf("bad heading: %q", l)
	}
	if l := lineWith(out, "▸ flaky"); !strings.Contains(l, "pods is forbidden") || !strings.Contains(l, "/month") {
		t.Errorf("flaky heading (error, cost kept): %q", l)
	}
	for _, name := range []string{"▸ good", "▸ demo-prod"} {
		if l := lineWith(out, name); !strings.HasSuffix(strings.TrimSpace(l), "ok") {
			t.Errorf("%s heading affected by another member: %q", name, l)
		}
	}
	if strings.Contains(out, "https://") {
		t.Errorf("an API server URL is on screen:\n%s", out)
	}

	flaky.Err = ""
	u.Update(tickMsg(time.Now()))
	if l := lineWith(fscreen(u), "▸ flaky"); !strings.HasSuffix(strings.TrimSpace(l), "ok") {
		t.Errorf("a recovered watch did not return to ok: %q", l)
	}
}

// clusters-screen R8
func TestClusterPickerDefaultTicks(t *testing.T) {
	extra := []ContextInfo{
		{Name: "other-aws", Server: "https://example-other.example.com", Cloud: "aws"},
		{Name: "aks", Server: "https://example-aks.example.com", Cloud: "azure"},
		{Name: "unk", Server: "https://example-unk.example.com"},
	}
	e := newFleetEnv(t, 40, extra, nil)
	command(e.u, "cl")
	out := fscreen(e.u)
	for _, want := range []string{"[x] demo-prod", "[x] other-aws", "[ ] aks", "[ ] unk"} {
		if !strings.Contains(out, want) {
			t.Errorf("first-use picker misses %q:\n%s", want, out)
		}
	}

	e = newFleetEnv(t, 40, extra, nil)
	command(e.u, "clusters azure")
	out = fscreen(e.u)
	for _, want := range []string{"[x] aks", "[ ] other-aws", "[ ] unk"} {
		if !strings.Contains(out, want) {
			t.Errorf("first-use picker after :clusters azure misses %q:\n%s", want, out)
		}
	}
	// a ticked context of another cloud is saved, not connected
	pressAll(e.u, "home")
	e.u.fl.picker.cursor = slices.IndexFunc(e.u.fl.picker.rows, func(r pickerRow) bool { return r.name == "other-aws" })
	pressAll(e.u, " ", "enter")
	if !slices.Contains(e.store.contexts, "other-aws") || e.store.cloud != "azure" {
		t.Errorf("saved %v scope %q", e.store.contexts, e.store.cloud)
	}
	if got := e.f.last(); slices.Contains(got, "other-aws") || !slices.Contains(got, "aks") {
		t.Errorf("wanted under the azure scope: %v", got)
	}
}

// clusters-screen R8
func TestClusterPickerCloudLineAndPendingTicks(t *testing.T) {
	e := newFleetEnv(t, 40, []ContextInfo{
		{Name: "other-aws", Server: "https://example-other.example.com", Cloud: "aws"},
		{Name: "aks", Server: "https://example-aks.example.com", Cloud: "azure"},
	}, []string{"demo-prod"})
	u := e.u
	command(u, "cl")
	pressAll(u, "c")
	if u.panel != panelClusters {
		t.Fatal("c did not open the cluster picker")
	}
	calls, saves := len(e.f.wanted), e.store.saves
	// rows: aws, demo-prod, other-aws, azure, aks; the cursor is on demo-prod
	pressAll(u, "up", " ")
	out := fscreen(u)
	if !strings.Contains(out, "[x] demo-prod") || !strings.Contains(out, "[x] other-aws") {
		t.Errorf("space on a cloud line with one unticked did not tick all:\n%s", out)
	}
	pressAll(u, " ")
	out = fscreen(u)
	if !strings.Contains(out, "[ ] demo-prod") || !strings.Contains(out, "[ ] other-aws") {
		t.Errorf("space on a fully ticked cloud line did not untick all:\n%s", out)
	}
	pressAll(u, "down", "down", "down", " ")
	if !strings.Contains(fscreen(u), "[x] aks") {
		t.Error("space on the azure line did not tick aks")
	}
	if len(e.f.wanted) != calls {
		t.Errorf("ticks changed the members before enter: %v", e.f.wanted[calls:])
	}
	pressAll(u, "esc")
	if u.panel != panelNone || e.store.saves != saves {
		t.Errorf("esc: panel %v saves %d, want %d", u.panel, e.store.saves, saves)
	}
	pressAll(u, "c")
	out = fscreen(u)
	if !strings.Contains(out, "[x] demo-prod") || !strings.Contains(out, "[ ] aks") {
		t.Errorf("esc did not drop the pending ticks:\n%s", out)
	}
	pressAll(u, "down", " ", "enter")
	if e.store.saves != saves+1 || !slices.Equal(e.store.contexts, []string{"demo-prod", "other-aws"}) {
		t.Errorf("enter saved %d times (before %d): %v", e.store.saves, saves, e.store.contexts)
	}
	if !slices.Equal(e.f.last(), []string{"other-aws"}) {
		t.Errorf("wanted after enter: %v", e.f.last())
	}
}

// clusters-screen R2, R8
func TestClusterPickerFirstUseEsc(t *testing.T) {
	e := newFleetEnv(t, 40, []ContextInfo{{Name: "other-aws", Server: "https://example-other.example.com", Cloud: "aws"}}, nil)
	u := e.u
	command(u, "cl")
	pressAll(u, "esc")
	if e.store.saves != 0 {
		t.Error("esc on first use saved a choice")
	}
	if len(e.f.last()) != 0 {
		t.Errorf("esc on first use wants %v", e.f.last())
	}
	out := fscreen(u)
	if !strings.Contains(out, "▸ demo-prod") || strings.Contains(out, "▸ other-aws") {
		t.Errorf("after esc on first use the screen is not the context on screen alone:\n%s", out)
	}
	pressAll(u, "tab")
	command(u, "cl")
	if u.panel != panelClusters {
		t.Error("the picker did not open again on the next visit")
	}
}

// clusters-screen R8, R17
func TestClusterPickerSameClusterMark(t *testing.T) {
	u0 := demoUI(t)
	e := newFleetEnv(t, 40, []ContextInfo{
		{Name: "dup", Server: strings.ToUpper(u0.info.Server) + "/", Cloud: "aws"},
		{Name: "one-a", Server: "https://example-one.example.com", Cloud: "aws"},
		{Name: "one-b", Server: "https://EXAMPLE-ONE.example.com:443/", Cloud: "aws"},
	}, []string{"demo-prod"})
	command(e.u, "cl")
	pressAll(e.u, "c")
	out := fscreen(e.u)
	if l := lineWith(out, "] dup"); !strings.Contains(l, "same cluster as demo-prod") {
		t.Errorf("dup line: %q", l)
	}
	if l := lineWith(out, "] one-b"); !strings.Contains(l, "same cluster as one-a") {
		t.Errorf("one-b line: %q", l)
	}
	if l := lineWith(out, "] one-a"); strings.Contains(l, "same cluster") {
		t.Errorf("one-a line: %q", l)
	}
}

// clusters-screen R11, R15
func TestClustersSpaceFoldsSections(t *testing.T) {
	u := clustersUI(t)
	f := u.fl.fleet.(*fakeFleet)
	u.Update(tea.WindowSizeMsg{Width: 168, Height: 40})
	out := fscreen(u)
	if strings.Contains(out, "▾") || !strings.Contains(out, "▸ demo-prod") || !strings.Contains(out, "▸ aks") {
		t.Fatalf("sections do not start folded:\n%s", out)
	}
	if len(f.pods) != 0 {
		t.Errorf("pods watched before any unfold: %v", f.pods)
	}
	unfold(u, "aks")
	if !strings.Contains(fscreen(u), "▾ aks") || !slices.Equal(f.pods, []string{"aks"}) {
		t.Errorf("unfold: pods %v", f.pods)
	}
	unfold(u, "aks")
	if !strings.Contains(fscreen(u), "▸ aks") {
		t.Error("space did not fold the section again")
	}
	unfold(u, "aks")
	if len(f.pods) != 1 {
		t.Errorf("a second unfold watched the pods again: %v", f.pods)
	}
	unfold(u, "demo-prod")
	if !strings.Contains(fscreen(u), "▾ demo-prod") || len(f.pods) != 1 {
		t.Errorf("unfolding the context on screen: pods %v", f.pods)
	}

	// space on a node row does nothing
	node := u.fl.slots["aks"].stats.Nodes[0].Name()
	u.View()
	u.cursorKey = "n:aks/" + node
	press(u, " ")
	if !u.fl.unfolded["aks"] || !u.fl.unfolded["demo-prod"] || len(f.pods) != 1 {
		t.Errorf("space on a node row changed the folds: %v pods %v", u.fl.unfolded, f.pods)
	}

	// the fold is forgotten when the screen is left
	pressAll(u, "tab")
	command(u, "cl")
	if u.fl.unfolded["aks"] || u.fl.unfolded["demo-prod"] {
		t.Errorf("folds kept across visits: %v", u.fl.unfolded)
	}
}

// clusters-screen R13
func TestClustersKeys(t *testing.T) {
	u := clustersUI(t)
	u.Update(tea.WindowSizeMsg{Width: 168, Height: 40})
	v := u.view
	v.SetGroupBy(FieldNodeGroup)
	u.SetView(v)
	u.View()

	// n d e do nothing
	before := u.view
	for _, k := range []string{"n", "d", "e"} {
		if cmd := press(u, k); cmd != nil {
			t.Errorf("%s returned a command", k)
		}
		if !reflect.DeepEqual(u.view, before) || u.panel != panelNone {
			t.Errorf("%s changed the view or opened a panel", k)
		}
	}

	// esc clears a kept search
	u.view.Search = "aks"
	press(u, "esc")
	if u.view.Search != "" || !u.onClusters() {
		t.Errorf("esc: search %q screen %q", u.view.Search, u.view.Screen)
	}

	panels := []struct {
		key   string
		panel panelKind
	}{{"f", panelFilters}, {"g", panelGroupBy}, {"s", panelSort}, {"o", panelColumns}, {"c", panelClusters}, {"?", panelHelp}, {"v", panelViews}}
	for _, p := range panels {
		pressAll(u, p.key)
		if u.panel != p.panel {
			t.Errorf("%s opened panel %v, want %v", p.key, u.panel, p.panel)
		}
		pressAll(u, "esc")
		if u.panel != panelNone {
			t.Errorf("esc did not close the panel of %s", p.key)
		}
	}

	// up/down move over the headings
	u.View()
	first := u.cursorKey
	press(u, "j")
	u.View()
	if u.cursorKey == first || !strings.HasPrefix(u.cursorKey, "c:") {
		t.Errorf("j: cursor %q -> %q", first, u.cursorKey)
	}
	press(u, "k")
	u.View()
	if u.cursorKey != first {
		t.Errorf("k: cursor %q, want %q", u.cursorKey, first)
	}

	// pages
	press(u, "l")
	if u.page != 1 {
		t.Errorf("l: page %d", u.page)
	}
	press(u, "home")
	if u.page != 0 {
		t.Errorf("home: page %d", u.page)
	}

	// I and r as in Overview
	desc := u.view.Sort.Descending
	press(u, "I")
	if u.view.Sort.Descending == desc {
		t.Error("I did not flip the sort")
	}
	res := slices.Clone(u.view.Resources)
	press(u, "r")
	if slices.Equal(u.view.Resources, res) {
		t.Error("r did not change the resources")
	}

	// / searches, : commands, ctrl+e toggles the header
	press(u, "/")
	if !u.searching {
		t.Error("/ did not start a search")
	}
	press(u, "esc")
	press(u, ":")
	if !u.commanding {
		t.Error(": did not open the command line")
	}
	press(u, "esc")
	size := u.headerSize()
	press(u, "ctrl+e")
	if u.headerSize() == size {
		t.Error("ctrl+e did not change the header")
	}

	// tab leaves for Overview, q quits
	if !quits(press(u, "q")) {
		t.Error("q did not quit")
	}
	press(u, "tab")
	if u.view.Screen != ScreenOverview {
		t.Errorf("tab: screen %q", u.view.Screen)
	}
}

// clusters-screen R13
func TestClustersEnteringClosesNodeDetails(t *testing.T) {
	u := clustersUI(t)
	pressAll(u, "tab")
	u.panel = panelNode
	u.cursorKey = "n:something"
	u.setScreen(ScreenClusters)
	if u.panel != panelNone || u.cursorKey != "" {
		t.Errorf("entering Clusters: panel %v cursor %q", u.panel, u.cursorKey)
	}
}

// clusters-screen R14
func TestClustersCloudScope(t *testing.T) {
	e := newFleetEnv(t, 40, []ContextInfo{
		{Name: "other-aws", Server: "https://example-other.example.com", Cloud: "aws"},
		{Name: "aks", Server: "https://example-aks.example.com", Cloud: "azure"},
		{Name: "unk", Server: "https://example-unk.example.com"},
	}, []string{"demo-prod", "other-aws", "aks", "unk"})
	u, f := e.u, e.f
	command(u, "clusters aws")
	if got := f.scopes[len(f.scopes)-1]; got != "aws" {
		t.Errorf("scope %q, want aws", got)
	}
	if got := f.last(); !slices.Equal(got, []string{"other-aws", "unk"}) {
		t.Errorf("wanted under aws: %v", got)
	}
	if e.store.cloud != "aws" {
		t.Errorf("scope not saved: %q", e.store.cloud)
	}
	if h := screenText(u.clustersHeader()); !strings.Contains(h, "Scope:    aws") {
		t.Errorf("header does not name the scope:\n%s", h)
	}

	command(u, "clusters")
	if got := f.scopes[len(f.scopes)-1]; got != "" {
		t.Errorf("bare :clusters scope %q", got)
	}
	if got := f.last(); !slices.Equal(got, []string{"aks", "other-aws", "unk"}) {
		t.Errorf("wanted with every cloud: %v", got)
	}
	if e.store.cloud != "" {
		t.Errorf("bare :clusters did not save every cloud: %q", e.store.cloud)
	}

	calls := len(f.wanted)
	command(u, "cl x")
	if n := u.currentNotice(); n != `unknown cloud "x" (aws, azure, gce)` {
		t.Errorf("notice %q", n)
	}
	if len(f.wanted) != calls || u.fl.scope != "" || !u.onClusters() {
		t.Error("an unknown cloud changed the screen")
	}
}

// clusters-screen R14
func TestClustersCloudWordCompletion(t *testing.T) {
	e := newFleetEnv(t, 40, nil, []string{"demo-prod"})
	u := e.u
	for _, typed := range []struct{ in, want string }{{"clusters az", "clusters azure"}, {"cl a", "cl a"}, {"cl g", "cl gce"}} {
		pressAll(u, ":")
		for _, r := range typed.in {
			pressAll(u, string(r))
		}
		pressAll(u, "tab")
		if u.command != typed.want {
			t.Errorf("tab after %q: %q, want %q", typed.in, u.command, typed.want)
		}
		pressAll(u, "esc")
	}
}

// clusters-screen R14
func TestClustersMemberLeftByLearnedCloud(t *testing.T) {
	extra := []ContextInfo{
		{Name: "other-aws", Server: "https://example-other.example.com", Cloud: "aws"},
		{Name: "unk", Server: "https://example-unk.example.com"},
	}
	chosen := []string{"demo-prod", "other-aws", "unk"}
	for _, back := range []string{"clusters azure", "clusters"} {
		e := newFleetEnv(t, 40, extra, chosen)
		u, f := e.u, e.f
		command(u, "clusters aws")
		if !slices.Contains(f.last(), "unk") {
			t.Fatalf("an unknown-cloud context is not connected under a scope: %v", f.last())
		}
		run(u, MemberLeftMsg{Context: "unk", Cloud: "azure"})
		if _, ok := u.fl.slots["unk"]; ok || strings.Contains(fscreen(u), "▸ unk") {
			t.Error("the member left by the scope still has a section")
		}
		command(u, "clusters aws")
		if slices.Contains(f.last(), "unk") {
			t.Errorf(":clusters aws again wants the member whose learned cloud is azure: %v", f.last())
		}
		command(u, back)
		if !slices.Contains(f.last(), "unk") {
			t.Errorf(":%s does not want the member again: %v", back, f.last())
		}
	}
}

// clusters-screen R14
func TestClustersScopeSavedWithTheChoice(t *testing.T) {
	e := newFleetEnv(t, 40, []ContextInfo{{Name: "aks", Server: "https://example-aks.example.com", Cloud: "azure"}}, nil)
	command(e.u, "clusters azure")
	if e.store.saves != 0 {
		t.Error("the scope was saved before any choice")
	}
	pressAll(e.u, "enter")
	if e.store.saves != 1 || e.store.cloud != "azure" {
		t.Errorf("saved %d times, scope %q", e.store.saves, e.store.cloud)
	}
}

// clusters-screen R17
func TestClustersOneMemberPerServer(t *testing.T) {
	u0 := demoUI(t)
	e := newFleetEnv(t, 40, []ContextInfo{
		{Name: "same-as-demo", Server: strings.ToUpper(u0.info.Server) + "/", Cloud: "aws"},
		{Name: "one-b", Server: "https://EXAMPLE-ONE.example.com:443/", Cloud: "aws"},
		{Name: "one-a", Server: "https://example-one.example.com", Cloud: "aws"},
		{Name: "one-c", Server: "https://example-one.example.com/", Cloud: "aws"},
		{Name: "two", Server: "https://example-one.example.com:8443", Cloud: "aws"},
	}, []string{"one-c", "one-b", "demo-prod", "same-as-demo", "one-a", "two"})
	command(e.u, "cl")
	if got := e.f.last(); !slices.Equal(got, []string{"one-a", "two"}) {
		t.Errorf("wanted %v, want [one-a two] (one per API server, first by name, none with Overview's)", got)
	}
	out := fscreen(e.u)
	for _, not := range []string{"▸ one-b", "▸ one-c", "▸ same-as-demo"} {
		if strings.Contains(out, not) {
			t.Errorf("a second section for one API server: %q", not)
		}
	}
}

// clusters-screen R12
func TestClustersCursorFollowsItsClusterWhenItMoves(t *testing.T) {
	e := newFleetEnv(t, 40, []ContextInfo{{Name: "cheap", Server: "https://example-cheap.example.com", Cloud: "aws"}}, []string{"demo-prod", "cheap"})
	u := e.u
	command(u, "cl")
	c := memberCluster(mnode{name: "cheap-a", provider: awsID("cheap-a"), price: 0.01})
	connect(u, "cheap", c, &ConnStatus{Synced: true, PodsSynced: true})
	out := fscreen(u)
	if strings.Index(out, "▸ cheap") < strings.Index(out, "▸ demo-prod") {
		t.Fatalf("cheap listed before demo-prod:\n%s", out)
	}
	u.cursorKey = "c:cheap"
	c.Stats().Nodes[0].SetPrice(50)
	changed(u, c)
	out = fscreen(u)
	if strings.Index(out, "▸ cheap") > strings.Index(out, "▸ demo-prod") {
		t.Errorf("the order did not follow the new price:\n%s", out)
	}
	if u.cursorKey != "c:cheap" {
		t.Errorf("cursor %q, want it on cheap", u.cursorKey)
	}
}

// clusters-screen R3
func TestClustersEmptySyncedClusterKeepsGuess(t *testing.T) {
	e := newFleetEnv(t, 40, []ContextInfo{{Name: "empty", Server: "https://example-empty.example.com", Cloud: "azure"}}, []string{"demo-prod", "empty"})
	u := e.u
	command(u, "cl")
	connect(u, "empty", NewCluster(), &ConnStatus{Synced: true, PodsSynced: true})
	l := lineWith(fscreen(u), "▸ empty")
	if !strings.Contains(l, "azure") || !strings.Contains(l, "0 nodes") || !strings.HasSuffix(strings.TrimSpace(l), "ok") {
		t.Errorf("an empty synced cluster: %q", l)
	}
}

// clusters-screen R15
func TestClustersContextOnScreenGatedByItsPods(t *testing.T) {
	e := newFleetEnv(t, 40, nil, []string{"demo-prod"})
	u := e.u
	command(u, "cl")
	st := ConnStatus{Synced: true}
	u.fl.overview = func() ConnStatus { return st }
	if l := lineWith(fscreen(u), "▸ demo-prod"); !strings.Contains(l, "idle …") {
		t.Errorf("context on screen before its pods synced: %q", l)
	}
	st.PodsSynced = true
	if l := lineWith(fscreen(u), "▸ demo-prod"); !strings.Contains(l, "idle $174.051/month") {
		t.Errorf("context on screen after its pods synced: %q", l)
	}
}

// clusters-screen R4
func TestClustersUnpricedNodesMarkTheTotal(t *testing.T) {
	e := newFleetEnv(t, 40, []ContextInfo{
		{Name: "free", Server: "https://example-free.example.com", Cloud: "aws"},
		{Name: "az", Server: "https://example-az.example.com", Cloud: "azure"},
	}, []string{"demo-prod", "free", "az"})
	u := e.u
	command(u, "cl")
	synced := &ConnStatus{Synced: true, PodsSynced: true}
	connect(u, "free", memberCluster(mnode{name: "free-a", provider: awsID("free-a")}), synced)
	connect(u, "az", memberCluster(mnode{name: "az-a", provider: azID("az-a"), price: 1}), synced)
	if l := cloudLine(fscreen(u), "aws"); !strings.Contains(l, ">= ") || !strings.Contains(l, "1 node unpriced") {
		t.Errorf("aws total with an unpriced node: %q", l)
	}
}

// clusters-screen R10
func TestClustersPricingDisabled(t *testing.T) {
	e := newFleetEnv(t, 40, []ContextInfo{
		{Name: "pending", Server: "https://example-pending.example.com", Cloud: "aws"},
		{Name: "az", Server: "https://example-az.example.com", Cloud: "azure"},
	}, []string{"demo-prod", "pending", "az"})
	u := e.u
	u.DisablePricing = true
	command(u, "cl")
	connect(u, "az", memberCluster(mnode{name: "az-a", provider: azID("az-a"), price: 1}), &ConnStatus{Synced: true, PodsSynced: true})
	out := fscreen(u)
	body := out[strings.Index(out, "╯"):]
	if strings.Contains(body, "$") || strings.Contains(body, ">=") {
		t.Errorf("costs shown with pricing disabled:\n%s", out)
	}
	if !strings.Contains(lineWith(out, "▸ demo-prod"), "10 nodes") {
		t.Errorf("node counts missing with pricing disabled:\n%s", out)
	}
}

// clusters-screen R2, R14
func TestClustersContextOnScreenAlwaysASection(t *testing.T) {
	e := newFleetEnv(t, 40, []ContextInfo{
		{Name: "other-aws", Server: "https://example-other.example.com", Cloud: "aws"},
		{Name: "aks", Server: "https://example-aks.example.com", Cloud: "azure"},
	}, []string{"other-aws", "aks"})
	u := e.u
	command(u, "cl")
	if !strings.Contains(fscreen(u), "▸ demo-prod") {
		t.Error("the context on screen, not ticked, has no section")
	}
	command(u, "clusters azure")
	out := fscreen(u)
	if strings.Contains(out, "▸ demo-prod") || strings.Contains(out, "▸ other-aws") {
		t.Errorf("the azure scope still shows aws sections:\n%s", out)
	}
	if _, ok := u.fl.slots["other-aws"]; ok {
		t.Error("a member left out by the scope was not closed at once")
	}
	if !slices.Equal(e.f.last(), []string{"aks"}) {
		t.Errorf("wanted under azure: %v", e.f.last())
	}
}
