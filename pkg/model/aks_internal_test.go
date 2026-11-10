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
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/staffops/kube-node-viewer/pkg/recommend"
)

// aksUI is an AKS cluster with no node priced yet (as before the Azure
// prices arrive), and no region from the server URL.
func aksUI(t *testing.T) *UIModel {
	u := newTestUI(t)
	u.SetClusterInfo(ClusterInfo{Context: "aks", Server: "https://example-aks.hcp.eastus2.azmk8s.io:443"})
	u.advisor = fakeAdvisor{}
	for i, pool := range []string{"system", "workers"} {
		n := NewNode(&v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "aks-" + pool + "-vmss00000" + string(rune('0'+i)),
				CreationTimestamp: metav1.NewTime(time.Now().Add(-time.Hour)),
				Labels: map[string]string{
					v1.LabelTopologyRegion:           "eastus2",
					v1.LabelTopologyZone:             "0",
					v1.LabelInstanceTypeStable:       "Standard_D2pds_v6",
					"kubernetes.azure.com/agentpool": pool,
				},
			},
			Spec: v1.NodeSpec{ProviderID: "azure:///subscriptions/s/resourceGroups/example-rg/providers/Microsoft.Compute/virtualMachineScaleSets/aks-" + pool + "-vmss/virtualMachines/0"},
			Status: v1.NodeStatus{
				Allocatable: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1900m"), v1.ResourceMemory: resource.MustParse("6Gi"), v1.ResourcePods: resource.MustParse("30")},
				Conditions:  []v1.NodeCondition{{Type: v1.NodeReady, Status: v1.ConditionTrue}},
			},
		})
		u.cluster.AddNode(n).Show()
		n.SetPrice(math.NaN())
	}
	u.Refresh()
	u.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	return u
}

func TestAKSShowsUnknownPricesNotZero(t *testing.T) {
	u := aksUI(t)
	out := u.View()
	if strings.Contains(out, "$0.000") {
		t.Errorf("unknown prices shown as $0:\n%s", out)
	}
	if !strings.Contains(out, "prices unknown") {
		t.Errorf("no 'prices unknown':\n%s", out)
	}
	if !strings.Contains(out, "eastus2") {
		t.Errorf("region not taken from the nodes:\n%s", out)
	}
	doc := u.exportDoc(time.Now())
	if doc.Cluster.PricePerHour != nil || doc.Cluster.UnknownPrice != 2 {
		t.Errorf("export totals: price %v unknown %d, want nil and 2", doc.Cluster.PricePerHour, doc.Cluster.UnknownPrice)
	}
}

// fakeAzureAdvisor is an Azure catalog and prices for the model tests: one
// size, notes and a price generation the test can set.
type fakeAzureAdvisor struct {
	notes  []string
	gen    *uint64
	block  chan struct{} // when set, prices wait on it
	begins *int
	price  float64 // when set, every Azure size costs this
}

func (a fakeAzureAdvisor) Begin([]recommend.Pair) {
	if a.begins != nil {
		*a.begins++
	}
}

func (fakeAzureAdvisor) Spec(t string) (recommend.Spec, bool) {
	if t == "Standard_D2pds_v6" {
		return recommend.Spec{VCPU: 2, MemGiB: 8, Arch: "arm64", Family: "dpdsv6"}, true
	}
	return recommend.Spec{}, false
}
func (a fakeAzureAdvisor) OnDemand(recommend.Node, string) (float64, bool) {
	if a.block != nil {
		<-a.block
	}
	return a.price, a.price > 0
}
func (a fakeAzureAdvisor) SpotIn(n recommend.Node, t string) (float64, bool) { return a.OnDemand(n, t) }
func (fakeAzureAdvisor) Types() []string                                     { return []string{"Standard_D2pds_v6"} }
func (a fakeAzureAdvisor) Notes() []string                                   { return a.notes }
func (a fakeAzureAdvisor) Generation() uint64 {
	if a.gen == nil {
		return 0
	}
	return *a.gen
}

// aks-recommendations R1: Azure nodes are analysed, with agent-pool rows.
// aks-pricing R3 is withdrawn: the EKS-only message is gone.
func TestAKSRecommendationsAnalyseAzureNodes(t *testing.T) {
	u := aksUI(t)
	u.advisor = AdvisorsByCloud{recommend.CloudAWS: fakeAdvisor{}, recommend.CloudAzure: fakeAzureAdvisor{}}
	for _, n := range u.stats.Nodes {
		n.SetPrice(0.0918)
	}
	u.Refresh()
	pressAll(u, "tab")
	out := u.View()
	if strings.Contains(out, "EKS nodes only") || strings.Contains(out, "EKS and AKS nodes only") {
		t.Errorf("Azure nodes not analysed:\n%s", out)
	}
	if !strings.Contains(out, "agent pool system (1 nodes)") || !strings.Contains(out, "Savings:") {
		t.Errorf("no agent-pool rows:\n%s", out)
	}
}

