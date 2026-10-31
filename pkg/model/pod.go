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

package model

import (
	"log"
	"regexp"
	"strconv"
	"sync"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	helperResource "k8s.io/component-helpers/resource"
)

// Pod is what the viewer keeps of a pod: where it runs, its phase and the
// resources it requests. The tool shows nodes, so the pod object itself is not
// kept; at 150 000 pods its size dominates memory and garbage collection.
type Pod struct {
	mu        sync.RWMutex
	namespace string
	name      string
	nodeName  string
	phase     v1.PodPhase
	// requested is computed once per pod object, it is read on every bind and
	// unbind
	requested v1.ResourceList
	// fargateCapacity is the CapacityProvisioned annotation, set on Fargate pods
	fargateCapacity string
}

// NewPod constructs a pod model based off of the K8s pod object
func NewPod(pod *v1.Pod) *Pod {
	p := &Pod{}
	p.set(pod)
	return p
}

// Update replaces the pod model with the provided pod. Don't update a pod held
// by a Cluster: pass Cluster.AddPod a new Pod instead, it needs the old one to
// take the old requests back out.
func (p *Pod) Update(pod *v1.Pod) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.set(pod)
}

func (p *Pod) set(pod *v1.Pod) {
	p.namespace = pod.Namespace
	p.name = pod.Name
	p.nodeName = pod.Spec.NodeName
	p.phase = pod.Status.Phase
	p.requested = computeRequests(pod)
	p.fargateCapacity = pod.Annotations["CapacityProvisioned"]
}

func (p *Pod) key() objectKey {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return objectKey{namespace: p.namespace, name: p.name}
}

// IsScheduled returns true if the pod has been scheduled to a node
func (p *Pod) IsScheduled() bool {
	return p.NodeName() != ""
}

// NodeName returns the node that the pod is scheduled against, or an empty string
func (p *Pod) NodeName() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.nodeName
}

// Namespace returns the namespace of the pod
func (p *Pod) Namespace() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.namespace
}

// Name returns the name of the pod
func (p *Pod) Name() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.name
}

// Phase returns the pod phase
func (p *Pod) Phase() v1.PodPhase {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.phase
}

// Requested returns the sum of the resources requested by the pod. The list is
// shared, callers must not modify it.
func (p *Pod) Requested() v1.ResourceList {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.requested
}

// computeRequests sums the pod's requests, including init containers that are
// sidecars as described in
// https://kubernetes.io/blog/2023/08/25/native-sidecar-containers .
func computeRequests(pod *v1.Pod) v1.ResourceList {
	requested := helperResource.PodRequests(pod, helperResource.PodResourcesOptions{
		SkipPodLevelResources:       false,
		SkipContainerLevelResources: false,
	})
	requested[v1.ResourcePods] = resource.MustParse("1")
	return requested
}

var fargateCapacityRe = regexp.MustCompile("(.*?)vCPU (.*?)GB")

func (p *Pod) FargateCapacityProvisioned() (float64, float64, bool) {
	p.mu.RLock()
	provisioned := p.fargateCapacity
	p.mu.RUnlock()
	if provisioned == "" {
		return 0, 0, false
	}

	match := fargateCapacityRe.FindStringSubmatch(provisioned)
	if len(match) != 3 {
		log.Printf("unable to parse %q for fargate provisioner capacity", provisioned)
	}
	cpu, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		log.Printf("unable to parse CPU from fargate capacity, %q, %s", provisioned, err)
		return 0, 0, false
	}
	mem, err := strconv.ParseFloat(match[2], 64)
	if err != nil {
		log.Printf("unable to parse memory from fargate capacity, %q, %s", provisioned, err)
		return 0, 0, false
	}
	return cpu, mem, true
}
