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
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// withReady is the node object with its Ready condition set to status,
// changed at the given time.
func withReady(n *Node, status v1.ConditionStatus, at time.Time) *v1.Node {
	obj := n.Object().DeepCopy()
	obj.Name = "ip-192-0-2-10.ec2.internal"
	obj.CreationTimestamp = metav1.NewTime(time.Unix(1700000600, 0))
	obj.Status.Conditions = []v1.NodeCondition{{Type: v1.NodeReady, Status: status, LastTransitionTime: metav1.NewTime(at)}}
	return obj
}

// fix-ready-lock-fargate-panic R1
func TestReadyTakesNoWriteLock(t *testing.T) {
	n := namelessNode("aws:///us-east-1a/i-0abc", nil)
	n.Update(withReady(n, v1.ConditionTrue, time.Unix(1700000700, 0)))

	// A reader holds the lock, as the view and pricing do. A Ready that takes
	// the write lock waits for it to finish.
	n.mu.RLock()
	got := make(chan bool, 1)
	go func() { got <- n.Ready() }()
	select {
	case ready := <-got:
		n.mu.RUnlock()
		if !ready {
			t.Error("Ready: got false for a Ready node")
		}
	case <-time.After(5 * time.Second):
		n.mu.RUnlock()
		<-got
		t.Fatal("Ready did not return while a reader held the lock: it takes the write lock")
	}
}

// fix-ready-lock-fargate-panic R1: Ready reads under the lock; the race
// detector (go test -race, as scripts/verify.sh runs it) fails an unlocked read
func TestReadyReadsUnderTheLock(t *testing.T) {
	n := namelessNode("aws:///us-east-1a/i-0abc", nil)
	ready := withReady(n, v1.ConditionTrue, time.Unix(1700000700, 0))
	notReady := withReady(n, v1.ConditionFalse, time.Unix(1700000800, 0))
	if !loop(100*time.Millisecond, 5*time.Second,
		func() { _ = n.Ready() },
		func() { n.Update(ready); n.Update(notReady) },
	) {
		t.Fatal("deadlock")
	}
}

// fix-ready-lock-fargate-panic R2
func TestReadyClearedOnUpdate(t *testing.T) {
	claim := time.Unix(1700000000, 0)
	n := namelessNode("aws:///us-east-1a/i-0abc", nil)

	n.Update(withReady(n, v1.ConditionFalse, time.Unix(1700000650, 0)))
	if got := n.Created(); !got.Equal(claim) {
		t.Errorf("NotReady: Created = %v, want the NodeClaim time %v", got, claim)
	}
	if got := n.NotReadyTime(); !got.Equal(claim) {
		t.Errorf("NotReady: NotReadyTime = %v, want the NodeClaim time %v", got, claim)
	}

	// Ready, with no Ready call in between: the view may not have rendered.
	n.Update(withReady(n, v1.ConditionTrue, time.Unix(1700000700, 0)))
	if got, want := n.Created(), time.Unix(1700000600, 0); !got.Equal(want) {
		t.Errorf("Ready: Created = %v, want the node's creation time %v", got, want)
	}

	notReady := time.Unix(1700000800, 0)
	n.Update(withReady(n, v1.ConditionUnknown, notReady))
	if got := n.NotReadyTime(); !got.Equal(notReady) {
		t.Errorf("NotReady again: NotReadyTime = %v, want the transition time %v", got, notReady)
	}
	if n.Ready() {
		t.Error("Ready: got true for a NotReady node")
	}
}

// fix-ready-lock-fargate-panic R3
func TestFargateCapacityMalformed(t *testing.T) {
	// 2vCPU and big panicked; vCPU GB matches with empty groups and already
	// returned 0, 0, false
	for _, value := range []string{"2vCPU", "big", "vCPU GB"} {
		t.Run(value, func(t *testing.T) {
			p := NewPod(&v1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "p",
				Annotations: map[string]string{"CapacityProvisioned": value}}})
			cpu, mem, ok := p.FargateCapacityProvisioned()
			if ok || cpu != 0 || mem != 0 {
				t.Errorf("got %v, %v, %v; want 0, 0, false", cpu, mem, ok)
			}
			n := NewNode(&v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "fargate-ip-192-0-2-11.ec2.internal", Labels: fargateLabels}})
			n.BindPod(p)
			if got := n.InstanceType(); got != "Fargate" {
				t.Errorf("InstanceType = %q, want Fargate", got)
			}
		})
	}
}

// fix-ready-lock-fargate-panic R2
func TestReadyClearedThroughTheStore(t *testing.T) {
	const id = "aws:///us-east-1a/i-0abc"
	nodeTime := time.Unix(1700000600, 0)
	t.Run("AddNode", func(t *testing.T) {
		c := NewCluster()
		stub := c.AddNode(namelessNode(id, nil))
		c.AddNode(NewNode(withReady(stub, v1.ConditionTrue, time.Unix(1700000700, 0))))
		n, ok := c.GetNode(id)
		if !ok {
			t.Fatal("node gone")
		}
		if got := n.Created(); !got.Equal(nodeTime) {
			t.Errorf("Created = %v, want the node's creation time %v", got, nodeTime)
		}
	})
	t.Run("UpdateNode", func(t *testing.T) {
		c := NewCluster()
		stub := c.AddNode(namelessNode(id, nil))
		if _, ok := c.UpdateNode(withReady(stub, v1.ConditionTrue, time.Unix(1700000700, 0))); !ok {
			t.Fatal("UpdateNode: no node")
		}
		n, _ := c.GetNode(id)
		if got := n.Created(); !got.Equal(nodeTime) {
			t.Errorf("Created = %v, want the node's creation time %v", got, nodeTime)
		}
	})
}
