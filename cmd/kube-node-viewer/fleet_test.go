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

package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/staffops/kube-node-viewer/pkg/model"
	"github.com/staffops/kube-node-viewer/pkg/pricing"
)

// Tests for the Clusters screen's members (clusters-screen D3, D8, D10,
// D11), with a fake connector: no cluster, no network.

const waitLong = 5 * time.Second

// fakeMember is one connection the fake connector made.
type fakeMember struct {
	name   string
	store  *model.Cluster
	ctx    context.Context
	cancel context.CancelFunc
	notify func(uint64)

	mu     sync.Mutex
	st     model.ConnStatus
	nodes  int
	podsN  int
	prices priceFunc
}

func (m *fakeMember) setStatus(st model.ConnStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.st = st
}

func (m *fakeMember) status() model.ConnStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.st
}

func (m *fakeMember) counts() (nodes, pods int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.nodes, m.podsN
}

func (m *fakeMember) cancelled() bool { return m.ctx.Err() != nil }

// fakeConnector records calls; a name with a gate blocks in connect until
// the gate is closed.
type fakeConnector struct {
	t *testing.T

	mu      sync.Mutex
	calls   []string
	gates   map[string]chan struct{}
	errs    map[string]error
	status  map[string]model.ConnStatus
	nodes   map[string][]string // provider IDs added to the store before connect returns
	members map[string][]*fakeMember
}

func newFakeConnector(t *testing.T) *fakeConnector {
	return &fakeConnector{
		t:       t,
		gates:   map[string]chan struct{}{},
		errs:    map[string]error{},
		status:  map[string]model.ConnStatus{},
		nodes:   map[string][]string{},
		members: map[string][]*fakeMember{},
	}
}

func (fc *fakeConnector) gate(name string) chan struct{} {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	g := make(chan struct{})
	fc.gates[name] = g
	return g
}

func (fc *fakeConnector) connect(name string, prices priceFunc, notify func(gen uint64)) (*memberConn, error) {
	fc.mu.Lock()
	fc.calls = append(fc.calls, name)
	g := fc.gates[name]
	fc.mu.Unlock()
	if g != nil {
		<-g
	}
	fc.mu.Lock()
	err := fc.errs[name]
	st := fc.status[name]
	ids := fc.nodes[name]
	fc.mu.Unlock()
	if err != nil {
		return nil, err
	}
	store := model.NewCluster()
	for i, id := range ids {
		store.AddNode(model.NewNode(&v1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-node-%d", name, i)},
			Spec:       v1.NodeSpec{ProviderID: id},
		})).Show()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m := &fakeMember{name: name, store: store, ctx: ctx, cancel: cancel, notify: notify, st: st, prices: prices}
	fc.mu.Lock()
	fc.members[name] = append(fc.members[name], m)
	fc.mu.Unlock()
	return &memberConn{
		store:  store,
		ctx:    ctx,
		cancel: cancel,
		status: m.status,
		startNodes: func() {
			m.mu.Lock()
			m.nodes++
			m.mu.Unlock()
		},
		startPods: func() {
			m.mu.Lock()
			m.podsN++
			m.mu.Unlock()
		},
	}, nil
}

func (fc *fakeConnector) callCount() int {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	return len(fc.calls)
}

func (fc *fakeConnector) callsOf(name string) int {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	n := 0
	for _, c := range fc.calls {
		if c == name {
			n++
		}
	}
	return n
}

func (fc *fakeConnector) called() []string {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	return slices.Clone(fc.calls)
}

// member is the latest connection made for name, waiting for it.
func (fc *fakeConnector) member(name string) *fakeMember {
	fc.t.Helper()
	var m *fakeMember
	waitFor(fc.t, "a connection of "+name, func() bool {
		fc.mu.Lock()
		defer fc.mu.Unlock()
		if l := fc.members[name]; len(l) > 0 {
			m = l[len(l)-1]
			return true
		}
		return false
	})
	return m
}

// recorder is the fleet's send.
type recorder struct {
	mu   sync.Mutex
	msgs []tea.Msg
}

func (r *recorder) send(msg tea.Msg) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.msgs = append(r.msgs, msg)
}

func (r *recorder) all() []tea.Msg {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.msgs)
}

