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

// Modified by the kube-node-viewer authors, 2026.
package model_test

import (
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/staffops/kube-node-viewer/pkg/model"
)

func testNode(name string) *v1.Node {
	n := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: v1.NodeStatus{
			Phase: v1.NodePending,
		},
	}
	return n
}

func TestNewNode(t *testing.T) {
	n := testNode("mynode")
	node := model.NewNode(n)
	if exp, got := "mynode", node.Name(); exp != got {
		t.Errorf("expeted Name == %s, got %s", exp, got)
	}
}

func TestNodeTypeUnknown(t *testing.T) {
	n := testNode("mynode")
	node := model.NewNode(n)
	if node.IsOnDemand() {
		t.Errorf("exepcted to not be on-demand")
	}
	if node.IsSpot() {
		t.Errorf("exepcted to not be spot")
	}
}

func TestNodeTypeOnDemand(t *testing.T) {
	for label, value := range map[string]string{
		"karpenter.sh/capacity-type":     "on-demand",
		"eks.amazonaws.com/capacityType": "ON_DEMAND",
	} {
		n := testNode("mynode")
		n.Labels = map[string]string{
			label: value,
		}
		node := model.NewNode(n)
		if !node.IsOnDemand() {
			t.Errorf("exepcted on-demand")
		}
		if node.IsSpot() {
			t.Errorf("exepcted to not be spot")
		}
		if node.IsFargate() {
			t.Errorf("exepcted to not be fargate")
		}
	}
}

func TestNodeTypeSpot(t *testing.T) {
	for label, value := range map[string]string{
		"karpenter.sh/capacity-type":     "spot",
		"eks.amazonaws.com/capacityType": "SPOT",
	} {
		n := testNode("mynode")
		n.Labels = map[string]string{
			label: value,
		}
		node := model.NewNode(n)
		if node.IsOnDemand() {
			t.Errorf("exepcted to not be on-demand")
		}
		if !node.IsSpot() {
			t.Errorf("exepcted to be spot")
		}
		if node.IsFargate() {
			t.Errorf("exepcted to not be fargate")
		}
	}
}

func TestNodeTypeFargate(t *testing.T) {
	for label, value := range map[string]string{
		"eks.amazonaws.com/compute-type": "fargate",
	} {
		n := testNode("mynode")
		n.Labels = map[string]string{
			label: value,
		}
		node := model.NewNode(n)
		if node.IsOnDemand() {
			t.Errorf("exepcted to not be on-demand")
		}
		if node.IsSpot() {
			t.Errorf("exepcted to not be spot")
		}
		if !node.IsFargate() {
			t.Errorf("exepcted to be fargate")
		}
	}
}

func TestNodeTypeAuto(t *testing.T) {
	for label, value := range map[string]string{
		"eks.amazonaws.com/compute-type": "auto",
	} {
		n := testNode("mynode")
		n.Labels = map[string]string{
			label: value,
		}
		node := model.NewNode(n)
		if node.IsOnDemand() {
			t.Errorf("exepcted to not be on-demand")
		}
		if node.IsSpot() {
			t.Errorf("exepcted to not be spot")
		}
		if node.IsFargate() {
			t.Errorf("exepcted to not be fargate")
		}
		if !node.IsAuto() {
			t.Errorf("exepcted to be auto")
		}
	}
}

func TestNodeNotReadyFalse(t *testing.T) {
	for _, status := range []v1.ConditionStatus{v1.ConditionFalse, v1.ConditionUnknown} {
		t.Run(string(status), func(t *testing.T) {
			n := testNode("mynode")
			n.Status.Phase = v1.NodeRunning
			notReadyTime := time.Now().Add(-1 * time.Hour)

			n.Status.Conditions = append(n.Status.Conditions, v1.NodeCondition{
				Type:   v1.NodeReady,
				Status: status,
				LastTransitionTime: metav1.Time{
					Time: notReadyTime,
				},
			})
			node := model.NewNode(n)
			if node.Ready() {
				t.Fatalf("expected node to be not ready")
			}

			if node.NotReadyTime() != notReadyTime {
				t.Errorf("expected not ready time = %s, got %s", notReadyTime, node.NotReadyTime())
			}
		})
	}
}

func TestNodeNotReadyNoCondition(t *testing.T) {
	for _, status := range []v1.ConditionStatus{v1.ConditionFalse, v1.ConditionUnknown} {
		t.Run(string(status), func(t *testing.T) {
			n := testNode("mynode")
			n.Status.Phase = v1.NodeRunning
			notReadyTime := time.Now().Add(-1 * time.Hour)
			n.CreationTimestamp = metav1.NewTime(notReadyTime)

			node := model.NewNode(n)
			if node.Ready() {
				t.Fatalf("expected node to be not ready")
			}

			if node.NotReadyTime() != notReadyTime {
				t.Errorf("expected not ready time = %s, got %s", notReadyTime, node.NotReadyTime())
			}
		})
	}
}

func TestComputeLabelPrefixes(t *testing.T) {
	n := testNode("mynode")
	n.CreationTimestamp = metav1.NewTime(time.Now().Add(-2 * time.Hour))
	n.Status.Allocatable = v1.ResourceList{v1.ResourceCPU: resource.MustParse("4")}
	node := model.NewNode(n)
	for _, name := range []string{"node-age", "node-cpu-usage"} {
		legacy := node.ComputeLabel(model.LegacyLabelPrefix + name)
		current := node.ComputeLabel(model.LabelPrefix + name)
		if legacy == "-" || legacy != current {
			t.Errorf("%s: legacy %q, new %q; want equal and computed", name, legacy, current)
		}
	}
	if got := node.ComputeLabel("other/node-age"); got != "-" {
		t.Errorf("unknown prefix computed %q, want -", got)
	}
}
