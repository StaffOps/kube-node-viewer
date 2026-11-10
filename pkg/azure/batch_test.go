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
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
)

const (
	capBody   = `{"Error":{"Code":"BadRequest","Message":"Invalid OData parameters supplied"}}`
	paramBody = `{"Error":{"Code":"BadRequest","Message":"Invalid parameters supplied"}}`
)

var skuRe = regexp.MustCompile(`armSkuName eq '([^']*)'`)

type answer struct {
	code   int
	body   string
	header http.Header
}

// capAPI acts like the Retail Prices API's limits: more than 20 comparisons
// in a filter, or a doubled quote, is a 400; otherwise each size asked for
// gets one regular price. answers overrides the answer to the n-th request
// (0-based).
type capAPI struct {
	mu      sync.Mutex
	filters []string
	answers map[int]answer
}

func (a *capAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	filter := r.URL.Query().Get("$filter")
	n := len(a.filters)
	a.filters = append(a.filters, filter)
	if ans, ok := a.answers[n]; ok {
		for k, v := range ans.header {
			w.Header()[k] = v
		}
		w.WriteHeader(ans.code)
		_, _ = w.Write([]byte(ans.body))
		return
	}
	if strings.Count(filter, " eq ") > 20 {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(capBody))
		return
	}
	if strings.Contains(filter, "''") {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(paramBody))
		return
	}
	var page retailPage
	for _, m := range skuRe.FindAllStringSubmatch(filter, -1) {
		page.Items = append(page.Items, item(m[1], "x", "Virtual Machines Test Series", 0.1, "2024-01-01T00:00:00Z"))
	}
	_ = json.NewEncoder(w).Encode(page)
}

func (a *capAPI) sent() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string{}, a.filters...)
}

// sizesIn is the number of sizes each request asked for.
func (a *capAPI) sizesIn() []int {
	var out []int
	for _, f := range a.sent() {
		out = append(out, len(skuRe.FindAllString(f, -1)))
	}
	return out
}

// newCapProvider is a provider on a fake clock whose passes the test runs
// itself (fetchWanted), with no background goroutine.
func newCapProvider(t *testing.T, api *capAPI) (*Provider, *clock, *int) {
	t.Helper()
	srv := httptest.NewServer(api)
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	p := NewProvider(ctx, srv.URL)
	p.retry = time.Minute
	c := &clock{t: t0}
	p.now = c.now
	updates := new(int)
	p.OnUpdate(func() { *updates++ })
	return p, c, updates
}

func testSizes(n int) []string {
	var out []string
	for i := 0; i < n; i++ {
		out = append(out, fmt.Sprintf("Standard_T%02ds_v1", i))
	}
	sort.Strings(out)
	return out
}

func queue(p *Provider, region string, sizes ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, s := range sizes {
		p.wanted[key{region, s}] = true
	}
}

// state tells whether a size is still wanted, stored, and stored with a price.
func state(p *Provider, region, size string) (wanted, stored, priced bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	sp, ok := p.prices[key{region, size}]
	return p.wanted[key{region, size}], ok, ok && sp.hasRegular
}

type syncBuf struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuf) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}
func (b *syncBuf) String() string { b.mu.Lock(); defer b.mu.Unlock(); return b.buf.String() }

func captureLog(t *testing.T) *syncBuf {
	t.Helper()
	b := &syncBuf{}
	prev := log.Writer()
	log.SetOutput(b)
	t.Cleanup(func() { log.SetOutput(prev) })
	return b
}

// fix-azure-price-batch R1
func TestBatchSizeFitsTheAPICap(t *testing.T) {
	if maxSizesPerRequest+baseConditions > maxFilterComparisons || maxFilterComparisons > 20 {
		t.Fatalf("batch %d + base %d over the cap %d", maxSizesPerRequest, baseConditions, maxFilterComparisons)
	}
	f := filterFor("eastus", testSizes(maxSizesPerRequest))
	if got := strings.Count(f, " eq "); got > 20 {
		t.Errorf("a full batch's filter has %d comparisons: %s", got, f)
	}
	if got := strings.Count(filterFor("eastus", nil), " eq "); got != baseConditions {
		t.Errorf("base conditions: %d in the filter, constant %d", got, baseConditions)
	}
}

