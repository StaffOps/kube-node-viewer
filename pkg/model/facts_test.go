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

func aksNode(labels map[string]string) *model.Node {
	n := testNode("aks-pool-1-vmss000000")
	n.Spec.ProviderID = "azure:///subscriptions/s/resourceGroups/mc_rg/providers/Microsoft.Compute/virtualMachineScaleSets/aks-pool-1-vmss/virtualMachines/0"
	n.Labels = labels
	return model.NewNode(n)
}

// AKS: agent pools are the node groups, Spot is marked by the scale set
// priority and every other VM is pay-as-you-go; a VM outside availability
// zones has its fault domain as zone.
func TestAKSNodeFacts(t *testing.T) {
	regular := aksNode(map[string]string{"kubernetes.azure.com/agentpool": "system", v1.LabelTopologyZone: "0"})
	if got := regular.CapacityType(); got != model.CapacityOnDemand {
		t.Errorf("capacity %s, want OnDemand", got)
	}
	if got := regular.NodeGroup(); got != "system" || !regular.IsManagedNodeGroup() {
		t.Errorf("node group %q managed %v, want system, true", got, regular.IsManagedNodeGroup())
	}
	if got := regular.Zone(); got != "" {
		t.Errorf("fault domain read as zone %q", got)
	}
	if got := regular.Cloud(); got != "azure" {
		t.Errorf("cloud %q", got)
	}
	spot := aksNode(map[string]string{"kubernetes.azure.com/scalesetpriority": "spot", v1.LabelTopologyZone: "eastus2-1"})
	if got := spot.CapacityType(); got != model.CapacitySpot {
		t.Errorf("capacity %s, want Spot", got)
	}
	if got := spot.Zone(); got != "eastus2-1" {
		t.Errorf("zone %q, want eastus2-1", got)
	}
	// an EKS node keeps its zone and an unlabelled one stays Unknown
	eks := testNode("ip-10-0-0-1")
	eks.Spec.ProviderID = "aws:///us-east-1a/i-0abc"
	eks.Labels = map[string]string{v1.LabelTopologyZone: "us-east-1a"}
	if n := model.NewNode(eks); n.Zone() != "us-east-1a" || n.CapacityType() != model.CapacityUnknown || n.Cloud() != "aws" {
		t.Errorf("EKS node: zone %q capacity %s cloud %q", n.Zone(), n.CapacityType(), n.Cloud())
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

// the zone label wins; without it an EC2 provider ID names the zone
func TestZoneFallsBackToTheProviderID(t *testing.T) {
	cases := []struct {
		providerID, label, want string
	}{
		{"aws:///us-east-1a/i-0abc", "us-east-1b", "us-east-1b"},
		{"aws:///us-east-1a/i-0abc", "", "us-east-1a"},
		{"aws:///us-east-1-bos-1a/i-0abc", "", "us-east-1-bos-1a"},
		{"aws:///us-east-1b/0123abcd/fargate-ip-10-0-0-1.ec2.internal", "", "us-east-1b"},
		{"kind://docker/kind/kind-worker", "", ""},
		{"", "", ""},
	}
	for _, c := range cases {
		n := testNode("n")
		n.Spec.ProviderID = c.providerID
		if c.label != "" {
			n.Labels = map[string]string{v1.LabelTopologyZone: c.label}
		}
		if got := model.NewNode(n).Zone(); got != c.want {
			t.Errorf("%q label %q: got %q, want %q", c.providerID, c.label, got, c.want)
		}
	}
}
