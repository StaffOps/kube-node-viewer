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
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
)

var regionRe = regexp.MustCompile(`armRegionName eq '([^']*)'`)

// regionOf is the region a request's filter asks for.
func regionOf(filter string) string {
	if m := regionRe.FindStringSubmatch(filter); m != nil {
		return m[1]
	}
	return ""
}

// clock is a settable time for the provider.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

var t0 = time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)

// fix-azure-retry R1
func TestRetryDelay(t *testing.T) {
	floor := time.Minute
	h := func(kv ...string) http.Header {
		out := http.Header{}
		for i := 0; i < len(kv); i += 2 {
			out.Set(kv[i], kv[i+1])
		}
		return out
	}
	date := func(d time.Duration) string { return t0.Add(d).Format(http.TimeFormat) }
	cases := []struct {
		name  string
		code  int
		h     http.Header
		want  time.Duration
		noted bool
	}{
		{"seconds above the floor", 429, h("Retry-After", "120"), 2 * time.Minute, true},
		{"seconds below the floor", 429, h("Retry-After", "10"), time.Minute, true},
		{"a date 5 minutes ahead", 429, h("Retry-After", date(5*time.Minute)), 5 * time.Minute, true},
		{"capped", 429, h("Retry-After", "3600"), 10 * time.Minute, true},
		{"absurd seconds", 429, h("Retry-After", "99999999999999999999"), 10 * time.Minute, true},
		{"503 too", 503, h("Retry-After", "180"), 3 * time.Minute, true},
		{"the larger of both headers", 429, h("Retry-After", "60", "x-ms-ratelimit-microsoft.consumption-retry-after", "240"), 4 * time.Minute, true},
		{"words", 429, h("Retry-After", "soon"), time.Minute, true},
		{"negative", 429, h("Retry-After", "-5"), time.Minute, true},
		{"plus sign", 429, h("Retry-After", "+120"), time.Minute, true},
		{"fraction", 429, h("Retry-After", "1.5"), time.Minute, true},
		{"a past date", 429, h("Retry-After", date(-time.Minute)), time.Minute, true},
		{"no header", 429, h(), time.Minute, true},
		{"other status", 500, h("Retry-After", "300"), time.Minute, false},
	}
	for _, c := range cases {
		got, note := retryDelay(c.code, c.h, t0, floor)
		if got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
		if (note != "") != c.noted {
			t.Errorf("%s: note %q", c.name, note)
		}
	}
	// a floor above the cap stays (TestNoRequestBeforeTheRetryDelay)
	if got, _ := retryDelay(429, h("Retry-After", "3600"), t0, time.Hour); got != time.Hour {
		t.Errorf("floor 1h: %v", got)
	}
}

// waitFor polls cond for up to 5 s.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); !cond(); time.Sleep(5 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal(what)
		}
	}
}

func (f *fakeAPI) requests(region string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.byRegion[region]
}

func (p *Provider) retryTime(region string) time.Time {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.retryAt[region]
}

func (p *Provider) poke() {
	select {
	case p.wake <- struct{}{}:
	default:
	}
}

// fix-azure-retry R1
func TestRetryAfterDelaysTheRegion(t *testing.T) {
	api := &fakeAPI{failIn: "eastus2", header: http.Header{"Retry-After": {"120"}}}
	p, _ := newTestProvider(t, api)
	c := &clock{t: t0}
	p.now = c.now
	p.NodePrice(aksNode("Standard_D2s_v5", nil))
	waitFor(t, "no retry time recorded", func() bool { return !p.retryTime("eastus2").IsZero() })
	if got := p.retryTime("eastus2"); !got.Equal(t0.Add(2 * time.Minute)) {
		t.Errorf("retry at %v, want %v", got, t0.Add(2*time.Minute))
	}
	// p.retry is 50 ms: woken again and again, nothing is asked
	for i := 0; i < 5; i++ {
		p.poke()
		time.Sleep(20 * time.Millisecond)
	}
	if n := api.requests("eastus2"); n != 1 {
		t.Errorf("%d requests during the Retry-After time, want 1", n)
	}
}

// A 429 on a later page of the answer is read the same way.
// fix-azure-retry R1
func TestRetryAfterOnALaterPage(t *testing.T) {
	api := &fakeAPI{failPage2: true, header: http.Header{"Retry-After": {"300"}}}
	p, _ := newTestProvider(t, api)
	c := &clock{t: t0}
	p.now = c.now
	p.NodePrice(aksNode("Standard_D2s_v5", nil))
	waitFor(t, "no retry time recorded", func() bool { return !p.retryTime("eastus2").IsZero() })
	if got := p.retryTime("eastus2"); !got.Equal(t0.Add(5 * time.Minute)) {
		t.Errorf("retry at %v, want %v", got, t0.Add(5*time.Minute))
	}
}

