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

// Modified by the kube-node-viewer authors, 2026.

package pricing

import "github.com/staffops/kube-node-viewer/pkg/model"

// Provider provides node prices for display in the node viewer
type Provider interface {
	NodePrice(n *model.Node) (float64, bool)
	OnUpdate(onUpdate func())
}

// ByCloud sends each node to the provider of its cloud, the scheme of its
// provider ID ("aws", "azure"); nodes of any other cloud go to Fallback
// (aks-pricing D1).
type ByCloud struct {
	Clouds   map[string]Provider
	Fallback Provider
}

func (b ByCloud) NodePrice(n *model.Node) (float64, bool) {
	if p, ok := b.Clouds[n.Cloud()]; ok {
		return p.NodePrice(n)
	}
	return b.Fallback.NodePrice(n)
}

func (b ByCloud) OnUpdate(f func()) {
	b.Fallback.OnUpdate(f)
	for _, p := range b.Clouds {
		if p != b.Fallback {
			p.OnUpdate(f)
		}
	}
}
