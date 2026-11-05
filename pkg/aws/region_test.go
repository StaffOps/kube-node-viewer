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
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/staffops/kube-node-viewer/pkg/model"
)

// Tests for specs/fix-aws-pricing-region.

// recorder builds static providers and records each region built and its
// callback.
type recorder struct {
	mu        sync.Mutex
	regions   []string
	callbacks []func()
}

func (r *recorder) build(region string, onUpdate func()) *pricingProvider {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.regions = append(r.regions, region)
	r.callbacks = append(r.callbacks, onUpdate)
	p := newStaticPricingProvider(region)
	// a price per region, so routing is visible (the built-in tables hold
	// only a few regions, and fall back to us-east-1's)
	p.onDemandPrices = map[ec2types.InstanceType]float64{"m5.large": regionPrice(region)}
	p.spotPrices["m5.large"] = zonalPricing{prices: map[string]float64{region + "a": regionPrice(region) / 2}}
	return p
}

// regionPrice is a made-up price distinct per region.
func regionPrice(region string) float64 {
	sum := 0
	for _, c := range region {
		sum = sum*31 + int(c)
	}
	return float64(sum%1000+1) / 1000
}

func awsNode(name, providerID string, labels map[string]string) *model.Node {
	n := &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{
		v1.LabelInstanceTypeStable:   "m5.large",
		"karpenter.sh/capacity-type": "on-demand",
	}}, Spec: v1.NodeSpec{ProviderID: providerID}}
	for k, v := range labels {
		n.Labels[k] = v
	}
	return model.NewNode(n)
}

// fix-aws-pricing-region R1, R2 and R3: each AWS node priced in its own
// region, three regions here; other nodes in the SDK's, with no provider.
func TestByRegionPricesEachAWSNodeInItsRegion(t *testing.T) {
	r := &recorder{}
	b := newByRegion("us-west-2", r.build)
	nodes := []*model.Node{
		// R1: the region label
		awsNode("a", "aws:///eu-west-1a/i-0abc", map[string]string{v1.LabelTopologyRegion: "eu-west-1"}),
		// R1: the zone label's region
		awsNode("b", "aws:///ap-southeast-2b/i-0abd", map[string]string{v1.LabelTopologyZone: "ap-southeast-2b"}),
		// R1: the zone in the provider ID
		awsNode("c", "aws:///us-east-1c/i-0abe", nil),
		// R2: not AWS nodes, or no AWS region
		awsNode("d", "kind://docker/kind/d", map[string]string{v1.LabelTopologyRegion: "eu-west-1"}),
		awsNode("e", "gce://p/us-central1-a/e", map[string]string{v1.LabelTopologyRegion: "us-central1"}),
		awsNode("f", "aws:///dc1-a/i-0abf", map[string]string{v1.LabelTopologyRegion: "dc1"}),
		awsNode("g", "", nil),
	}
	for _, n := range nodes {
		b.NodePrice(n)
	}
	want := []string{"eu-west-1", "ap-southeast-2", "us-east-1", "us-west-2"}
	if !reflect.DeepEqual(r.regions, want) {
		t.Errorf("built %v, want %v", r.regions, want)
	}
	// the price is that region's static list price
	for _, n := range nodes[:3] {
		got, ok := b.NodePrice(n)
		region := nodeRegion(n)
		if wantPrice := regionPrice(region); !ok || got != wantPrice {
			t.Errorf("%s: %v %v, want %v (%s)", n.Name(), got, ok, wantPrice, region)
		}
	}
}

func TestByRegionBuildsNothingBeforeANodeIsPriced(t *testing.T) {
	r := &recorder{}
	b := newByRegion("us-west-2", r.build)
	if _, ok := b.OnDemandPrice("m5.large"); !ok {
		t.Error("no static on-demand price before any node")
	}
	b.SpotPriceInZone("m5.large", "eu-west-1a")
	if len(r.regions) != 0 {
		t.Errorf("built %v before any node was priced", r.regions)
	}
}

