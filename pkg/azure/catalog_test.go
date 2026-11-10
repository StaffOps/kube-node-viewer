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

import "testing"

// aks-recommendations R2: the copied table loads, with exact memory, the
// family and architecture words the rules use, constrained sizes flagged and
// truncated 0 entries counted as missing.
func TestSizeCatalog(t *testing.T) {
	if n := len(SizeNames()); n < 1000 {
		t.Fatalf("%d sizes, want the cluster autoscaler's table (over 1,000)", n)
	}
	for _, tt := range []struct {
		name, family, arch string
		vcpu, mem          float64
		constrained        bool
	}{
		{"Standard_D64ps_v5", "dpsv5", "arm64", 64, 208, false}, // not 4 GiB × vCPU
		{"Standard_D4s_v5", "dsv5", "x86_64", 4, 16, false},     // amd64 read as x86_64
		{"Standard_D4as_v6", "dav6", "x86_64", 4, 16, false},    // standardDav6Family holds Dasv6
		{"Standard_E8-4ds_v5", "edsv5", "x86_64", 8, 64, true},  // full vCPU count, flagged
		{"Standard_DS1_v2", "dsv2", "x86_64", 1, 3, false},      // 3.5 GiB truncated to 3
	} {
		s, ok := LookupSize(tt.name)
		if !ok {
			t.Errorf("%s missing", tt.name)
			continue
		}
		if s.Family != tt.family || s.Arch != tt.arch || s.VCPU != tt.vcpu || s.MemGiB != tt.mem || s.Constrained != tt.constrained {
			t.Errorf("%s = %+v, want family %s arch %s %v vCPU %v GiB constrained %v", tt.name, s, tt.family, tt.arch, tt.vcpu, tt.mem, tt.constrained)
		}
	}
	if s, ok := LookupSize("Standard_B1ls"); ok {
		t.Errorf("Standard_B1ls (0 MB in the table) is usable: %+v", s)
	}
	if _, ok := LookupSize("Standard_Made_up_v9"); ok {
		t.Error("a made-up size is in the catalog")
	}
}

// aks-recommendations R2: SkuFamily is normalised, never parsed from the name.
func TestNormalizeFamily(t *testing.T) {
	for in, want := range map[string]string{
		"standardDSv5Family": "dsv5", "StandardDpsv6Family": "dpsv6", "standardBSFamily": "bs",
	} {
		if got := NormalizeFamily(in); got != want {
			t.Errorf("NormalizeFamily(%q) = %q, want %q", in, got, want)
		}
	}
}
