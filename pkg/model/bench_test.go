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
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/staffops/kube-node-viewer/pkg/client"
	"github.com/staffops/kube-node-viewer/pkg/model"
)

// Synthetic cluster sizes for the interactive-tui benchmarks. They are test
// inputs, not targets: the spec compares render time between pod counts at the
// same node count.
const benchNodes = 500

var benchPodCounts = []int{15_000, 150_000}

func benchNode(i int) *v1.Node {
	return &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:              fmt.Sprintf("ip-10-0-%d-%d.ec2.internal", i/250, i%250),
			CreationTimestamp: metav1.NewTime(time.Now().Add(-time.Duration(i) * time.Minute)),
			Labels: map[string]string{
				"karpenter.sh/capacity-type":       []string{"spot", "on-demand"}[i%2],
				"karpenter.sh/nodepool":            []string{"default", "gpu", "batch"}[i%3],
				"node.kubernetes.io/instance-type": "m6i.xlarge",
				"topology.kubernetes.io/zone":      []string{"us-east-1a", "us-east-1b", "us-east-1c"}[i%3],
				"topology.kubernetes.io/region":    "us-east-1",
			},
		},
		Spec: v1.NodeSpec{ProviderID: fmt.Sprintf("aws:///us-east-1a/i-%017d", i)},
		Status: v1.NodeStatus{
			Allocatable: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("4"),
				v1.ResourceMemory: resource.MustParse("16Gi"),
				v1.ResourcePods:   resource.MustParse("110"),
			},
			Conditions: []v1.NodeCondition{{Type: v1.NodeReady, Status: v1.ConditionTrue}},
		},
	}
}

// benchOwners is the controller reference the bench pods keep after the
// informer transform, to measure what keeping the owner name costs
// (node-pods D3).
type benchOwners int

const (
	// ownersKindAndName is what the transform keeps: the controller's kind
	// and name
	ownersKindAndName benchOwners = iota
	// ownersKindOnly is what it kept before node-pods: the kind only
	ownersKindOnly
	// ownersNone is a pod with no owner reference at all
	ownersNone
)

func (o benchOwners) String() string {
	return [...]string{"kind+name", "kind", "none"}[o]
}

// benchPod is what the model receives for a pod: a realistic pod (labels,
// annotations, env, volumes, managed fields, controller) passed through the
// informer transform, as the running tool does.
func benchPod(i, pods int) *v1.Pod {
	return benchPodWith(i, pods, ownersKindAndName)
}

func benchPodWith(i, pods int, owners benchOwners) *v1.Pod {
	full := fullBenchPod(i, pods)
	if owners == ownersNone {
		full.OwnerReferences = nil
	}
	slim, err := client.SlimPod(full)
	if err != nil {
		panic(err)
	}
	pod := slim.(*v1.Pod)
	if owners == ownersKindOnly {
		for r := range pod.OwnerReferences {
			pod.OwnerReferences[r].Name = ""
		}
	}
	return pod
}

// fullBenchPod requests enough that nodes end up about 70% allocated whatever
// the pod count, as on a real cluster where requests fit the node.
func fullBenchPod(i, pods int) *v1.Pod {
	perNode := pods / benchNodes
	cpu := resource.NewMilliQuantity(int64(4000*0.7)/int64(2*perNode), resource.DecimalSI)
	mem := resource.NewQuantity(int64(16<<30)*7/10/int64(2*perNode), resource.BinarySI)
	env := make([]v1.EnvVar, 8)
	for e := range env {
		env[e] = v1.EnvVar{Name: fmt.Sprintf("ENV_%d", e), Value: "some-configuration-value"}
	}
	container := func(name string) v1.Container {
		return v1.Container{
			Name:  name,
			Image: "registry.example.com/team/app:1.2.3",
			Env:   env,
			Resources: v1.ResourceRequirements{Requests: v1.ResourceList{
				v1.ResourceCPU:    *cpu,
				v1.ResourceMemory: *mem,
			}},
			VolumeMounts: []v1.VolumeMount{{Name: "config", MountPath: "/etc/app"}},
		}
	}
	// one pod in ten belongs to a DaemonSet, the others to a ReplicaSet.
	// Kind and name are new strings per pod, as decoding each object makes.
	yes := true
	owner := metav1.OwnerReference{APIVersion: "apps/v1", Kind: strings.Clone("ReplicaSet"),
		Name: fmt.Sprintf("app-%d-7d9c8b6f5", i%200), UID: "0b7e3c1e-6f0a-4c43-9f0e-2d5b1c0a9e11", Controller: &yes}
	if i%10 == 0 {
		owner.Kind, owner.Name = strings.Clone("DaemonSet"), fmt.Sprintf("ds-%d", i%20)
	}
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       fmt.Sprintf("ns-%d", i%40),
			Name:            fmt.Sprintf("app-%d-7d9c8b6f5-%05d", i%200, i),
			OwnerReferences: []metav1.OwnerReference{owner},
			Labels:          map[string]string{"app": fmt.Sprintf("app-%d", i%200), "pod-template-hash": "7d9c8b6f5"},
			Annotations: map[string]string{
				"kubectl.kubernetes.io/restartedAt": "2026-10-01T00:00:00Z",
				"prometheus.io/scrape":              "true",
			},
			ManagedFields: []metav1.ManagedFieldsEntry{{Manager: "kube-controller-manager", Operation: metav1.ManagedFieldsOperationUpdate,
				FieldsV1: metav1.NewFieldsV1(string(make([]byte, 1024)))}},
		},
		Spec: v1.PodSpec{
			NodeName:   benchNode(i % benchNodes).Name,
			Containers: []v1.Container{container("app"), container("sidecar")},
			Volumes:    []v1.Volume{{Name: "config"}},
		},
		Status: v1.PodStatus{Phase: v1.PodRunning},
	}
}

