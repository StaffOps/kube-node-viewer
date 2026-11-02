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
	v1 "k8s.io/api/core/v1"
)

// UngroupedValue names the group holding nodes without the label.
const UngroupedValue = "<none>"

// GroupStats is usage and cost summed over the nodes sharing a label value.
type GroupStats struct {
	Name                 string
	NumNodes             int
	NumPods              int
	AllocatableResources v1.ResourceList
	UsedResources        v1.ResourceList
	TotalPrice           float64
	// PricedNodes is how many nodes contributed to TotalPrice
	PricedNodes int
	// ReservedNodes have only an equivalent price, left out of TotalPrice
	ReservedNodes int
}

// PercentUsed is the group's usage as a whole, so weighted by node size rather
// than the mean of each node's own percentage.
func (g GroupStats) PercentUsed(resource v1.ResourceName) float64 {
	allocatable := g.AllocatableResources[resource]
	if allocatable.AsApproximateFloat64() == 0 {
		return 0
	}
	used := g.UsedResources[resource]
	return 100 * (used.AsApproximateFloat64() / allocatable.AsApproximateFloat64())
}

// FullyPriced tells whether TotalPrice is the real cost or a lower bound.
// Reserved nodes are prepaid and reported apart, they don't make it a bound.
func (g GroupStats) FullyPriced() bool {
	return g.PricedNodes+g.ReservedNodes == g.NumNodes
}

func newGroupStats(name string) GroupStats {
	return GroupStats{
		Name:                 name,
		AllocatableResources: v1.ResourceList{},
		UsedResources:        v1.ResourceList{},
	}
}

// add counts the node towards the group's totals.
func (g *GroupStats) add(n *Node) {
	g.NumNodes++
	g.NumPods += n.NumPods()
	n.AddAllocatableTo(g.AllocatableResources)
	n.AddUsedTo(g.UsedResources)
	if n.HasActualPrice() {
		g.TotalPrice += n.Price()
		g.PricedNodes++
	} else if n.PriceIsEquivalent() {
		g.ReservedNodes++
	}
}

// GroupNodes aggregates nodes by their value for the label, sorted by name with
// the nodes missing the label last.
func GroupNodes(nodes []*Node, label string) []GroupStats {
	byValue := map[string]*GroupStats{}
	for _, n := range nodes {
		value := n.LabelValue(label)
		group, ok := byValue[value]
		if !ok {
			g := newGroupStats(value)
			group = &g
			byValue[value] = group
		}
		group.add(n)
	}

	groups := make([]GroupStats, 0, len(byValue))
	for _, g := range byValue {
		groups = append(groups, *g)
	}
	sortGroups(groups, func(g GroupStats) string { return g.Name })
	return groups
}
