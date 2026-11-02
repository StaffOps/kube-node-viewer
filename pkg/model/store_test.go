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

package model_test

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	"github.com/staffops/kube-node-viewer/pkg/model"
)

func storeNode(name string) *model.Node {
	n := testNode(name)
	n.Spec.ProviderID = "aws:///us-west-2a/i-" + name
	node := model.NewNode(n)
	node.Show()
	return node
}

func scheduledPod(name, nodeName string, phase v1.PodPhase) *model.Pod {
	p := testPod("default", name)
	p.Spec.NodeName = nodeName
	p.Status.Phase = phase
	return model.NewPod(p)
}

func usedCPU(t *testing.T, c *model.Cluster) resource.Quantity {
	t.Helper()
	return c.Stats().UsedResources[v1.ResourceCPU]
}

func expectCPU(t *testing.T, c *model.Cluster, want string) {
	t.Helper()
	if got := usedCPU(t, c); got.Cmp(resource.MustParse(want)) != 0 {
		t.Errorf("used cpu = %s, want %s", got.String(), want)
	}
}

// The pod informer starts before the node informer, so pods often arrive first.
func TestStorePodBeforeNode(t *testing.T) {
	c := model.NewCluster()
	c.AddPod(scheduledPod("early", "node-a", v1.PodRunning))
	c.AddNode(storeNode("node-a"))

	st := c.Stats()
	if st.TotalPods != 1 || st.BoundPodCount != 1 {
		t.Errorf("pods total=%d bound=%d, want 1 and 1", st.TotalPods, st.BoundPodCount)
	}
	expectCPU(t, c, "2")
}

// A nodeclaim shows up before its node; pods bind once the node registers.
func TestStoreNodeClaimThenNode(t *testing.T) {
	c := model.NewCluster()
	nc := &karpv1.NodeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "claim", CreationTimestamp: metav1.Now()},
		Status:     karpv1.NodeClaimStatus{ProviderID: "aws:///us-west-2a/i-node-a"},
	}
	c.AddNode(model.NewNodeFromNodeClaim(nc)).Show()
	c.AddPod(scheduledPod("p", "node-a", v1.PodRunning))
	if st := c.Stats(); st.BoundPodCount != 0 {
		t.Fatalf("bound = %d before the node registered, want 0", st.BoundPodCount)
	}

	c.AddNode(storeNode("node-a"))
	if st := c.Stats(); st.NumNodes != 1 || st.BoundPodCount != 1 {
		t.Errorf("nodes=%d bound=%d after registration, want 1 and 1", st.NumNodes, st.BoundPodCount)
	}
	if _, ok := c.GetNodeByName("node-a"); !ok {
		t.Error("node-a not indexed by name")
	}
}

func TestStorePodUpdates(t *testing.T) {
	c := model.NewCluster()
	c.AddNode(storeNode("node-a"))
	c.AddNode(storeNode("node-b"))

	// unscheduled, then scheduled on a, then moved to b: counted once each time
	c.AddPod(scheduledPod("p", "", v1.PodPending))
	if st := c.Stats(); st.TotalPods != 1 || st.PodsByPhase[v1.PodPending] != 1 || st.BoundPodCount != 0 {
		t.Errorf("unscheduled: %+v", st.PodsByPhase)
	}
	c.AddPod(scheduledPod("p", "node-a", v1.PodRunning))
	if st := c.Stats(); st.TotalPods != 1 || st.PodsByPhase[v1.PodPending] != 0 || st.PodsByPhase[v1.PodRunning] != 1 {
		t.Errorf("on node-a: total=%d phases=%v", st.TotalPods, st.PodsByPhase)
	}
	expectCPU(t, c, "2")
	c.AddPod(scheduledPod("p", "node-b", v1.PodRunning))
	expectCPU(t, c, "2")
	a, _ := c.GetNodeByName("node-a")
	b, _ := c.GetNodeByName("node-b")
	if a.NumPods() != 0 || b.NumPods() != 1 {
		t.Errorf("pods a=%d b=%d, want 0 and 1", a.NumPods(), b.NumPods())
	}

	// requests changing in place are reflected, not double counted
	bigger := testPod("default", "p")
	bigger.Spec.NodeName = "node-b"
	bigger.Status.Phase = v1.PodRunning
	bigger.Spec.Containers[0].Resources.Requests[v1.ResourceCPU] = resource.MustParse("3")
	c.AddPod(model.NewPod(bigger))
	if got := usedCPU(t, c); got.Cmp(resource.MustParse("2")) <= 0 {
		t.Errorf("used cpu = %s after the request grew, want more than 2", got.String())
	}

	c.DeletePod("default", "p")
	expectCPU(t, c, "0")
	if st := c.Stats(); st.TotalPods != 0 {
		t.Errorf("total = %d after delete, want 0", st.TotalPods)
	}
}

// A deleted node's pods wait for its name; deleting them later still works.
func TestStoreDeleteNodeKeepsPods(t *testing.T) {
	c := model.NewCluster()
	c.AddNode(storeNode("node-a"))
	c.AddPod(scheduledPod("p", "node-a", v1.PodRunning))
	c.DeleteNode("aws:///us-west-2a/i-node-a")
	if st := c.Stats(); st.NumNodes != 0 || st.TotalPods != 0 {
		t.Errorf("nodes=%d pods=%d after node delete, want 0 and 0", st.NumNodes, st.TotalPods)
	}
	c.AddNode(storeNode("node-a"))
	if st := c.Stats(); st.BoundPodCount != 1 {
		t.Errorf("bound = %d after the node came back, want 1", st.BoundPodCount)
	}
	c.DeletePod("default", "p")
	if st := c.Stats(); st.BoundPodCount != 0 {
		t.Errorf("bound = %d after pod delete, want 0", st.BoundPodCount)
	}
}

func TestStoreReset(t *testing.T) {
	c := model.NewCluster()
	gen := c.Generation()
	c.AddNode(storeNode("node-a"))
	c.AddPod(scheduledPod("p", "node-a", v1.PodRunning))
	c.AddPod(scheduledPod("q", "", v1.PodPending))

	if next := c.Reset(); next == gen || c.Generation() != next {
		t.Errorf("generation %d → %d, want a new one", gen, next)
	}
	if st := c.Stats(); st.NumNodes != 0 || st.TotalPods != 0 {
		t.Errorf("nodes=%d pods=%d after reset, want 0 and 0", st.NumNodes, st.TotalPods)
	}
	if _, ok := c.GetNodeByName("node-a"); ok {
		t.Error("node-a still indexed after reset")
	}
}

func TestStatsLeaveOutEquivalentPrices(t *testing.T) {
	c := model.NewCluster()
	paid := storeNode("paid")
	paid.SetPrice(2)
	reserved := storeNode("reserved")
	reserved.SetEquivalentPrice(5)
	c.AddNode(paid)
	c.AddNode(reserved)

	st := c.Stats()
	if st.TotalPrice != 2 || st.UnpricedReserved != 1 {
		t.Errorf("total %v reserved %d, want 2 and 1", st.TotalPrice, st.UnpricedReserved)
	}
	groups := model.GroupNodes(st.Nodes, "missing-label")
	if len(groups) != 1 || groups[0].TotalPrice != 2 || groups[0].ReservedNodes != 1 || !groups[0].FullyPriced() {
		t.Errorf("group: %+v", groups)
	}
}
