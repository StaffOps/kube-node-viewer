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

import "strings"

// the Provider answers candidate prices for the Recommendations advisor; a
// change of either signature fails the build here, not silently in the
// session (aks-recommendations T2)
var _ PriceLister = (*Provider)(nil)

// ListPrice is the hourly list price of any size in region, for the OS and
// market, from the same cache, queue and fetch loop as NodePrice
// (aks-recommendations R3, D3). A miss queues the size (batched, at most
// maxSizesPerRequest a request) and answers pending; the answer arrives
// through OnUpdate. A size the API doesn't know is cached as having no price
// and not asked again.
func (p *Provider) ListPrice(region, size string, windows, spot bool) (price float64, known, pending bool) {
	if region == "" || size == "" {
		return 0, false, false
	}
	k := key{strings.ToLower(region), size}
	if !validName(k.region) || !validName(size) {
		return 0, false, false // never put it in a filter (fix-azure-price-batch R5)
	}
	p.mu.Lock()
	sp, ok := p.prices[k]
	if !ok {
		p.wanted[k] = true
	}
	p.mu.Unlock()
	if !ok {
		p.startOnce.Do(func() { go p.run() })
		select {
		case p.wake <- struct{}{}:
		default:
		}
		return 0, false, true
	}
	switch {
	case windows && spot:
		return sp.windowsSpot, sp.hasWindowsSpot, false
	case windows:
		return sp.windows, sp.hasWindows, false
	case spot:
		return sp.spot, sp.hasSpot, false
	}
	return sp.regular, sp.hasRegular, false
}

// Generation changes whenever answers are stored: the cache only grows, one
// entry per (region, size) answered, so its size is the generation.
func (p *Provider) Generation() uint64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return uint64(len(p.prices))
}
