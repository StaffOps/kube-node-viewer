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
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync"
	"time"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/duration"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
)

var (
	instanceIDRegex = regexp.MustCompile(`aws:///(?P<AZ>.*)/(?P<InstanceID>.*)`)
)

type objectKey struct {
	namespace string
	name      string
}
type Node struct {
	mu          sync.RWMutex
	visible     bool
	node        v1.Node
	pods        map[objectKey]*Pod
	used        v1.ResourceList
	podsByPhase map[v1.PodPhase]int
	price       float64
	// priceIsEquivalent marks a price that is not what the node costs, the
	// on-demand price of a reserved node
	priceIsEquivalent     bool
	nodeclaimCreationTime time.Time
}

func NewNode(n *v1.Node) *Node {
	node := &Node{
		node:        *n,
		pods:        map[objectKey]*Pod{},
		used:        v1.ResourceList{},
		podsByPhase: map[v1.PodPhase]int{},
	}

	return node
}

func NewNodeFromNodeClaim(nc *karpv1.NodeClaim) *Node {
	node := NewNode(&v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:              nc.Status.NodeName,
			CreationTimestamp: nc.CreationTimestamp,
			Labels:            nc.Labels,
			Annotations:       nc.Annotations,
		},
		Spec: v1.NodeSpec{
			Taints:     nc.Spec.Taints,
			ProviderID: nc.Status.ProviderID,
		},
		Status: v1.NodeStatus{
			Capacity:    nc.Status.Capacity,
			Allocatable: nc.Status.Allocatable,
		},
	})
	node.nodeclaimCreationTime = nc.CreationTimestamp.Time
	return node
}

func (n *Node) IsOnDemand() bool {
	return n.node.Labels["karpenter.sh/capacity-type"] == "on-demand" ||
		n.node.Labels["eks.amazonaws.com/capacityType"] == "ON_DEMAND"
}

func (n *Node) IsSpot() bool {
	return n.node.Labels["karpenter.sh/capacity-type"] == "spot" ||
		n.node.Labels["eks.amazonaws.com/capacityType"] == "SPOT"
}

func (n *Node) IsFargate() bool {
	return n.node.Labels["eks.amazonaws.com/compute-type"] == "fargate"
}

func (n *Node) IsAuto() bool {
	return n.node.Labels["eks.amazonaws.com/compute-type"] == "auto"
}

// CapacityType is how a node is bought.
type CapacityType string

const (
	CapacityOnDemand CapacityType = "OnDemand"
	CapacitySpot     CapacityType = "Spot"
	// CapacityReserved covers Karpenter reserved capacity and EKS capacity
	// blocks: prepaid, so its price is only an on-demand equivalent.
	CapacityReserved CapacityType = "Reserved"
	CapacityFargate  CapacityType = "Fargate"
	CapacityUnknown  CapacityType = "Unknown"
)

// CapacityType reads the Karpenter and EKS capacity labels. EKS Auto Mode nodes
// carry the Karpenter label too, so they report spot or on-demand; IsAuto tells
// them apart.
func (n *Node) CapacityType() CapacityType {
	n.mu.RLock()
	defer n.mu.RUnlock()
	labels := n.node.Labels
	if labels["eks.amazonaws.com/compute-type"] == "fargate" {
		return CapacityFargate
	}
	switch labels["karpenter.sh/capacity-type"] {
	case "on-demand":
		return CapacityOnDemand
	case "spot":
		return CapacitySpot
	case "reserved":
		return CapacityReserved
	}
	switch labels["eks.amazonaws.com/capacityType"] {
	case "ON_DEMAND":
		return CapacityOnDemand
	case "SPOT":
		return CapacitySpot
	case "CAPACITY_BLOCK":
		return CapacityReserved
	}
	// AKS marks only Spot VMs; every other agent pool VM is regular pay-as-you-go
	if strings.HasPrefix(n.node.Spec.ProviderID, "azure://") {
		if labels["kubernetes.azure.com/scalesetpriority"] == "spot" {
			return CapacitySpot
		}
		return CapacityOnDemand
	}
	return CapacityUnknown
}