func (r *recorder) ready(name string) []model.MemberReadyMsg {
	var out []model.MemberReadyMsg
	for _, m := range r.all() {
		if rm, ok := m.(model.MemberReadyMsg); ok && rm.Context == name {
			out = append(out, rm)
		}
	}
	return out
}

func (r *recorder) left() []model.MemberLeftMsg {
	var out []model.MemberLeftMsg
	for _, m := range r.all() {
		if lm, ok := m.(model.MemberLeftMsg); ok {
			out = append(out, lm)
		}
	}
	return out
}

func (r *recorder) changed() []model.MembersChangedMsg {
	var out []model.MembersChangedMsg
	for _, m := range r.all() {
		if cm, ok := m.(model.MembersChangedMsg); ok {
			out = append(out, cm)
		}
	}
	return out
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(waitLong)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// settle gives the fleet's goroutines time to do what they would do.
func settle() { time.Sleep(3 * statusPoll / 2) }

func noPrices(context.Context) priceFunc {
	return func(context.Context, string) (pricing.Provider, string, error) {
		return pricing.None{}, "", nil
	}
}

func infos(m map[string]string) func() ([]model.ContextInfo, error) {
	return func() ([]model.ContextInfo, error) {
		var out []model.ContextInfo
		for name, cloud := range m {
			out = append(out, model.ContextInfo{Name: name, Cloud: cloud})
		}
		return out, nil
	}
}

func newTestFleet(t *testing.T, guesses map[string]string) (*fleet, *fakeConnector, *recorder) {
	fc := newFakeConnector(t)
	rec := &recorder{}
	f := newFleet(fc.connect, rec.send, infos(guesses), noPrices)
	t.Cleanup(f.Close)
	return f, fc, rec
}

func names(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("ctx-%02d", i)
	}
	return out
}

// clusters-screen R2, D3: no more than 4 members connect at once; the
// others wait in order.
func TestFleetAtMostFourStarting(t *testing.T) {
	f, fc, _ := newTestFleet(t, nil)
	all := names(10)
	for _, n := range all {
		fc.gate(n)
	}
	f.SetWanted(all, "")
	waitFor(t, "4 connects", func() bool { return fc.callCount() == maxStarting })
	settle()
	if got := fc.called(); len(got) != maxStarting {
		t.Fatalf("%d connects in flight, want %d: %v", len(got), maxStarting, got)
	}
	got := fc.called()
	slices.Sort(got)
	if !reflect.DeepEqual(got, all[:maxStarting]) {
		t.Errorf("first connects %v, want the first four %v", got, all[:maxStarting])
	}
}

// clusters-screen R2, D3: a start place is given back when the node watch
// synced, when the status has an error, when connect fails, and when the
// member is closed; each frees exactly one place.
func TestFleetStartPlaceFreed(t *testing.T) {
	cases := []struct {
		name string
		// free makes ctx-00 give its place back; its connect is gated
		free func(t *testing.T, f *fleet, fc *fakeConnector, gate chan struct{}, all []string)
	}{
		{"node watch synced", func(t *testing.T, f *fleet, fc *fakeConnector, gate chan struct{}, _ []string) {
			close(gate)
			fc.member("ctx-00").setStatus(model.ConnStatus{Synced: true})
		}},
		{"watch error", func(t *testing.T, f *fleet, fc *fakeConnector, gate chan struct{}, _ []string) {
			close(gate)
			fc.member("ctx-00").setStatus(model.ConnStatus{Err: "Unauthorized"})
		}},
		{"connect error", func(t *testing.T, f *fleet, fc *fakeConnector, gate chan struct{}, _ []string) {
			fc.mu.Lock()
			fc.errs["ctx-00"] = errors.New("context not found")
			fc.mu.Unlock()
			close(gate)
		}},
		{"closed while connected, not synced", func(t *testing.T, f *fleet, fc *fakeConnector, gate chan struct{}, all []string) {
			close(gate)
			m := fc.member("ctx-00")
			settle() // polling its status
			f.SetWanted(all[1:], "")
			waitFor(t, "ctx-00 cancelled", m.cancelled)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, fc, _ := newTestFleet(t, nil)
			all := names(6)
			gates := map[string]chan struct{}{}
			for _, n := range all {
				gates[n] = fc.gate(n)
			}
			f.SetWanted(all, "")
			waitFor(t, "4 connects", func() bool { return fc.callCount() == maxStarting })
			tc.free(t, f, fc, gates["ctx-00"], all)
			waitFor(t, "a fifth connect", func() bool { return fc.callCount() == maxStarting+1 })
			settle()
			if n := fc.callCount(); n != maxStarting+1 {
				t.Errorf("%d connects after one place was freed, want %d: %v", n, maxStarting+1, fc.called())
			}
			if fc.callsOf("ctx-04") != 1 {
				t.Errorf("the next queued member did not start: %v", fc.called())
			}
		})
	}
}

