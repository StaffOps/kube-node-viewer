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

package client

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	goruntime "runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/staffops/kube-node-viewer/pkg/model"
)

// The reflector's first retry comes 0.8 to 1.6 s after a failure, the next
// 1.6 to 3.2 s after that (client-go tools/cache/reflector.go defaults).
const (
	waitFor   = 10 * time.Second
	noRetryIn = 3500 * time.Millisecond
)

// listCalls counts the list calls a fake clientset sees for one resource and
// keeps the field selector of each.
type listCalls struct {
	mu     sync.Mutex
	n      int
	fields []string
}

func (c *listCalls) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

func (c *listCalls) selectors() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.fields...)
}

// onList counts the list calls for resource; fail, when it returns an error
// for the n-th call (1-based), answers it with that error, otherwise the
// call goes on to the fake's object tracker.
func onList(cs *fake.Clientset, resource string, fail func(n int) error) *listCalls {
	c := &listCalls{}
	cs.PrependReactor("list", resource, func(a k8stesting.Action) (bool, runtime.Object, error) {
		c.mu.Lock()
		c.n++
		n := c.n
		sel := ""
		if la, ok := a.(k8stesting.ListAction); ok && la.GetListRestrictions().Fields != nil {
			sel = la.GetListRestrictions().Fields.String()
		}
		c.fields = append(c.fields, sel)
		c.mu.Unlock()
		if fail != nil {
			if err := fail(n); err != nil {
				return true, nil, err
			}
		}
		return false, nil, nil
	})
	return c
}

func always(err error) func(int) error { return func(int) error { return err } }

var (
	nodesGR = schema.GroupResource{Resource: "nodes"}
	podsGR  = schema.GroupResource{Resource: "pods"}
)

func newTestController(t *testing.T, cs *fake.Clientset) (*Controller, *model.Cluster, context.Context) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	cluster := model.NewCluster()
	return NewController(cs, nil, cluster, labels.Everything(), fixedPricing(1)), cluster, ctx
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := wait.PollUntilContextTimeout(ctx, 20*time.Millisecond, waitFor, true,
		func(context.Context) (bool, error) { return cond(), nil })
	if err != nil {
		t.Fatalf("timed out waiting for %s", what)
	}
}

// stays checks that n keeps its value for noRetryIn.
func stays(t *testing.T, what string, n func() int) {
	t.Helper()
	before := n()
	time.Sleep(noRetryIn)
	if after := n(); after != before {
		t.Errorf("%s: %d calls, then %d after %v", what, before, after, noRetryIn)
	}
}

func nodeCount(c *model.Cluster) int {
	n := 0
	c.ForEachNode(func(*model.Node) { n++ })
	return n
}

func testNode(name string, nodeLabels map[string]string) *v1.Node {
	return &v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: nodeLabels},
		Spec:       v1.NodeSpec{ProviderID: "aws:///us-east-1a/" + name},
	}
}

func testPod(name, nodeName, scheduler string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec:       v1.PodSpec{NodeName: nodeName, SchedulerName: scheduler},
		Status:     v1.PodStatus{Phase: v1.PodRunning},
	}
}

// An empty cluster leaves "connecting": the node watch's first list is
// enough, with no node in it.
// clusters-screen R2, R3, R15
func TestStatusEmptyClusterSynced(t *testing.T) {
	cs := fake.NewClientset()
	ctl, cluster, ctx := newTestController(t, cs)
	if s := ctl.Status(); s.Synced || s.PodsSynced || s.Err != "" {
		t.Fatalf("before StartNodes: %+v, want the zero Status", s)
	}
	ctl.StartNodes(ctx)
	eventually(t, "Synced", func() bool { return ctl.Status().Synced })
	if s := ctl.Status(); s.PodsSynced || s.Err != "" {
		t.Errorf("after StartNodes: %+v, want PodsSynced false and no error", s)
	}
	if n := nodeCount(cluster); n != 0 {
		t.Errorf("%d nodes in an empty cluster", n)
	}
}

