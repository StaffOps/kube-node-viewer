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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
)

// fargateCapacityAnnotation carries the size Fargate provisioned for a pod, the
// only way to price a Fargate node.
const fargateCapacityAnnotation = "CapacityProvisioned"

// SlimPod keeps only what per-node totals and Fargate pricing read from a pod:
// identity, node, phase, deletion, the Fargate capacity annotation and the
// requests that PodRequests sums (pod level, overhead, containers, and init
// containers with their restart policy for sidecars). Everything else, most of
// a pod's size, is dropped before the informer caches it.
func SlimPod(obj interface{}) (interface{}, error) {
	p, ok := obj.(*v1.Pod)
	if !ok {
		// tombstones and anything unexpected pass through untouched
		return obj, nil
	}
	slim := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:         p.Namespace,
			Name:              p.Name,
			UID:               p.UID,
			ResourceVersion:   p.ResourceVersion,
			DeletionTimestamp: p.DeletionTimestamp,
		},
		Spec: v1.PodSpec{
			NodeName:       p.Spec.NodeName,
			Resources:      p.Spec.Resources,
			Overhead:       p.Spec.Overhead,
			Containers:     slimContainers(p.Spec.Containers),
			InitContainers: slimContainers(p.Spec.InitContainers),
		},
		Status: v1.PodStatus{Phase: p.Status.Phase},
	}
	if v, ok := p.Annotations[fargateCapacityAnnotation]; ok {
		slim.Annotations = map[string]string{fargateCapacityAnnotation: v}
	}
	return slim, nil
}

func slimContainers(containers []v1.Container) []v1.Container {
	if len(containers) == 0 {
		return nil
	}
	slim := make([]v1.Container, len(containers))
	for i, c := range containers {
		slim[i] = v1.Container{
			Name:          c.Name,
			Resources:     v1.ResourceRequirements{Requests: c.Resources.Requests},
			RestartPolicy: c.RestartPolicy,
		}
	}
	return slim
}

// slimNode drops the managed fields and the image list, the large parts of a
// node object that the viewer never reads.
func slimNode(obj interface{}) (interface{}, error) {
	n, ok := obj.(*v1.Node)
	if !ok {
		return obj, nil
	}
	slim := n.DeepCopy()
	slim.ManagedFields = nil
	slim.Status.Images = nil
	return slim, nil
}

// slimNodeClaim drops the managed fields.
func slimNodeClaim(obj interface{}) (interface{}, error) {
	nc, ok := obj.(*karpv1.NodeClaim)
	if !ok {
		return obj, nil
	}
	slim := nc.DeepCopy()
	slim.ManagedFields = nil
	return slim, nil
}