// clusters-screen R2: a member closed while its connect is still running
// gives its place back once connect returns, and that connection is
// cancelled.
func TestFleetClosedDuringConnect(t *testing.T) {
	f, fc, rec := newTestFleet(t, nil)
	all := names(5)
	gates := map[string]chan struct{}{}
	for _, n := range all {
		gates[n] = fc.gate(n)
	}
	f.SetWanted(all, "")
	waitFor(t, "4 connects", func() bool { return fc.callCount() == maxStarting })
	f.SetWanted(all[1:], "")
	close(gates["ctx-00"])
	m := fc.member("ctx-00")
	waitFor(t, "ctx-00 cancelled", m.cancelled)
	waitFor(t, "ctx-04 connect", func() bool { return fc.callsOf("ctx-04") == 1 })
	if r := rec.ready("ctx-00"); len(r) != 0 {
		t.Errorf("closed member sent %+v", r)
	}
}

// clusters-screen R2, R3: a member that connected says so with its store,
// generation and status; a failed one says why, without the API server URL.
func TestFleetReadyMessages(t *testing.T) {
	f, fc, rec := newTestFleet(t, nil)
	fc.status["ok"] = model.ConnStatus{Err: "still listing"}
	fc.errs["bad"] = &url.Error{Op: "Get", URL: "https://api.example.com/api/v1/nodes", Err: errors.New("dial tcp: connection refused")}
	fc.errs["bad2"] = fmt.Errorf("creating client: %w", errors.New(`Get "https://api.example.com:443/version": x509: certificate signed by unknown authority`))
	f.SetWanted([]string{"ok", "bad", "bad2"}, "")

	waitFor(t, "three ready messages", func() bool {
		return len(rec.ready("ok")) == 1 && len(rec.ready("bad")) == 1 && len(rec.ready("bad2")) == 1
	})
	m := fc.member("ok")
	ok := rec.ready("ok")[0]
	if ok.Err != "" || ok.Cluster != m.store || ok.Gen != m.store.Generation() || ok.Status == nil {
		t.Fatalf("ready = %+v; want the member's store %p, gen %d, a status", ok, m.store, m.store.Generation())
	}
	if got := ok.Status(); got.Err != "still listing" {
		t.Errorf("Status() = %+v, not the connection's", got)
	}
	waitFor(t, "startNodes", func() bool { n, _ := m.counts(); return n == 1 })

	for _, name := range []string{"bad", "bad2"} {
		r := rec.ready(name)[0]
		if r.Err == "" || r.Cluster != nil || r.Status != nil {
			t.Errorf("%s: ready = %+v, want only an error", name, r)
		}
		if strings.Contains(r.Err, "://") || strings.Contains(r.Err, "example.com") {
			t.Errorf("%s: error %q shows the API server", name, r.Err)
		}
	}
	if r := rec.ready("bad")[0]; !strings.Contains(r.Err, "connection refused") {
		t.Errorf("bad: error %q lost the reason", r.Err)
	}
	if r := rec.ready("bad2")[0]; !strings.Contains(r.Err, "certificate signed by unknown authority") {
		t.Errorf("bad2: error %q lost the reason", r.Err)
	}
}

