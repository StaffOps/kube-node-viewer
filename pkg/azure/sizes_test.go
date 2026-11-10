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
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/staffops/kube-node-viewer/pkg/recommend"
)

// goAz is a fake AzRunner in Go: accounts and token answers the test sets.
type goAz struct {
	mu       sync.Mutex
	accounts string
	token    string
	fail     error
	stderr   string
	tokens   int
	// now issues expires_on (the test's clock); delay slows account list
	now   func() time.Time
	delay time.Duration
	argv  [][]string
}

func newGoAz() *goAz {
	return &goAz{accounts: enabledAccountJSON, token: "tok-1", now: time.Now}
}

const enabledAccountJSON = `[{"cloudName":"AzureCloud","id":"00000000-0000-0000-0000-000000000000","state":"Enabled","tenantId":"tenant-secret","user":{"name":"user-secret@example.com"}}]`

func (g *goAz) run(_ context.Context, args ...string) ([]byte, []byte, error) {
	g.mu.Lock()
	g.argv = append(g.argv, args)
	delay := g.delay
	g.mu.Unlock()
	time.Sleep(delay)
	g.mu.Lock()
	defer g.mu.Unlock()
	if strings.Join(args[:2], " ") == "account list" {
		return []byte(g.accounts), nil, nil
	}
	g.tokens++
	if g.fail != nil {
		return nil, []byte(g.stderr), g.fail
	}
	exp := g.now().Add(time.Hour).Unix()
	return []byte(`{"accessToken":"` + g.token + `","expires_on":` + strconv.FormatInt(exp, 10) + `}`), nil, nil
}

// waitGen waits for the sizes' generation to pass g.
func waitGen(t *testing.T, s *Sizes, g uint64) uint64 {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if n := s.generation(); n > g {
			return n
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("generation stayed at %d", g)
	return 0
}

var eastus = []recommend.Pair{{Subscription: zeroSub, Region: "eastus", AgentPools: []string{"userpool"}}}

// azure-live-skus R1, D2: the first analysis starts one read per pair and
// notes it is loading; the answer moves the generation (so a static cluster
// analyses again) and the next snapshot has the sizes.
func TestSizesLoadOncePerPair(t *testing.T) {
	arm := newFakeARM(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(recorded(t)) })
	az := newGoAz()
	s := newSizes(context.Background(), az.run, arm.srv.URL, arm.srv.Client().Transport, time.Now)
	g := s.generation()
	snap := s.begin(eastus)
	if strings.Join(snap.notes, "|") != "Azure VM sizes: loading" || len(snap.names) != 0 {
		t.Errorf("first snapshot: notes %v, %d sizes", snap.notes, len(snap.names))
	}
	waitGen(t, s, g)
	snap = s.begin(eastus)
	if len(snap.notes) != 0 || snap.union["Standard_D4s_v5"].VCPU != 4 {
		t.Errorf("after the answer: notes %v, %d sizes", snap.notes, len(snap.names))
	}
	s.begin(eastus)
	if arm.requests.Load() != 1 {
		t.Errorf("%d requests for one pair", arm.requests.Load())
	}
	two := append(append([]recommend.Pair(nil), eastus...), recommend.Pair{Subscription: zeroSub, Region: "westus"})
	g = s.generation()
	s.begin(two)
	waitGen(t, s, g)
	if arm.requests.Load() != 2 || az.tokens != 1 {
		t.Errorf("two regions: %d requests, %d token runs", arm.requests.Load(), az.tokens)
	}
	if s := newSizes(context.Background(), az.run, arm.srv.URL, nil, time.Now); len(s.begin(nil).notes) != 0 || arm.requests.Load() != 2 {
		t.Error("no pair, yet a request or a note")
	}
}