// fix-azure-price-batch R1
func TestManySizesAreBatched(t *testing.T) {
	for _, c := range []struct {
		sizes int
		want  []int
	}{
		{17, []int{17}},
		{18, []int{17, 1}},
		{34, []int{17, 17}},
		{35, []int{17, 17, 1}},
	} {
		t.Run(fmt.Sprint(c.sizes), func(t *testing.T) {
			api := &capAPI{}
			p, _, updates := newCapProvider(t, api)
			sizes := testSizes(c.sizes)
			queue(p, "eastus", sizes...)
			p.fetchWanted()
			if got := api.sizesIn(); fmt.Sprint(got) != fmt.Sprint(c.want) {
				t.Errorf("sizes per request %v, want %v", got, c.want)
			}
			for _, s := range sizes {
				if w, _, priced := state(p, "eastus", s); w || !priced {
					t.Errorf("%s: wanted %v priced %v", s, w, priced)
				}
			}
			if *updates != 1 {
				t.Errorf("%d updates, want 1", *updates)
			}
		})
	}
}

// fix-azure-price-batch R2
func TestFailedBatchKeepsEarlierPrices(t *testing.T) {
	api := &capAPI{answers: map[int]answer{1: {http.StatusTooManyRequests, "busy", http.Header{"Retry-After": {"120"}}}}}
	p, c, updates := newCapProvider(t, api)
	sizes := testSizes(35)
	queue(p, "eastus", sizes...)
	p.fetchWanted()
	if got := len(api.sent()); got != 2 {
		t.Fatalf("%d requests, want 2 (no batch after the failed one)", got)
	}
	for i, s := range sizes {
		w, _, priced := state(p, "eastus", s)
		if i < 17 && (w || !priced) || i >= 17 && (!w || priced) {
			t.Errorf("%d %s: wanted %v priced %v", i, s, w, priced)
		}
	}
	if got := p.retryTime("eastus"); !got.Equal(t0.Add(2 * time.Minute)) {
		t.Errorf("retry at %v", got)
	}
	if *updates != 1 {
		t.Errorf("%d updates, want 1", *updates)
	}
	p.fetchWanted() // before the retry time: nothing
	if got := len(api.sent()); got != 2 {
		t.Errorf("%d requests before the retry time", got)
	}
	c.add(2 * time.Minute)
	p.fetchWanted()
	if got := api.sizesIn(); fmt.Sprint(got[2:]) != "[17 1]" {
		t.Errorf("after the retry time: sizes per request %v, want [17 1]", got[2:])
	}
	for _, s := range sizes {
		if _, _, priced := state(p, "eastus", s); !priced {
			t.Errorf("%s not priced", s)
		}
	}
}

// fix-azure-price-batch R3
func TestBadRequestIsNotRetried(t *testing.T) {
	logs := captureLog(t)
	api := &capAPI{answers: map[int]answer{0: {http.StatusBadRequest, capBody, nil}}}
	p, c, _ := newCapProvider(t, api)
	sizes := testSizes(18)
	queue(p, "eastus", sizes...)
	p.fetchWanted()
	if got := len(api.sent()); got != 2 {
		t.Fatalf("%d requests, want 2 (the other batch still sent)", got)
	}
	for i, s := range sizes {
		w, stored, priced := state(p, "eastus", s)
		if w || !stored || priced != (i == 17) {
			t.Errorf("%d %s: wanted %v stored %v priced %v", i, s, w, stored, priced)
		}
	}
	if !p.retryTime("eastus").IsZero() {
		t.Error("a refused batch gave the region a retry time")
	}
	for i := 0; i < 5; i++ {
		c.add(time.Hour)
		p.fetchWanted()
	}
	if got := len(api.sent()); got != 2 {
		t.Errorf("%d requests after 5 more passes, want 2", got)
	}
	// a second refused batch in the region: still one line; a later success
	// is no resolution of it
	more := testSizes(40)[18:35]
	api.mu.Lock()
	api.answers[2] = answer{http.StatusBadRequest, capBody, nil}
	api.mu.Unlock()
	queue(p, "eastus", more...)
	p.fetchWanted()
	queue(p, "eastus", "Standard_Z1_v1")
	p.fetchWanted()
	if _, _, priced := state(p, "eastus", "Standard_Z1_v1"); !priced {
		t.Error("a new size of the region was not priced after the refusals")
	}
	// another region refused too: its own line
	api.mu.Lock()
	api.answers[len(api.filters)] = answer{http.StatusBadRequest, capBody, nil}
	api.mu.Unlock()
	queue(p, "westeurope", "Standard_D2s_v5")
	p.fetchWanted()
	out := logs.String()
	for _, region := range []string{"eastus", "westeurope"} {
		line := "Azure pricing for " + region + ": request refused (retail prices API: 400 Bad Request: Invalid OData parameters supplied), some sizes have no price"
		if got := strings.Count(out, line); got != 1 {
			t.Errorf("%s: %d refusal lines, want 1:\n%s", region, got, out)
		}
	}
	if strings.Contains(out, "updating Azure pricing for") || strings.Contains(out, "resolved:") {
		t.Errorf("a refusal logged as a failure or resolved:\n%s", out)
	}
}