// clusters-screen R9: SetWanted(nil) closes every member, connected or
// still connecting; a connect that returns after it is cancelled at once
// and says nothing.
func TestFleetLeaveClosesAll(t *testing.T) {
	f, fc, rec := newTestFleet(t, nil)
	late := fc.gate("late")
	f.SetWanted([]string{"a", "b", "late"}, "")
	a, b := fc.member("a"), fc.member("b")
	waitFor(t, "late connecting", func() bool { return fc.callsOf("late") == 1 })
	waitFor(t, "a and b ready", func() bool { return len(rec.ready("a")) == 1 && len(rec.ready("b")) == 1 })

	f.SetWanted(nil, "")
	waitFor(t, "a and b cancelled", func() bool { return a.cancelled() && b.cancelled() })

	before := len(rec.all())
	close(late)
	l := fc.member("late")
	waitFor(t, "the late connection cancelled", l.cancelled)
	settle()
	if r := rec.ready("late"); len(r) != 0 {
		t.Errorf("a connection of a closed visit sent %+v", r)
	}
	if n := len(rec.all()); n != before {
		t.Errorf("messages after leaving: %v", rec.all()[before:])
	}
	if n, p := l.counts(); n != 0 || p != 0 {
		t.Errorf("a late connection started watches: nodes %d, pods %d", n, p)
	}
}

// clusters-screen R9: a late connect of an older visit is cancelled even
// when a new visit wants the same context; the new visit connects it again.
func TestFleetOlderEpochAcrossVisits(t *testing.T) {
	f, fc, rec := newTestFleet(t, nil)
	g := fc.gate("a")
	f.SetWanted([]string{"a"}, "")
	waitFor(t, "a connecting", func() bool { return fc.callsOf("a") == 1 })
	f.SetWanted(nil, "")
	fc.mu.Lock()
	delete(fc.gates, "a") // the next visit's connect does not block
	fc.mu.Unlock()
	f.SetWanted([]string{"a"}, "")
	// the place of the first, still connecting, member is not given back
	// until its connect returns
	close(g)
	waitFor(t, "a connected again", func() bool { return fc.callsOf("a") == 2 })
	waitFor(t, "a ready once", func() bool { return len(rec.ready("a")) == 1 })
	fc.mu.Lock()
	first, second := fc.members["a"][0], fc.members["a"][len(fc.members["a"])-1]
	fc.mu.Unlock()
	if first == second {
		// connect returns in call order is not guaranteed; find the old one
		t.Fatal("only one connection recorded")
	}
	waitFor(t, "the old visit's connection cancelled", func() bool {
		fc.mu.Lock()
		defer fc.mu.Unlock()
		n := 0
		for _, m := range fc.members["a"] {
			if m.cancelled() {
				n++
			}
		}
		return n == 1
	})
	settle()
	if r := rec.ready("a"); len(r) != 1 || r[0].Cluster == nil {
		t.Errorf("ready messages %+v, want one of the live connection", r)
	}
	live := rec.ready("a")[0].Cluster
	fc.mu.Lock()
	defer fc.mu.Unlock()
	for _, m := range fc.members["a"] {
		if m.store == live && m.cancelled() {
			t.Error("the new visit's connection was cancelled")
		}
	}
}

// clusters-screen R9: quitting (Close) closes every member.
func TestFleetCloseClosesAll(t *testing.T) {
	f, fc, _ := newTestFleet(t, nil)
	f.SetWanted([]string{"a", "b"}, "")
	a, b := fc.member("a"), fc.member("b")
	f.Close()
	waitFor(t, "all cancelled", func() bool { return a.cancelled() && b.cancelled() })
	f.Close() // twice is harmless
}

// clusters-screen R15, R11, D10: a member's pods are watched only after
// WatchPods, once; WatchPods before the connection is kept for it.
func TestFleetWatchPods(t *testing.T) {
	f, fc, rec := newTestFleet(t, nil)
	g := fc.gate("early")
	f.SetWanted([]string{"folded", "early"}, "")
	folded := fc.member("folded")
	waitFor(t, "folded ready", func() bool { return len(rec.ready("folded")) == 1 })
	settle()
	if _, p := folded.counts(); p != 0 {
		t.Fatalf("pods watched with no WatchPods: %d", p)
	}

	f.WatchPods("folded")
	f.WatchPods("folded")
	waitFor(t, "folded pods", func() bool { _, p := folded.counts(); return p == 1 })

	waitFor(t, "early connecting", func() bool { return fc.callsOf("early") == 1 })
	f.WatchPods("early") // unfolded while still connecting
	close(g)
	early := fc.member("early")
	waitFor(t, "early pods once connected", func() bool { _, p := early.counts(); return p == 1 })
	f.WatchPods("early")
	f.WatchPods("nobody") // not a member: nothing
	settle()
	if _, p := folded.counts(); p != 1 {
		t.Errorf("folded: startPods %d times, want 1", p)
	}
	if _, p := early.counts(); p != 1 {
		t.Errorf("early: startPods %d times, want 1", p)
	}
}