// azure-live-skus R5: every row's line, with no subscription id, token or
// Resource Manager message in it.
func TestSizeFailureLines(t *testing.T) {
	const msg = "The client 'someone@example.com' with object id 'abc' does not have authorization over scope '/subscriptions/00000000-0000-0000-0000-000000000000'"
	answer := func(code int, errCode string) func(w http.ResponseWriter, r *http.Request) {
		return func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(code)
			_, _ = w.Write([]byte(`{"error":{"code":"` + errCode + `","message":"` + msg + `"}}`))
		}
	}
	for _, tt := range []struct {
		name string
		az   func(g *goAz)
		arm  func(w http.ResponseWriter, r *http.Request)
		pair recommend.Pair
		want string
	}{
		{"no az", func(g *goAz) { g.fail = ErrAzNotFound }, nil, eastus[0], "the Azure CLI (az) was not found; install it and run az login"},
		{"not signed in", func(g *goAz) { g.accounts = "[]" }, nil, eastus[0], "not signed in to Azure; run az login"},
		{"other subscription", func(g *goAz) { g.accounts = strings.Replace(enabledAccountJSON, "00000000-0000", "22222222-0000", 1) }, nil, eastus[0], "the signed-in account can't see the nodes' subscription; az login with an account that can"},
		{"disabled", func(g *goAz) { g.accounts = strings.Replace(enabledAccountJSON, "Enabled", "Deleted", 1) }, nil, eastus[0], "the nodes' subscription was not found or is disabled"},
		{"az failed", func(g *goAz) { g.fail, g.stderr = context.Canceled, "boom" }, nil, eastus[0], "the Azure CLI failed to give a token; run az account get-access-token to see why"},
		{"401", nil, answer(401, "InvalidAuthenticationToken"), eastus[0], "Azure refused the token; run az login"},
		{"403", nil, answer(403, "AuthorizationFailed"), eastus[0], "no permission to read VM sizes (Microsoft.Compute/skus/read on the subscription, e.g. the Reader role)"},
		{"403 policy", nil, answer(403, "RequestDisallowedByPolicy"), eastus[0], "Azure refused the request (403); check the subscription's policies"},
		{"404", nil, answer(404, "SubscriptionNotFound"), eastus[0], "the nodes' subscription was not found or is disabled"},
		{"400", nil, answer(400, "InvalidSubscriptionId"), eastus[0], "Azure refused the request (400)"},
		{"409", nil, answer(409, "Conflict"), eastus[0], "Azure refused the request (409)"},
		{"503", nil, answer(503, "ServiceUnavailable"), eastus[0], "Azure Resource Manager unavailable (503); retrying in 1m"},
		{"malformed", nil, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("{")) }, eastus[0], "Azure Resource Manager gave an answer that can't be read; retrying in 1m"},
		{"bad subscription", nil, nil, recommend.Pair{Subscription: "x&y", Region: "eastus"}, "the nodes' provider ID names no valid subscription"},
		{"bad region", nil, nil, recommend.Pair{Subscription: zeroSub, Region: ""}, "a node has no valid region label"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var logs bytes.Buffer
			oldLog := log.Writer()
			log.SetOutput(&logs)
			defer log.SetOutput(oldLog)
			oldErr := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w
			defer func() { os.Stderr = oldErr }()
			h := tt.arm
			if h == nil {
				h = func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(recorded(t)) }
			}
			arm := newFakeARM(t, h)
			az := newGoAz()
			if tt.az != nil {
				tt.az(az)
			}
			s := newSizes(context.Background(), az.run, arm.srv.URL, arm.srv.Client().Transport, time.Now)
			pairs := []recommend.Pair{tt.pair}
			g := s.generation()
			snap := s.begin(pairs)
			if !strings.HasPrefix(tt.name, "bad") {
				waitGen(t, s, g)
				snap = s.begin(pairs)
			} else if az.tokens != 0 || arm.requests.Load() != 0 {
				t.Error("an invalid value reached az or Resource Manager")
			}
			got := strings.Join(snap.notes, "|")
			if got != "Azure VM sizes: "+tt.want {
				t.Errorf("notes %q, want %q", got, tt.want)
			}
			_ = w.Close()
			stderr, _ := io.ReadAll(r)
			for _, out := range []string{got, logs.String(), string(stderr)} {
				for _, leak := range []string{zeroSub, "tok-1", "tenant-secret", "user-secret", "someone", "object id", "boom"} {
					if strings.Contains(out, leak) {
						t.Errorf("leaks %q: %q", leak, out)
					}
				}
			}
			if !strings.Contains(logs.String(), tt.want) {
				t.Errorf("not logged: %q", logs.String())
			}
		})
	}
}

