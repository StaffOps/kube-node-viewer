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

package client

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/staffops/kube-node-viewer/pkg/model"
)

type fixedPricing float64

func (p fixedPricing) NodePrice(*model.Node) (float64, bool) { return float64(p), true }
func (p fixedPricing) OnUpdate(func())                       {}

func TestUpdatePriceLabelPrefixes(t *testing.T) {
	const key = model.LabelPrefix + "instance-price"
	cases := []struct {
		name   string
		labels map[string]string
		want   float64
	}{
		{"no label uses provider", nil, 9},
		{"override label", map[string]string{key: "0.5"}, 0.5},
		{"unparsable label, provider used", map[string]string{key: "abc"}, 9},
		{"other tools' labels are ignored", map[string]string{"eks-node-viewer/instance-price": "0.0416"}, 9},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			node := model.NewNode(&v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "n", Labels: c.labels}})
			(&Controller{pricing: fixedPricing(9)}).updatePrice(node)
			if node.Price() != c.want {
				t.Errorf("price = %v, want %v", node.Price(), c.want)
			}
		})
	}
}

// The override label wins on an AKS node too: the Azure provider is never
// consulted for it.
// aks-pricing R1
func TestUpdatePriceOverrideOnAKSNode(t *testing.T) {
	node := model.NewNode(&v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "aks-node", Labels: map[string]string{
			model.LabelPrefix + "instance-price": "0.5",
			v1.LabelTopologyRegion:               "eastus2",
			v1.LabelInstanceTypeStable:           "Standard_D2s_v5",
		}},
		Spec: v1.NodeSpec{ProviderID: "azure:///subscriptions/s/resourceGroups/rg/providers/Microsoft.Compute/virtualMachineScaleSets/x/virtualMachines/0"},
	})
	(&Controller{pricing: fixedPricing(9)}).updatePrice(node)
	if node.Price() != 0.5 {
		t.Errorf("price = %v, want the label's 0.5", node.Price())
	}
}

func TestUpdatePriceReservedIsEquivalent(t *testing.T) {
	reserved := model.NewNode(&v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "r",
		Labels: map[string]string{"karpenter.sh/capacity-type": "reserved"}}})
	c := &Controller{pricing: fixedPricing(9)}
	c.updatePrice(reserved)
	if !reserved.PriceIsEquivalent() || reserved.Price() != 9 || reserved.HasActualPrice() {
		t.Errorf("reserved: price %v equivalent %v actual %v", reserved.Price(), reserved.PriceIsEquivalent(), reserved.HasActualPrice())
	}

	// an explicit override is what the user says the node costs
	reserved.Labels()[model.LabelPrefix+"instance-price"] = "1.5"
	c.updatePrice(reserved)
	if reserved.PriceIsEquivalent() || reserved.Price() != 1.5 {
		t.Errorf("override: price %v equivalent %v", reserved.Price(), reserved.PriceIsEquivalent())
	}
}