// NodeGroup is the Karpenter NodePool, the EKS managed node group or the AKS
// agent pool name, or "-". IsManagedNodeGroup tells which: the screen colours
// managed node groups and agent pools rather than prefixing them.
func (n *Node) NodeGroup() string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	if pool := n.node.Labels["karpenter.sh/nodepool"]; pool != "" {
		return pool
	}
	if group := n.node.Labels["eks.amazonaws.com/nodegroup"]; group != "" {
		return group
	}
	if pool := n.node.Labels["kubernetes.azure.com/agentpool"]; pool != "" {
		return pool
	}
	return "-"
}

// IsManagedNodeGroup tells whether NodeGroup is an EKS managed node group or an
// AKS agent pool rather than a Karpenter NodePool.
func (n *Node) IsManagedNodeGroup() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	l := n.node.Labels
	return l["karpenter.sh/nodepool"] == "" && (l["eks.amazonaws.com/nodegroup"] != "" || l["kubernetes.azure.com/agentpool"] != "")
}

// Cloud is the scheme of the node's provider ID ("aws", "azure", "gce",
// "kind"…), empty when the node has none.
func (n *Node) Cloud() string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	scheme, _, ok := strings.Cut(n.node.Spec.ProviderID, "://")
	if !ok {
		return ""
	}
	return scheme
}

// Region is the node's region label, empty when unset.
func (n *Node) Region() string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.node.Labels[v1.LabelTopologyRegion]
}

// InternalIP is the node's first internal address, "-" for a node known only
// from its NodeClaim.
func (n *Node) InternalIP() string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	for _, addr := range n.node.Status.Addresses {
		if addr.Type == v1.NodeInternalIP {
			return addr.Address
		}
	}
	return "-"
}

// Status flags a node can carry at the same time.
const (
	StatusReady     = "Ready"
	StatusNotReady  = "NotReady"
	StatusCordoned  = "Cordoned"
	StatusDeleting  = "Deleting"
	StatusLaunching = "Launching"
)

// StatusFlags lists the node's status: Ready or NotReady, then Cordoned,
// Deleting and Launching (a NodeClaim whose node hasn't registered) as they
// apply.
func (n *Node) StatusFlags() []string {
	flags := []string{StatusNotReady}
	if n.Ready() {
		flags[0] = StatusReady
	}
	if n.Cordoned() {
		flags = append(flags, StatusCordoned)
	}
	if n.Deleting() {
		flags = append(flags, StatusDeleting)
	}
	if n.RegisteredName() == "" {
		flags = append(flags, StatusLaunching)
	}
	return flags
}

func (n *Node) Labels() map[string]string {
	return n.node.Labels
}

func (n *Node) Update(node *v1.Node) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.node = *node
}

func (n *Node) Name() string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	if n.node.Name == "" {
		return n.InstanceID()
	}
	return n.node.Name
}

// RegisteredName is the node's name in the API server, empty for a node we only
// know from its NodeClaim. Unlike Name it doesn't fall back to the instance ID.
func (n *Node) RegisteredName() string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.node.Name
}

func (n *Node) ProviderID() string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.node.Spec.ProviderID
}

func (n *Node) InstanceID() string {
	providerID := n.ProviderID()
	matches := instanceIDRegex.FindStringSubmatch(providerID)
	if matches == nil {
		return providerID
	}
	for i, name := range instanceIDRegex.SubexpNames() {
		if name == "InstanceID" {
			return matches[i]
		}
	}
	return providerID
}

// BindPod counts the pod's requests and phase towards the node, replacing a
// pod bound before under the same key.
func (n *Node) BindPod(pod *Pod) {
	n.mu.Lock()
	defer n.mu.Unlock()
	key := pod.key()
	if old, ok := n.pods[key]; ok {
		n.unbind(old)
	}
	n.pods[key] = pod
	for rn, q := range pod.Requested() {
		existing := n.used[rn]
		existing.Add(q)
		n.used[rn] = existing
	}
	n.podsByPhase[pod.Phase()]++
}

func (n *Node) DeletePod(namespace string, name string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	key := objectKey{namespace: namespace, name: name}
	if p, ok := n.pods[key]; ok {
		n.unbind(p)
		delete(n.pods, key)
	}
}