// PodsSynced stays false until StartPods, and a second StartPods starts no
// second pod watch.
// clusters-screen R15
func TestStatusPodsSyncedOnlyAfterStartPods(t *testing.T) {
	cs := fake.NewClientset(testNode("n1", nil), testPod("p1", "n1", "default-scheduler"))
	pods := onList(cs, "pods", nil)
	ctl, cluster, ctx := newTestController(t, cs)
	ctl.StartNodes(ctx)
	eventually(t, "Synced", func() bool { return ctl.Status().Synced })
	time.Sleep(300 * time.Millisecond)
	if ctl.Status().PodsSynced {
		t.Error("PodsSynced before StartPods")
	}
	if n := pods.count(); n != 0 {
		t.Errorf("%d pod lists before StartPods, want 0", n)
	}

	ctl.StartPods(ctx)
	eventually(t, "PodsSynced", func() bool { return ctl.Status().PodsSynced })
	ctl.StartPods(ctx)
	time.Sleep(300 * time.Millisecond)
	if n := pods.count(); n != 1 {
		t.Errorf("%d pod lists after StartPods twice, want 1", n)
	}
	if _, ok := cluster.GetPod("default", "p1"); !ok {
		t.Error("pod p1 not in the store")
	}
	if got := pods.selectors(); len(got) == 1 && got[0] != "" {
		t.Errorf("the full pod watch used field selector %q, want none", got[0])
	}
}

// Start is StartPods and StartNodes.
// clusters-screen R15
func TestStatusStartRunsBoth(t *testing.T) {
	cs := fake.NewClientset(testNode("n1", nil))
	pods := onList(cs, "pods", nil)
	ctl, cluster, ctx := newTestController(t, cs)
	ctl.Start(ctx)
	eventually(t, "Synced and PodsSynced", func() bool {
		s := ctl.Status()
		return s.Synced && s.PodsSynced
	})
	ctl.StartPods(ctx)
	time.Sleep(300 * time.Millisecond)
	if n := pods.count(); n != 1 {
		t.Errorf("%d pod lists after Start and StartPods, want 1", n)
	}
	if n := nodeCount(cluster); n != 1 {
		t.Errorf("%d nodes, want 1", n)
	}
}

// A node without a provider ID is not added (behaviour from upstream).
// clusters-screen R2
func TestStatusNodeWithoutProviderIDSkipped(t *testing.T) {
	cs := fake.NewClientset(&v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "bare"}}, testNode("n1", nil))
	ctl, cluster, ctx := newTestController(t, cs)
	ctl.StartNodes(ctx)
	eventually(t, "Synced", func() bool { return ctl.Status().Synced })
	if n := nodeCount(cluster); n != 1 {
		t.Errorf("%d nodes, want only the one with a provider ID", n)
	}
	if _, ok := cluster.GetNodeByName("bare"); ok {
		t.Error("node without provider ID was added")
	}
}

// A failing list sets Err without the URL; a later list that succeeds
// clears it. A transient error never stops a member's watch.
// clusters-screen R3, R7
func TestStatusErrorThenRecovery(t *testing.T) {
	for _, member := range []bool{false, true} {
		t.Run(fmt.Sprintf("member=%v", member), func(t *testing.T) {
			cs := fake.NewClientset(testNode("n1", nil))
			var failing atomic.Bool
			failing.Store(true)
			timeout := errors.New(`Get "https://example.com:443/api/v1/nodes?limit=500": dial tcp 192.0.2.10:443: i/o timeout`)
			nodes := onList(cs, "nodes", func(int) error {
				if failing.Load() {
					return timeout
				}
				return nil
			})
			ctl, cluster, ctx := newTestController(t, cs)
			if member {
				ctl.SetMember(false)
			}
			ctl.StartNodes(ctx)
			eventually(t, "Err", func() bool { return ctl.Status().Err != "" })
			s := ctl.Status()
			if s.Synced {
				t.Error("Synced while the first list fails")
			}
			if strings.Contains(s.Err, "https://") || strings.Contains(s.Err, "example.com") {
				t.Errorf("Err shows the URL: %q", s.Err)
			}
			if !strings.Contains(s.Err, "i/o timeout") {
				t.Errorf("Err = %q, want the cause", s.Err)
			}
			failing.Store(false)
			eventually(t, "Synced and Err cleared", func() bool {
				s := ctl.Status()
				return s.Synced && s.Err == ""
			})
			if nodes.count() < 2 {
				t.Errorf("%d node lists, want a retry", nodes.count())
			}
			if n := nodeCount(cluster); n != 1 {
				t.Errorf("%d nodes after recovery, want 1", n)
			}
		})
	}
}