// clusters-screen R11, R9: an unfold is forgotten with the member; after
// leaving and opening again the pods wait for a new WatchPods.
func TestFleetWatchPodsForgottenOnLeave(t *testing.T) {
	f, fc, rec := newTestFleet(t, nil)
	f.SetWanted([]string{"a"}, "")
	waitFor(t, "a ready", func() bool { return len(rec.ready("a")) == 1 })
	f.WatchPods("a")
	f.SetWanted(nil, "")
	f.SetWanted([]string{"a"}, "")
	waitFor(t, "a again", func() bool { return fc.callsOf("a") == 2 })
	waitFor(t, "a ready again", func() bool { return len(rec.ready("a")) == 2 })
	settle()
	if _, p := fc.member("a").counts(); p != 0 {
		t.Errorf("new visit watched pods with no WatchPods: %d", p)
	}
}

func azureIDs(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("azure:///subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/mc_example-rg/providers/Microsoft.Compute/virtualMachineScaleSets/pool/virtualMachines/%d", i)
	}
	return out
}

// clusters-screen R14, R3, D11: once its node watch synced, a member whose
// nodes' cloud is not the scope's is closed and its cloud is told.
func TestFleetScopeLearnedCloud(t *testing.T) {
	cases := []struct {
		name     string
		guess    string
		nodes    []string
		scope    string
		wantLeft bool
		cloud    string
	}{
		{"azure nodes under aws", "", azureIDs(2), "aws", true, "azure"},
		{"azure nodes under aws, guessed aws", "aws", azureIDs(1), "aws", true, "azure"},
		{"aws nodes under aws", "", []string{"aws:///us-east-1a/i-0123456789abcdef0"}, "aws", false, ""},
		{"empty, guessed aws, under aws", "aws", nil, "aws", false, ""},
		{"empty, unknown guess, under aws", "", nil, "aws", true, ""},
		{"empty, guessed azure, under aws", "azure", nil, "aws", true, "azure"},
		{"mixed schemes", "aws", append([]string{"aws:///us-east-1a/i-0123456789abcdef0"}, azureIDs(1)...), "aws", true, "mixed"},
		{"kind nodes", "", []string{"kind://docker/kind/kind-control-plane"}, "aws", true, "other"},
		{"kind nodes, no scope", "", []string{"kind://docker/kind/kind-control-plane"}, "", false, ""},
		{"gce nodes under gce", "", []string{"gce://example-project/us-central1-a/node-1"}, "gce", false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, fc, rec := newTestFleet(t, map[string]string{"m": tc.guess})
			fc.status["m"] = model.ConnStatus{Synced: true}
			fc.nodes["m"] = tc.nodes
			f.SetWanted([]string{"m"}, tc.scope)
			m := fc.member("m")
			waitFor(t, "m ready", func() bool { return len(rec.ready("m")) == 1 })
			if tc.wantLeft {
				waitFor(t, "m left", func() bool { return len(rec.left()) == 1 })
				if got := rec.left()[0]; got != (model.MemberLeftMsg{Context: "m", Cloud: tc.cloud}) {
					t.Errorf("left = %+v, want cloud %q", got, tc.cloud)
				}
				waitFor(t, "m cancelled", m.cancelled)
				// a closed member is no member: WatchPods does nothing
				f.WatchPods("m")
				settle()
				if _, p := m.counts(); p != 0 {
					t.Errorf("pods watched on a member left out")
				}
				return
			}
			settle()
			if l := rec.left(); len(l) != 0 {
				t.Errorf("member kept by its scope left: %+v", l)
			}
			if m.cancelled() {
				t.Error("member kept by its scope was cancelled")
			}
		})
	}
}

