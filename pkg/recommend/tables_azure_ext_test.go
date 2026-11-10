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

package recommend_test

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/staffops/kube-node-viewer/pkg/azure"
	"github.com/staffops/kube-node-viewer/pkg/recommend"
)

func catalogFamilies() map[string]string {
	arch := map[string]string{}
	for _, name := range azure.SizeNames() {
		s, _ := azure.LookupSize(name)
		arch[s.Family] = s.Arch
	}
	return arch
}

// aks-recommendations R8, R9: every key and target of the Azure tables is a
// family of the copied table, x86 → x86 for generations and x86 → arm64 for
// the Arm table, and every mapping keeps the temp disk (design D5).
func TestAzureTablesAgainstTheCatalog(t *testing.T) {
	fam := catalogFamilies()
	check := func(table string, m map[string]string, targetArch string) {
		for from, to := range m {
			if fam[from] != "x86_64" {
				t.Errorf("%s: %s is %q in the catalog, want an x86_64 family", table, from, fam[from])
			}
			if fam[to] != targetArch {
				t.Errorf("%s: %s → %s, which is %q, want %s", table, from, to, fam[to], targetArch)
			}
			if recommend.AzureTempDisk[from] != recommend.AzureTempDisk[to] {
				t.Errorf("%s: %s → %s changes the temp disk (%v → %v)", table, from, to, recommend.AzureTempDisk[from], recommend.AzureTempDisk[to])
			}
		}
	}
	check("generation", recommend.AzureGeneration, "x86_64")
	check("arm", recommend.AzureArm, "arm64")
	for f := range recommend.AzureTempDisk {
		if _, ok := fam[f]; !ok {
			t.Errorf("temp-disk family %s is not in the catalog", f)
		}
	}
}

// aks-recommendations R9: every x86 d*/e* family of v2 to v6 in the copied
// table has an Arm counterpart or a listed reason not to (design D5).
func TestAzureArmCoverage(t *testing.T) {
	general := regexp.MustCompile(`^[de][a-z]*v[2-6]$`)
	for f, arch := range catalogFamilies() {
		if arch != "x86_64" || !general.MatchString(f) {
			continue
		}
		if _, ok := recommend.AzureArm[f]; ok {
			continue
		}
		if _, ok := recommend.AzureNoArm[f]; !ok {
			t.Errorf("x86 family %s has no Arm counterpart and no reason in the exclusion list", f)
		}
	}
}

// aks-recommendations R7, R2: Standard_B1ls (0 MB in the table) is never a
// rightsize candidate.
func TestB1lsNeverACandidate(t *testing.T) {
	spec := func(t string) (recommend.Spec, bool) {
		s, ok := azure.LookupSize(t)
		return recommend.Spec{VCPU: s.VCPU, MemGiB: s.MemGiB, Arch: s.Arch, Family: s.Family, Constrained: s.Constrained}, ok
	}
	now := time.Now()
	n := recommend.Node{Name: "b", Type: "Standard_B2s", Cloud: recommend.CloudAzure, Region: "eastus", Capacity: recommend.OnDemand,
		ManagedGroup: "burst", Allocatable: recommend.Resources{CPU: 1.9, Mem: 2.5, Pods: 30}, NodeCapacity: recommend.Resources{CPU: 2, Mem: 3.8, Pods: 30},
		Price: 0.0416, PriceKnown: true, Created: now,
		Pods: []recommend.Pod{{Req: recommend.Resources{CPU: 0.01, Mem: 0.01, Pods: 1}}}}
	price := func(_ recommend.Node, t string) (float64, bool) {
		if strings.HasPrefix(t, "Standard_B1") {
			return 0.0052, true
		}
		return 0, false
	}
	r := recommend.Analyze(recommend.Input{Nodes: []recommend.Node{n}, Now: now, Headroom: 0.8, Types: azure.SizeNames(),
		Spec: spec, OnDemand: price, SpotIn: price})
	for _, x := range r.Recs {
		if x.Kind == recommend.Rightsize && strings.Contains(x.Action, "Standard_B1ls") && !strings.Contains(x.Action, "Standard_B1ls2") {
			t.Errorf("rightsized to Standard_B1ls: %+v", x)
		}
	}
}