// fix-azure-price-batch R3
func TestOtherBadRequestIsRetriedSlowly(t *testing.T) {
	captureLog(t)
	api := &capAPI{answers: map[int]answer{0: {http.StatusBadRequest, paramBody, nil}}}
	p, _, _ := newCapProvider(t, api)
	queue(p, "eastus", "Standard_D2s_v5")
	p.fetchWanted()
	if got := p.retryTime("eastus"); !got.Equal(t0.Add(maxRetryAfter)) {
		t.Errorf("retry at %v, want %v", got, t0.Add(maxRetryAfter))
	}
	if w, stored, _ := state(p, "eastus", "Standard_D2s_v5"); !w || stored {
		t.Errorf("wanted %v stored %v, want still wanted", w, stored)
	}
}

// fix-azure-price-batch R7
func TestOtherFailuresAreRetried(t *testing.T) {
	captureLog(t)
	for _, code := range []int{http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError, 0} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			api := &capAPI{answers: map[int]answer{0: {code, capBody, nil}}}
			p, _, _ := newCapProvider(t, api)
			if code == 0 { // a network error: nothing listens there
				srv := httptest.NewServer(api)
				srv.Close()
				p.endpoint = srv.URL
			}
			sizes := testSizes(18)
			queue(p, "eastus", sizes...)
			p.fetchWanted()
			if got := p.retryTime("eastus"); !got.Equal(t0.Add(time.Minute)) {
				t.Errorf("retry at %v, want %v", got, t0.Add(time.Minute))
			}
			for _, s := range sizes {
				if w, stored, _ := state(p, "eastus", s); !w || stored {
					t.Errorf("%s: wanted %v stored %v", s, w, stored)
				}
			}
		})
	}
}

// fix-azure-price-batch R4
func TestResolvedOnlyWhenEveryBatchSucceeds(t *testing.T) {
	unavailable := answer{http.StatusServiceUnavailable, "busy", nil}
	resolved := "resolved: updating Azure pricing for eastus\n"
	t.Run("a later batch fails", func(t *testing.T) {
		logs := captureLog(t)
		api := &capAPI{answers: map[int]answer{0: unavailable, 2: unavailable}}
		p, c, _ := newCapProvider(t, api)
		queue(p, "eastus", testSizes(18)...)
		p.fetchWanted() // 0: 503
		c.add(time.Minute)
		p.fetchWanted() // 1: 200, 2: 503
		if strings.Contains(logs.String(), resolved) {
			t.Errorf("resolved though a batch failed:\n%s", logs.String())
		}
		c.add(time.Minute)
		p.fetchWanted() // 3: 200, the last size
		if got := strings.Count(logs.String(), resolved); got != 1 {
			t.Errorf("%d resolution lines, want 1:\n%s", got, logs.String())
		}
	})
	t.Run("a later batch is refused", func(t *testing.T) {
		logs := captureLog(t)
		api := &capAPI{answers: map[int]answer{0: unavailable, 2: {http.StatusBadRequest, capBody, nil}}}
		p, c, _ := newCapProvider(t, api)
		queue(p, "eastus", testSizes(18)...)
		p.fetchWanted() // 0: 503
		c.add(time.Minute)
		p.fetchWanted() // 1: 200, 2: the cap's 400
		out := logs.String()
		if got := strings.Count(out, resolved); got != 1 {
			t.Errorf("%d resolution lines, want 1:\n%s", got, out)
		}
		if got := strings.Count(out, "request refused"); got != 1 {
			t.Errorf("%d refusal lines, want 1:\n%s", got, out)
		}
	})
}