// Each informer keeps its own error: another watch's events do not hide the
// error of a stopped one (clusters-screen D7).
// clusters-screen R7
func TestStatusOtherWatchKeepsErr(t *testing.T) {
	cs := fake.NewClientset()
	onList(cs, "pods", always(apierrors.NewForbidden(podsGR, "", errors.New("no list"))))
	ctl, _, ctx := newTestController(t, cs)
	ctl.SetMember(false)
	ctl.StartNodes(ctx)
	eventually(t, "Synced", func() bool { return ctl.Status().Synced })
	ctl.StartPods(ctx)
	eventually(t, "Err", func() bool { return ctl.Status().Err != "" })
	if !strings.Contains(ctl.Status().Err, "forbidden") {
		t.Errorf("Err = %q, want the 403", ctl.Status().Err)
	}
	if _, err := cs.CoreV1().Nodes().Create(ctx, testNode("n1", nil), metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if !strings.Contains(ctl.Status().Err, "forbidden") {
		t.Errorf("Err = %q after a node event, want the pod watch's 403 kept", ctl.Status().Err)
	}
}

// A watch refused once, then a quiet stream: its first bookmark clears the
// error, with no node event (clusters-screen D7).
// clusters-screen R7
func TestStatusQuietWatchClearsErr(t *testing.T) {
	cs := fake.NewClientset()
	var mu sync.Mutex
	calls := 0
	stream := watch.NewFake()
	cs.PrependWatchReactor("nodes", func(k8stesting.Action) (bool, watch.Interface, error) {
		mu.Lock()
		defer mu.Unlock()
		calls++
		if calls == 1 {
			return true, nil, errors.New("connection refused")
		}
		return true, stream, nil
	})
	ctl, _, ctx := newTestController(t, cs)
	ctl.SetMember(false)
	ctl.StartNodes(ctx)
	eventually(t, "the refused watch's error", func() bool { return ctl.Status().Err != "" })
	eventually(t, "a second watch", func() bool { mu.Lock(); defer mu.Unlock(); return calls >= 2 })
	stream.Action(watch.Bookmark, &v1.Node{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "2"}})
	eventually(t, "Err cleared by the bookmark", func() bool { return ctl.Status().Err == "" })
}

// A Fargate node is priced from its pod by the Fargate-only watch, before
// the full pod watch.
// clusters-screen R16
func TestFargateNodePricedBeforeStartPods(t *testing.T) {
	fargateNode := testNode("fargate-ip-192-0-2-38", map[string]string{"eks.amazonaws.com/compute-type": "fargate"})
	cs := fake.NewClientset(fargateNode, testPod("fp", fargateNode.Name, FargateScheduler))
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	cluster := model.NewCluster()
	ctl := NewController(cs, nil, cluster, labels.Everything(), podPricing{})
	ctl.SetMember(true)
	ctl.StartNodes(ctx)
	eventually(t, "the Fargate node priced", func() bool {
		n, ok := cluster.GetNodeByName(fargateNode.Name)
		return ok && n.HasPrice()
	})
	if ctl.Status().PodsSynced {
		t.Error("PodsSynced without StartPods")
	}
}