// clusters-screen R14, R7: a member whose node watch failed once and then
// recovered has synced; its cloud is final then, so under another cloud's
// scope it is closed like any synced member.
func TestFleetScopeAfterRecoveredError(t *testing.T) {
	f, fc, rec := newTestFleet(t, nil)
	fc.status["aks"] = model.ConnStatus{Err: "connection refused"}
	fc.nodes["aks"] = azureIDs(1)
	f.SetWanted([]string{"aks"}, "aws")
	m := fc.member("aks")
	waitFor(t, "aks ready", func() bool { return len(rec.ready("aks")) == 1 })
	settle()
	m.setStatus(model.ConnStatus{Synced: true}) // the watch recovered
	time.Sleep(3 * statusPoll)
	f.SetWanted([]string{"aks"}, "aws") // the next fleetSync does not judge it either
	time.Sleep(3 * statusPoll)
	if !m.cancelled() {
		t.Error("the recovered out-of-scope member was not cancelled")
	}
	if got := rec.left(); len(got) != 1 || got[0] != (model.MemberLeftMsg{Context: "aks", Cloud: "azure"}) {
		t.Errorf("left = %+v, want aks/azure once", got)
	}
}

// clusters-screen R14: a member synced under every cloud is judged again
// when the scope narrows; one in the scope is not restarted.
func TestFleetScopeChangeJudgesAgain(t *testing.T) {
	f, fc, rec := newTestFleet(t, map[string]string{"aks": "", "eks": "aws"})
	fc.status["aks"] = model.ConnStatus{Synced: true}
	fc.status["eks"] = model.ConnStatus{Synced: true}
	fc.nodes["aks"] = azureIDs(1)
	fc.nodes["eks"] = []string{"aws:///us-east-1a/i-0123456789abcdef0"}
	both := []string{"aks", "eks"}
	f.SetWanted(both, "")
	aks, eks := fc.member("aks"), fc.member("eks")
	waitFor(t, "both ready", func() bool { return len(rec.ready("aks")) == 1 && len(rec.ready("eks")) == 1 })
	settle()
	if len(rec.left()) != 0 {
		t.Fatalf("left with no scope: %+v", rec.left())
	}

	f.SetWanted(both, "aws")
	waitFor(t, "aks left", func() bool { return len(rec.left()) == 1 })
	if got := rec.left()[0]; got != (model.MemberLeftMsg{Context: "aks", Cloud: "azure"}) {
		t.Errorf("left = %+v", got)
	}
	waitFor(t, "aks cancelled", aks.cancelled)

	f.SetWanted(both, "aws") // repeated: the same members
	settle()
	if eks.cancelled() || fc.callsOf("eks") != 1 {
		t.Errorf("eks restarted: cancelled %v, connects %d", eks.cancelled(), fc.callsOf("eks"))
	}
	if len(rec.ready("eks")) != 1 {
		t.Errorf("eks ready %d times", len(rec.ready("eks")))
	}
}

// clusters-screen R14: a bare `:clusters` after a scope connects again the
// member the scope had left out.
func TestFleetScopeWidenedConnectsAgain(t *testing.T) {
	f, fc, rec := newTestFleet(t, nil)
	fc.status["aks"] = model.ConnStatus{Synced: true}
	fc.nodes["aks"] = azureIDs(1)
	f.SetWanted([]string{"aks"}, "aws")
	waitFor(t, "aks left", func() bool { return len(rec.left()) == 1 })
	f.SetWanted([]string{"aks"}, "")
	waitFor(t, "aks connected again", func() bool { return fc.callsOf("aks") == 2 })
	waitFor(t, "aks ready again", func() bool { return len(rec.ready("aks")) == 2 })
	settle()
	if len(rec.left()) != 1 {
		t.Errorf("left again with every cloud: %+v", rec.left())
	}
}

