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
	"strings"
	"sync"

	"github.com/staffops/kube-node-viewer/pkg/recommend"
)

// PriceLister answers the list price of any size in a region, for sizes no
// node runs (aks-recommendations R3, D3). The Provider implements it with
// candidate prices (task T2); until then the session gives the advisor none.
type PriceLister interface {
	// ListPrice is the hourly price of size in region for the OS and the
	// market. known is false when there is no price; pending is true while
	// the answer is awaited (the size is queued, never fetched here)
	ListPrice(region, size string, windows, spot bool) (price float64, known, pending bool)
	// Generation changes whenever new prices are stored
	Generation() uint64
}

// Advisor answers the recommendations for Azure nodes from the size catalog
// and a PriceLister (aks-recommendations D2). Safe for concurrent use: the
// analysis runs off the UI goroutine.
type Advisor struct {
	prices   PriceLister
	disabled bool

	// pending holds the sizes this analysis found awaited, by region and
	// size: the provider caches all meters of a size together (regular,
	// Spot, Windows), so one answer settles every variant
	mu      sync.Mutex
	pending map[key]bool
}

// Begin starts an analysis: the sizes awaited are counted afresh, so a size
// no row asks for any more leaves the note (aks-recommendations R3).
func (a *Advisor) Begin() {
	a.mu.Lock()
	a.pending = map[key]bool{}
	a.mu.Unlock()
}

// NewAdvisor makes an advisor over prices; nil prices with disabled means
// --disable-pricing: lookups answer "no price" and nothing is queued (R3).
func NewAdvisor(prices PriceLister, disabled bool) *Advisor {
	return &Advisor{prices: prices, disabled: disabled, pending: map[key]bool{}}
}

// Spec is the size from the catalog.
func (a *Advisor) Spec(t string) (recommend.Spec, bool) {
	s, ok := LookupSize(t)
	if !ok {
		return recommend.Spec{}, false
	}
	return recommend.Spec{VCPU: s.VCPU, MemGiB: s.MemGiB, Arch: s.Arch, Family: s.Family, Constrained: s.Constrained}, true
}

// Types lists the catalog's sizes.
func (a *Advisor) Types() []string { return SizeNames() }

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

// Notes say why Azure rows may be missing: pricing disabled, or prices
// awaited (aks-recommendations R3).
func (a *Advisor) Notes() []string {
	switch {
	case a.disabled:
		return []string{"pricing disabled: Azure rows that need a price are not shown"}
	case a.prices == nil:
		return []string{"Azure candidate prices not available yet: rows that need them are not shown"}
	}
	a.mu.Lock()
	n := len(a.pending)
	a.mu.Unlock()
	switch {
	case n == 1:
		return []string{"waiting for the price of 1 Azure size"}
	case n > 1:
		return []string{fmt.Sprintf("waiting for the prices of %d Azure sizes", n)}
	}
	return nil
}

// Generation changes when new prices arrive, so the screen analyses again.
func (a *Advisor) Generation() uint64 {
	if a.prices == nil {
		return 0
	}
	return a.prices.Generation()
}
