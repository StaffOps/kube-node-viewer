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
	"bufio"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/staffops/kube-node-viewer/pkg/azure"
	"github.com/staffops/kube-node-viewer/pkg/recommend"
)

// frozenSizes reads testdata/azure-sizes.tsv, the frozen test-only extract of
// the autoscaler's Azure table (azure-live-skus R12, D5).
func frozenSizes(t *testing.T) map[string]recommend.Spec {
	t.Helper()
	f, err := os.Open("testdata/azure-sizes.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	out := map[string]recommend.Spec{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "name\t") {
			continue
		}
		c := strings.Split(line, "\t")
		if len(c) != 5 {
			t.Fatalf("bad line %q", line)
		}
		vcpu, _ := strconv.ParseFloat(c[2], 64)
		mb, _ := strconv.ParseFloat(c[3], 64)
		arch := "x86_64"
		if c[4] == "arm64" {
			arch = "arm64"
		}
		out[c[0]] = recommend.Spec{VCPU: vcpu, MemGiB: mb / 1024, Arch: arch, Family: azure.NormalizeFamily(c[1]), Constrained: strings.Contains(c[0], "-")}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// azure-live-skus R12: the copied table is gone, and the frozen extract carries its copyright line, the
// Apache-2.0 notice and the statement of change (headers.py skips .tsv).
func TestFrozenExtractHeader(t *testing.T) {
	raw, err := os.ReadFile("testdata/azure-sizes.tsv")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# Copyright The Kubernetes Authors.", "# Licensed under the Apache License, Version 2.0", "at commit 03e49d9cb822: name, family, vCPU, memory and architecture only."} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("header lacks %q", want)
		}
	}
	if n := len(frozenSizes(t)); n != 1370 {
		t.Errorf("%d sizes, want 1370", n)
	}
	for _, gone := range []string{"../azure/zz_generated_instance_types.go", "../azure/catalog.go"} {
		if _, err := os.Stat(gone); err == nil {
			t.Errorf("%s is back: the live sizes are the only run-time source", gone)
		}
	}
}

func catalogFamilies(t *testing.T) map[string]string {
	arch := map[string]string{}
	for _, s := range frozenSizes(t) {
		arch[s.Family] = s.Arch
	}
	return arch
}

// aks-recommendations R8, R9: every key and target of the Azure tables is a
// family of the frozen extract, x86 → x86 for generations and x86 → arm64 for
// the Arm table, and every mapping keeps the temp disk (design D5).
func TestAzureTablesAgainstTheCatalog(t *testing.T) {
	fam := catalogFamilies(t)
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
	for f, arch := range catalogFamilies(t) {
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

// aks-recommendations R7, R2; azure-live-skus R11: Standard_B1ls, read with
// its real 0.5 GiB from the live sizes, is never a user-pool rightsize
// candidate: under the 2 vCPU / 2 GiB minimum.
func TestB1lsNeverACandidate(t *testing.T) {
	sizes := frozenSizes(t)
	sizes["Standard_B1ls"] = recommend.Spec{VCPU: 1, MemGiB: 0.5, Arch: "x86_64", Family: sizes["Standard_B2s"].Family}
	spec := func(t string) (recommend.Spec, bool) {
		s, ok := sizes[t]
		return s, ok
	}
	var names []string
	for n := range sizes {
		names = append(names, n)
	}
	sort.Strings(names)
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
	r := recommend.Analyze(recommend.Input{Nodes: []recommend.Node{n}, Now: now, Headroom: 0.8, Types: names,
		Spec: spec, OnDemand: price, SpotIn: price})
	for _, x := range r.Recs {
		if x.Kind == recommend.Rightsize && strings.Contains(x.Action, "Standard_B1ls") && !strings.Contains(x.Action, "Standard_B1ls2") {
			t.Errorf("rightsized to Standard_B1ls: %+v", x)
		}
	}
}