// clusters-screen D3, R2: change notices of several members within one
// interval reach the model as one message with all their generations.
func TestFleetChangesCoalesced(t *testing.T) {
	f, fc, rec := newTestFleet(t, nil)
	f.SetWanted([]string{"a", "b", "c"}, "")
	a, b, c := fc.member("a"), fc.member("b"), fc.member("c")
	ga, gb, gc := a.store.Generation(), b.store.Generation(), c.store.Generation()

	// the first notice fires at once; the ones after it wait out the interval
	a.notify(ga)
	waitFor(t, "first change", func() bool { return len(rec.changed()) == 1 })
	b.notify(gb)
	c.notify(gc)
	a.notify(ga)
	waitFor(t, "second change", func() bool { return len(rec.changed()) == 2 })
	time.Sleep(3 * changeInterval)
	got := rec.changed()
	if len(got) != 2 {
		t.Fatalf("%d change messages, want 2: %+v", len(got), got)
	}
	if !reflect.DeepEqual(got[0].Gens, []uint64{ga}) {
		t.Errorf("first change %v, want [%d]", got[0].Gens, ga)
	}
	second := slices.Clone(got[1].Gens)
	slices.Sort(second)
	want := []uint64{ga, gb, gc}
	slices.Sort(want)
	if !reflect.DeepEqual(second, want) {
		t.Errorf("second change %v, want %v in one message", second, want)
	}

	// after leaving, a notice of a closed member is dropped
	f.SetWanted(nil, "")
	a.notify(ga)
	time.Sleep(3 * changeInterval)
	if n := len(rec.changed()); n != 2 {
		t.Errorf("a change after leaving reached the model: %+v", rec.changed())
	}
}

// clusters-screen D8: one price set per visit, given to every member, and a
// new one after leaving.
func TestFleetPricesPerVisit(t *testing.T) {
	fc := newFakeConnector(t)
	rec := &recorder{}
	var mu sync.Mutex
	var visits []context.Context
	prices := func(ctx context.Context) priceFunc {
		mu.Lock()
		visits = append(visits, ctx)
		mu.Unlock()
		return noPrices(ctx)
	}
	f := newFleet(fc.connect, rec.send, infos(nil), prices)
	defer f.Close()
	f.SetWanted([]string{"a", "b"}, "")
	f.SetWanted([]string{"a", "b", "c"}, "")
	fc.member("a")
	fc.member("b")
	fc.member("c")
	mu.Lock()
	if len(visits) != 1 {
		t.Errorf("%d price sets in one visit, want 1", len(visits))
	}
	first := visits[0]
	mu.Unlock()
	f.SetWanted(nil, "")
	if first.Err() == nil {
		t.Error("leaving kept the visit's prices running")
	}
	f.SetWanted([]string{"a"}, "")
	mu.Lock()
	defer mu.Unlock()
	if len(visits) != 2 || visits[1].Err() != nil {
		t.Errorf("a new visit got no live price set: %d", len(visits))
	}
}

// clusters-screen R8: the fleet lists the kubeconfig's contexts with the
// cloud each one's name or server says.
func TestFleetContexts(t *testing.T) {
	kubeconfig := `apiVersion: v1
kind: Config
current-context: eks
clusters:
- name: eks
  cluster:
    server: https://ABC.gr7.us-east-1.eks.amazonaws.com
- name: aks
  cluster:
    server: https://example-aks.hcp.eastus2.azmk8s.io:443
- name: lab
  cluster:
    server: https://lab.example.com
contexts:
- name: eks
  context: {cluster: eks}
- name: aks
  context: {cluster: aks}
- name: gke_example-project_us-central1_c1
  context: {cluster: lab}
- name: lab
  context: {cluster: lab}
users: []
`
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte(kubeconfig), 0o600); err != nil {
		t.Fatal(err)
	}
	s := &sessions{flags: Flags{Kubeconfig: path}}
	got, err := s.fleetContexts()
	if err != nil {
		t.Fatal(err)
	}
	want := []model.ContextInfo{
		{Name: "aks", Server: "https://example-aks.hcp.eastus2.azmk8s.io:443", Cloud: "azure"},
		{Name: "eks", Server: "https://ABC.gr7.us-east-1.eks.amazonaws.com", Cloud: "aws"},
		{Name: "gke_example-project_us-central1_c1", Server: "https://lab.example.com", Cloud: "gce"},
		{Name: "lab", Server: "https://lab.example.com", Cloud: ""},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("contexts:\n got %+v\nwant %+v", got, want)
	}
}

type fixedPrice float64

func (p fixedPrice) NodePrice(*model.Node) (float64, bool) { return float64(p), true }
func (fixedPrice) OnUpdate(func())                         {}

