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

package aws

import (
	"context"
	"math"
	"regexp"
	"slices"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/staffops/kube-node-viewer/pkg/model"
)

// ByRegion prices each AWS node in its own region, for clusters whose API
// server URL names none (fix-aws-pricing-region D1). A region's provider is
// built the first time one of its nodes is priced; other nodes go to the
// fallback region, the SDK's.
type ByRegion struct {
	build    func(region string, onUpdate func()) *pricingProvider
	fallback string

	mu        sync.Mutex
	providers map[string]*pricingProvider
	// node (provider ID, which a Karpenter node keeps from NodeClaim to Node)
	// → region, and nodes per region, for the main region. Deleted nodes are
	// not forgotten: the count is of nodes seen this connection.
	nodes    map[string]string
	counts   map[string]int
	main     string
	zones    map[string]string // zone → region, from priced nodes
	onUpdate []func()
}

// NewRegionalPricingProvider looks prices up in each node's region with
// the SDK config cfg; nodes with no AWS region use cfg's region (or
// us-west-2).
func NewRegionalPricingProvider(ctx context.Context, cfg aws.Config) *ByRegion {
	fallback := cfg.Region
	if fallback == "" {
		fallback = defaultLookupRegion
	}
	return newByRegion(fallback, func(region string, onUpdate func()) *pricingProvider {
		c := cfg.Copy()
		c.Region = region
		return newPricingProvider(ctx, c, onUpdate)
	})
}

func newByRegion(fallback string, build func(string, func()) *pricingProvider) *ByRegion {
	return &ByRegion{
		build:     build,
		fallback:  fallback,
		providers: map[string]*pricingProvider{},
		nodes:     map[string]string{},
		counts:    map[string]int{},
		zones:     map[string]string{},
	}
}

// awsRegion is the shape of an AWS region name: us-east-1, us-gov-west-1,
// cn-north-1.
var awsRegion = regexp.MustCompile(`^[a-z]{2}(-gov)?-[a-z]+-\d+$`)

// zonePrefix is the region at the start of an AWS zone name, also for local
// zones (us-east-1-bos-1a) and Wavelength zones (us-east-1-wl1-bos-wlz-1).
var zonePrefix = regexp.MustCompile(`^[a-z]{2}(-[a-z]+)+-\d+`)

// knownRegion: the shape of an AWS region name. The built-in price tables
// cover only a few regions, so they cannot tell real ones apart; a label that
// only looks like a region (us-east-9) gets a provider whose lookups find
// nothing.
func knownRegion(r string) bool { return awsRegion.MatchString(r) }

// zoneRegion is the region of an AWS zone name, or "".
func zoneRegion(zone string) string {
	if r := zonePrefix.FindString(zone); knownRegion(r) {
		return r
	}
	return ""
}

// nodeRegion is the AWS region a node is priced in: its region label, else
// its zone's region, for AWS nodes only; "" for the fallback.
func nodeRegion(n *model.Node) string {
	if n.Cloud() != "aws" {
		return ""
	}
	if r := n.Region(); knownRegion(r) {
		return r
	}
	return zoneRegion(n.Zone())
}

// provider returns the region's provider, building it on first use; the
// caller holds b.mu. Building does no network I/O.
func (b *ByRegion) provider(region string) *pricingProvider {
	if p, ok := b.providers[region]; ok {
		return p
	}
	p := b.build(region, b.fanout)
	b.providers[region] = p
	return p
}

// fanout calls every OnUpdate callback, without b.mu held: a callback
// reprices the nodes, which calls NodePrice.
func (b *ByRegion) fanout() {
	b.mu.Lock()
	funcs := slices.Clone(b.onUpdate)
	b.mu.Unlock()
	for _, f := range funcs {
		f()
	}
}

func (b *ByRegion) NodePrice(n *model.Node) (float64, bool) {
	region := nodeRegion(n)
	b.mu.Lock()
	if region == "" {
		region = b.fallback
	} else {
		b.count(n, region)
		if z := n.Zone(); z != "" {
			b.zones[z] = region
		}
	}
	p := b.provider(region)
	b.mu.Unlock()
	return p.NodePrice(n)
}

// OnUpdate keeps f for every provider, built now or later.
func (b *ByRegion) OnUpdate(f func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onUpdate = append(b.onUpdate, f)
}

// count records the node's region and keeps the main region: the one with
// the most nodes, ties broken by name. The caller holds b.mu.
func (b *ByRegion) count(n *model.Node, region string) {
	key := n.ProviderID()
	if key == "" {
		key = n.Name()
	}
	old, seen := b.nodes[key]
	if seen && old == region {
		return
	}
	if seen {
		b.counts[old]--
	}
	b.nodes[key] = region
	b.counts[region]++
	b.main = ""
	for r, c := range b.counts {
		if b.main == "" || c > b.counts[b.main] || (c == b.counts[b.main] && r < b.main) {
			b.main = r
		}
	}
}

// OnDemandPrice answers the Recommendations screen from the main region
// (fix-aws-pricing-region R5), else the fallback's provider, else the
// fallback's static table, without building a provider for it.
func (b *ByRegion) OnDemandPrice(t ec2types.InstanceType) (float64, bool) {
	b.mu.Lock()
	p := b.providers[b.main]
	if p == nil {
		p = b.providers[b.fallback]
	}
	b.mu.Unlock()
	if p == nil {
		price, ok := getStaticPrices(b.fallback)[t]
		return price, ok
	}
	return p.OnDemandPrice(t)
}

// SpotPriceInZone answers from the zone's region, never a default.
func (b *ByRegion) SpotPriceInZone(t ec2types.InstanceType, zone string) (float64, bool) {
	b.mu.Lock()
	region, ok := b.zones[zone]
	if !ok {
		region = zoneRegion(zone)
	}
	p := b.providers[region]
	b.mu.Unlock()
	if p == nil {
		return math.NaN(), false
	}
	return p.SpotPriceInZone(t, zone)
}
