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

package azure

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/staffops/kube-node-viewer/pkg/recommend"
)

// PriceLister answers the list price of any size in a region, for sizes no
// node runs (aks-recommendations R3, D3).
type PriceLister interface {
	// ListPrice is the hourly price of size in region for the OS and the
	// market. known is false when there is no price; pending is true while
	// the answer is awaited (the size is queued, never fetched here)
	ListPrice(region, size string, windows, spot bool) (price float64, known, pending bool)
	// Generation changes whenever new prices are stored
	Generation() uint64
}

// Advisor answers the recommendations for Azure nodes from the live VM sizes
// (azure-live-skus) and a PriceLister (aks-recommendations D2). Begin takes a
// snapshot of the sizes for the analysis that follows, so Types, Spec,
// Available and Notes agree while it runs off the UI goroutine.
type Advisor struct {
	prices   PriceLister
	disabled bool
	sizes    *Sizes

	mu      sync.Mutex
	pending map[key]bool
	snap    *snapshot
}

// NewAdvisor makes an advisor over prices and sizes; disabled means
// --disable-pricing: lookups answer "no price", nothing is queued and no size
// is read (aks-recommendations R3, azure-live-skus R7).
func NewAdvisor(prices PriceLister, disabled bool, sizes *Sizes) *Advisor {
	if disabled {
		sizes = nil
	}
	return &Advisor{prices: prices, disabled: disabled, sizes: sizes, pending: map[key]bool{}, snap: &snapshot{}}
}

// Begin starts an analysis of the nodes of pairs: the sizes awaited are
// counted afresh, the size reads they need are started, and a snapshot is
// taken (aks-recommendations R3, azure-live-skus R1, D2).
func (a *Advisor) Begin(pairs []recommend.Pair) {
	s := &snapshot{}
	if a.sizes != nil {
		s = a.sizes.begin(pairs)
	}
	a.mu.Lock()
	a.pending = map[key]bool{}
	a.snap = s
	a.mu.Unlock()
}

func (a *Advisor) snapshot() *snapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.snap
}

// Spec is the size from the loaded answers (azure-live-skus R2).
func (a *Advisor) Spec(t string) (recommend.Spec, bool) {
	s, ok := a.snapshot().union[t]
	if !ok {
		return recommend.Spec{}, false
	}
	return recommend.Spec{VCPU: s.VCPU, MemGiB: s.MemGiB, Arch: s.Arch, Family: s.Family, Constrained: s.Constrained, TempDiskMB: s.TempDiskMB}, true
}

// Types lists the loaded sizes.
func (a *Advisor) Types() []string { return a.snapshot().names }

// Available tells whether size t can run on node n: in the answer of n's
// own subscription and region, not restricted there or in its zone
// (azure-live-skus R3).
func (a *Advisor) Available(n recommend.Node, t string) bool {
	p, ok := a.snapshot().loaded[pairKeyOf(n.Subscription, n.Region)]
	if !ok {
		return false
	}
	s, ok := p[t]
	return ok && s.AvailableIn(zoneOf(n))
}

// SizesLoaded tells whether n's pair has its sizes (azure-live-skus R3: Spot
// needs no size while they are loading or failed).
func (a *Advisor) SizesLoaded(n recommend.Node) bool {
	_, ok := a.snapshot().loaded[pairKeyOf(n.Subscription, n.Region)]
	return ok
}

// zoneOf is the zone of a zonal AKS node (label <region>-<n>), empty for a
// fault domain (azure-live-skus R3).
func zoneOf(n recommend.Node) string {
	region := strings.ToLower(n.Region)
	if z, ok := strings.CutPrefix(strings.ToLower(n.Zone), region+"-"); ok && region != "" {
		return z
	}
	return ""
}

// OnDemand is the regular price of a size in node n's region, for n's OS.
func (a *Advisor) OnDemand(n recommend.Node, t string) (float64, bool) { return a.price(n, t, false) }

// SpotIn is the Spot price of a size in node n's region (regional on Azure).
func (a *Advisor) SpotIn(n recommend.Node, t string) (float64, bool) { return a.price(n, t, true) }

func (a *Advisor) price(n recommend.Node, t string, spot bool) (float64, bool) {
	if a.disabled || a.prices == nil || n.Region == "" {
		return 0, false
	}
	p, known, pending := a.prices.ListPrice(n.Region, t, n.Windows, spot)
	k := key{strings.ToLower(n.Region), t}
	a.mu.Lock()
	if pending {
		a.pending[k] = true
	} else {
		delete(a.pending, k)
	}
	a.mu.Unlock()
	return p, known
}

// Notes say why Azure rows may be missing: pricing disabled, sizes loading or
// not readable, or prices awaited (aks-recommendations R3, azure-live-skus
// R1, R5).
func (a *Advisor) Notes() []string {
	if a.disabled {
		return []string{"pricing disabled: Azure rows that need a price are not shown"}
	}
	out := append([]string(nil), a.snapshot().notes...)
	if a.prices == nil {
		return append(out, "Azure candidate prices not available yet: rows that need them are not shown")
	}
	a.mu.Lock()
	n := len(a.pending)
	a.mu.Unlock()
	switch {
	case n == 1:
		out = append(out, "waiting for the price of 1 Azure size")
	case n > 1:
		out = append(out, fmt.Sprintf("waiting for the prices of %d Azure sizes", n))
	}
	return out
}

// Generation changes when new prices arrive and when the sizes' state
// changes or a retry falls due, so the screen analyses again
// (azure-live-skus D2).
func (a *Advisor) Generation() uint64 {
	var g uint64
	if a.prices != nil {
		g = a.prices.Generation()
	}
	if a.sizes != nil {
		g += a.sizes.generation()
	}
	return g
}

// pairNotes words the size notes of a snapshot: loading, then one line per
// failure, naming the pools when other pairs are fine (azure-live-skus R5).
func pairNotes(pairs []recommend.Pair, states map[string]pairView) []string {
	loading := false
	type fail struct{ agent, np map[string]bool }
	fails := map[string]*fail{}
	var order []string
	fine := 0
	for _, p := range pairs {
		v := states[pairKeyOf(p.Subscription, p.Region)]
		switch {
		case v.err != nil:
			line := v.err.line()
			f, seen := fails[line]
			if !seen {
				f = &fail{agent: map[string]bool{}, np: map[string]bool{}}
				fails[line] = f
				order = append(order, line)
			}
			for _, x := range p.AgentPools {
				f.agent[x] = true
			}
			for _, x := range p.NodePools {
				f.np[x] = true
			}
		case v.loading:
			loading = true
		default:
			fine++
		}
	}
	var out []string
	if loading {
		out = append(out, "Azure VM sizes: loading")
	}
	for _, line := range order {
		f := fails[line]
		var parts []string
		if fine > 0 || len(order) > 1 || loading {
			if len(f.agent) > 0 {
				parts = append(parts, "agent pools "+strings.Join(sortedKeys(f.agent), ", "))
			}
			if len(f.np) > 0 {
				parts = append(parts, "NodePools "+strings.Join(sortedKeys(f.np), ", "))
			}
		}
		if len(parts) == 0 {
			out = append(out, "Azure VM sizes: "+line)
			continue
		}
		out = append(out, "Azure VM sizes ("+strings.Join(parts, "; ")+"): "+line)
	}
	return out
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