func populate(m *model.UIModel, pods int) {
	populateWith(m, pods, ownersKindAndName)
}

// populateWith fills the cluster and returns the slim pods, which the
// informer's store keeps alive next to the model in the running tool.
func populateWith(m *model.UIModel, pods int, owners benchOwners) []*v1.Pod {
	cluster := m.Cluster()
	for i := 0; i < benchNodes; i++ {
		cluster.AddNode(model.NewNode(benchNode(i))).Show()
	}
	slim := make([]*v1.Pod, pods)
	for i := 0; i < pods; i++ {
		slim[i] = benchPodWith(i, pods, owners)
		cluster.AddPod(model.NewPod(slim[i]))
	}
	// leave the garbage of building the cluster out of the timed part
	runtime.GC()
	return slim
}

func newBenchUI(b *testing.B) *model.UIModel {
	style, err := model.ParseStyle("#04B575,#FFFF00,#FF0000")
	if err != nil {
		b.Fatal(err)
	}
	m := model.NewUIModel(style)
	m.SetResources([]string{"cpu", "memory"})
	m.Update(tea.WindowSizeMsg{Width: 200, Height: 60})
	return m
}

// BenchmarkPodEvent is the cost of applying one pod event to a populated cluster.
func BenchmarkPodEvent(b *testing.B) {
	for _, pods := range benchPodCounts {
		b.Run(fmt.Sprintf("pods=%d", pods), func(b *testing.B) {
			m := newBenchUI(b)
			populate(m, pods)
			extra := make([]*v1.Pod, b.N)
			for i := range extra {
				extra[i] = benchPod(pods+i, pods)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				m.Cluster().AddPod(model.NewPod(extra[i]))
			}
		})
	}
}

// BenchmarkRender is the cost of a snapshot and one frame.
func BenchmarkRender(b *testing.B) {
	for _, pods := range benchPodCounts {
		b.Run(fmt.Sprintf("pods=%d", pods), func(b *testing.B) {
			m := newBenchUI(b)
			populate(m, pods)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				m.Refresh()
				_ = m.View()
			}
		})
	}
}

// BenchmarkHeap reports the live heap held by the model and the informer's
// slim pods after populating them, for each owner reference the transform
// can keep (node-pods D3): kind+name is the tool today, kind the tool before
// node-pods.
func BenchmarkHeap(b *testing.B) {
	for _, pods := range benchPodCounts {
		for _, owners := range []benchOwners{ownersNone, ownersKindOnly, ownersKindAndName} {
			b.Run(fmt.Sprintf("pods=%d/owners=%s", pods, owners), func(b *testing.B) {
				var heap uint64
				for i := 0; i < b.N; i++ {
					var before, after runtime.MemStats
					runtime.GC()
					runtime.ReadMemStats(&before)
					m := newBenchUI(b)
					slim := populateWith(m, pods, owners)
					runtime.GC()
					runtime.ReadMemStats(&after)
					heap = after.HeapAlloc - before.HeapAlloc
					runtime.KeepAlive(m)
					runtime.KeepAlive(slim)
				}
				b.ReportMetric(float64(heap)/(1<<20), "MiB")
			})
		}
	}
}
