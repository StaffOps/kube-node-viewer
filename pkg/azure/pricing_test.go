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
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/staffops/kube-node-viewer/pkg/model"
)

func item(sku, meter, product string, price float64, start string) retailItem {
	at, _ := time.Parse(time.RFC3339, start)
	return retailItem{CurrencyCode: "USD", UnitPrice: price, ArmSkuName: sku, MeterName: meter, ProductName: product,
		UnitOfMeasure: "1 Hour", Type: "Consumption", EffectiveStartDate: at}
}

// a region's items as the API gives them: regular, Spot, Low Priority and
// Windows meters, an old and a future price, split over two pages
var items = []retailItem{
	item("Standard_D2s_v5", "D2s v5", "Virtual Machines Dsv5 Series", 0.090, "2023-01-01T00:00:00Z"),
	item("Standard_D2s_v5", "D2s v5", "Virtual Machines Dsv5 Series", 0.096, "2024-01-01T00:00:00Z"),
	item("Standard_D2s_v5", "D2s v5", "Virtual Machines Dsv5 Series", 0.500, "2999-01-01T00:00:00Z"),
	item("Standard_D2s_v5", "D2s v5 Spot", "Virtual Machines Dsv5 Series", 0.019, "2024-01-01T00:00:00Z"),
	item("Standard_D2s_v5", "D2s v5 Low Priority", "Virtual Machines Dsv5 Series", 0.010, "2024-01-01T00:00:00Z"),
	item("Standard_D2s_v5", "D2s v5", "Virtual Machines Dsv5 Series Windows", 0.188, "2024-01-01T00:00:00Z"),
	item("Standard_D2s_v5", "D2s v5 Spot", "Virtual Machines Dsv5 Series Windows", 0.037, "2024-01-01T00:00:00Z"),
	item("Standard_D2pds_v6", "D2pds v6", "Virtual Machines Dpdsv6 Series", 0.0918, "2024-10-01T00:00:00Z"),
}

type fakeAPI struct {
	mu      sync.Mutex
	filters []string
	fail    bool
}

func (f *fakeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		http.Error(w, "busy", http.StatusTooManyRequests)
		return
	}
	if r.URL.Query().Get("page") == "2" {
		_ = json.NewEncoder(w).Encode(retailPage{Items: items[4:]})
		return
	}
	f.filters = append(f.filters, r.URL.Query().Get("$filter"))
	_ = json.NewEncoder(w).Encode(retailPage{Items: items[:4], NextPageLink: "http://" + r.Host + "/?page=2"})
}

func aksNode(size string, labels map[string]string) *model.Node {
	l := map[string]string{
		v1.LabelTopologyRegion:     "eastus2",
		v1.LabelInstanceTypeStable: size,
	}
	for k, v := range labels {
		l[k] = v
	}
	return model.NewNode(&v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "aks-" + size, Labels: l},
		Spec:       v1.NodeSpec{ProviderID: "azure:///subscriptions/s/resourceGroups/rg/providers/Microsoft.Compute/virtualMachineScaleSets/x/virtualMachines/0"},
	})
}

// settled waits until the provider has nothing left to ask for.
func settled(t *testing.T, p *Provider) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		p.mu.Lock()
		n := len(p.wanted)
		p.mu.Unlock()
		if n == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("prices never arrived")
}

func newTestProvider(t *testing.T, api *fakeAPI) (*Provider, chan struct{}) {
	t.Helper()
	srv := httptest.NewServer(api)
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	p := NewProvider(ctx, srv.URL)
	p.retry = 50 * time.Millisecond
	updated := make(chan struct{}, 10)
	p.OnUpdate(func() { updated <- struct{}{} })
	return p, updated
}

// aks-pricing R1 and R2: Azure nodes priced by region and VM size, looked
// up in the background, with a re-price signal when the answer arrives.
func TestPricesByMeter(t *testing.T) {
	api := &fakeAPI{}
	p, updated := newTestProvider(t, api)

	cases := []struct {
		name   string
		node   *model.Node
		want   float64
		wantOK bool
	}{
		{"regular, latest effective price", aksNode("Standard_D2s_v5", nil), 0.096, true},
		{"spot", aksNode("Standard_D2s_v5", map[string]string{"kubernetes.azure.com/scalesetpriority": "spot"}), 0.019, true},
		{"windows", aksNode("Standard_D2s_v5", map[string]string{"kubernetes.io/os": "windows"}), 0.188, true},
		{"windows spot", aksNode("Standard_D2s_v5", map[string]string{"kubernetes.io/os": "windows", "kubernetes.azure.com/scalesetpriority": "spot"}), 0.037, true},
		{"arm, second size in the same request", aksNode("Standard_D2pds_v6", nil), 0.0918, true},
		{"unknown size", aksNode("Standard_Nope", nil), 0, false},
	}
	// ask for all of them first, as the informer does, so one request carries
	// every size
	for _, c := range cases {
		p.NodePrice(c.node)
	}
	settled(t, p)
	select {
	case <-updated:
	default:
		t.Error("no update after the prices arrived")
	}
	for _, c := range cases {
		got, ok := p.NodePrice(c.node)
		if ok != c.wantOK || got != c.want {
			t.Errorf("%s: got %v %v, want %v %v", c.name, got, ok, c.want, c.wantOK)
		}
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	if len(api.filters) == 0 || len(api.filters) > 2 {
		t.Fatalf("requests: %v", api.filters)
	}
	f := api.filters[0]
	for _, want := range []string{"armRegionName eq 'eastus2'", "priceType eq 'Consumption'", "armSkuName eq 'Standard_D2s_v5'"} {
		if !strings.Contains(f, want) {
			t.Errorf("filter %q lacks %q", f, want)
		}
	}
	// an unknown size is not asked again
	before := len(api.filters)
	api.mu.Unlock()
	p.NodePrice(aksNode("Standard_Nope", nil))
	time.Sleep(100 * time.Millisecond)
	api.mu.Lock()
	if len(api.filters) != before {
		t.Errorf("an unknown size was asked again: %v", api.filters)
	}
}

func TestFailureIsRetriedLater(t *testing.T) {
	api := &fakeAPI{fail: true}
	p, updated := newTestProvider(t, api)
	n := aksNode("Standard_D2s_v5", nil)
	if _, ok := p.NodePrice(n); ok {
		t.Fatal("a price before any answer")
	}
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		p.mu.Lock()
		failed := !p.failedAt.IsZero()
		p.mu.Unlock()
		if failed {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the failure was not recorded")
		}
	}
	api.mu.Lock()
	api.fail = false
	api.mu.Unlock()
	settled(t, p)
	<-updated
	if got, ok := p.NodePrice(n); !ok || got != 0.096 {
		t.Errorf("after the retry: %v %v", got, ok)
	}
}

func TestNoNodeNoRequest(t *testing.T) {
	api := &fakeAPI{}
	p, _ := newTestProvider(t, api)
	// a node without region or size never asks
	p.NodePrice(model.NewNode(&v1.Node{Spec: v1.NodeSpec{ProviderID: "azure:///x"}}))
	time.Sleep(100 * time.Millisecond)
	api.mu.Lock()
	defer api.mu.Unlock()
	if len(api.filters) != 0 {
		t.Errorf("requests without an Azure node: %v", api.filters)
	}
}
