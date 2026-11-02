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

package model

import (
	"fmt"
	"sort"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// openNodeDetails opens the details popup on the cursor's node (node-details
// R1); false when the cursor is not on a node.
func (u *UIModel) openNodeDetails() bool {
	name, ok := strings.CutPrefix(u.cursorKey, "n:")
	if !ok {
		return false
	}
	u.panel, u.detailsNode, u.detailsTop = panelNode, name, 0
	// the frame laid out to find the cursor is stale now
	u.dirty = true
	return true
}

func (u *UIModel) nodeDetailsKey(key string) {
	switch key {
	case "esc", "enter":
		u.panel = panelNone
	case "up", "k":
		u.detailsTop = max(u.detailsTop-1, 0)
	case "down", "j":
		u.detailsTop = min(u.detailsTop+1, max(u.detailsRows-1, 0))
	case "pgup":
		u.detailsTop = max(u.detailsTop-10, 0)
	case "pgdown", " ":
		u.detailsTop = min(u.detailsTop+10, max(u.detailsRows-1, 0))
	case "home":
		u.detailsTop = 0
	}
}

func (u *UIModel) findNode(name string) (*Node, bool) {
	for _, n := range u.stats.Nodes {
		if n.Name() == name {
			return n, true
		}
	}
	return nil, false
}

// nodeDetailsPanel is rebuilt each frame from the snapshot, so it follows the
// live node.
func (u *UIModel) nodeDetailsPanel() panelView {
	n, ok := u.findNode(u.detailsNode)
	if !ok {
		u.detailsRows = 1
		return panelView{title: u.detailsNode, rows: []string{u.style.yellow("This node is no longer in the cluster (or no longer shown).")}, cursor: -1}
	}
	rows := u.nodeDetailsRows(n, time.Now())
	u.detailsRows = len(rows)
	return panelView{title: n.Name(), rows: rows, cursor: -1, top: u.detailsTop}
}

func (u *UIModel) nodeDetailsRows(n *Node, now time.Time) []string {
	obj := n.Object()
	field := func(label, value string) string {
		return headerLabel(fmt.Sprintf("%-12s", label)) + " " + value
	}
	section := func(title string) string { return titleStyle(title) }

	instance := []string{orDash(string(n.InstanceType())), u.cell(n, FieldCapacity, now)}
	if !u.DisablePricing {
		price := u.cell(n, FieldPrice, now)
		if price != "-" {
			price += "/h"
		}
		instance = append(instance, price)
	}
	where := orDash(n.Zone())
	if r := n.Region(); r != "" {
		where += " (" + r + ")"
	}
	rows := []string{
		field("Instance", strings.Join(instance, " · ")),
		field("Node group", u.cell(n, FieldNodeGroup, now)),
		field("Zone", where),
		field("Status", u.cell(n, FieldStatus, now)+" · age "+u.cell(n, FieldAge, now)+" · created "+n.Created().Local().Format("2006-01-02 15:04")),
		field("Internal IP", n.InternalIP()),
		field("Provider ID", orDash(obj.Spec.ProviderID)),
	}
	if info := obj.Status.NodeInfo; info.KubeletVersion != "" {
		rows = append(rows,
			field("Kubelet", info.KubeletVersion),
			field("OS", fmt.Sprintf("%s · %s · %s", info.OSImage, info.KernelVersion, info.Architecture)),
			field("Runtime", info.ContainerRuntimeVersion),
		)
	}

	// resources: requested / allocatable / capacity
	used := n.Used()
	rows = append(rows, "", section("Resources")+helpStyle("  requested / allocatable / capacity"))
	// resources the node has none of (hugepages on most nodes) are noise
	names := map[v1.ResourceName]bool{}
	for _, list := range []v1.ResourceList{obj.Status.Allocatable, obj.Status.Capacity, used} {
		for r, q := range list {
			if !q.IsZero() {
				names[r] = true
			}
		}
	}
	ordered := resourceOrder(names)
	width := 0
	for _, r := range ordered {
		width = max(width, len(r))
	}
	for _, r := range ordered {
		alloc, capacity := obj.Status.Allocatable[r], obj.Status.Capacity[r]
		req, hasReq := used[r]
		if r == v1.ResourcePods {
			req, hasReq = *resource.NewQuantity(int64(n.NumPods()), resource.DecimalSI), true
		}
		reqText := "-"
		if hasReq {
			reqText = detailQuantity(r, req)
			if a := alloc.AsApproximateFloat64(); a > 0 {
				pct := 100 * req.AsApproximateFloat64() / a
				text := fmt.Sprintf("%.0f%%", pct)
				// usage colours tell waste on requests; a pod count isn't one
				if r != v1.ResourcePods {
					text = u.colorizeUsage(pct, text)
				}
				reqText += " (" + text + ")"
			}
		}
		rows = append(rows, fmt.Sprintf("  %-*s  %s / %s / %s", width, r, reqText, detailQuantity(r, alloc), detailQuantity(r, capacity)))
	}

	// conditions
	if len(obj.Status.Conditions) > 0 {
		rows = append(rows, "", section("Conditions"))
		width := 0
		for _, c := range obj.Status.Conditions {
			width = max(width, len(c.Type))
		}
		for _, c := range obj.Status.Conditions {
			status := string(c.Status)
			if conditionBad(c) {
				status = u.style.red(status)
			} else {
				status = u.style.green(status)
			}
			line := fmt.Sprintf("  %-*s  %s", width, c.Type, pad(status, 7))
			if c.Reason != "" {
				line += "  " + helpStyle(c.Reason)
			}
			if conditionBad(c) && c.Message != "" {
				line += "  " + helpStyle(c.Message)
			}
			rows = append(rows, line)
		}
	}

	// taints
	if len(obj.Spec.Taints) == 0 {
		rows = append(rows, "", field("Taints", helpStyle("none")))
	} else {
		rows = append(rows, "", section("Taints"))
		for _, t := range obj.Spec.Taints {
			taint := t.Key
			if t.Value != "" {
				taint += "=" + t.Value
			}
			rows = append(rows, "  "+u.style.yellow(taint+":"+string(t.Effect)))
		}
	}

	// labels
	keys := make([]string, 0, len(obj.Labels))
	for k := range obj.Labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	rows = append(rows, "", section(fmt.Sprintf("Labels (%d)", len(keys))))
	for _, k := range keys {
		rows = append(rows, "  "+k+helpStyle("=")+obj.Labels[k])
	}
	return rows
}

// resourceOrder puts cpu, memory, pods and ephemeral storage first, then the
// rest (extended resources such as nvidia.com/gpu, hugepages) by name.
func resourceOrder(names map[v1.ResourceName]bool) []v1.ResourceName {
	first := []v1.ResourceName{v1.ResourceCPU, v1.ResourceMemory, v1.ResourcePods, v1.ResourceEphemeralStorage}
	var out, rest []v1.ResourceName
	for _, r := range first {
		if names[r] {
			out = append(out, r)
		}
	}
	for r := range names {
		switch r {
		case v1.ResourceCPU, v1.ResourceMemory, v1.ResourcePods, v1.ResourceEphemeralStorage:
		default:
			rest = append(rest, r)
		}
	}
	sort.Slice(rest, func(i, j int) bool { return rest[i] < rest[j] })
	return append(out, rest...)
}

// detailQuantity shows bytes in Gi with one decimal, other quantities as
// Kubernetes writes them.
func detailQuantity(name v1.ResourceName, q resource.Quantity) string {
	if q.IsZero() && q.Format == "" {
		return "-"
	}
	if isByteValued(name, q) {
		return fmt.Sprintf("%.1fGi", q.AsApproximateFloat64()/(1<<30))
	}
	return q.String()
}

// conditionBad tells whether a node condition is in its bad state: Ready
// not True, or a pressure / unavailable condition True.
func conditionBad(c v1.NodeCondition) bool {
	if c.Type == v1.NodeReady {
		return c.Status != v1.ConditionTrue
	}
	return c.Status == v1.ConditionTrue
}