// azure-live-skus R6, R4: after a failure, no request before a minute, one
// after it (the generation moves when the retry falls due, so a static
// cluster analyses again). A 401 or 403 drops the token, still valid by the
// test's clock, so the retry carries the new sign-in's; after a 404 the
// account is checked again and a changed account's token is used.
func TestSizesRetryAfterSignIn(t *testing.T) {
	for _, tt := range []struct {
		name    string
		status  int
		code    string
		account bool // the account entry changes before the retry
	}{
		{"401", 401, "InvalidAuthenticationToken", false},
		{"403", 403, "AuthorizationFailed", false},
		{"404 then another account", 404, "SubscriptionNotFound", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var fail atomic.Bool
			fail.Store(true)
			var auth atomic.Value
			arm := newFakeARM(t, func(w http.ResponseWriter, r *http.Request) {
				auth.Store(r.Header.Get("Authorization"))
				if fail.Load() {
					w.WriteHeader(tt.status)
					_, _ = w.Write([]byte(`{"error":{"code":"` + tt.code + `"}}`))
					return
				}
				_, _ = w.Write(recorded(t))
			})
			c := &clock{t: time.Unix(1_800_000_000, 0)}
			az := newGoAz()
			az.now = c.now
			s := newSizes(context.Background(), az.run, arm.srv.URL, arm.srv.Client().Transport, c.now)
			g := s.generation()
			s.begin(eastus)
			g = waitGen(t, s, g)
			c.add(59 * time.Second)
			if s.generation() != g {
				t.Error("the retry fell due before a minute")
			}
			s.begin(eastus)
			if arm.requests.Load() != 1 {
				t.Errorf("%d requests before the minute", arm.requests.Load())
			}
			fail.Store(false)
			az.mu.Lock()
			az.token = "tok-2"
			if tt.account {
				az.accounts = strings.Replace(enabledAccountJSON, "user-secret@example.com", "other@example.com", 1)
			}
			az.mu.Unlock()
			c.add(time.Second)
			if s.generation() == g {
				t.Fatal("no new generation when the retry fell due")
			}
			g = s.generation()
			s.begin(eastus)
			waitGen(t, s, g)
			if got := auth.Load(); got != "Bearer tok-2" {
				t.Errorf("the retry carried %q, want the new sign-in's token", got)
			}
			if snap := s.begin(eastus); len(snap.notes) != 0 || len(snap.names) == 0 || arm.requests.Load() != 2 {
				t.Errorf("after the retry: notes %v, %d sizes, %d requests", snap.notes, len(snap.names), arm.requests.Load())
			}
		})
	}
}

// azure-live-skus R6: a 429 waits for its Retry-After: no request at 60 s,
// one at 120 s.
func TestSizesRetryAfterHeader(t *testing.T) {
	var limited atomic.Bool
	limited.Store(true)
	arm := newFakeARM(t, func(w http.ResponseWriter, _ *http.Request) {
		if limited.Load() {
			w.Header().Set("Retry-After", "120")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write(recorded(t))
	})
	c := &clock{t: time.Unix(1_800_000_000, 0)}
	s := newSizes(context.Background(), newGoAz().run, arm.srv.URL, arm.srv.Client().Transport, c.now)
	g := s.generation()
	s.begin(eastus)
	waitGen(t, s, g)
	if notes := s.begin(eastus).notes; len(notes) != 1 || notes[0] != "Azure VM sizes: Azure Resource Manager unavailable (429); retrying in 2m" {
		t.Errorf("notes %v", notes)
	}
	limited.Store(false)
	c.add(60 * time.Second)
	s.generation()
	s.begin(eastus)
	time.Sleep(50 * time.Millisecond)
	if arm.requests.Load() != 1 {
		t.Errorf("%d requests at 60 s, want 1", arm.requests.Load())
	}
	c.add(60 * time.Second)
	g = s.generation()
	s.begin(eastus)
	waitGen(t, s, g)
	if arm.requests.Load() != 2 {
		t.Errorf("%d requests at 120 s, want 2", arm.requests.Load())
	}
}

// azure-live-skus R4, D2: two regions of one subscription share a slow token
// request, and their failures are separate values (run with -race).
func TestSharedTokenFailureNoRace(t *testing.T) {
	arm := newFakeARM(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(recorded(t)) })
	az := newGoAz()
	az.accounts = "[]"
	az.delay = 200 * time.Millisecond
	s := newSizes(context.Background(), az.run, arm.srv.URL, arm.srv.Client().Transport, time.Now)
	pairs := []recommend.Pair{eastus[0], {Subscription: zeroSub, Region: "westus"}}
	g := s.generation()
	s.begin(pairs)
	for s.generation() < g+2 {
		waitGen(t, s, s.generation())
	}
	for range 20 {
		_ = s.begin(pairs).notes
	}
	if notes := s.begin(pairs).notes; len(notes) != 1 || notes[0] != "Azure VM sizes: not signed in to Azure; run az login" {
		t.Errorf("notes %v", notes)
	}
}

// azure-live-skus R10: every argument given to az is a plain word (Go does
// not escape az.cmd's arguments for cmd.exe, so validation is the only
// barrier on Windows).
func TestAzArgumentsArePlain(t *testing.T) {
	arm := newFakeARM(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(recorded(t)) })
	az := newGoAz()
	s := newSizes(context.Background(), az.run, arm.srv.URL, arm.srv.Client().Transport, time.Now)
	g := s.generation()
	s.begin(append(eastus, recommend.Pair{Subscription: zeroSub + "&calc", Region: "eastus"}, recommend.Pair{Subscription: zeroSub, Region: "east us"}))
	waitGen(t, s, g)
	plain := regexp.MustCompile(`^[A-Za-z0-9:/._-]+$`)
	az.mu.Lock()
	defer az.mu.Unlock()
	if len(az.argv) == 0 {
		t.Fatal("az never ran")
	}
	for _, argv := range az.argv {
		for _, a := range argv {
			if !plain.MatchString(a) {
				t.Errorf("az argument %q", a)
			}
		}
	}
}

