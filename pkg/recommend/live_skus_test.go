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

package recommend

import (
	"strings"
	"testing"
	"time"
)

// azure-live-skus R8: Azure rightsize, generation and arm math names the
// source of the sizes, after the "after the move" line or the target line;
// AWS math doesn't.
func TestLiveSizesLine(t *testing.T) {
	r := Analyze(azInput(azFixtureNodes()))
	rs := find(t, r, Rightsize, "aks-userpool-12345678-vmss000003")
	if !strings.HasPrefix(rs.Math[3], "after the move") || rs.Math[4] != liveSizes {
		t.Errorf("rightsize math: %q", rs.Math)
	}
	for _, x := range kinds(r, Graviton) {
		if x.Math[1] != liveSizes {
			t.Errorf("arm math: %q", x.Math)
		}
	}
	in := azInput(azFixtureNodes())
	in.Nodes[7].Type, in.Nodes[8].Type = "Standard_D4s_v4", "Standard_D4s_v4"
	saveGen := azureGeneration["dsv4"]
	defer func() { azureGeneration["dsv4"] = saveGen }()
	azureGeneration["dsv4"] = "dsv5"
	for _, x := range kinds(Analyze(in), Generation) {
		if x.Math[1] != liveSizes {
			t.Errorf("generation math: %q", x.Math)
		}
	}
	for _, x := range Analyze(fixtureInput(fixtureNodes())).Recs {
		if strings.Contains(strings.Join(x.Math, "\n"), liveSizes) {
			t.Errorf("AWS row with the Azure sizes line: %+v", x)
		}
	}
}

// azure-live-skus R3: a candidate or target not available to the node is
// never offered; a node whose own size is not available is still compared
// but gets no Spot row.
func TestAvailabilityPerRule(t *testing.T) {
	in := azInput(azFixtureNodes())
	in.Available = func(n Node, t string) bool { return t != "Standard_D4s_v5" && t != "Standard_D8ps_v5" }
	r := Analyze(in)
	for _, x := range r.Recs {
		if strings.Contains(x.Action, "Standard_D4s_v5") || strings.Contains(x.Action, "Standard_D8ps_v5") {
			t.Errorf("an unavailable size is offered: %s | %s | %s", x.Label(), x.Target, x.Action)
		}
	}
	in = azInput(azFixtureNodes())
	in.Available = func(n Node, t string) bool { return t != "Standard_D8s_v5" }
	r = Analyze(in)
	find(t, r, Rightsize, "aks-userpool-12345678-vmss000003")
	for _, x := range kinds(r, SpotMove) {
		if strings.Contains(x.Target, "userpool") {
			t.Errorf("Spot row for a size not available to its nodes: %s", x.Target)
		}
	}
	if len(kinds(r, SpotMove)) != 1 {
		t.Errorf("want legacy's Spot row only, got %d", len(kinds(r, SpotMove)))
	}
}

// azure-live-skus R3 (decided 2026-11-10): while the sizes can't be read,
// Spot rows stay with their amounts, outside the total; size rows don't.
func TestSpotWhileSizesFail(t *testing.T) {
	in := azInput(azFixtureNodes())
	in.Spec = func(string) (Spec, bool) { return Spec{}, false }
	in.Types = nil
	in.SizesLoaded = func(Node) bool { return false }
	in.Available = func(Node, string) bool { return false }
	r := Analyze(in)
	spots := kinds(r, SpotMove)
	if len(spots) != 2 || !near(spots[0].Monthly, 884.58) || !near(spots[1].Monthly, 224.28) {
		for _, x := range spots {
			t.Logf("%s %.2f", x.Target, x.Monthly)
		}
		t.Errorf("%d Spot rows", len(spots))
	}
	for _, k := range []Kind{Rightsize, Generation, Graviton} {
		if len(kinds(r, k)) != 0 {
			t.Errorf("%s rows without sizes", k)
		}
	}
	if r.Total != 0 {
		t.Errorf("total %.2f, want 0 (Spot stays out of it)", r.Total)
	}
}

// azure-live-skus R9: a generation or arm target without a temp disk is
// refused for a node whose size has one, read from the live sizes.
func TestTempDiskLive(t *testing.T) {
	withTemp := func(node, target float64) Result {
		in := azInput(azFixtureNodes())
		spec := in.Spec
		in.Spec = func(t string) (Spec, bool) {
			s, ok := spec(t)
			switch t {
			case "Standard_D4s_v3":
				s.TempDiskMB = node
			case "Standard_D4pds_v5":
				s.TempDiskMB = target
			}
			return s, ok
		}
		return Analyze(in)
	}
	legacyArm := func(r Result) bool {
		for _, x := range kinds(r, Graviton) {
			if strings.Contains(x.Target, "legacy") {
				return true
			}
		}
		return false
	}
	if legacyArm(withTemp(32768, 0)) {
		t.Error("an arm target without a temp disk offered to a node with one")
	}
	if !legacyArm(withTemp(32768, 153600)) {
		t.Error("an arm target with a temp disk refused")
	}
}

// azure-live-skus R11: a user-pool node is never rightsized to a size under 2
// vCPU or 2 GiB, however well it fits.
func TestUserPoolMinimum(t *testing.T) {
	in := azInput(nil)
	spec := in.Spec
	in.Spec = func(t string) (Spec, bool) {
		if t == "Standard_D1s_v5" {
			return Spec{VCPU: 1, MemGiB: 4, Arch: "x86_64", Family: "dsv5"}, true
		}
		return spec(t)
	}
	in.Types = append(in.Types, "Standard_D1s_v5")
	price := in.OnDemand
	in.OnDemand = func(n Node, t string) (float64, bool) {
		if t == "Standard_D1s_v5" {
			return 0.048, true
		}
		return price(n, t)
	}
	n := azNode("aks-small-12345678-vmss000000", "Standard_D2s_v5", "small", false, Resources{1.9, 6, 30}, Resources{2, 8, 30}, time.Hour, app(0.05, 0.1))
	in.Nodes = []Node{n}
	for _, x := range kinds(Analyze(in), Rightsize) {
		if strings.Contains(x.Action, "Standard_D1s_v5") {
			t.Errorf("rightsized under the user-pool minimum: %s", x.Action)
		}
	}
	if !userSizeOK(Spec{VCPU: 2, MemGiB: 2}) || userSizeOK(Spec{VCPU: 2, MemGiB: 1.9}) || userSizeOK(Spec{VCPU: 1, MemGiB: 8}) {
		t.Error("userSizeOK boundaries")
	}
}
