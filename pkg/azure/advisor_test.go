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
	"strings"
	"testing"

	"github.com/staffops/kube-node-viewer/pkg/recommend"
)

// fakeLister answers from a map; sizes in wait are pending; it records the
// calls.
type fakeLister struct {
	prices map[string]float64
	wait   map[string]bool
	calls  []string
	gen    uint64
}

func (f *fakeLister) ListPrice(region, size string, windows, spot bool) (float64, bool, bool) {
	f.calls = append(f.calls, strings.Join([]string{region, size, map[bool]string{true: "windows", false: "linux"}[windows], map[bool]string{true: "spot", false: "regular"}[spot]}, "/"))
	if f.wait[size] {
		return 0, false, true
	}
	p, ok := f.prices[size]
	return p, ok, false
}

func (f *fakeLister) Generation() uint64 { return f.gen }

// aks-recommendations R3: prices follow the node's region, OS and market;
// awaited sizes make the waiting note, which clears when they arrive.
func TestAdvisorPricesAndNotes(t *testing.T) {
	f := &fakeLister{prices: map[string]float64{"Standard_D4s_v5": 0.192}, wait: map[string]bool{"Standard_D2s_v5": true}}
	a := NewAdvisor(f, false, nil)
	n := recommend.Node{Cloud: recommend.CloudAzure, Region: "eastus", Windows: true}
	if p, ok := a.OnDemand(n, "Standard_D4s_v5"); !ok || p != 0.192 {
		t.Errorf("OnDemand = %v %v", p, ok)
	}
	if _, ok := a.SpotIn(n, "Standard_D2s_v5"); ok {
		t.Error("a pending size has a price")
	}
	if strings.Join(f.calls, " ") != "eastus/Standard_D4s_v5/windows/regular eastus/Standard_D2s_v5/windows/spot" {
		t.Errorf("calls %v", f.calls)
	}
	if notes := a.Notes(); len(notes) != 1 || notes[0] != "waiting for the price of 1 Azure size" {
		t.Errorf("notes %v", notes)
	}
	delete(f.wait, "Standard_D2s_v5")
	f.gen++
	a.SpotIn(n, "Standard_D2s_v5")
	if notes := a.Notes(); len(notes) != 0 || a.Generation() != 1 {
		t.Errorf("after the answer: notes %v, generation %d", notes, a.Generation())
	}
}

// aks-recommendations R3: with pricing disabled nothing is asked and the
// note says why the price rows are missing.
func TestAdvisorPricingDisabled(t *testing.T) {
	f := &fakeLister{prices: map[string]float64{"Standard_D4s_v5": 0.192}}
	a := NewAdvisor(f, true, nil)
	if _, ok := a.OnDemand(recommend.Node{Region: "eastus"}, "Standard_D4s_v5"); ok || len(f.calls) != 0 {
		t.Errorf("pricing disabled, but asked: %v", f.calls)
	}
	if notes := a.Notes(); len(notes) != 1 || !strings.HasPrefix(notes[0], "pricing disabled") {
		t.Errorf("notes %v", notes)
	}
	if NewAdvisor(nil, true, nil).Generation() != 0 {
		t.Error("generation without prices")
	}
}

// aks-recommendations R3: each analysis counts the awaited sizes afresh, so
// a size no row asks for any more (after a filter change) leaves the note;
// with no price source the note says candidate prices are not available.
func TestAdvisorPendingPerAnalysis(t *testing.T) {
	f := &fakeLister{wait: map[string]bool{"Standard_D2s_v5": true, "Standard_D4s_v5": true}}
	a := NewAdvisor(f, false, nil)
	n := recommend.Node{Region: "eastus"}
	a.Begin(nil)
	a.OnDemand(n, "Standard_D2s_v5")
	a.OnDemand(n, "Standard_D4s_v5")
	if notes := a.Notes(); len(notes) != 1 || notes[0] != "waiting for the prices of 2 Azure sizes" {
		t.Errorf("notes %v", notes)
	}
	a.Begin(nil) // the filter now keeps nodes that ask for neither
	if notes := a.Notes(); len(notes) != 0 {
		t.Errorf("a size nobody asks for kept the note: %v", notes)
	}
	if notes := NewAdvisor(nil, false, nil).Notes(); len(notes) != 1 || !strings.HasPrefix(notes[0], "Azure candidate prices not available yet") {
		t.Errorf("no price source: notes %v", notes)
	}
}