// unbind takes the pod's requests and phase back out. Called with the lock held.
func (n *Node) unbind(p *Pod) {
	for rn, q := range p.Requested() {
		existing := n.used[rn]
		existing.Sub(q)
		n.used[rn] = existing
	}
	phase := p.Phase()
	if n.podsByPhase[phase]--; n.podsByPhase[phase] <= 0 {
		delete(n.podsByPhase, phase)
	}
}

// AddPodPhasesTo adds the node's pod count per phase to phases and returns the
// number of pods on the node.
func (n *Node) AddPodPhasesTo(phases map[v1.PodPhase]int) int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	for phase, count := range n.podsByPhase {
		phases[phase] += count
	}
	return len(n.pods)
}

func (n *Node) Allocatable() v1.ResourceList {
	n.mu.RLock()
	defer n.mu.RUnlock()
	// shouldn't be modified so it's safe to return
	return n.node.Status.Allocatable
}

func (n *Node) Used() v1.ResourceList {
	n.mu.RLock()
	defer n.mu.RUnlock()
	used := v1.ResourceList{}
	for rn, q := range n.used {
		used[rn] = q.DeepCopy()
	}
	return used
}

// AddUsedTo sums the node's pod requests into list, skipping the copy that Used
// has to make to protect our state.
func (n *Node) AddUsedTo(list v1.ResourceList) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	addResources(list, n.used)
}

// AddAllocatableTo sums the node's allocatable resources into list.
func (n *Node) AddAllocatableTo(list v1.ResourceList) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	addResources(list, n.node.Status.Allocatable)
}

func (n *Node) Cordoned() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	if n.node.Spec.Unschedulable {
		return true
	}
	for _, taint := range n.node.Spec.Taints {
		if taint.Key == "karpenter.sh/disruption" && taint.Effect == v1.TaintEffectNoSchedule {
			return true
		}
	}
	return false
}

func (n *Node) Ready() bool {
	ready := false
	n.mu.RLock()
	for _, c := range n.node.Status.Conditions {
		if c.Status == v1.ConditionTrue && c.Type == v1.NodeReady {
			ready = true
			break
		}
	}
	n.mu.RUnlock()
	// when the node goes ready, remove the nodeclaim creation ts, if any
	if ready {
		n.mu.Lock()
		n.nodeclaimCreationTime = time.Time{}
		n.mu.Unlock()
	}
	return ready
}

func (n *Node) Created() time.Time {
	n.mu.RLock()
	defer n.mu.RUnlock()
	if !n.nodeclaimCreationTime.IsZero() {
		return n.nodeclaimCreationTime
	}
	return n.node.CreationTimestamp.Time
}

func (n *Node) InstanceType() ec2types.InstanceType {
	n.mu.RLock()
	defer n.mu.RUnlock()
	if n.IsFargate() {
		if len(n.Pods()) == 1 {
			cpu, mem, ok := n.Pods()[0].FargateCapacityProvisioned()
			if ok {
				return ec2types.InstanceType(fmt.Sprintf("%gvCPU-%gGB", cpu, mem))
			}
		}
		return "Fargate"
	}
	return ec2types.InstanceType(n.node.Labels[v1.LabelInstanceTypeStable])
}

func (n *Node) Zone() string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	zone := n.node.Labels[v1.LabelTopologyZone]
	// AKS VMs outside availability zones carry their fault domain ("0", "1"…)
	// instead; zonal ones read "<region>-<n>"
	if strings.HasPrefix(n.node.Spec.ProviderID, "azure://") && !strings.Contains(zone, "-") {
		return ""
	}
	return zone
}

func (n *Node) NumPods() int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return len(n.pods)
}

func (n *Node) Hide() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.visible = false
}
func (n *Node) Visible() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.visible
}

func (n *Node) Show() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.visible = true
}

func (n *Node) Deleting() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return !n.node.DeletionTimestamp.IsZero()
}

// PodsCopy lists the pods bound to the node under its lock, for readers
// outside the node such as the recommendations.
func (n *Node) PodsCopy() []*Pod {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.Pods()
}

