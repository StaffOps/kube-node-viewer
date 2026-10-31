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
	"sync"

	v1 "k8s.io/api/core/v1"
)

// Cluster is the indexed store behind the view. Nodes are keyed by provider ID
// and indexed by registered name; pods are linked to their node as they arrive,
// so each node keeps running totals and nothing on the render path walks pods.
type Cluster struct {
	mu sync.RWMutex
	// generation changes on Reset, so events from a previous cluster can be told
	// apart from current ones
	generation  uint64
	nodes       map[string]*Node
	nodesByName map[string]*Node
	pods        map[objectKey]*Pod
	// pending holds scheduled pods whose node we haven't seen yet, by node name.
	// They are bound when the node, or the nodeclaim's registered node, arrives.
	pending            map[string]map[objectKey]*Pod
	unscheduledByPhase map[v1.PodPhase]int
	resources          []v1.ResourceName
}

func NewCluster() *Cluster {
	c := &Cluster{resources: []v1.ResourceName{v1.ResourceCPU}}
	c.clear()
	return c
}

func (c *Cluster) clear() {
	c.nodes = map[string]*Node{}
	c.nodesByName = map[string]*Node{}
	c.pods = map[objectKey]*Pod{}
	c.pending = map[string]map[objectKey]*Pod{}
	c.unscheduledByPhase = map[v1.PodPhase]int{}
}

// Generation identifies the cluster data currently held.
func (c *Cluster) Generation() uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.generation
}

// Reset drops every node and pod and starts a new generation, returned.
func (c *Cluster) Reset() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clear()
	c.generation++
	return c.generation
}

func (c *Cluster) AddNode(node *Node) *Node {
	c.mu.Lock()
	defer c.mu.Unlock()
	if existing, ok := c.nodes[node.ProviderID()]; ok {
		oldName := existing.RegisteredName()
		existing.Update(&node.node)
		c.indexName(existing, oldName)
		return existing
	}

	c.nodes[node.ProviderID()] = node
	c.indexName(node, "")
	return node
}

// UpdateNode applies a node object to the node we already know with the same
// provider ID, reporting whether there was one.
func (c *Cluster) UpdateNode(n *v1.Node) (*Node, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	existing, ok := c.nodes[n.Spec.ProviderID]
	if !ok {
		return nil, false
	}
	oldName := existing.RegisteredName()
	existing.Update(n)
	c.indexName(existing, oldName)
	return existing, true
}

// indexName keeps nodesByName current when a node gets or changes its name, as
// a nodeclaim-only node does once its node registers, and binds the pods that
// were waiting for that name. Called with the lock held.
func (c *Cluster) indexName(n *Node, oldName string) {
	name := n.RegisteredName()
	if name == oldName {
		return
	}
	if oldName != "" && c.nodesByName[oldName] == n {
		delete(c.nodesByName, oldName)
	}
	if name == "" {
		return
	}
	c.nodesByName[name] = n
	for _, p := range c.pending[name] {
		n.BindPod(p)
	}
	delete(c.pending, name)
}

func (c *Cluster) DeleteNode(providerID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	n, ok := c.nodes[providerID]
	if !ok {
		return
	}
	// the pods outlive the node until their own delete events arrive, keep them
	// waiting for the name in case the node comes back
	if name := n.RegisteredName(); name != "" {
		for _, p := range n.Pods() {
			c.addPending(name, p)
		}
		if c.nodesByName[name] == n {
			delete(c.nodesByName, name)
		}
	}
	delete(c.nodes, providerID)
}

func (c *Cluster) ForEachNode(f func(n *Node)) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, n := range c.nodes {
		f(n)
	}
}

func (c *Cluster) GetNode(providerID string) (*Node, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	n, ok := c.nodes[providerID]
	return n, ok
}

func (c *Cluster) GetNodeByName(name string) (*Node, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	n, ok := c.nodesByName[name]
	return n, ok
}

// AddPod adds a pod or replaces the one with the same namespace and name,
// moving its requests to the node it is now on. Pass a new Pod for every
// update: the old one is needed to take its requests back out.
func (c *Cluster) AddPod(pod *Pod) (totalPods int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := pod.key()
	if old, ok := c.pods[key]; ok {
		c.unlink(old)
	}
	c.pods[key] = pod
	c.link(pod)
	return len(c.pods)
}

func (c *Cluster) DeletePod(namespace, name string) (totalPods int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := objectKey{namespace: namespace, name: name}
	if p, ok := c.pods[key]; ok {
		c.unlink(p)
		delete(c.pods, key)
	}
	return len(c.pods)
}

// link counts a pod towards its node, or towards the unscheduled pods. Called
// with the lock held.
func (c *Cluster) link(p *Pod) {
	nodeName := p.NodeName()
	if nodeName == "" {
		c.unscheduledByPhase[p.Phase()]++
		return
	}
	if n, ok := c.nodesByName[nodeName]; ok {
		n.BindPod(p)
		return
	}
	c.addPending(nodeName, p)
}

// unlink reverses link. Called with the lock held.
func (c *Cluster) unlink(p *Pod) {
	nodeName := p.NodeName()
	if nodeName == "" {
		if c.unscheduledByPhase[p.Phase()]--; c.unscheduledByPhase[p.Phase()] <= 0 {
			delete(c.unscheduledByPhase, p.Phase())
		}
		return
	}
	if n, ok := c.nodesByName[nodeName]; ok {
		n.DeletePod(p.Namespace(), p.Name())
	}
	if waiting, ok := c.pending[nodeName]; ok {
		delete(waiting, p.key())
		if len(waiting) == 0 {
			delete(c.pending, nodeName)
		}
	}
}

func (c *Cluster) addPending(nodeName string, p *Pod) {
	waiting, ok := c.pending[nodeName]
	if !ok {
		waiting = map[objectKey]*Pod{}
		c.pending[nodeName] = waiting
	}
	waiting[p.key()] = p
}

func (c *Cluster) GetPod(namespace string, name string) (*Pod, bool) {
	c.mu.RLock()
	pod, ok := c.pods[objectKey{namespace: namespace, name: name}]
	c.mu.RUnlock()
	return pod, ok
}

// Stats sums the visible nodes. Pod counts come from the per-node totals plus
// the unscheduled pods, which belong to no node and are always counted as they
// are capacity still needed. Pods on nodes we aren't displaying are left out.
func (c *Cluster) Stats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	st := Stats{
		AllocatableResources: v1.ResourceList{},
		UsedResources:        v1.ResourceList{},
		PercentUsedResoruces: map[v1.ResourceName]float64{},
		PodsByPhase:          map[v1.PodPhase]int{},
		Nodes:                make([]*Node, 0, len(c.nodes)),
	}

	for phase, count := range c.unscheduledByPhase {
		st.PodsByPhase[phase] += count
		st.TotalPods += count
	}

	for _, n := range c.nodes {
		if !n.Visible() {
			continue
		}
		// only add the price if it's not NaN which is used to indicate an unknown
		// price
		if n.HasPrice() {
			st.TotalPrice += n.Price()
		}
		st.NumNodes++
		st.Nodes = append(st.Nodes, n)
		n.AddAllocatableTo(st.AllocatableResources)
		n.AddUsedTo(st.UsedResources)
		bound := n.AddPodPhasesTo(st.PodsByPhase)
		st.BoundPodCount += bound
		st.TotalPods += bound
	}
	return st
}

// addResources sets lhs = lhs + rhs
func addResources(lhs v1.ResourceList, rhs v1.ResourceList) {
	for rn, q := range rhs {
		existing := lhs[rn]
		existing.Add(q)
		lhs[rn] = existing
	}
}
