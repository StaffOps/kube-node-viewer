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
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

func recorded(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile("testdata/skus-eastus.json")
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// azure-live-skus R2: virtualMachines only; usable vCPUs, fractional memory,
// architecture, family and temp disk; an entry missing a field is missing.
// aks-recommendations R2: the family is the normalised SKU family, never
// parsed from the size name; arm64 is read as arm64, x64 as x86_64.
func TestParseRecordedSKUs(t *testing.T) {
	sizes, next, err := parseSKUs(recorded(t), "eastus")
	if err != nil || next != "" {
		t.Fatalf("parse: %v, next %q", err, next)
	}
	for _, tt := range []struct {
		name                 string
		vcpu, mem, temp      float64
		arch, family         string
		constrained, present bool
	}{
		{"Standard_D64ps_v5", 64, 208, 0, "arm64", "dpsv5", false, true},
		{"Standard_DS1_v2", 1, 3.5, 7168, "x86_64", "dsv2", false, true},
		{"Standard_B1ls", 1, 0.5, 4096, "x86_64", "bs", false, true},
		{"Standard_E8-4ds_v5", 4, 64, 307200, "x86_64", "edsv5", true, true},
		{"Standard_D4s_v3", 4, 16, 32768, "x86_64", "dsv3", false, true},
		{"Standard_Made_up_v9", 0, 0, 0, "", "", false, false},
		{"Premium_LRS", 0, 0, 0, "", "", false, false},
	} {
		s, ok := sizes[tt.name]
		if ok != tt.present {
			t.Errorf("%s present %v, want %v", tt.name, ok, tt.present)
			continue
		}
		if !ok {
			continue
		}
		if s.VCPU != tt.vcpu || s.MemGiB != tt.mem || s.TempDiskMB != tt.temp || s.Arch != tt.arch || s.Family != tt.family || s.Constrained != tt.constrained {
			t.Errorf("%s = %+v", tt.name, s)
		}
	}
	if got := NormalizeFamily("StandardDpsv6Family"); got != "dpsv6" {
		t.Errorf("NormalizeFamily = %q", got)
	}
}

// azure-live-skus R3: a Location restriction makes a size unavailable; a
// zonal node needs its zone offered and not restricted; a regional node gets
// the regional check only.
func TestSizeAvailability(t *testing.T) {
	sizes, _, err := parseSKUs(recorded(t), "eastus")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name, zone string
		want       bool
	}{
		{"Standard_D4s_v5", "1", true},
		{"Standard_D4s_v5", "", true},
		{"Standard_D16s_v5", "", false},
		{"Standard_D16s_v5", "1", false},
		{"Standard_D2ds_v5", "1", true},
		{"Standard_D2ds_v5", "3", false},
		{"Standard_D2ds_v5", "", true},
		{"Standard_D8ds_v5", "2", true},
		{"Standard_D8ds_v5", "3", false},
	} {
		if got := sizes[tt.name].AvailableIn(tt.zone); got != tt.want {
			t.Errorf("%s in zone %q: %v, want %v", tt.name, tt.zone, got, tt.want)
		}
	}
}

// fakeARM serves pages and records the requests it got.
type fakeARM struct {
	srv      *httptest.Server
	requests atomic.Int32
	last     atomic.Value // *http.Request
}

func newFakeARM(t *testing.T, h func(w http.ResponseWriter, r *http.Request)) *fakeARM {
	f := &fakeARM{}
	f.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.requests.Add(1)
		f.last.Store(r)
		h(w, r)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeARM) client() *skuClient { return newSKUClient(f.srv.URL, f.srv.Client().Transport) }

const zeroSub = "00000000-0000-0000-0000-000000000000"

// azure-live-skus R1, R10: the request names the subscription and region,
// the query is built with url.Values, the token is sent as a bearer, and
// nextLink pages are followed on the same https host.
func TestSKUClientPages(t *testing.T) {
	var f *fakeARM
	f = newFakeARM(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "" {
			if got := r.URL.Query().Get("$filter"); got != "location eq 'eastus'" {
				t.Errorf("$filter = %q", got)
			}
			if r.URL.Query().Get("api-version") != "2021-07-01" || r.URL.Path != "/subscriptions/"+zeroSub+"/providers/Microsoft.Compute/skus" {
				t.Errorf("request %s", r.URL)
			}
			_, _ = w.Write([]byte(`{"value":[],"nextLink":"` + f.srv.URL + `/next?page=2"}`))
			return
		}
		_, _ = w.Write(recorded(t))
	})
	sizes, err := f.client().list(context.Background(), "tok", zeroSub, "eastus")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := sizes["Standard_D4s_v5"]; !ok || f.requests.Load() != 2 {
		t.Errorf("%d sizes in %d requests", len(sizes), f.requests.Load())
	}
	if got := f.last.Load().(*http.Request).Header.Get("Authorization"); got != "Bearer tok" {
		t.Errorf("Authorization %q", got)
	}
	if !strings.Contains(f.client().firstURL(zeroSub, "eastus"), "%24filter=location%20eq%20%27eastus%27") {
		t.Errorf("query not encoded: %s", f.client().firstURL(zeroSub, "eastus"))
	}
	if c := newSKUClient(DefaultARM, nil); c.base != "https://management.azure.com" || !c.sameHost("https://management.azure.com/x") || c.sameHost("https://example.com/x") || c.sameHost("http://management.azure.com/x") {
		t.Error("the default base or its host check is wrong")
	}
}

// azure-live-skus R10, R5 row 12: a nextLink to another host or over http,
// a redirect, an answer over the cap or malformed is not read, and no
// request reaches another host with the token.
func TestSKUClientRefusesForeignLinks(t *testing.T) {
	var other atomic.Int32
	foreign := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { other.Add(1) }))
	defer foreign.Close()
	for _, tt := range []struct {
		name string
		h    func(w http.ResponseWriter, r *http.Request)
	}{
		{"foreign host", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"value":[],"nextLink":"` + foreign.URL + `/x"}`))
		}},
		{"http", func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"value":[],"nextLink":"http://` + r.Host + `/x"}`))
		}},
		{"redirect", func(w http.ResponseWriter, _ *http.Request) {
			http.Redirect(w, &http.Request{}, foreign.URL+"/x", http.StatusFound)
		}},
		{"malformed", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"value":`)) }},
		{"over the cap", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"value":[` + strings.Repeat(" ", 2048) + `]}`))
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "over the cap" {
				old := skuPageCap
				skuPageCap = 1024
				defer func() { skuPageCap = old }()
			}
			f := newFakeARM(t, tt.h)
			_, err := f.client().list(context.Background(), "tok", zeroSub, "eastus")
			if !errors.Is(err, errBadAnswer) {
				t.Errorf("err = %v, want a bad answer", err)
			}
		})
	}
	if other.Load() != 0 {
		t.Errorf("%d requests reached another host", other.Load())
	}
}

// azure-live-skus R5: a failed answer keeps its status and error code, never
// its message.
func TestSKUClientErrors(t *testing.T) {
	f := newFakeARM(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"code":"AuthorizationFailed","message":"The client 'someone' with object id 'x' does not have authorization over scope '/subscriptions/` + zeroSub + `'"}}`))
	})
	_, err := f.client().list(context.Background(), "tok", zeroSub, "eastus")
	var ae *armError
	if !errors.As(err, &ae) || ae.status != 403 || ae.code != "AuthorizationFailed" {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(err.Error(), "someone") || strings.Contains(err.Error(), zeroSub) {
		t.Errorf("the message leaked: %v", err)
	}
}
