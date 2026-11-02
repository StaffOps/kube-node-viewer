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

package pricing_test

import (
	"testing"

	v1 "k8s.io/api/core/v1"

	"github.com/staffops/kube-node-viewer/pkg/model"
	"github.com/staffops/kube-node-viewer/pkg/pricing"
)

type fixed struct {
	price   float64
	updates *int
}

func (f fixed) NodePrice(*model.Node) (float64, bool) { return f.price, true }
func (f fixed) OnUpdate(func())                       { *f.updates++ }

func node(providerID string) *model.Node {
	return model.NewNode(&v1.Node{Spec: v1.NodeSpec{ProviderID: providerID}})
}

func TestByCloudSendsEachNodeToItsCloud(t *testing.T) {
	updates := 0
	awsP, azureP := fixed{1, &updates}, fixed{2, &updates}
	b := pricing.ByCloud{Clouds: map[string]pricing.Provider{"aws": awsP, "azure": azureP}, Fallback: awsP}
	for id, want := range map[string]float64{
		"aws:///us-east-1a/i-0abc": 1,
		"azure:///subscriptions/s": 2,
		"kind://docker/kind/node":  1, // any other cloud: the fallback
	} {
		if got, _ := b.NodePrice(node(id)); got != want {
			t.Errorf("%s: got %v, want %v", id, got, want)
		}
	}
	b.OnUpdate(func() {})
	if updates != 2 {
		t.Errorf("OnUpdate reached %d providers, want 2 (the fallback once)", updates)
	}
}