// aks-recommendations R1: nodes of another cloud get the EKS-and-AKS
// message; with no node shown the screen says there is nothing to recommend.
func TestRecommendationsOtherCloudAndEmpty(t *testing.T) {
	u := aksUI(t)
	u.advisor = AdvisorsByCloud{recommend.CloudAWS: fakeAdvisor{}, recommend.CloudAzure: fakeAzureAdvisor{}}
	for _, n := range u.stats.Nodes {
		obj := n.Object().DeepCopy()
		obj.Spec.ProviderID = "gce://example-project/us-central1-a/" + obj.Name
		n.Update(obj)
	}
	u.Refresh()
	pressAll(u, "tab")
	if out := u.View(); !strings.Contains(out, "Recommendations cover EKS and AKS nodes only for now; the nodes shown run on Google Cloud.") {
		t.Errorf("no message for GCP nodes:\n%s", out)
	}
	e := recUI(t)
	e.view.Search = "no-such-node"
	e.Refresh()
	pressAll(e, "tab")
	if out := e.View(); !strings.Contains(out, "Nothing to recommend for the nodes shown.") {
		t.Errorf("empty view:\n%s", out)
	}
}

// aks-recommendations R3: the advisor's notes show with Azure nodes, a new
// price generation runs the analysis again, and the UI never waits on a
// price.
func TestAzureNotesGenerationAndNoWait(t *testing.T) {
	u := aksUI(t)
	var gen uint64
	u.advisor = AdvisorsByCloud{recommend.CloudAzure: fakeAzureAdvisor{notes: []string{"waiting for the prices of 5 Azure sizes"}, gen: &gen}}
	pressAll(u, "tab")
	if out := u.View(); !strings.Contains(out, "waiting for the prices of 5 Azure sizes") {
		t.Errorf("no waiting note:\n%s", out)
	}
	before := u.recFor
	gen++
	if u.maybeAnalyze() == nil || u.recKey() == before {
		t.Error("a new price generation did not start a new analysis")
	}
	u.recPending = false

	block := make(chan struct{})
	defer close(block)
	u.advisor = AdvisorsByCloud{recommend.CloudAzure: fakeAzureAdvisor{block: block, gen: &gen}}
	gen++
	done := make(chan tea.Cmd, 1)
	go func() { done <- u.maybeAnalyze() }()
	select {
	case cmd := <-done:
		if cmd == nil {
			t.Error("no analysis started")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the UI waited on a price")
	}
}

// aks-recommendations R4: the AKS labels and status.capacity reach the rules:
// the agent pool is the managed group, a NodePool label wins, no mode label
// is a user pool, and the virtual-node, Windows and VMs-pool marks.
func TestRecommendNodeFromAKSLabels(t *testing.T) {
	mk := func(labels map[string]string, providerID string) recommend.Node {
		labels[v1.LabelTopologyRegion] = "eastus"
		return recommendNode(NewNode(&v1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "aks-pool-12345678-vmss000000", Labels: labels},
			Spec:       v1.NodeSpec{ProviderID: providerID},
			Status: v1.NodeStatus{
				Capacity:    v1.ResourceList{v1.ResourceCPU: resource.MustParse("4"), v1.ResourceMemory: resource.MustParse("16Gi")},
				Allocatable: v1.ResourceList{v1.ResourceCPU: resource.MustParse("3860m"), v1.ResourceMemory: resource.MustParse("12Gi")},
			},
		}))
	}
	vmss := "azure:///subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/example-rg/providers/Microsoft.Compute/virtualMachineScaleSets/aks-pool-vmss/virtualMachines/0"
	vms := "azure:///subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/example-rg/providers/Microsoft.Compute/virtualMachines/aks-pool-12345678-vm0"
	n := mk(map[string]string{"kubernetes.azure.com/agentpool": "pool", "kubernetes.azure.com/mode": "system", v1.LabelOSStable: "windows"}, vmss)
	if n.Cloud != recommend.CloudAzure || n.Region != "eastus" || n.ManagedGroup != "pool" || !n.System || !n.Windows || n.VMPool {
		t.Errorf("system Windows VMSS node: %+v", n)
	}
	if n.NodeCapacity.CPU != 4 || n.NodeCapacity.Mem != 16 {
		t.Errorf("status.capacity not passed: %+v", n.NodeCapacity)
	}
	n = mk(map[string]string{"kubernetes.azure.com/agentpool": "pool"}, vms)
	if n.System || !n.VMPool {
		t.Errorf("no mode label must be a user pool, VMs provider ID a VMs pool: %+v", n)
	}
	n = mk(map[string]string{"kubernetes.azure.com/agentpool": "pool", "karpenter.sh/nodepool": "default"}, vmss)
	if n.NodePool != "default" || n.ManagedGroup != "" {
		t.Errorf("a NodePool label must win: %+v", n)
	}
	n = mk(map[string]string{"type": "virtual-kubelet"}, "")
	if !n.Virtual {
		t.Errorf("virtual node not marked: %+v", n)
	}
}