// podPricing prices a node only once a pod is bound to it, as Fargate is.
type podPricing struct{}

func (podPricing) NodePrice(n *model.Node) (float64, bool) {
	if n.NumPods() == 0 {
		return 0, false
	}
	return 0.05, true
}
func (podPricing) OnUpdate(func()) {}

// A member's watch refused with 403 stops; the other watches go on.
// clusters-screen R7
func TestStatusMemberForbiddenStopsThatWatch(t *testing.T) {
	cs := fake.NewClientset(testPod("p1", "n1", "default-scheduler"))
	nodes := onList(cs, "nodes", always(apierrors.NewForbidden(nodesGR, "", errors.New("no list"))))
	ctl, cluster, ctx := newTestController(t, cs)
	ctl.SetMember(false)
	ctl.StartNodes(ctx)
	ctl.StartPods(ctx)
	eventually(t, "a node list", func() bool { return nodes.count() >= 1 })
	eventually(t, "PodsSynced", func() bool { return ctl.Status().PodsSynced })
	stays(t, "node lists after a 403", nodes.count)
	if n := nodes.count(); n != 1 {
		t.Errorf("%d node lists, want 1", n)
	}
	if ctl.Status().Synced {
		t.Error("Synced with the node list refused")
	}
	// the pod watch still delivers
	if _, err := cs.CoreV1().Pods("default").Create(ctx, testPod("p2", "n1", "default-scheduler"), metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	eventually(t, "pod p2 in the store", func() bool {
		_, ok := cluster.GetPod("default", "p2")
		return ok
	})
}

// Overview's controller (no SetMember) keeps retrying a 403.
// clusters-screen R7
func TestStatusOverviewForbiddenRetries(t *testing.T) {
	cs := fake.NewClientset()
	nodes := onList(cs, "nodes", always(apierrors.NewForbidden(nodesGR, "", errors.New("no list"))))
	ctl, _, ctx := newTestController(t, cs)
	ctl.StartNodes(ctx)
	eventually(t, "a second node list", func() bool { return nodes.count() >= 2 })
	if !strings.Contains(ctl.Status().Err, "forbidden") {
		t.Errorf("Err = %q, want the 403", ctl.Status().Err)
	}
}

// A member's 401 stops its watch only when it repeats.
// clusters-screen R7
func TestStatusMemberUnauthorizedStopsOnSecond(t *testing.T) {
	cs := fake.NewClientset()
	nodes := onList(cs, "nodes", always(apierrors.NewUnauthorized("token expired")))
	ctl, _, ctx := newTestController(t, cs)
	ctl.SetMember(false)
	ctl.StartNodes(ctx)
	eventually(t, "a second node list after one 401", func() bool { return nodes.count() >= 2 })
	stays(t, "node lists after two 401s", nodes.count)
	if n := nodes.count(); n != 2 {
		t.Errorf("%d node lists, want 2", n)
	}
	if ctl.Status().Err == "" {
		t.Error("no Err after 401")
	}
}

// A single 401 followed by a success does not stop the watch, and the
// count starts again after the success.
// clusters-screen R7
func TestStatusMemberUnauthorizedOnceRecovers(t *testing.T) {
	cs := fake.NewClientset(testNode("n1", nil))
	onList(cs, "nodes", func(n int) error {
		if n == 1 {
			return apierrors.NewUnauthorized("token expired")
		}
		return nil
	})
	ctl, cluster, ctx := newTestController(t, cs)
	ctl.SetMember(false)
	ctl.StartNodes(ctx)
	eventually(t, "Synced after one 401", func() bool {
		s := ctl.Status()
		return s.Synced && s.Err == ""
	})
	if n := nodeCount(cluster); n != 1 {
		t.Errorf("%d nodes, want 1", n)
	}
}

// A member's watch stops when the exec credential plugin fails; Overview's
// keeps retrying.
// clusters-screen R7
func TestStatusExecFailure(t *testing.T) {
	execErr := fmt.Errorf("getting credentials: %v", errors.New("exec: executable aws failed with exit code 255")) //nolint:errorlint // %v as client-go's exec.go writes it
	t.Run("member stops", func(t *testing.T) {
		cs := fake.NewClientset()
		nodes := onList(cs, "nodes", always(execErr))
		ctl, _, ctx := newTestController(t, cs)
		ctl.SetMember(false)
		ctl.StartNodes(ctx)
		eventually(t, "a node list", func() bool { return nodes.count() >= 1 })
		stays(t, "node lists after an exec failure", nodes.count)
		if n := nodes.count(); n != 1 {
			t.Errorf("%d node lists, want 1", n)
		}
		if !strings.Contains(ctl.Status().Err, "getting credentials:") {
			t.Errorf("Err = %q, want the exec failure", ctl.Status().Err)
		}
	})
	t.Run("overview retries", func(t *testing.T) {
		cs := fake.NewClientset()
		nodes := onList(cs, "nodes", always(execErr))
		ctl, _, ctx := newTestController(t, cs)
		ctl.StartNodes(ctx)
		eventually(t, "a second node list", func() bool { return nodes.count() >= 2 })
	})
}

// client-go v0.37.2 builds the exec plugin's error as
// fmt.Errorf("getting credentials: %v", err) in
// k8s.io/client-go/plugin/pkg/client/auth/exec/exec.go; IsExecFailure must
// recognise that text, also wrapped again.
// clusters-screen R7
func TestExecFailurePrefix(t *testing.T) {
	err := fmt.Errorf("getting credentials: %v", errors.New("x")) //nolint:errorlint // %v as client-go's exec.go writes it
	if !IsExecFailure(err) {
		t.Errorf("IsExecFailure(%q) = false", err)
	}
	wrapped := &url.Error{Op: "Get", URL: "https://example.com/api", Err: err}
	if !IsExecFailure(wrapped) {
		t.Errorf("IsExecFailure(%q) = false", wrapped)
	}
	for _, other := range []error{nil, errors.New("dial tcp: i/o timeout"), apierrors.NewUnauthorized("x")} {
		if IsExecFailure(other) {
			t.Errorf("IsExecFailure(%v) = true", other)
		}
	}
}

// clusters-screen R3, R7
func TestSanitize(t *testing.T) {
	cases := []struct {
		name string
		err  error
		keep string
	}{
		{"nil", nil, ""},
		{"plain", errors.New("forbidden"), "forbidden"},
		{"quoted URL", errors.New(`Get "https://example.com:443/api/v1/nodes": dial tcp: i/o timeout`), "i/o timeout"},
		{"bare URL", errors.New("failed to list https://example.com/api/v1/pods?limit=500 connection refused"), "connection refused"},
		{"http URL", errors.New(`Get "http://example.com:8080/api": EOF`), "EOF"},
		{"url.Error", &url.Error{Op: "Get", URL: "https://example.com:443/api/v1/nodes", Err: errors.New("dial tcp: i/o timeout")}, "i/o timeout"},
		{"wrapped url.Error", fmt.Errorf("listing: %w", &url.Error{Op: "Get", URL: "https://example.com/x", Err: errors.New("EOF")}), "EOF"},
		{"DNS lookup", errors.New("dial tcp: lookup example.com on 192.0.2.53:53: no such host"), "no such host"},
		{"IP and port", errors.New("dial tcp 192.0.2.10:443: connect: connection refused"), "connection refused"},
		{"host and port", errors.New("tls: handshake with example.com:443 failed"), "handshake"},
		{"x509 names", errors.New("tls: failed to verify certificate: x509: certificate is valid for example.com, kubernetes.default, not other.example.com"), "x509: certificate is valid for"},
		{"x509 mixed case", errors.New("x509: certificate is valid for API.Example.COM, kube-apiserver-example, not Other.Example.com"), "x509: certificate is valid for"},
		{"x509 authority", errors.New(`x509: certificate signed by unknown authority (possibly because of "crypto/rsa: verification error" while trying to verify candidate authority certificate "Example Corp Root CA")`), "unknown authority"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Sanitize(c.err)
			if c.err == nil {
				if got != "" {
					t.Errorf("Sanitize(nil) = %q", got)
				}
				return
			}
			if strings.Contains(got, "://") || strings.Contains(strings.ToLower(got), "example") || strings.Contains(got, "192.0.2.") {
				t.Errorf("Sanitize = %q, still has the URL", got)
			}
			if !strings.Contains(got, c.keep) {
				t.Errorf("Sanitize = %q, lost %q", got, c.keep)
			}
		})
	}
}

