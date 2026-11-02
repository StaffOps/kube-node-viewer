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
	"testing"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
)

type ec2InstanceType = ec2types.InstanceType

func TestGeneratedCatalogLoads(t *testing.T) {
	if len(InstanceTypes()) < 500 {
		t.Fatalf("catalog has %d types", len(InstanceTypes()))
	}
	s, ok := LookupInstance("m7g.xlarge")
	if !ok || s.VCPU != 4 || s.MemoryGiB != 16 || s.Arch != "arm64" || !s.CurrentGeneration {
		t.Errorf("m7g.xlarge: %+v %v", s, ok)
	}
	if s, _ := LookupInstance("m6i.large"); s.Arch != "x86_64" || s.Class != "General purpose" {
		t.Errorf("m6i.large: %+v", s)
	}
}

func TestPriceListAttributes(t *testing.T) {
	a := PriceListAttributes{InstanceType: "x9.large", VCPU: "2", Memory: "1,024 GiB", PhysicalProcessor: "AWS Graviton4 Processor", InstanceFamily: "Memory optimized", CurrentGeneration: "Yes"}
	s, ok := a.Spec()
	if !ok || s.MemoryGiB != 1024 || s.Arch != "arm64" || !s.CurrentGeneration {
		t.Errorf("%+v %v", s, ok)
	}
	if _, ok := (PriceListAttributes{VCPU: "NA", Memory: "8 GiB"}).Spec(); ok {
		t.Error("a product without vCPU was accepted")
	}
}

func TestCatalogLearnsFromPriceListPages(t *testing.T) {
	p := &pricingProvider{region: "us-east-1"}
	page := `{"product":{"attributes":{"instanceType":"zz9.large","vcpu":"2","memory":"8 GiB","physicalProcessor":"Intel Xeon","instanceFamily":"General purpose","currentGeneration":"Yes"}},"terms":{"OnDemand":{"a":{"priceDimensions":{"b":{"pricePerUnit":{"USD":"0.1000"}}}}}}}`
	out := map[ec2InstanceType]float64{}
	p.processInstancePricingPage(&pricing.GetProductsOutput{PriceList: []string{page}}, out)
	if s, ok := LookupInstance("zz9.large"); !ok || s.VCPU != 2 {
		t.Errorf("not learned: %+v %v", s, ok)
	}
	if out["zz9.large"] != 0.1 {
		t.Errorf("price %v", out["zz9.large"])
	}
}

func TestSpotPriceInZoneNeverDefaults(t *testing.T) {
	p := &pricingProvider{spotPrices: map[ec2InstanceType]zonalPricing{
		"m6i.xlarge": {defaultPrice: 0, prices: map[string]float64{"us-east-1a": 0.07}},
	}}
	if v, ok := p.SpotPriceInZone("m6i.xlarge", "us-east-1a"); !ok || v != 0.07 {
		t.Errorf("own zone: %v %v", v, ok)
	}
	if _, ok := p.SpotPriceInZone("m6i.xlarge", "us-east-1b"); ok {
		t.Error("a zone without data priced")
	}
	if v, ok := p.SpotPrice("m6i.xlarge", "us-east-1b"); !ok || v != 0 {
		t.Errorf("SpotPrice keeps its old fallback: %v %v", v, ok)
	}
}