// D1: callbacks stored before builds reach each provider once, and a
// callback that reprices (calls NodePrice) does not deadlock.
func TestByRegionOnUpdate(t *testing.T) {
	r := &recorder{}
	b := newByRegion("us-west-2", r.build)
	calls := 0
	b.OnUpdate(func() {
		calls++
		b.NodePrice(awsNode("a", "aws:///eu-west-1a/i-0abc", nil))
	})
	b.NodePrice(awsNode("a", "aws:///eu-west-1a/i-0abc", nil))
	b.NodePrice(awsNode("b", "aws:///eu-central-1a/i-0abd", nil))
	done := make(chan struct{})
	go func() {
		for _, f := range r.callbacks {
			f()
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("a callback that calls NodePrice deadlocked")
	}
	if calls != len(r.regions) || calls != 2 {
		t.Errorf("%d calls for %d providers, want 2", calls, len(r.regions))
	}
}

// fix-aws-pricing-region R5: spot by the zone's region, on-demand from the region with most nodes.
func TestByRegionAdvisorPrices(t *testing.T) {
	r := &recorder{}
	b := newByRegion("us-west-2", r.build)
	for _, n := range []*model.Node{
		awsNode("a", "aws:///eu-west-1a/i-1", nil),
		awsNode("b", "aws:///eu-west-1a/i-2", nil),
		awsNode("c", "aws:///ap-south-1a/i-3", nil),
	} {
		b.NodePrice(n)
	}
	if got, ok := b.SpotPriceInZone("m5.large", "ap-south-1a"); !ok || got != regionPrice("ap-south-1")/2 {
		t.Errorf("spot in ap-south-1a: %v %v", got, ok)
	}
	if _, ok := b.SpotPriceInZone("m5.large", "sa-east-1a"); ok {
		t.Error("spot in a region with no provider answered")
	}
	if got, _ := b.OnDemandPrice("m5.large"); got != regionPrice("eu-west-1") {
		t.Errorf("on-demand %v, want eu-west-1's", got)
	}
	// a tie goes to the alphabetically first region
	b.NodePrice(awsNode("d", "aws:///ap-south-1a/i-4", nil))
	if got, _ := b.OnDemandPrice("m5.large"); got != regionPrice("ap-south-1") {
		t.Errorf("on-demand on a tie %v, want ap-south-1's", got)
	}
}

// fix-aws-pricing-region R1, R2: which zones and regions are AWS ones.
func TestZoneRegion(t *testing.T) {
	for zone, want := range map[string]string{
		"eu-west-1a":              "eu-west-1",
		"us-gov-west-1a":          "us-gov-west-1",
		"cn-north-1a":             "cn-north-1",
		"ap-southeast-2a":         "ap-southeast-2",
		"us-east-1-bos-1a":        "us-east-1",
		"us-east-1-wl1-bos-wlz-1": "us-east-1",
		"us-central1-a":           "",
		"dc1-a":                   "",
		"1":                       "",
		"":                        "",
	} {
		if got := zoneRegion(zone); got != want {
			t.Errorf("zoneRegion(%q) = %q, want %q", zone, got, want)
		}
	}
	for r, want := range map[string]bool{
		"us-east-1": true, "us-gov-west-1": true, "cn-northwest-1": true,
		"dc-east": false, "us-central1": false, "dc1": false, "eu-west-1a": false, "": false,
	} {
		if knownRegion(r) != want {
			t.Errorf("knownRegion(%q) = %v", r, !want)
		}
	}
}

// fix-aws-pricing-region R4: with no SDK region, the EC2 client (spot) uses the on-demand lookup
// region.
func TestPricingProviderWithoutRegionUsesOneRegion(t *testing.T) {
	// no AWS config, credentials or instance metadata from this machine
	for k, v := range map[string]string{
		"AWS_REGION": "", "AWS_DEFAULT_REGION": "", "AWS_PROFILE": "",
		"AWS_CONFIG_FILE": "/dev/null", "AWS_SHARED_CREDENTIALS_FILE": "/dev/null",
		"AWS_EC2_METADATA_DISABLED": "true",
	} {
		t.Setenv(k, v)
	}
	ctx, cancel := context.WithCancel(context.Background())
	// cancelled: the one refresh the goroutine runs first fails at once,
	// with no network call, and the loop stops
	cancel()
	p := newPricingProvider(ctx, aws.Config{}, nil)
	if p.region != defaultLookupRegion || p.ec2Client.Options().Region != defaultLookupRegion {
		t.Errorf("lookup region %q, EC2 region %q, want both %q", p.region, p.ec2Client.Options().Region, defaultLookupRegion)
	}
}

// A Karpenter node is counted once, from its NodeClaim (instance id as the
// name) to the Node, since both carry the same provider ID.
func TestByRegionCountsANodeOnce(t *testing.T) {
	r := &recorder{}
	b := newByRegion("us-west-2", r.build)
	claim := awsNode("", "aws:///ap-south-1a/i-0aaa", nil)
	b.NodePrice(claim)
	b.NodePrice(awsNode("ip-a", "aws:///ap-south-1a/i-0aaa", nil))
	b.NodePrice(awsNode("ip-b", "aws:///eu-west-1a/i-0bbb", nil))
	b.mu.Lock()
	counts, main := b.counts["ap-south-1"], b.main
	b.mu.Unlock()
	if counts != 1 || main != "ap-south-1" {
		t.Errorf("ap-south-1 counted %d, main %q; want 1 and ap-south-1 (tie, alphabetical)", counts, main)
	}
}

// Providers' refreshes (fanout) race informer NodePrice calls; run under
// -race.
func TestByRegionConcurrent(t *testing.T) {
	r := &recorder{}
	b := newByRegion("us-west-2", r.build)
	b.OnUpdate(func() { b.NodePrice(awsNode("x", "aws:///eu-west-1a/i-x", nil)) })
	regions := []string{"eu-west-1", "eu-central-1", "ap-south-1", "us-east-1"}
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			region := regions[i%len(regions)]
			for j := range 50 {
				b.NodePrice(awsNode("", "aws:///"+region+"a/i-"+string(rune('a'+j%26)), nil))
				b.OnDemandPrice("m5.large")
				b.SpotPriceInZone("m5.large", region+"a")
			}
		}()
		go func() {
			defer wg.Done()
			for range 20 {
				b.fanout()
			}
		}()
	}
	wg.Wait()
}
