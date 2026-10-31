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
	const newKey = model.LabelPrefix + "instance-price"
	const oldKey = model.LegacyLabelPrefix + "instance-price"
	cases := []struct {
		name   string
		labels map[string]string
		want   float64
	}{
		{"no label uses provider", nil, 9},
		{"legacy key", map[string]string{oldKey: "0.0416"}, 0.0416},
		{"new key", map[string]string{newKey: "0.5"}, 0.5},
		{"both, new wins", map[string]string{newKey: "0.5", oldKey: "0.0416"}, 0.5},
		{"new unparsable, legacy used", map[string]string{newKey: "abc", oldKey: "0.0416"}, 0.0416},
		{"both unparsable, provider used", map[string]string{newKey: "abc", oldKey: "x"}, 9},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			node := model.NewNode(&v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "n", Labels: c.labels}})
			Controller{pricing: fixedPricing(9)}.updatePrice(node)
			if node.Price != c.want {
				t.Errorf("price = %v, want %v", node.Price, c.want)
			}
		})
	}
}
