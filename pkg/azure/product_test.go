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
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// Items as the public Retail Prices API listed them on 2026-11-04
// (westeurope and eastus2), reduced to the fields the provider reads.
var (
	e4asV5 = []retailItem{ // westeurope
		item("Standard_E4as_v5", "E4as v5 Spot", "Virtual Machines Easv5 Series Windows", 0.088852, "2026-11-01T00:00:00Z"),
		item("Standard_E4as_v5", "E4as v5 Spot", "Virtual Machines Easv5 Series", 0.053156, "2026-11-01T00:00:00Z"),
		item("Standard_E4as_v5", "E4as v5", "Virtual Machines Easv5 Series Windows", 0.458, "2021-11-01T00:00:00Z"),
		item("Standard_E4as_v5", "E4as v5", "Easv5 Series CloudServices", 0.458, "2026-01-01T00:00:00Z"),
		item("Standard_E4as_v5", "E4as v5 Low Priority", "Virtual Machines Easv5 Series", 0.0548, "2021-11-01T00:00:00Z"),
		item("Standard_E4as_v5", "E4as v5", "Virtual Machines Easv5 Series", 0.274, "2021-11-01T00:00:00Z"),
		item("Standard_E4as_v5", "E4as v5 Low Priority", "Easv5 Series CloudServices", 0.183, "2026-01-01T00:00:00Z"),
	}
	d2asV5 = []retailItem{ // eastus2
		item("Standard_D2as_v5", "D2as v5 Low Priority", "Dasv5 Series Cloud Services", 0.0712, "2023-10-01T00:00:00Z"),
		item("Standard_D2as_v5", "D2as v5", "Virtual Machines Dasv5 Series", 0.086, "2021-11-01T00:00:00Z"),
		item("Standard_D2as_v5", "D2as v5", "Virtual Machines Dasv5 Series Windows", 0.178, "2021-11-01T00:00:00Z"),
		item("Standard_D2as_v5", "D2as v5", "Dasv5 Series Cloud Services", 0.178, "2023-10-01T00:00:00Z"),
		item("Standard_D2as_v5", "D2as v5 Spot", "Virtual Machines Dasv5 Series", 0.015919, "2026-09-01T00:00:00Z"),
	}
	b2sV2 = []retailItem{ // westeurope: Cloud Services on the same date as Linux
		item("Standard_B2s_v2", "B2s v2", "Bsv2 Series Cloud Services", 0.105, "2023-09-01T00:00:00Z"),
		item("Standard_B2s_v2", "B2s v2", "Virtual Machines Bsv2 Series", 0.096, "2023-09-01T00:00:00Z"),
		item("Standard_B2s_v2", "B2s v2", "Virtual Machines Bsv2 Series Windows", 0.105, "2023-09-01T00:00:00Z"),
	}
	l8asV3 = []retailItem{ // westeurope: no "Virtual Machines" prefix at all
		item("Standard_L8as_v3", "L8as v3", "Lasv3 Series Windows", 1.112, "2022-06-01T00:00:00Z"),
		item("Standard_L8as_v3", "L8as v3 Spot", "Lasv3 Series Linux", 0.144336, "2026-11-01T00:00:00Z"),
		item("Standard_L8as_v3", "L8as v3", "Lasv3 Series Linux", 0.744, "2022-06-01T00:00:00Z"),
	}
	ec2asV7 = []retailItem{ // eastus2
		item("Standard_EC2as_v7", "EC2asv7", "ECasv7 Series VM", 0.164, "2026-10-01T00:00:00Z"),
		item("Standard_EC2as_v7", "EC2asv7", "ECasv7 Series Windows VM", 0.256, "2026-10-01T00:00:00Z"),
	}
	ncc40 = []retailItem{ // eastus2: "Srs Win" is the Windows product, same date as Linux
		item("Standard_NCC40ads_H100_v5", "NCC40adsH100v5", "Virtual Machines NCCadsv5 Srs Win", 8.82, "2024-12-01T00:00:00Z"),
		item("Standard_NCC40ads_H100_v5", "NCC40adsH100v5", "Virtual Machines NCCadsv5 Srs", 6.98, "2024-12-01T00:00:00Z"),
	}
	dedicated = []retailItem{ // a Dedicated Host sku: listed, but no node has this size, so it prices nothing
		item("Lasv3_Type1", "Lasv3 Type1", "Lasv3 Series DedicatedHost", 7.0, "2022-06-01T00:00:00Z"),
	}
)