// aks-recommendations R1, D2: AdvisorsByCloud sends Azure sizes and nodes to
// the Azure advisor and the rest to AWS's; each analysis begins afresh; nodes
// of another cloud are left out.
func TestAdvisorsByCloudRouting(t *testing.T) {
	begins := 0
	b := AdvisorsByCloud{recommend.CloudAWS: fakeAdvisor{}, recommend.CloudAzure: fakeAzureAdvisor{price: 0.5, begins: &begins}}
	if _, ok := b.Spec("Standard_D2pds_v6"); !ok {
		t.Error("an Azure size not routed to the Azure catalog")
	}
	if _, ok := b.Spec("m6i.xlarge"); !ok {
		t.Error("an AWS type not routed to the AWS catalog")
	}
	if p, _ := b.OnDemand(recommend.Node{Cloud: recommend.CloudAzure}, "m6i.xlarge"); p != 0.5 {
		t.Errorf("an Azure node priced by %v, want the Azure advisor's 0.5", p)
	}
	if p, ok := b.OnDemand(recommend.Node{Cloud: recommend.CloudAWS}, "m6i.xlarge"); !ok || p != 0.192 {
		t.Errorf("an AWS node priced %v %v, want the AWS advisor's 0.192", p, ok)
	}

	u := aksUI(t)
	u.advisor = b
	for _, n := range u.stats.Nodes {
		n.SetPrice(0.0918)
		if strings.Contains(n.Name(), "workers") {
			obj := n.Object().DeepCopy()
			obj.Spec.ProviderID = "gce://example-project/us-central1-a/" + obj.Name
			n.Update(obj)
		}
	}
	u.Refresh()
	pressAll(u, "tab")
	if begins == 0 {
		t.Error("the analysis did not begin the Azure advisor")
	}
	out := u.View()
	if !strings.Contains(out, "agent pool system") || strings.Contains(out, "workers") {
		t.Errorf("a GCE node got advice, or the Azure one none:\n%s", out)
	}
}

// fakePricedAzure prices a small Azure catalog, for the golden screen with
// rightsize, generation and arm rows and a waiting note.
type fakePricedAzure struct{}

var pricedSpecs = map[string]recommend.Spec{
	"Standard_D4s_v3":   {VCPU: 4, MemGiB: 16, Arch: "x86_64", Family: "dsv3"},
	"Standard_D2s_v3":   {VCPU: 2, MemGiB: 8, Arch: "x86_64", Family: "dsv3"},
	"Standard_D4ds_v5":  {VCPU: 4, MemGiB: 16, Arch: "x86_64", Family: "ddsv5"},
	"Standard_D4pds_v5": {VCPU: 4, MemGiB: 16, Arch: "arm64", Family: "dpdsv5"},
}

var pricedList = map[string]float64{"Standard_D4s_v3": 0.192, "Standard_D2s_v3": 0.096, "Standard_D4ds_v5": 0.192, "Standard_D4pds_v5": 0.181}

func (fakePricedAzure) Spec(t string) (recommend.Spec, bool) { s, ok := pricedSpecs[t]; return s, ok }
func (fakePricedAzure) OnDemand(_ recommend.Node, t string) (float64, bool) {
	p, ok := pricedList[t]
	return p, ok
}
func (fakePricedAzure) SpotIn(recommend.Node, string) (float64, bool) { return 0, false }
func (fakePricedAzure) Types() []string {
	return []string{"Standard_D4s_v3", "Standard_D2s_v3", "Standard_D4ds_v5", "Standard_D4pds_v5"}
}
func (fakePricedAzure) Notes() []string { return []string{"waiting for the price of 1 Azure size"} }

// aksPricedUI is aksUI with D4s_v3 nodes measured as AKS reports them.
func aksPricedUI(t *testing.T) *UIModel {
	u := aksUI(t)
	for _, n := range u.stats.Nodes {
		obj := n.Object().DeepCopy()
		obj.Labels[v1.LabelInstanceTypeStable] = "Standard_D4s_v3"
		obj.Status.Capacity = v1.ResourceList{v1.ResourceCPU: resource.MustParse("4"), v1.ResourceMemory: resource.MustParse("15974Mi"), v1.ResourcePods: resource.MustParse("250")}
		obj.Status.Allocatable = v1.ResourceList{v1.ResourceCPU: resource.MustParse("3860m"), v1.ResourceMemory: resource.MustParse("12185Mi"), v1.ResourcePods: resource.MustParse("250")}
		n.Update(obj)
		n.SetPrice(0.192)
	}
	u.Refresh()
	return u
}