// fix-azure-price-batch R5
func TestInvalidNamesNeverReachTheAPI(t *testing.T) {
	api := &capAPI{}
	p, _, _ := newCapProvider(t, api)
	for _, n := range []struct{ region, size string }{
		{"eastus", "it's"},
		{"east us", "Standard_D2s_v5"},
		{"eastus", "-Standard_D2s_v5"},
		{"eastus", "Standard_D2s_v5."},
	} {
		node := aksNode(n.size, map[string]string{v1.LabelTopologyRegion: n.region})
		if _, ok := p.NodePrice(node); ok {
			t.Errorf("%q %q has a price", n.region, n.size)
		}
		if w, stored, _ := state(p, n.region, n.size); w || !stored {
			t.Errorf("%q %q: wanted %v stored %v, want stored with no price", n.region, n.size, w, stored)
		}
	}
	sizes := append(testSizes(16), "bogus.x_y-z")
	queue(p, "eastus", sizes...)
	p.fetchWanted()
	if got := api.sizesIn(); fmt.Sprint(got) != "[17]" {
		t.Errorf("sizes per request %v, want [17]", got)
	}
	for _, f := range api.sent() {
		if strings.Contains(f, "it'") || strings.Contains(f, "''") || strings.Contains(f, "east us") {
			t.Errorf("an invalid name reached the API: %s", f)
		}
	}
	for _, s := range sizes {
		if _, _, priced := state(p, "eastus", s); !priced {
			t.Errorf("%s not priced", s)
		}
	}
	for _, ok := range []string{"bogus.x_y-z", "Standard_D2s_v5", "a", "eastus2"} {
		if !validName(ok) {
			t.Errorf("%q refused", ok)
		}
	}
}

// fix-azure-price-batch R6
func TestErrorMessageInTheLogLine(t *testing.T) {
	logs := captureLog(t)
	// a newline in the message could start a line of its own, and a huge
	// body is not read
	twoLines := `{"Error":{"Code":"BadRequest","Message":"bad\nresolved: updating Azure pricing for x"}}`
	huge := `{"Error":{"Code":"BadRequest","Message":"` + strings.Repeat("m", 5000) + `"}}`
	long := `{"Error":{"Code":"BadRequest","Message":"` + strings.Repeat("y", 300) + `"}}`
	api := &capAPI{answers: map[int]answer{
		0: {http.StatusBadRequest, twoLines, nil},
		1: {http.StatusBadRequest, paramBody, nil},
		2: {http.StatusBadRequest, huge, nil},
		3: {http.StatusBadRequest, long, nil},
		4: {http.StatusTooManyRequests, "busy", nil},
	}}
	p, _, _ := newCapProvider(t, api)
	for _, region := range []string{"a1", "b1", "c1", "d1", "e1"} {
		queue(p, region, "Standard_D2s_v5")
	}
	p.fetchWanted()
	out := logs.String()
	for _, want := range []string{
		"updating Azure pricing for a1, retail prices API: 400 Bad Request: bad resolved: updating Azure pricing for x (waiting 10m0s)\n",
		"updating Azure pricing for b1, retail prices API: 400 Bad Request: Invalid parameters supplied (waiting 10m0s)\n",
		"updating Azure pricing for c1, retail prices API: 400 Bad Request (waiting 10m0s)\n",
		"updating Azure pricing for d1, retail prices API: 400 Bad Request: " + strings.Repeat("y", maxMessage) + " (waiting 10m0s)\n",
		"updating Azure pricing for e1, retail prices API: 429 Too Many Requests (no retry header)\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "\nresolved:") {
		t.Errorf("the message started a line of its own:\n%s", out)
	}
}
