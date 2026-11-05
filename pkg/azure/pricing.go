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

// Package azure prices AKS nodes from the public Azure Retail Prices API
// (aks-pricing R1, R2). No Azure credentials are used.
package azure

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/staffops/kube-node-viewer/pkg/model"
)

// Endpoint is the Retail Prices API.
const Endpoint = "https://prices.azure.com/api/retail/prices"

const (
	requestTimeout = 20 * time.Second
	retryAfter     = time.Minute
	maxPages       = 20
)

// sizePrices are the prices of one VM size in one region; the has* fields
// tell which the API gave. All false: the API has no such size there.
type sizePrices struct {
	regular, spot, windows, windowsSpot             float64
	hasRegular, hasSpot, hasWindows, hasWindowsSpot bool
}

type key struct{ region, size string }

// Provider is a pricing.Provider for Azure nodes.
type Provider struct {
	ctx      context.Context
	endpoint string
	client   *http.Client
	now      func() time.Time
	retry    time.Duration

	mu        sync.Mutex
	prices    map[key]*sizePrices
	wanted    map[key]bool
	retryAt   map[string]time.Time // region → not asked again before (fix-azure-retry R2)
	failed    map[string]bool      // regions whose failure was logged and not yet resolved
	cutShort  map[string]bool      // regions whose cut-short answer was logged
	onUpdate  []func()
	wake      chan struct{}
	startOnce sync.Once
}

// NewProvider makes a provider that fetches from endpoint until ctx ends.
// Nothing is fetched until a node asks for a price.
func NewProvider(ctx context.Context, endpoint string) *Provider {
	return &Provider{
		ctx: ctx, endpoint: endpoint,
		client: &http.Client{Timeout: requestTimeout},
		now:    time.Now,
		retry:  retryAfter,
		prices: map[key]*sizePrices{}, wanted: map[key]bool{}, cutShort: map[string]bool{},
		retryAt: map[string]time.Time{}, failed: map[string]bool{},
		wake: make(chan struct{}, 1),
	}
}

// OnUpdate registers a function called after new prices arrive.
func (p *Provider) OnUpdate(f func()) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onUpdate = append(p.onUpdate, f)
}

// NodePrice is the node's hourly list price; on a miss the size is queued and
// the price arrives through OnUpdate.
func (p *Provider) NodePrice(n *model.Node) (float64, bool) {
	region, size := n.Region(), n.InstanceType()
	if region == "" || size == "" {
		return 0, false
	}
	spot := n.CapacityType() == model.CapacitySpot
	windows := n.LabelValue("kubernetes.io/os") == "windows"
	k := key{strings.ToLower(region), size}

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
		return 0, false
	}
	switch {
	case windows && spot:
		return sp.windowsSpot, sp.hasWindowsSpot
	case windows:
		return sp.windows, sp.hasWindows
	case spot:
		return sp.spot, sp.hasSpot
	}
	return sp.regular, sp.hasRegular
}

// run fetches wanted sizes whenever woken, one request per due region, and
// wakes itself when the earliest deferred region is due again.
func (p *Provider) run() {
	var due <-chan time.Time
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-p.wake:
		case <-due:
		}
		due = nil
		if next := p.fetchWanted(); !next.IsZero() {
			due = time.After(next.Sub(p.now()))
		}
	}
}

// failurePrefix starts the log line of a failed request for region; a
// resolution line repeats it after model.ResolvedPrefix (fix-azure-retry R3).
func failurePrefix(region string) string {
	return "updating Azure pricing for " + region
}

// fetchWanted asks for the wanted sizes of every region whose retry time has
// passed, and returns the earliest retry time of a region still waiting
// (zero: none).
func (p *Provider) fetchWanted() time.Time {
	p.mu.Lock()
	now := p.now()
	byRegion := map[string][]string{}
	var next time.Time
	for k := range p.wanted {
		if at := p.retryAt[k.region]; at.After(now) {
			if next.IsZero() || at.Before(next) {
				next = at
			}
			continue
		}
		byRegion[k.region] = append(byRegion[k.region], k.size)
	}
	p.mu.Unlock()
	got := false
	regions := make([]string, 0, len(byRegion))
	for region := range byRegion {
		regions = append(regions, region)
	}
	sort.Strings(regions) // a fixed order, so a pass is the same every time
	for _, region := range regions {
		sizes := byRegion[region]
		sort.Strings(sizes)
		found, err := p.fetch(region, sizes)
		p.mu.Lock()
		if err != nil {
			delay, note := p.retry, ""
			var se *statusError
			if errors.As(err, &se) {
				delay, note = retryDelay(se.code, se.header, p.now(), p.retry)
			}
			at := p.now().Add(delay)
			p.retryAt[region] = at
			p.failed[region] = true
			if next.IsZero() || at.Before(next) {
				next = at
			}
			p.mu.Unlock()
			if note != "" {
				log.Printf("%s, %s (%s)", failurePrefix(region), err, note)
			} else {
				log.Printf("%s, %s", failurePrefix(region), err)
			}
			continue
		}
		delete(p.retryAt, region)
		resolved := p.failed[region]
		delete(p.failed, region)
		for _, size := range sizes {
			k := key{region, size}
			sp := found[size]
			if sp == nil {
				// the API has no such size here: don't ask again
				sp = &sizePrices{}
			}
			p.prices[k] = sp
			delete(p.wanted, k)
		}
		p.mu.Unlock()
		if resolved {
			log.Printf("%s%s", model.ResolvedPrefix, failurePrefix(region))
		}
		got = true
	}
	if got {
		p.mu.Lock()
		fns := append([]func(){}, p.onUpdate...)
		p.mu.Unlock()
		for _, f := range fns {
			f()
		}
	}
	return next
}

