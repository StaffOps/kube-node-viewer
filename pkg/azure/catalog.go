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

import (
	"sort"
	"strings"
	"sync"
)

// Size is an Azure VM size from the cluster autoscaler's table
// (zz_generated_instance_types.go, copied verbatim by make azure-catalog;
// aks-recommendations D1).
type Size struct {
	Name string
	// Family is the table's SkuFamily, lower-cased, without the "standard"
	// prefix and the "Family" suffix: standardDSv5Family → dsv5
	Family string
	VCPU   float64
	// MemGiB is MemoryMb / 1024; the table truncates to whole GB
	MemGiB float64
	// Arch is x86_64 or arm64, the words the rules use
	Arch string
	// Constrained marks a constrained-vCPU size (Standard_E8-4ds_v5); the
	// table gives its full vCPU count
	Constrained bool
}

var (
	sizesOnce sync.Once
	sizes     map[string]Size
	sizeNames []string
)

func loadSizes() {
	sizes = map[string]Size{}
	for name, it := range InstanceTypes {
		// an entry with 0 vCPU or 0 memory counts as missing (R2): the
		// table truncates Standard_B1ls to 0 MB
		if it == nil || it.VCPU <= 0 || it.MemoryMb <= 0 {
			continue
		}
		arch := "x86_64"
		if it.Architecture == "arm64" {
			arch = "arm64"
		}
		sizes[name] = Size{
			Name: name, Family: NormalizeFamily(it.SkuFamily),
			VCPU: float64(it.VCPU), MemGiB: float64(it.MemoryMb) / 1024,
			Arch: arch, Constrained: strings.Contains(name, "-"),
		}
		sizeNames = append(sizeNames, name)
	}
	sort.Strings(sizeNames)
}

// NormalizeFamily turns a SkuFamily into the key the rules use.
func NormalizeFamily(skuFamily string) string {
	f := strings.ToLower(skuFamily)
	f = strings.TrimPrefix(f, "standard")
	return strings.TrimSuffix(f, "family")
}

// LookupSize finds a size by name; false for sizes not in the table and for
// entries with 0 vCPU or memory.
func LookupSize(name string) (Size, bool) {
	sizesOnce.Do(loadSizes)
	s, ok := sizes[name]
	return s, ok
}

// SizeNames lists every usable size, sorted.
func SizeNames() []string {
	sizesOnce.Do(loadSizes)
	return append([]string(nil), sizeNames...)
}