// azure-live-skus R5: a failure is logged once, its resolution once; with
// one of two pairs failing, the line names that pair's pools.
func TestSizesLogOnceAndNamePools(t *testing.T) {
	var buf bytes.Buffer
	old := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(old)
	arm := newFakeARM(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "westus") {
			w.WriteHeader(403)
			_, _ = w.Write([]byte(`{"error":{"code":"AuthorizationFailed"}}`))
			return
		}
		_, _ = w.Write(recorded(t))
	})
	c := &clock{t: time.Unix(1_800_000_000, 0)}
	s := newSizes(context.Background(), newGoAz().run, arm.srv.URL, arm.srv.Client().Transport, c.now)
	pairs := []recommend.Pair{eastus[0], {Subscription: zeroSub, Region: "westus", AgentPools: []string{"system", "legacy"}, NodePools: []string{"burst"}}}
	g := s.generation()
	s.begin(pairs)
	g = waitGen(t, s, g)
	for s.begin(pairs).notes[0] == "Azure VM sizes: loading" {
		g = waitGen(t, s, g)
	}
	c.add(time.Minute)
	g = s.generation()
	s.begin(pairs)
	waitGen(t, s, g)
	snap := s.begin(pairs)
	if want := "Azure VM sizes (agent pools legacy, system; NodePools burst): no permission to read VM sizes (Microsoft.Compute/skus/read on the subscription, e.g. the Reader role)"; strings.Join(snap.notes, "|") != want {
		t.Errorf("notes %q, want %q", snap.notes, want)
	}
	if n := strings.Count(buf.String(), "Azure VM sizes: no permission"); n != 1 {
		t.Errorf("logged %d times:\n%s", n, buf.String())
	}
}

// azure-live-skus R3, R2, R7: the advisor answers sizes from the snapshot;
// a size is available to a node only in its own pair's answer, and per zone;
// with pricing disabled nothing is read.
func TestAdvisorAvailability(t *testing.T) {
	arm := newFakeARM(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "westus") {
			_, _ = w.Write([]byte(`{"value":[{"resourceType":"virtualMachines","name":"Standard_D96s_v5","family":"standardDSv5Family","locationInfo":[{"location":"westus","zones":["1"]}],"capabilities":[{"name":"vCPUs","value":"96"},{"name":"MemoryGB","value":"384"},{"name":"CpuArchitectureType","value":"x64"}]}]}`))
			return
		}
		_, _ = w.Write(recorded(t))
	})
	s := newSizes(context.Background(), newGoAz().run, arm.srv.URL, arm.srv.Client().Transport, time.Now)
	a := NewAdvisor(&fakeLister{}, false, s)
	pairs := []recommend.Pair{eastus[0], {Subscription: zeroSub, Region: "westus"}}
	g := s.generation()
	a.Begin(pairs)
	if a.SizesLoaded(recommend.Node{Subscription: zeroSub, Region: "eastus"}) || len(a.Types()) != 0 {
		t.Error("sizes before the answer")
	}
	for s.generation() < g+2 {
		waitGen(t, s, s.generation())
	}
	a.Begin(pairs)
	east := recommend.Node{Cloud: recommend.CloudAzure, Subscription: zeroSub, Region: "eastus", Zone: "eastus-3"}
	for _, tt := range []struct {
		n    recommend.Node
		size string
		want bool
	}{
		{east, "Standard_D4s_v5", true},
		{east, "Standard_D16s_v5", false},
		{east, "Standard_D2ds_v5", false},
		{recommend.Node{Subscription: zeroSub, Region: "eastus", Zone: "eastus-1"}, "Standard_D2ds_v5", true},
		// a fault domain, not a zone: the regional check only
		{recommend.Node{Subscription: zeroSub, Region: "eastus", Zone: "3"}, "Standard_D2ds_v5", true},
		{east, "Standard_D96s_v5", false},
		{recommend.Node{Subscription: zeroSub, Region: "westus", Zone: "westus-1"}, "Standard_D96s_v5", true},
	} {
		if got := a.Available(tt.n, tt.size); got != tt.want {
			t.Errorf("Available(%s %s, %s) = %v, want %v", tt.n.Region, tt.n.Zone, tt.size, got, tt.want)
		}
	}
	if sp, ok := a.Spec("Standard_D4ds_v5"); !ok || sp.TempDiskMB != 153600 || sp.Family != "ddsv5" {
		t.Errorf("Spec = %+v %v", sp, ok)
	}
	if !a.SizesLoaded(east) {
		t.Error("eastus not loaded")
	}
	off := NewAdvisor(nil, true, s)
	off.Begin(pairs)
	if len(off.Types()) != 0 || off.sizes != nil {
		t.Error("pricing disabled, yet sizes are read")
	}
}
