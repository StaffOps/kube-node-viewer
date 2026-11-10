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
	"fmt"
	"strings"
	"testing"
	"time"
)

// aks-recommendations R3: a candidate size no node runs is queued, answered
// pending until the price arrives, then priced by OS and market; the
// generation moves; a size the API doesn't know is not asked again.
func TestListPriceQueuesAndAnswers(t *testing.T) {
	api := &fakeAPI{}
	p, updated := newTestProvider(t, api)
	g0 := p.Generation()
	if _, known, pending := p.ListPrice("eastus2", "Standard_D2s_v5", false, false); known || !pending {
		t.Fatalf("first ask: known %v pending %v, want queued", known, pending)
	}
	p.ListPrice("eastus2", "Standard_Nope", false, false)
	settled(t, p)
	<-updated
	if p.Generation() == g0 {
		t.Error("generation unchanged after the answer")
	}
	for _, c := range []struct {
		windows, spot bool
		want          float64
	}{{false, false, 0.096}, {false, true, 0.019}, {true, false, 0.188}, {true, true, 0.037}} {
		got, known, pending := p.ListPrice("eastus2", "Standard_D2s_v5", c.windows, c.spot)
		if !known || pending || got != c.want {
			t.Errorf("windows %v spot %v: %v %v %v, want %v", c.windows, c.spot, got, known, pending, c.want)
		}
	}
	api.mu.Lock()
	before := len(api.filters)
	api.mu.Unlock()
	if _, known, pending := p.ListPrice("eastus2", "Standard_Nope", false, false); known || pending {
		t.Errorf("unknown size: known %v pending %v, want neither", known, pending)
	}
	time.Sleep(100 * time.Millisecond)
	api.mu.Lock()
	defer api.mu.Unlock()
	if len(api.filters) != before {
		t.Errorf("an unknown size was asked again: %v", api.filters)
	}
	if _, known, pending := p.ListPrice("", "Standard_D2s_v5", false, false); known || pending {
		t.Error("no region: queued")
	}
}

// aks-recommendations R3: more than 17 candidate sizes in one region go out
// in two requests (the fix-azure-price-batch cap, not redefined here).
func TestListPriceBatches(t *testing.T) {
	api := &fakeAPI{}
	p, _ := newTestProvider(t, api)
	for i := 0; i < maxSizesPerRequest+1; i++ {
		p.ListPrice("eastus2", fmt.Sprintf("Standard_D%ds_v5", i+1), false, false)
	}
	settled(t, p)
	api.mu.Lock()
	defer api.mu.Unlock()
	if len(api.filters) != 2 {
		t.Fatalf("%d requests for %d sizes, want 2", len(api.filters), maxSizesPerRequest+1)
	}
	for _, f := range api.filters {
		if n := strings.Count(f, "armSkuName eq"); n > maxSizesPerRequest {
			t.Errorf("a request asks for %d sizes", n)
		}
	}
}
