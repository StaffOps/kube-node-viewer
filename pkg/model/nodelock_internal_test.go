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

package model

import (
	"reflect"
	"sync"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
)

// fargateLabels marks a node as Fargate.
var fargateLabels = map[string]string{"eks.amazonaws.com/compute-type": "fargate"}

// namelessNode is a node known only from its NodeClaim.
func namelessNode(providerID string, labels map[string]string) *Node {
	nc := &karpv1.NodeClaim{ObjectMeta: metav1.ObjectMeta{Labels: labels, CreationTimestamp: metav1.NewTime(time.Unix(1700000000, 0))}}
	nc.Status.ProviderID = providerID
	return NewNodeFromNodeClaim(nc)
}

func fargatePod(name string) *Pod {
	return NewPod(&v1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: name,
		Annotations: map[string]string{"CapacityProvisioned": "0.25vCPU 0.5GB"}}})
}

// loop runs each func in its own goroutine until the deadline, and reports
// whether they all returned within the wait after it.
func loop(d, wait time.Duration, fs ...func()) bool {
	deadline := time.Now().Add(d)
	var wg sync.WaitGroup
	for _, f := range fs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Now().Before(deadline) {
				f()
			}
		}()
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
		return true
	case <-time.After(d + wait):
		return false
	}
}

// fix-node-name-lock R1
func TestNodeLocksDoNotDeadlock(t *testing.T) {
	n := namelessNode("aws:///us-east-1a/i-0abc", fargateLabels)
	n.BindPod(fargatePod("p"))
	obj := n.Object()
	if !loop(time.Second, 10*time.Second,
		func() { _ = n.Name() },
		func() { _ = n.Name() },
		func() { _ = n.PodsCopy() },
		func() { _ = n.InstanceType() },
		func() { n.Update(obj) },
		func() { n.SetPrice(1) },
	) {
		t.Fatal("Name, PodsCopy or InstanceType did not return while the node was updated: deadlock")
	}
	if got := n.Name(); got != "i-0abc" {
		t.Errorf("Name() = %q, want the instance id", got)
	}
}

// fix-node-name-lock R2 R4
func TestNodeReadersAreLocked(t *testing.T) {
	// each reader against the writer it raced; the race detector (go test
	// -race, as scripts/verify.sh runs it) fails the test on an unlocked read
	update := func(n *Node) func() {
		obj := n.Object()
		return func() { n.Update(obj) }
	}
	bind := func(n *Node) func() {
		return func() {
			n.BindPod(fargatePod("p"))
			n.DeletePod("demo", "p")
		}
	}
	for _, tc := range []struct {
		name   string
		read   func(n *Node)
		writer func(n *Node) func()
	}{
		{"IsOnDemand", func(n *Node) { _ = n.IsOnDemand() }, update},
		{"IsSpot", func(n *Node) { _ = n.IsSpot() }, update},
		{"IsFargate", func(n *Node) { _ = n.IsFargate() }, update},
		{"IsAuto", func(n *Node) { _ = n.IsAuto() }, update},
		{"Labels", func(n *Node) { _ = n.Labels()["x"] }, update},
		{"Pods", func(n *Node) { _ = n.Pods() }, bind},
		{"PodsCopy", func(n *Node) { _ = n.PodsCopy() }, bind},
		{"InstanceType", func(n *Node) { _ = n.InstanceType() }, bind},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := NewNode(&v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "n", Labels: fargateLabels}})
			if !loop(100*time.Millisecond, 10*time.Second, func() { tc.read(n) }, tc.writer(n)) {
				t.Fatal("deadlock")
			}
		})
	}

	// a NodeClaim node whose Ready condition flips: Update clears the NodeClaim
	// creation time (fix-ready-lock-fargate-panic R2) while NotReadyTime and
	// Created read it
	for name, read := range map[string]func(n *Node){
		"NotReadyTime": func(n *Node) { _ = n.NotReadyTime() },
		"Created":      func(n *Node) { _ = n.Created() },
	} {
		t.Run(name, func(t *testing.T) {
			n := namelessNode("aws:///us-east-1a/i-0abc", nil)
			ready := &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "n"}, Status: v1.NodeStatus{Conditions: []v1.NodeCondition{{Type: v1.NodeReady, Status: v1.ConditionTrue}}}}
			notReady := &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "n"}, Status: v1.NodeStatus{Conditions: []v1.NodeCondition{{Type: v1.NodeReady, Status: v1.ConditionFalse, LastTransitionTime: metav1.Now()}}}}
			n.Update(notReady)
			if !loop(100*time.Millisecond, 10*time.Second,
				func() { read(n) },
				func() { n.Update(ready); n.Update(notReady) },
			) {
				t.Fatal("deadlock")
			}
		})
	}
}

// fix-node-name-lock R1 R4: what the locked methods return
func TestNodeLocksKeepTheirResults(t *testing.T) {
	fargate := NewNode(&v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "f", Labels: fargateLabels}})
	if got := fargate.InstanceType(); got != "Fargate" {
		t.Errorf("Fargate node without a pod: %q", got)
	}
	fargate.BindPod(fargatePod("p"))
	if got := fargate.InstanceType(); got != "0.25vCPU-0.5GB" {
		t.Errorf("Fargate node with one pod: %q", got)
	}
	if got := len(fargate.PodsCopy()); got != 1 {
		t.Errorf("PodsCopy: %d pods", got)
	}

	// Labels is the node's own map, not a copy
	labels := map[string]string{"a": "b"}
	if got := NewNode(&v1.Node{ObjectMeta: metav1.ObjectMeta{Labels: labels}}).Labels(); reflect.ValueOf(got).Pointer() != reflect.ValueOf(labels).Pointer() {
		t.Error("Labels() returned a copy")
	}
	azure := namelessNode("azure:///subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/example-rg/providers/Microsoft.Compute/virtualMachines/demo-vm", nil)
	if got := azure.Name(); got != "azure:///…/virtualMachines/demo-vm" {
		t.Errorf("nameless Azure node: %q", got)
	}

	created := time.Unix(1700000000, 0)
	n := namelessNode("gce://example-project/us-central1-a/demo-1", nil)
	if got := n.Name(); got != "gce://…/us-central1-a/demo-1" {
		t.Errorf("nameless GCE node: %q", got)
	}
	n.Update(&v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "n"}, Status: v1.NodeStatus{Conditions: []v1.NodeCondition{
		{Type: v1.NodeReady, Status: v1.ConditionFalse, LastTransitionTime: metav1.NewTime(created.Add(time.Hour))},
	}}})
	if got := n.NotReadyTime(); !got.Equal(created) {
		t.Errorf("NotReadyTime of a NodeClaim node never Ready: %v, want the NodeClaim creation time %v", got, created)
	}
}