// Pods lists the pods bound to the node. Callers hold no lock: InstanceType
// reads it under the node's read lock.
func (n *Node) Pods() []*Pod {
	var pods []*Pod
	for _, p := range n.pods {
		pods = append(pods, p)
	}
	return pods
}

func (n *Node) HasPrice() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	// we use NaN for an unknown price, so if this is true the price is known
	return !math.IsNaN(n.price)
}

// PriceIsEquivalent tells whether Price is only an on-demand equivalent, as
// for reserved nodes, rather than what the node costs. Totals leave it out.
func (n *Node) PriceIsEquivalent() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.priceIsEquivalent
}

// HasActualPrice tells whether the node has a known price that counts towards
// totals.
func (n *Node) HasActualPrice() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return !math.IsNaN(n.price) && !n.priceIsEquivalent
}

// Price is the node's hourly price, NaN when unknown.
func (n *Node) Price() float64 {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.price
}

// LabelValue is the node's value for the label, falling back to a computed label
// and then to UngroupedValue.
func (n *Node) LabelValue(label string) string {
	n.mu.RLock()
	value, ok := n.node.Labels[label]
	n.mu.RUnlock()
	if ok && value != "" {
		return value
	}
	// ComputeLabel locks, so it can't be called while holding the lock
	if computed := n.ComputeLabel(label); computed != "-" {
		return computed
	}
	return UngroupedValue
}

// LabelPrefix prefixes the node labels and computed label names this tool
// understands.
const LabelPrefix = "kube-node-viewer/"

var resourceLabelRe = regexp.MustCompile("^kube-node-viewer/node-(.*?)-usage$")

// ComputeLabel computes dynamic labels
func (n *Node) ComputeLabel(labelName string) string {
	switch labelName {
	case LabelPrefix + "node-age":
		return duration.HumanDuration(time.Since(n.Created()))
	}
	// resource based custom labels
	if match := resourceLabelRe.FindStringSubmatch(labelName); len(match) > 0 {
		return pctUsage(n.Allocatable(), n.Used(), match[1])
	}
	return "-"
}

// NotReadyTime is the time that the node went NotReady, or when it was created if it hasn't been marked as NotReady.
func (n *Node) NotReadyTime() time.Time {
	n.mu.RLock()
	var notReadyTransitionTime time.Time
	for _, c := range n.node.Status.Conditions {
		if c.Type == v1.NodeReady && (c.Status == v1.ConditionFalse || c.Status == v1.ConditionUnknown) {
			notReadyTransitionTime = c.LastTransitionTime.Time
			break
		}
	}
	n.mu.RUnlock()
	if !notReadyTransitionTime.IsZero() {
		// if there's a nodeclaim creation ts, use it if the node has never been Ready before
		if !n.nodeclaimCreationTime.IsZero() {
			return n.nodeclaimCreationTime
		}
		return notReadyTransitionTime
	}
	return n.Created()
}

// SetPrice sets the hourly price, NaN for unknown. Prices are refreshed from
// the pricing goroutine while the view reads them, so it locks.
func (n *Node) SetPrice(price float64) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.price = price
	n.priceIsEquivalent = false
}

// SetEquivalentPrice sets a price that is only an on-demand equivalent.
func (n *Node) SetEquivalentPrice(price float64) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.price = price
	n.priceIsEquivalent = true
}

func pctUsage(allocatable v1.ResourceList, used v1.ResourceList, resource string) string {
	allocRes, hasAlloc := allocatable[v1.ResourceName(resource)]
	if !hasAlloc {
		return "N/A"
	}
	usedRes, hasUsed := used[v1.ResourceName(resource)]
	if !hasUsed || usedRes.AsApproximateFloat64() == 0 {
		return "0%"
	}
	pctUsed := 0.0
	if allocRes.AsApproximateFloat64() != 0 {
		pctUsed = 100 * (usedRes.AsApproximateFloat64() / allocRes.AsApproximateFloat64())
	}
	return fmt.Sprintf("%.0f%%", pctUsed)
}

// addLabelKeys adds the node's label keys to the set.
func (n *Node) addLabelKeys(keys map[string]struct{}) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	for k := range n.node.Labels {
		keys[k] = struct{}{}
	}
}