// fix-azure-retry R2
func TestRegionsRetryIndependently(t *testing.T) {
	api := &fakeAPI{failIn: "westeurope", header: http.Header{"Retry-After": {"120"}}}
	p, updated := newTestProvider(t, api)
	c := &clock{t: t0}
	p.now = c.now
	west := aksNode("Standard_D2s_v5", map[string]string{v1.LabelTopologyRegion: "westeurope"})
	p.NodePrice(west)
	waitFor(t, "westeurope never failed", func() bool { return !p.retryTime("westeurope").IsZero() })

	// a size of another region is asked for in the same pass, without
	// waiting for westeurope's retry time
	east := aksNode("Standard_D2s_v5", nil)
	p.NodePrice(east)
	waitFor(t, "eastus2 was not asked", func() bool { _, ok := p.NodePrice(east); return ok })
	<-updated
	if n := api.requests("westeurope"); n > 2 { // one more may be in flight with eastus2's pass
		t.Errorf("westeurope asked %d times during its retry time", n)
	}
	if !p.retryTime("eastus2").IsZero() {
		t.Error("eastus2 has a retry time without failing")
	}

	// each keeps its own retry time: eastus2 now fails with a longer one;
	// westeurope's success after its time does not touch eastus2's
	api.mu.Lock()
	api.failIn = "eastus2"
	api.header = http.Header{"Retry-After": {"300"}}
	api.mu.Unlock()
	p.NodePrice(aksNode("Standard_D2pds_v6", nil))
	waitFor(t, "eastus2 never failed", func() bool { return !p.retryTime("eastus2").IsZero() })
	eastAt := p.retryTime("eastus2")
	if !eastAt.Equal(t0.Add(5 * time.Minute)) {
		t.Errorf("eastus2 retry at %v", eastAt)
	}
	before := api.requests("eastus2")
	c.add(3 * time.Minute) // westeurope due, eastus2 not
	p.poke()
	waitFor(t, "westeurope was not retried", func() bool { _, ok := p.NodePrice(west); return ok })
	if !p.retryTime("westeurope").IsZero() {
		t.Error("westeurope kept a retry time after succeeding")
	}
	if got := p.retryTime("eastus2"); !got.Equal(eastAt) {
		t.Errorf("eastus2 retry time moved: %v, want %v", got, eastAt)
	}
	if n := api.requests("eastus2"); n != before {
		t.Errorf("eastus2 asked before its retry time: %d, want %d", n, before)
	}
}

// fix-azure-retry R3
func TestSuccessAfterFailureLogsResolved(t *testing.T) {
	var buf bytes.Buffer
	var mu sync.Mutex
	prev := log.Writer()
	log.SetOutput(writerFunc(func(b []byte) (int, error) { mu.Lock(); defer mu.Unlock(); return buf.Write(b) }))
	t.Cleanup(func() { log.SetOutput(prev) })

	api := &fakeAPI{failTimes: 2} // two failures, then answers
	p, updated := newTestProvider(t, api)
	n := aksNode("Standard_D2s_v5", nil)
	p.NodePrice(n)
	settled(t, p)
	<-updated
	// a region that never failed: no resolution
	west := aksNode("Standard_D2s_v5", map[string]string{"topology.kubernetes.io/region": "westeurope"})
	p.NodePrice(west)
	waitFor(t, "westeurope was not priced", func() bool { _, ok := p.NodePrice(west); return ok })

	mu.Lock()
	out := buf.String()
	mu.Unlock()
	if got := strings.Count(out, "updating Azure pricing for eastus2, retail prices API: 429"); got != 2 {
		t.Errorf("%d failure lines, want 2:\n%s", got, out)
	}
	if !strings.Contains(out, "(no retry header)") {
		t.Errorf("the failure line does not say there was no retry header:\n%s", out)
	}
	if got := strings.Count(out, "resolved: updating Azure pricing for eastus2\n"); got != 1 {
		t.Errorf("%d resolution lines for eastus2, want 1:\n%s", got, out)
	}
	if strings.Contains(out, "resolved: updating Azure pricing for westeurope") {
		t.Errorf("a resolution for a region that never failed:\n%s", out)
	}
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(b []byte) (int, error) { return f(b) }
