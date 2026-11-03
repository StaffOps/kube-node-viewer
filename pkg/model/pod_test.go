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

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/staffops/kube-node-viewer/pkg/client"
	"github.com/staffops/kube-node-viewer/pkg/model"
)

func testPod(namespace, name string) *v1.Pod {
	restartAlways := v1.ContainerRestartPolicyAlways
	p := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Status: v1.PodStatus{
			Phase: v1.PodPending,
		},
		Spec: v1.PodSpec{
			InitContainers: []v1.Container{
				{
					Image: "normalinit",
					Name:  "container",
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("1"),
							v1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
				},
				{
					Image: "sidecar",
					Name:  "container",
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("1"),
							v1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
					RestartPolicy: &restartAlways,
				},
			},
			Containers: []v1.Container{
				{
					Image: "test-image",
					Name:  "container",
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("1"),
							v1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
				},
			},
		},
	}
	return p
}

func testPodWithPodLevelResources(namespace, name string) *v1.Pod {
	restartAlways := v1.ContainerRestartPolicyAlways
	p := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Status: v1.PodStatus{
			Phase: v1.PodPending,
		},
		Spec: v1.PodSpec{
			Resources: &v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("10"),
					v1.ResourceMemory: resource.MustParse("12Gi"),
				},
			},
			InitContainers: []v1.Container{
				{
					Image: "normalinit",
					Name:  "container",
				},
				{
					Image:         "sidecar",
					Name:          "container",
					RestartPolicy: &restartAlways,
				},
			},
			Containers: []v1.Container{
				{
					Image: "test-image",
					Name:  "container",
				},
			},
		},
	}
	return p
}

func TestNewPod(t *testing.T) {
	pod := testPod("default", "mypod")
	pod.Spec.NodeName = "mynode"
	p := model.NewPod(pod)
	if exp, got := "default", p.Namespace(); exp != got {
		t.Errorf("expected Namespace = %s, got %s", exp, got)
	}
	if exp, got := "mypod", p.Name(); exp != got {
		t.Errorf("expected Name = %s, got %s", exp, got)
	}
	if exp, got := "mynode", p.NodeName(); exp != got {
		t.Errorf("expected NodeName = %s, got %s", exp, got)
	}
	if exp, got := true, p.IsScheduled(); exp != got {
		t.Errorf("expected IsScheduled = %v, got %v", exp, got)
	}
	if exp, got := v1.PodPending, p.Phase(); exp != got {
		t.Errorf("expected Phase = %v, got %v", exp, got)
	}

	if exp, got := resource.MustParse("2"), p.Requested()[v1.ResourceCPU]; exp.Cmp(got) != 0 {
		t.Errorf("expected CPU = %s, got %s", exp.String(), got.String())
	}
	if exp, got := resource.MustParse("2Gi"), p.Requested()[v1.ResourceMemory]; exp.Cmp(got) != 0 {
		t.Errorf("expected Memory = %s, got %s", exp.String(), got.String())
	}

	podWithPodLevelResources := testPodWithPodLevelResources("default", "mypod")
	pWithPodLevelResources := model.NewPod(podWithPodLevelResources)
	if exp, got := resource.MustParse("10"), pWithPodLevelResources.Requested()[v1.ResourceCPU]; exp.Cmp(got) != 0 {
		t.Errorf("expected CPU = %s, got %s", exp.String(), got.String())
	}
	if exp, got := resource.MustParse("12Gi"), pWithPodLevelResources.Requested()[v1.ResourceMemory]; exp.Cmp(got) != 0 {
		t.Errorf("expected Memory = %s, got %s", exp.String(), got.String())
	}
}

func TestPodUpdate(t *testing.T) {
	p := model.NewPod(testPod("default", "mypod"))
	if exp, got := "", p.NodeName(); got != exp {
		t.Errorf("expeted NodeName == %s, got %s", exp, got)
	}
	replacement := testPod("default", "mypod")
	replacement.Spec.NodeName = "scheduled.node"
	p.Update(replacement)
	if exp, got := "scheduled.node", p.NodeName(); got != exp {
		t.Errorf("expeted NodeName == %s, got %s", exp, got)
	}
}

func TestFargateCapacity(t *testing.T) {
	tp := testPod("default", "mypod")
	tp.Annotations = map[string]string{
		"CapacityProvisioned": "0.25vCPU 0.5GB",
	}
	p := model.NewPod(tp)
	cpu, mem, ok := p.FargateCapacityProvisioned()
	if !ok {
		t.Errorf("expected to have a fargate capacity")
	}
	if cpu != 0.25 {
		t.Errorf("expected to have a cpu capacity of 0.25, got %f", cpu)
	}
	if mem != 0.5 {
		t.Errorf("expected to have a mem capacity of 0.5, got %f", mem)
	}
}

func TestPodStaysOnItsNode(t *testing.T) {
	yes := true
	for kind, want := range map[string]bool{"DaemonSet": true, "Node": true, "ReplicaSet": false, "": false} {
		p := testPod("ns", "p")
		if kind != "" {
			p.OwnerReferences = []metav1.OwnerReference{{Kind: kind, Controller: &yes}}
		}
		if got := model.NewPod(p).StaysOnItsNode(); got != want {
			t.Errorf("%q: got %v", kind, got)
		}
	}
}

// node-pods R6: a pod exposes its controller's kind and name, also after
// the informer transform.
func TestPodOwner(t *testing.T) {
	yes, no := true, false
	tests := []struct {
		name               string
		refs               []metav1.OwnerReference
		wantKind, wantName string
	}{
		{"daemonset", []metav1.OwnerReference{{Kind: "DaemonSet", Name: "fluent-bit", Controller: &yes}}, "DaemonSet", "fluent-bit"},
		{"replicaset after a non-controller", []metav1.OwnerReference{
			{Kind: "ConfigMap", Name: "cm", Controller: &no}, {Kind: "ReplicaSet", Name: "api-7f9c", Controller: &yes},
		}, "ReplicaSet", "api-7f9c"},
		{"static pod", []metav1.OwnerReference{{Kind: "Node", Name: "node-a", Controller: &yes}}, "Node", "node-a"},
		{"no controller", []metav1.OwnerReference{{Kind: "DaemonSet", Name: "x"}}, "", ""},
		{"no owners", nil, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := testPod("ns", "p")
			raw.OwnerReferences = tt.refs
			slim, err := client.SlimPod(raw.DeepCopy())
			if err != nil {
				t.Fatal(err)
			}
			for via, obj := range map[string]*v1.Pod{"full": raw, "slim": slim.(*v1.Pod)} {
				kind, name := model.NewPod(obj).Owner()
				if kind != tt.wantKind || name != tt.wantName {
					t.Errorf("%s: owner (%q, %q), want (%q, %q)", via, kind, name, tt.wantKind, tt.wantName)
				}
			}
		})
	}
	// an update that drops the controller drops the owner
	yesRef := []metav1.OwnerReference{{Kind: "DaemonSet", Name: "fluent-bit", Controller: &yes}}
	raw := testPod("ns", "p")
	raw.OwnerReferences = yesRef
	p := model.NewPod(raw)
	p.Update(testPod("ns", "p"))
	if kind, name := p.Owner(); kind != "" || name != "" {
		t.Errorf("owner after the update (%q, %q), want none", kind, name)
	}
}