// pagedAPI serves a fixed list of items on one page; with endless set every
// page links to another.
type pagedAPI struct {
	mu       sync.Mutex
	items    []retailItem
	endless  bool
	requests int
}

func (f *pagedAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests++
	if f.endless {
		_ = json.NewEncoder(w).Encode(retailPage{Items: f.items, NextPageLink: "http://" + r.Host + "/?page=more"})
		return
	}
	_ = json.NewEncoder(w).Encode(retailPage{Items: f.items})
}

func pricesOf(t *testing.T, items []retailItem, region string) func(size string, labels map[string]string) (float64, bool) {
	t.Helper()
	api := &pagedAPI{items: items}
	srv := httptest.NewServer(api)
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	p := NewProvider(ctx, srv.URL)
	return func(size string, labels map[string]string) (float64, bool) {
		l := map[string]string{"topology.kubernetes.io/region": region}
		for k, v := range labels {
			l[k] = v
		}
		n := aksNode(size, l)
		if got, ok := p.NodePrice(n); ok {
			return got, ok
		}
		settled(t, p)
		return p.NodePrice(n)
	}
}

var (
	windowsOS = map[string]string{"kubernetes.io/os": "windows"}
	spotNode  = map[string]string{"kubernetes.azure.com/scalesetpriority": "spot"}
)

// fix-azure-price-product R1 R2 R3
func TestProductsOfTheVirtualMachinesService(t *testing.T) {
	all := append(append(append(append(append(append(append([]retailItem{},
		e4asV5...), d2asV5...), b2sV2...), l8asV3...), ec2asV7...), ncc40...), dedicated...)
	cases := []struct {
		name   string
		size   string
		labels map[string]string
		want   float64
	}{
		// R1: Cloud Services in both spellings never price a node
		{"linux, CloudServices dated later", "Standard_E4as_v5", nil, 0.274},
		{"windows keeps its price", "Standard_E4as_v5", windowsOS, 0.458},
		{"spot keeps its price", "Standard_E4as_v5", spotNode, 0.053156},
		{"linux, Cloud Services dated later", "Standard_D2as_v5", nil, 0.086},
		{"windows, Cloud Services spelling", "Standard_D2as_v5", windowsOS, 0.178},
		{"linux, Cloud Services on the same date", "Standard_B2s_v2", nil, 0.096},
		// products without the Virtual Machines prefix keep their prices
		{"Lasv3 Series Linux", "Standard_L8as_v3", nil, 0.744},
		{"Lasv3 Series Windows", "Standard_L8as_v3", windowsOS, 1.112},
		{"Lasv3 spot", "Standard_L8as_v3", spotNode, 0.144336},
		{"ECasv7 Series VM", "Standard_EC2as_v7", nil, 0.164},
		{"ECasv7 Series Windows VM", "Standard_EC2as_v7", windowsOS, 0.256},
		// R2: "Srs Win" is Windows
		{"Srs, linux", "Standard_NCC40ads_H100_v5", nil, 6.98},
		{"Srs Win, windows", "Standard_NCC40ads_H100_v5", windowsOS, 8.82},
	}
	// R3: the answer does not depend on the order the API lists items in
	for _, order := range []string{"as listed", "reversed"} {
		items := append([]retailItem{}, all...)
		if order == "reversed" {
			for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
				items[i], items[j] = items[j], items[i]
			}
		}
		price := pricesOf(t, items, "westeurope")
		for _, c := range cases {
			got, ok := price(c.size, c.labels)
			if !ok || got != c.want {
				t.Errorf("%s, %s: got %v %v, want %v", order, c.name, got, ok, c.want)
			}
		}
	}
}

// fix-azure-price-product R3
func TestSameDateKeepsTheLowerPrice(t *testing.T) {
	for _, items := range [][]retailItem{
		{item("Standard_X", "X", "Virtual Machines X Series", 0.2, "2024-01-01T00:00:00Z"),
			item("Standard_X", "X", "Virtual Machines X Series", 0.1, "2024-01-01T00:00:00Z")},
		{item("Standard_X", "X", "Virtual Machines X Series", 0.1, "2024-01-01T00:00:00Z"),
			item("Standard_X", "X", "Virtual Machines X Series", 0.2, "2024-01-01T00:00:00Z")},
	} {
		price := pricesOf(t, items, "eastus")
		if got, ok := price("Standard_X", nil); !ok || got != 0.1 {
			t.Errorf("%v: got %v %v, want 0.1", items, got, ok)
		}
	}
}

