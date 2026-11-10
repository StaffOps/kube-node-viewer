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

// aks-pricing R3: with only Azure nodes, Recommendations says EKS only.
func TestAKSRecommendationsSayEKSOnly(t *testing.T) {
	u := aksUI(t)
	pressAll(u, "tab")
	out := u.View()
	if !strings.Contains(out, "Recommendations cover EKS nodes only for now; the nodes shown run on Azure.") {
		t.Errorf("no EKS-only note:\n%s", out)
	}
	if strings.Contains(out, "Savings:") {
		t.Errorf("a savings total for nodes the rules can't read:\n%s", out)
	}
	// with prices known, the rules would find idle cost: they must not run,
	// and enter must not open the math of a row that isn't shown
	for _, n := range u.stats.Nodes {
		n.SetPrice(0.0918)
	}
	u.Refresh()
	pressAll(u, "enter")
	if u.panel != panelNone || u.recFor != "" {
		t.Errorf("rules ran on Azure nodes: panel %v, analysed %q", u.panel, u.recFor)
	}
}