func nodeWithID(id string) *model.Node {
	return model.NewNode(&v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "n", Labels: map[string]string{
			v1.LabelInstanceTypeStable: "m5.large", v1.LabelTopologyRegion: "us-east-1",
		}},
		Spec: v1.NodeSpec{ProviderID: id},
	})
}

// clusters-screen R4, D4: a gce node is never priced by another cloud's
// provider, even the fallback's.
func TestByCloudGCENotPricedByFallback(t *testing.T) {
	b := pricing.ByCloud{Clouds: map[string]pricing.Provider{"gce": pricing.None{}}, Fallback: fixedPrice(1)}
	if _, ok := b.NodePrice(nodeWithID("gce://example-project/us-central1-a/n")); ok {
		t.Error("gce node priced by the fallback")
	}
	if p, ok := b.NodePrice(nodeWithID("aws:///us-east-1a/i-0123456789abcdef0")); !ok || p != 1 {
		t.Errorf("aws node: %v, %v; want the fallback's price", p, ok)
	}
}

// clusters-screen R4, D4, D8: a visit's price set routes gce to
// nothing, shares one Azure provider and one AWS provider per region across
// its members, and builds no AWS lookup for an AKS server.
func TestVisitPricing(t *testing.T) {
	for k, v := range map[string]string{
		"AWS_REGION": "", "AWS_DEFAULT_REGION": "", "AWS_PROFILE": "",
		"AWS_CONFIG_FILE": "/dev/null", "AWS_SHARED_CREDENTIALS_FILE": "/dev/null",
		"AWS_EC2_METADATA_DISABLED": "true",
	} {
		t.Setenv(k, v)
	}
	const (
		eks1 = "https://ABC.gr7.us-east-1.eks.amazonaws.com"
		eks2 = "https://DEF.gr7.us-east-1.eks.amazonaws.com"
		eks3 = "https://GHI.gr7.eu-west-1.eks.amazonaws.com"
		aks  = "https://example-aks.hcp.eastus2.azmk8s.io:443"
	)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // providers stop before any refresh: no network
	s := &sessions{flags: Flags{}}
	visit := s.visitPricing(ctx)
	get := func(server string) (pricing.ByCloud, string) {
		t.Helper()
		p, region, err := visit(ctx, server)
		if err != nil {
			t.Fatal(err)
		}
		b, ok := p.(pricing.ByCloud)
		if !ok {
			t.Fatalf("%s: %T, want a ByCloud", server, p)
		}
		return b, region
	}
	b1, r1 := get(eks1)
	b2, _ := get(eks2)
	b3, r3 := get(eks3)
	ba, ra := get(aks)
	if r1 != "us-east-1" || r3 != "eu-west-1" || ra != "" {
		t.Errorf("regions %q %q %q", r1, r3, ra)
	}
	if b1.Clouds["aws"] != b2.Clouds["aws"] {
		t.Error("two clusters of one region got two AWS providers")
	}
	if b1.Clouds["aws"] == b3.Clouds["aws"] {
		t.Error("two regions share one AWS provider")
	}
	if b1.Clouds["azure"] != ba.Clouds["azure"] || b3.Clouds["azure"] != ba.Clouds["azure"] {
		t.Error("the visit has more than one Azure provider")
	}
	if _, ok := ba.Clouds["aws"].(pricing.None); !ok {
		t.Errorf("AKS server: AWS provider %T, want none", ba.Clouds["aws"])
	}
	if _, ok := ba.Fallback.(pricing.None); !ok {
		t.Errorf("AKS server: fallback %T, want none", ba.Fallback)
	}
	for _, b := range []pricing.ByCloud{b1, b3, ba} {
		if _, ok := b.Clouds["gce"].(pricing.None); !ok {
			t.Errorf("gce provider %T, want none", b.Clouds["gce"])
		}
		if _, ok := b.NodePrice(nodeWithID("gce://example-project/us-central1-a/n")); ok {
			t.Error("a gce node was priced")
		}
	}

	// pricing off: no ByCloud, no Azure provider
	off := (&sessions{flags: Flags{DisablePricing: true}}).visitPricing(ctx)
	p, _, err := off(ctx, aks)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := p.(pricing.ByCloud); ok {
		t.Error("pricing off, but the visit can reach the Azure prices API")
	}
}
