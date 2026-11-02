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
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	"github.com/staffops/kube-node-viewer/pkg/model"
)

func labeledNode(labels map[string]string) *model.Node {
	n := testNode("n")
	n.Labels = labels
	return model.NewNode(n)
}

func TestCapacityType(t *testing.T) {
	cases := []struct {
		labels map[string]string
		want   model.CapacityType
	}{
		{map[string]string{"karpenter.sh/capacity-type": "on-demand"}, model.CapacityOnDemand},
		{map[string]string{"karpenter.sh/capacity-type": "spot"}, model.CapacitySpot},
		{map[string]string{"karpenter.sh/capacity-type": "reserved"}, model.CapacityReserved},
		{map[string]string{"eks.amazonaws.com/capacityType": "ON_DEMAND"}, model.CapacityOnDemand},
		{map[string]string{"eks.amazonaws.com/capacityType": "SPOT"}, model.CapacitySpot},
		{map[string]string{"eks.amazonaws.com/capacityType": "CAPACITY_BLOCK"}, model.CapacityReserved},
		{map[string]string{"eks.amazonaws.com/compute-type": "fargate"}, model.CapacityFargate},
		{map[string]string{"eks.amazonaws.com/compute-type": "auto", "karpenter.sh/capacity-type": "spot"}, model.CapacitySpot},
		{nil, model.CapacityUnknown},
	}
	for _, c := range cases {
		if got := labeledNode(c.labels).CapacityType(); got != c.want {
			t.Errorf("%v: got %s, want %s", c.labels, got, c.want)
		}
	}
}

func TestNodeGroup(t *testing.T) {
	cases := []struct {
		labels  map[string]string
		want    string
		managed bool
	}{
		{map[string]string{"karpenter.sh/nodepool": "default"}, "default", false},
		{map[string]string{"eks.amazonaws.com/nodegroup": "workers"}, "workers", true},
		{map[string]string{"karpenter.sh/nodepool": "gpu", "eks.amazonaws.com/nodegroup": "workers"}, "gpu", false},
		{nil, "-", false},
	}
	for _, c := range cases {
		n := labeledNode(c.labels)
		if got := n.NodeGroup(); got != c.want {
			t.Errorf("%v: got %q, want %q", c.labels, got, c.want)
		}
		if got := n.IsManagedNodeGroup(); got != c.managed {
			t.Errorf("%v: managed %v, want %v", c.labels, got, c.managed)
		}
	}
}

func TestRegionAndInternalIP(t *testing.T) {
	n := testNode("n")
	n.Labels = map[string]string{v1.LabelTopologyRegion: "us-east-1"}
	n.Status.Addresses = []v1.NodeAddress{
		{Type: v1.NodeHostName, Address: "host"},
		{Type: v1.NodeInternalIP, Address: "10.0.1.5"},
	}
	node := model.NewNode(n)
	if node.Region() != "us-east-1" || node.InternalIP() != "10.0.1.5" {
		t.Errorf("region %q ip %q", node.Region(), node.InternalIP())
	}
	if got := labeledNode(nil).InternalIP(); got != "-" {
		t.Errorf("ip without addresses = %q, want -", got)
	}
}

func TestStatusFlags(t *testing.T) {
	ready := testNode("n")
	ready.Status.Conditions = []v1.NodeCondition{{Type: v1.NodeReady, Status: v1.ConditionTrue}}
	if got := model.NewNode(ready).StatusFlags(); !reflect.DeepEqual(got, []string{"Ready"}) {
		t.Errorf("ready: %v", got)
	}

	draining := testNode("n")
	now := metav1.Now()
	draining.DeletionTimestamp = &now
	draining.Spec.Unschedulable = true
	if got := model.NewNode(draining).StatusFlags(); !reflect.DeepEqual(got, []string{"NotReady", "Cordoned", "Deleting"}) {
		t.Errorf("draining: %v", got)
	}

	claim := model.NewNodeFromNodeClaim(&karpv1.NodeClaim{
		Status: karpv1.NodeClaimStatus{ProviderID: "aws:///us-east-1a/i-123"},
	})
	if got := claim.StatusFlags(); !reflect.DeepEqual(got, []string{"NotReady", "Launching"}) {
		t.Errorf("launching: %v", got)
	}
}
