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
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	helperResource "k8s.io/component-helpers/resource"
)

func fullPod() *v1.Pod {
	always := v1.ContainerRestartPolicyAlways
	now := metav1.Now()
	req := func(cpu string) v1.ResourceRequirements {
		return v1.ResourceRequirements{
			Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse(cpu)},
			Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("8")},
		}
	}
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns", Name: "p", UID: "uid", ResourceVersion: "42",
			DeletionTimestamp: &now,
			Labels:            map[string]string{"app": "x"},
			Annotations: map[string]string{
				"CapacityProvisioned": "0.25vCPU 0.5GB",
				"other":               "dropped",
			},
			ManagedFields: []metav1.ManagedFieldsEntry{{Manager: "m"}},
		},
		Spec: v1.PodSpec{
			NodeName: "node-a",
			Overhead: v1.ResourceList{v1.ResourceCPU: resource.MustParse("100m")},
			Resources: &v1.ResourceRequirements{
				Requests: v1.ResourceList{v1.ResourceMemory: resource.MustParse("1Gi")},
			},
			Containers: []v1.Container{{
				Name: "app", Image: "img", Resources: req("1"),
				Env: []v1.EnvVar{{Name: "A", Value: "b"}},
			}},
			InitContainers: []v1.Container{
				{Name: "sidecar", Resources: req("500m"), RestartPolicy: &always},
				{Name: "init", Resources: req("2")},
			},
			Volumes: []v1.Volume{{Name: "v"}},
		},
		Status: v1.PodStatus{Phase: v1.PodRunning, PodIP: "10.0.0.1"},
	}
}

func TestSlimPodKeepsWhatTotalsNeed(t *testing.T) {
	full := fullPod()
	out, err := SlimPod(full)
	if err != nil {
		t.Fatal(err)
	}
	slim := out.(*v1.Pod)

	opts := helperResource.PodResourcesOptions{}
	want := helperResource.PodRequests(full, opts)
	got := helperResource.PodRequests(slim, opts)
	for rn, q := range want {
		if g := got[rn]; g.Cmp(q) != 0 {
			t.Errorf("%s requests = %s, want %s", rn, g.String(), q.String())
		}
	}

	if slim.Namespace != "ns" || slim.Name != "p" || slim.Spec.NodeName != "node-a" ||
		slim.Status.Phase != v1.PodRunning || slim.DeletionTimestamp.IsZero() {
		t.Errorf("identity, node, phase or deletion lost: %+v", slim.ObjectMeta)
	}
	if got := slim.Annotations["CapacityProvisioned"]; got != "0.25vCPU 0.5GB" {
		t.Errorf("fargate annotation = %q", got)
	}
	if _, ok := slim.Annotations["other"]; ok || slim.Labels != nil || slim.ManagedFields != nil ||
		slim.Spec.Volumes != nil || slim.Spec.Containers[0].Env != nil || slim.Spec.Containers[0].Resources.Limits != nil ||
		slim.Status.PodIP != "" {
		t.Errorf("fields that should be dropped were kept: %+v", slim)
	}
}

func TestSlimPodPassesTombstones(t *testing.T) {
	tomb := cache.DeletedFinalStateUnknown{Key: "ns/p", Obj: fullPod()}
	out, err := SlimPod(tomb)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := out.(cache.DeletedFinalStateUnknown); !ok {
		t.Errorf("tombstone became %T", out)
	}
}

func TestSlimNodeDropsImagesAndManagedFields(t *testing.T) {
	n := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "n", Labels: map[string]string{"a": "b"},
			ManagedFields: []metav1.ManagedFieldsEntry{{Manager: "m"}}},
		Status: v1.NodeStatus{Images: []v1.ContainerImage{{Names: []string{"img"}}},
			Allocatable: v1.ResourceList{v1.ResourceCPU: resource.MustParse("2")}},
	}
	out, _ := slimNode(n)
	slim := out.(*v1.Node)
	if slim.ManagedFields != nil || slim.Status.Images != nil {
		t.Error("managed fields or images kept")
	}
	if slim.Labels["a"] != "b" || slim.Status.Allocatable.Cpu().String() != "2" {
		t.Error("labels or allocatable lost")
	}
	if n.Status.Images == nil {
		t.Error("the input node was modified")
	}
}

func TestSlimPodKeepsTheControllerKind(t *testing.T) {
	yes := true
	no := false
	p := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "ds-x", OwnerReferences: []metav1.OwnerReference{
		{Kind: "Something", Name: "other", Controller: &no},
		{APIVersion: "apps/v1", Kind: "DaemonSet", Name: "fluent-bit", UID: "u", Controller: &yes},
	}}}
	out, _ := SlimPod(p)
	refs := out.(*v1.Pod).OwnerReferences
	if len(refs) != 1 || refs[0].Kind != "DaemonSet" || refs[0].Name != "fluent-bit" || refs[0].UID != "" {
		t.Errorf("owners: %+v", refs)
	}
	if out, _ := SlimPod(&v1.Pod{}); out.(*v1.Pod).OwnerReferences != nil {
		t.Error("an owner appeared from nothing")
	}
}

// node-pods R6: the transform keeps the controller's kind and name, and
// nothing of the other owner references.
func TestSlimPodKeepsTheControllerName(t *testing.T) {
	yes, no := true, false
	tests := []struct {
		name     string
		refs     []metav1.OwnerReference
		wantKind string
		wantName string
	}{
		{"daemonset controller", []metav1.OwnerReference{{Kind: "DaemonSet", Name: "aws-node", UID: "u", Controller: &yes}}, "DaemonSet", "aws-node"},
		{"replicaset controller after a non-controller", []metav1.OwnerReference{
			{Kind: "ConfigMap", Name: "cm", Controller: &no},
			{Kind: "ReplicaSet", Name: "api-7f9c", Controller: &yes},
		}, "ReplicaSet", "api-7f9c"},
		{"controller with no name", []metav1.OwnerReference{{Kind: "DaemonSet", Controller: &yes}}, "DaemonSet", ""},
		{"owners but no controller", []metav1.OwnerReference{{Kind: "DaemonSet", Name: "x"}, {Kind: "Job", Name: "y", Controller: &no}}, "", ""},
		{"no owners", nil, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := SlimPod(&v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p", OwnerReferences: tt.refs}})
			if err != nil {
				t.Fatal(err)
			}
			refs := out.(*v1.Pod).OwnerReferences
			if tt.wantKind == "" {
				if refs != nil {
					t.Errorf("owners %+v, want none", refs)
				}
				return
			}
			if len(refs) != 1 || refs[0].Kind != tt.wantKind || refs[0].Name != tt.wantName || refs[0].UID != "" || refs[0].APIVersion != "" {
				t.Errorf("owners %+v, want only {%s %s}", refs, tt.wantKind, tt.wantName)
			}
		})
	}
}