// fix-azure-price-product R1
func TestCloudServicesIsNotLogged(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	price := pricesOf(t, e4asV5, "westeurope")
	if got, _ := price("Standard_E4as_v5", nil); got != 0.274 {
		t.Fatalf("got %v", got)
	}
	if buf.Len() != 0 {
		t.Errorf("logged %q", buf.String())
	}
}

func TestVariantOf(t *testing.T) {
	cases := []struct {
		product, meter, want string
		ok                   bool
	}{
		{"Virtual Machines Dsv5 Series", "D2s v5", "regular", true},
		{"Virtual Machines Dsv5 Series Windows", "D2s v5 Spot", "windowsSpot", true},
		{"Easv5 Series CloudServices", "E4as v5", "", false},                              // fix-azure-price-product R1
		{"Dasv5 Series Cloud Services", "D2as v5", "", false},                             // fix-azure-price-product R1
		{"Virtual Machines NCCadsv5 Srs Win", "NCC40adsH100v5 Spot", "windowsSpot", true}, // fix-azure-price-product R2
		{"DCasv5-series Windows", "DC2as v5", "windows", true},
		{"DCasv5-series Linux", "DC2as v5", "regular", true},
		{"Virtual Machines Winx Series", "W2", "regular", true}, // "Win" only as a word
		{"Virtual Machines Dsv5 Series", "D2s v5 Low Priority", "", false},
	}
	for _, c := range cases {
		it := item("Standard_Y", c.meter, c.product, 1, "2024-01-01T00:00:00Z")
		got, ok := variantOf(it)
		if got != c.want || ok != c.ok {
			t.Errorf("%q %q: got %q %v, want %q %v", c.product, c.meter, got, ok, c.want, c.ok)
		}
	}
}

// fix-azure-price-product R4
func TestCutShortIsLoggedOncePerRegion(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	api := &pagedAPI{items: d2asV5, endless: true}
	srv := httptest.NewServer(api)
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	p := NewProvider(ctx, srv.URL)
	for _, size := range []string{"Standard_D2as_v5", "Standard_Other"} {
		p.NodePrice(aksNode(size, map[string]string{"topology.kubernetes.io/region": "eastus2"}))
		settled(t, p)
	}
	api.mu.Lock()
	requests := api.requests
	api.mu.Unlock()
	if requests != 2*maxPages {
		t.Errorf("requests: %d, want %d", requests, 2*maxPages)
	}
	if n := strings.Count(buf.String(), "cut short"); n != 1 {
		t.Errorf("cut-short lines: %d in %q", n, buf.String())
	}
	if got, ok := p.NodePrice(aksNode("Standard_D2as_v5", map[string]string{"topology.kubernetes.io/region": "eastus2"})); !ok || got != 0.086 {
		t.Errorf("price from the pages read: %v %v", got, ok)
	}
}

// TestLivePrices asks the public API (KNV_AZURE_LIVE=1). List prices change;
// a failure here means the expected values below need a look, not
// necessarily a bug.
func TestLivePrices(t *testing.T) {
	if os.Getenv("KNV_AZURE_LIVE") == "" {
		t.Skip("set KNV_AZURE_LIVE=1 to ask the public Retail Prices API")
	}
	cases := []struct {
		region, size  string
		linux, window float64
	}{
		{"westeurope", "Standard_E4as_v5", 0.274, 0.458},
		{"westeurope", "Standard_B2s_v2", 0.096, 0.105},
		{"westeurope", "Standard_L8as_v3", 0.744, 1.112},
		{"eastus2", "Standard_D2as_v5", 0.086, 0.178},
		{"eastus2", "Standard_NCC40ads_H100_v5", 6.98, 8.82},
	}
	for _, c := range cases {
		ctx, cancel := context.WithCancel(context.Background())
		p := NewProvider(ctx, Endpoint)
		l := map[string]string{"topology.kubernetes.io/region": c.region}
		w := map[string]string{"topology.kubernetes.io/region": c.region, "kubernetes.io/os": "windows"}
		p.NodePrice(aksNode(c.size, l))
		settled(t, p)
		linux, ok1 := p.NodePrice(aksNode(c.size, l))
		win, ok2 := p.NodePrice(aksNode(c.size, w))
		cancel()
		t.Logf("%s %s: linux %v %v, windows %v %v", c.region, c.size, linux, ok1, win, ok2)
		if !ok1 || !ok2 || linux != c.linux || win != c.window {
			t.Errorf("%s %s: got linux %v windows %v, want %v %v", c.region, c.size, linux, win, c.linux, c.window)
		}
		time.Sleep(time.Second) // the API rate-limits
	}
}
