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

package main

import (
	"context"
	"strings"
	"testing"

	"github.com/staffops/kube-node-viewer/pkg/aws"
	"github.com/staffops/kube-node-viewer/pkg/azure"
	"github.com/staffops/kube-node-viewer/pkg/model"
	"github.com/staffops/kube-node-viewer/pkg/pricing"
	"github.com/staffops/kube-node-viewer/pkg/recommend"
)

// interactive-tui R10: each connection gets the pricing of its server.
// aks-pricing R2: with --disable-pricing no Azure provider is built, so
// nothing is fetched from Azure.
// Tests for specs/fix-aws-pricing-region: which AWS provider each kind of
// API server gets, and that the Recommendations advisor still finds it.
func TestSessionPricingPerServer(t *testing.T) {
	// no AWS config, credentials or instance metadata from this machine
	for k, v := range map[string]string{
		"AWS_REGION": "", "AWS_DEFAULT_REGION": "", "AWS_PROFILE": "",
		"AWS_CONFIG_FILE": "/dev/null", "AWS_SHARED_CREDENTIALS_FILE": "/dev/null",
		"AWS_EC2_METADATA_DISABLED": "true",
	} {
		t.Setenv(k, v)
	}
	const (
		eks   = "https://0A1B2C3D4E5F.gr7.us-east-1.eks.amazonaws.com"
		aks   = "https://example-aks.hcp.eastus2.azmk8s.io:443"
		proxy = "http://127.0.0.1:8001"
	)
	for _, tt := range []struct {
		name       string
		server     string
		disable    bool
		wantRegion string
		regional   bool
	}{
		{"EKS URL: one provider, the URL's region", eks, false, "us-east-1", false},
		{"AKS: static, no AWS lookup", aks, false, "", false},
		{"other server: per node region", proxy, false, "", true},
		{"pricing off, EKS URL: static", eks, true, "us-east-1", false},
		{"pricing off, AKS: static", aks, true, "", false},
		{"pricing off, other server: static, as before", proxy, true, "", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel() // providers stop before any refresh
			s := &sessions{flags: Flags{DisablePricing: tt.disable}}
			p, region, err := s.pricing(ctx, tt.server)
			if err != nil {
				t.Fatal(err)
			}
			if region != tt.wantRegion {
				t.Errorf("region %q, want %q", region, tt.wantRegion)
			}
			awsP := p
			if b, ok := p.(pricing.ByCloud); ok {
				awsP = b.Clouds["aws"]
			}
			if _, regional := awsP.(*aws.ByRegion); regional != tt.regional {
				t.Errorf("AWS provider %T, per-region %v, want %v", awsP, regional, tt.regional)
			}
			if tt.disable {
				switch p.(type) {
				case pricing.ByCloud, *pricing.ByCloud:
					t.Errorf("pricing off, but %T can reach the Azure prices API", p)
				}
			}
			adv, ok := newAdvisor(p, tt.disable).(model.AdvisorsByCloud)
			if !ok || adv["aws"] == nil {
				t.Error("no Recommendations advisor for this provider")
			}
			// aks-recommendations R3: an Azure advisor always; with pricing
			// off it asks for no price and says so
			az, ok := adv["azure"].(*azure.Advisor)
			if !ok {
				t.Fatalf("no Azure advisor: %T", adv["azure"])
			}
			notes := strings.Join(az.Notes(), " ")
			if tt.disable != strings.Contains(notes, "pricing disabled") {
				t.Errorf("pricing off %v, notes %q", tt.disable, notes)
			}
			if _, ok := az.OnDemand(recommend.Node{Cloud: "azure", Region: "eastus"}, "Standard_D4s_v5"); ok {
				t.Error("a candidate price before any answer")
			}
			if !tt.disable && !strings.Contains(strings.Join(az.Notes(), " "), "waiting for the price of 1 Azure size") {
				t.Errorf("the candidate was not queued with the Azure provider: notes %q", az.Notes())
			}
			// aks-recommendations T2: the seam can't fail silently: the
			// ByCloud Azure provider lists candidate prices
			if b, ok := p.(pricing.ByCloud); ok {
				if _, ok := b.Clouds["azure"].(azure.PriceLister); !ok {
					t.Errorf("the Azure provider %T doesn't list candidate prices", b.Clouds["azure"])
				}
			}
		})
	}
}