type retailItem struct {
	CurrencyCode       string    `json:"currencyCode"`
	UnitPrice          float64   `json:"unitPrice"`
	ArmSkuName         string    `json:"armSkuName"`
	MeterName          string    `json:"meterName"`
	ProductName        string    `json:"productName"`
	UnitOfMeasure      string    `json:"unitOfMeasure"`
	Type               string    `json:"type"`
	EffectiveStartDate time.Time `json:"effectiveStartDate"`
}

type retailPage struct {
	Items        []retailItem `json:"Items"`
	NextPageLink string       `json:"NextPageLink"`
}

// fetch asks for the sizes in one region and returns their prices by size.
func (p *Provider) fetch(region string, sizes []string) (map[string]*sizePrices, error) {
	var or []string
	for _, s := range sizes {
		or = append(or, fmt.Sprintf("armSkuName eq '%s'", strings.ReplaceAll(s, "'", "''")))
	}
	filter := fmt.Sprintf("serviceName eq 'Virtual Machines' and priceType eq 'Consumption' and armRegionName eq '%s' and (%s)",
		strings.ReplaceAll(region, "'", "''"), strings.Join(or, " or "))
	next := p.endpoint + "?" + url.Values{"$filter": {filter}}.Encode()

	type pick struct {
		price float64
		at    time.Time
	}
	best := map[string]map[string]pick{} // size → variant → latest price
	now := p.now()
	pages := 0
	for ; next != "" && pages < maxPages; pages++ {
		page, err := p.get(next)
		if err != nil {
			return nil, err
		}
		for _, it := range page.Items {
			variant, ok := variantOf(it)
			if !ok || it.EffectiveStartDate.After(now) {
				continue
			}
			if best[it.ArmSkuName] == nil {
				best[it.ArmSkuName] = map[string]pick{}
			}
			// a later date wins; on the same date the lower price, so the
			// API's order never decides (fix-azure-price-product R3)
			cur, seen := best[it.ArmSkuName][variant]
			if !seen || it.EffectiveStartDate.After(cur.at) ||
				(it.EffectiveStartDate.Equal(cur.at) && it.UnitPrice < cur.price) {
				best[it.ArmSkuName][variant] = pick{it.UnitPrice, it.EffectiveStartDate}
			}
		}
		next = page.NextPageLink
	}
	if next != "" {
		// defensive: a filtered answer is one page (fix-azure-price-product R4)
		p.mu.Lock()
		first := !p.cutShort[region]
		p.cutShort[region] = true
		p.mu.Unlock()
		if first {
			log.Printf("Azure pricing for %s: answer cut short after %d pages, some sizes have no price", region, pages)
		}
	}
	out := map[string]*sizePrices{}
	for size, variants := range best {
		sp := &sizePrices{}
		for v, pk := range variants {
			switch v {
			case "regular":
				sp.regular, sp.hasRegular = pk.price, true
			case "spot":
				sp.spot, sp.hasSpot = pk.price, true
			case "windows":
				sp.windows, sp.hasWindows = pk.price, true
			case "windowsSpot":
				sp.windowsSpot, sp.hasWindowsSpot = pk.price, true
			}
		}
		out[size] = sp
	}
	return out, nil
}

// variantOf names the price an item is: regular, spot, windows or
// windowsSpot; false for items never used (Low Priority, other units, Cloud
// Services).
func variantOf(it retailItem) (string, bool) {
	if it.Type != "Consumption" || it.UnitOfMeasure != "1 Hour" || it.CurrencyCode != "USD" ||
		strings.Contains(it.MeterName, "Low Priority") || isCloudServices(it.ProductName) {
		return "", false
	}
	spot := strings.HasSuffix(it.MeterName, " Spot")
	windows := isWindows(it.ProductName)
	switch {
	case windows && spot:
		return "windowsSpot", true
	case windows:
		return "windows", true
	case spot:
		return "spot", true
	}
	return "regular", true
}

// isCloudServices tells a Cloud Services product ("Easv5 Series
// CloudServices", "Dasv5 Series Cloud Services"): the same sizes at the
// Windows rate, never a node's price (fix-azure-price-product R1). Not
// logged: it is expected, and logs reach the header.
func isCloudServices(product string) bool {
	return strings.Contains(strings.ToLower(strings.ReplaceAll(product, " ", "")), "cloudservices")
}

// isWindows tells a Windows product: "Windows" in the name, or "Win" as a
// word ("Virtual Machines NCCadsv5 Srs Win"; fix-azure-price-product R2).
func isWindows(product string) bool {
	if strings.Contains(product, "Windows") {
		return true
	}
	for _, w := range strings.FieldsFunc(product, func(r rune) bool { return r == ' ' || r == '-' }) {
		if w == "Win" {
			return true
		}
	}
	return false
}

func (p *Provider) get(u string) (retailPage, error) {
	ctx, cancel := context.WithTimeout(p.ctx, requestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return retailPage{}, err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return retailPage{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return retailPage{}, &statusError{code: resp.StatusCode, status: resp.Status, header: resp.Header}
	}
	var page retailPage
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return retailPage{}, fmt.Errorf("retail prices API: %w", err)
	}
	return page, nil
}