// A member with Fargate pricing watches only the Fargate scheduler's pods
// once a Fargate node shows up, before StartPods.
// clusters-screen R15, R16
func TestFargateOnlyPodWatch(t *testing.T) {
	fargateNode := testNode("fargate-ip-192-0-2-38", map[string]string{"eks.amazonaws.com/compute-type": "fargate"})
	cs := fake.NewClientset(fargateNode, testPod("fp", fargateNode.Name, FargateScheduler))
	pods := onList(cs, "pods", nil)
	ctl, cluster, ctx := newTestController(t, cs)
	ctl.SetMember(true)
	ctl.StartNodes(ctx)
	eventually(t, "a pod list", func() bool { return pods.count() >= 1 })
	for _, sel := range pods.selectors() {
		if sel != "spec.schedulerName=fargate-scheduler" {
			t.Errorf("pod list field selector %q, want spec.schedulerName=fargate-scheduler", sel)
		}
	}
	eventually(t, "the Fargate pod in the store", func() bool {
		_, ok := cluster.GetPod("default", "fp")
		return ok
	})
	if ctl.Status().PodsSynced {
		t.Error("PodsSynced from the Fargate-only watch")
	}
}

// No pod list before StartPods without Fargate pricing or Fargate nodes.
// clusters-screen R15, R16
func TestNoPodWatchBeforeStartPods(t *testing.T) {
	fargate := map[string]string{"eks.amazonaws.com/compute-type": "fargate"}
	cases := []struct {
		name        string
		member      bool
		fargatePods bool
		nodeLabels  map[string]string
	}{
		{"overview, Fargate node", false, false, fargate},
		{"member without Fargate pricing, Fargate node", true, false, fargate},
		{"member with Fargate pricing, no Fargate node", true, true, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cs := fake.NewClientset(testNode("n1", c.nodeLabels))
			pods := onList(cs, "pods", nil)
			ctl, cluster, ctx := newTestController(t, cs)
			if c.member {
				ctl.SetMember(c.fargatePods)
			}
			ctl.StartNodes(ctx)
			eventually(t, "Synced", func() bool { return ctl.Status().Synced && nodeCount(cluster) == 1 })
			time.Sleep(500 * time.Millisecond)
			if n := pods.count(); n != 0 {
				t.Errorf("%d pod lists before StartPods, want 0 (selectors %q)", n, pods.selectors())
			}
		})
	}
}

// Stopping the stream wrapper with an event waiting to be read ends its
// goroutine (a reflector stops reading without draining).
// clusters-screen R9
func TestEventWatchStopReleasesPendingEvent(t *testing.T) {
	before := goruntime.NumGoroutine()
	for range 50 {
		inner := watch.NewFakeWithChanSize(1, false)
		w := newEventWatch(inner, &watchStatus{})
		inner.Add(&v1.Node{})
		time.Sleep(time.Millisecond) // the loop takes the event and waits to send it
		w.Stop()
		for range w.ResultChan() {
		}
	}
	eventually(t, "the wrappers' goroutines gone", func() bool { return goruntime.NumGoroutine() <= before+2 })
}
