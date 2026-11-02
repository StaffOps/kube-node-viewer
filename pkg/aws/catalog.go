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
	"strconv"
	"strings"
	"sync"
)

// InstanceSpec is what the recommendations need to know about an instance
// type (recommendations D2): its size, architecture, class and generation.
type InstanceSpec struct {
	VCPU              float64
	MemoryGiB         float64
	Arch              string // "x86_64" or "arm64"
	Class             string // Price List instanceFamily, e.g. "General purpose"
	CurrentGeneration bool
}

// PriceListAttributes are the Price List product attributes the catalog
// reads, as they come in GetProducts.
type PriceListAttributes struct {
	InstanceType      string `json:"instanceType"`
	VCPU              string `json:"vcpu"`
	Memory            string `json:"memory"`
	PhysicalProcessor string `json:"physicalProcessor"`
	InstanceFamily    string `json:"instanceFamily"`
	CurrentGeneration string `json:"currentGeneration"`
}

// Spec turns Price List attributes into an InstanceSpec; ok is false when
// they lack the size.
func (a PriceListAttributes) Spec() (InstanceSpec, bool) {
	vcpu, err := strconv.ParseFloat(strings.TrimSpace(a.VCPU), 64)
	if err != nil || vcpu <= 0 {
		return InstanceSpec{}, false
	}
	mem, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(strings.ReplaceAll(a.Memory, ",", ""), "GiB")), 64)
	if err != nil || mem <= 0 {
		return InstanceSpec{}, false
	}
	arch := "x86_64"
	if strings.Contains(a.PhysicalProcessor, "Graviton") {
		arch = "arm64"
	}
	return InstanceSpec{VCPU: vcpu, MemoryGiB: mem, Arch: arch, Class: a.InstanceFamily, CurrentGeneration: a.CurrentGeneration == "Yes"}, true
}

// catalog is the embedded table, refreshed from the Price List products the
// pricing provider reads at run time.
var catalog = struct {
	sync.RWMutex
	specs map[string]InstanceSpec
}{specs: map[string]InstanceSpec{}}

func init() {
	for k, v := range generatedInstanceSpecs {
		catalog.specs[k] = v
	}
}

// LookupInstance returns what the catalog knows about an instance type.
func LookupInstance(instanceType string) (InstanceSpec, bool) {
	catalog.RLock()
	defer catalog.RUnlock()
	s, ok := catalog.specs[instanceType]
	return s, ok
}

// InstanceTypes lists every type in the catalog.
func InstanceTypes() []string {
	catalog.RLock()
	defer catalog.RUnlock()
	out := make([]string, 0, len(catalog.specs))
	for k := range catalog.specs {
		out = append(out, k)
	}
	return out
}

// learnInstance records a type seen in the Price List at run time.
func learnInstance(a PriceListAttributes) {
	if a.InstanceType == "" {
		return
	}
	if s, ok := a.Spec(); ok {
		catalog.Lock()
		catalog.specs[a.InstanceType] = s
		catalog.Unlock()
	}
}
