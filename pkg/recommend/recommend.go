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

// Package recommend works out cost recommendations for a set of nodes with
// fixed rules and arithmetic (specs/recommendations, specs/aks-recommendations):
// no AI, no I/O, no Kubernetes, AWS or Azure types. The same input always
// gives the same result.
package recommend

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// HoursPerMonth turns an hourly price into a monthly one.
const HoursPerMonth = 730

// Resources are cpu cores, memory GiB and a pod count.
type Resources struct {
	CPU, Mem, Pods float64
}

func (r Resources) add(o Resources) Resources {
	return Resources{r.CPU + o.CPU, r.Mem + o.Mem, r.Pods + o.Pods}
}

// Pod is a pod's requests and whether it stays on its node (DaemonSet or
// static pod).
type Pod struct {
	Req   Resources
	Fixed bool
}

// Capacity types, as the model names them.
const (
	OnDemand = "OnDemand"
	Spot     = "Spot"
	Reserved = "Reserved"
	Fargate  = "Fargate"
)

// Clouds, as the provider-ID scheme names them.
const (
	CloudAWS   = "aws"
	CloudAzure = "azure"
)

// Node is what the rules know about a node.
type Node struct {
	Name, Type, Zone, Capacity string
	// Cloud is the provider-ID scheme; empty counts as AWS. Region is the
	// node's region label (Azure prices are per region). Subscription is an
	// Azure node's subscription, from its provider ID: never shown
	// (azure-live-skus R1, R8)
	Cloud, Region, Subscription string
	// Auto marks EKS Auto Mode nodes
	Auto bool
	// Windows marks a Windows node; System an AKS agent pool of mode
	// system; VMPool an AKS Virtual Machines pool (several sizes); Virtual an
	// ACI virtual node (type=virtual-kubelet)
	Windows, System, VMPool, Virtual bool
	// NodePool is the Karpenter NodePool; ManagedGroup the EKS managed node
	// group or the AKS agent pool
	NodePool, ManagedGroup string
	Allocatable            Resources
	// NodeCapacity is the node's status.capacity (aks-recommendations D4)
	NodeCapacity Resources
	// Price is the node's hourly price when PriceKnown; Equivalent marks a
	// reserved node's on-demand equivalent; Override a price from the
	// instance-price label
	Price                            float64
	PriceKnown, Equivalent, Override bool
	Created                          time.Time
	Pods                             []Pod
}

func (n Node) requested() Resources {
	var r Resources
	for _, p := range n.Pods {
		r = r.add(p.Req)
	}
	return r
}

func (n Node) fixed() Resources {
	var r Resources
	for _, p := range n.Pods {
		if p.Fixed {
			r = r.add(p.Req)
		}
	}
	return r
}

// Spec is an instance type from the catalog.
type Spec struct {
	VCPU, MemGiB float64
	Arch, Class  string
	Current      bool
	// Family is the Azure family (normalised SkuFamily); empty for AWS,
	// whose family is the part of the type before the dot
	Family string
	// Constrained marks an Azure constrained-vCPU size: never a candidate
	// or a target
	Constrained bool
	// TempDiskMB is an Azure size's temp disk (MaxResourceVolumeMB), 0 for
	// none (azure-live-skus R9)
	TempDiskMB float64
}

// Pair is a subscription and region of shown Azure nodes, with their pools,
// whose VM sizes the Azure advisor reads (azure-live-skus R1, R5).
type Pair struct {
	Subscription, Region  string
	AgentPools, NodePools []string
}

// Input is everything the rules use.
type Input struct {
	Nodes       []Node
	PendingPods int
	Now         time.Time
	// Headroom is the highest share of allocatable a node may reach after a
	// change, e.g. 0.8
	Headroom float64
	// OldAge is the age past which a node should be recycled
	OldAge time.Duration
	// Spec, OnDemand and SpotIn answer from the catalog and prices, for a
	// size in the market of node n: SpotIn is the price in n's zone on AWS,
	// in n's region on Azure; both follow n's region and OS
	Spec     func(instanceType string) (Spec, bool)
	OnDemand func(n Node, instanceType string) (float64, bool)
	SpotIn   func(n Node, instanceType string) (float64, bool)
	// Available tells whether a size can run on node n (azure-live-skus R3);
	// nil means every size can, as on AWS
	Available func(n Node, instanceType string) bool
	// SizesLoaded tells whether n's Azure sizes are read; nil means they are
	// (azure-live-skus R3: Spot needs none while they are not)
	SizesLoaded func(n Node) bool
	// Types lists the catalog's instance types, the rightsize candidates
	Types []string
	// Notes, when set, adds the advisor's notes (prices awaited, pricing
	// disabled) to the result
	Notes func() []string
}

// Kind is a kind of recommendation, in the order the screen lists them.
type Kind int

const (
	Idle Kind = iota
	Consolidate
	Rightsize
	Generation
	Graviton
	OldNode
	SpotMove
)

func (k Kind) String() string {
	return [...]string{"idle", "consolidate", "rightsize", "generation", "graviton", "old node", "spot"}[k]
}

// Rec is one recommendation.
type Rec struct {
	Kind   Kind
	Target string
	Action string
	// Monthly saving; Priced is false when it can't be worked out (shown
	// "—"); Info rows inform and save nothing
	Monthly float64
	Priced  bool
	Info    bool
	Reason  string
	Math    []string
	// Cloud is the cloud of the nodes the row is about
	Cloud string
	// perNode is each node's share of the saving, for the total
	perNode map[string]float64
}

// Label is the kind's word on the screen: the arm64 swap is "graviton" on
// AWS rows and "arm" on Azure rows (aks-recommendations D6).
func (r Rec) Label() string {
	if r.Kind == Graviton && r.Cloud == CloudAzure {
		return "arm"
	}
	return r.Kind.String()
}

// Result is the recommendations, ordered, and the total saving (R14).
type Result struct {
	Recs  []Rec
	Total float64
	Notes []string
}

// Analyze applies every rule to the nodes.
func Analyze(in Input) Result {
	if in.Headroom <= 0 || in.Headroom > 1 {
		in.Headroom = 0.8
	}
	if in.OldAge <= 0 {
		in.OldAge = 30 * 24 * time.Hour
	}
	var res Result
	res.Recs = append(res.Recs, idle(in)...)
	if in.PendingPods > 0 {
		res.Notes = append(res.Notes, fmt.Sprintf("%d pods pending: no consolidation or rightsizing until they are scheduled", in.PendingPods))
	} else {
		res.Recs = append(res.Recs, consolidate(in)...)
		res.Recs = append(res.Recs, rightsize(in)...)
	}
	res.Recs = append(res.Recs, swapFamily(in, Generation)...)
	res.Recs = append(res.Recs, swapFamily(in, Graviton)...)
	res.Recs = append(res.Recs, oldNodes(in)...)
	res.Recs = append(res.Recs, spot(in)...)
	if in.Notes != nil {
		res.Notes = append(res.Notes, in.Notes()...)
	}
	sort.SliceStable(res.Recs, func(a, b int) bool {
		ra, rb := res.Recs[a], res.Recs[b]
		if ra.Kind != rb.Kind {
			return ra.Kind < rb.Kind
		}
		if ra.Priced != rb.Priced {
			return ra.Priced
		}
		return ra.Monthly > rb.Monthly
	})
	res.Total = total(res.Recs)
	return res
}

// total counts each node once with its best non-Spot saving (R14).
func total(recs []Rec) float64 {
	best := map[string]float64{}
	for _, r := range recs {
		if r.Info || !r.Priced || r.Kind == SpotMove {
			continue
		}
		for node, s := range r.perNode {
			best[node] = math.Max(best[node], s)
		}
	}
	sum := 0.0
	for _, s := range best {
		sum += s
	}
	return math.Round(sum*100) / 100
}

// comparable tells whether catalog-based advice applies to the node
// (recommendations R15, aks-recommendations R12).
func comparableSpec(in Input, n Node) (Spec, bool) {
	if n.Capacity == Fargate || n.Auto || n.Virtual || n.Type == "" {
		return Spec{}, false
	}
	return in.Spec(n.Type)
}

func isAzure(n Node) bool { return n.Cloud == CloudAzure }

// available tells whether size t can run on n (azure-live-skus R3).
func available(in Input, n Node, t string) bool {
	return in.Available == nil || in.Available(n, t)
}

// sizesLoaded tells whether n's sizes are read (azure-live-skus R3).
func sizesLoaded(in Input, n Node) bool {
	return in.SizesLoaded == nil || in.SizesLoaded(n)
}

// userSizeOK tells whether an AKS user pool may use the size: at least 2
// vCPU and 2 GiB (azure-live-skus R11).
func userSizeOK(s Spec) bool { return s.VCPU >= 2 && s.MemGiB >= 2 }

// keepsTempDisk tells whether a target keeps the node's temp disk, read from
// the live sizes (azure-live-skus R9).
func keepsTempDisk(node, target Spec) bool { return node.TempDiskMB <= 0 || target.TempDiskMB > 0 }

// liveSizes is the math line naming the source of Azure sizes
// (azure-live-skus R8).
const liveSizes = "sizes: Azure Resource SKUs API"

// groupOf names the pool or group a node belongs to, for targets (R13); the
// key carries the cloud, so groups never mix clouds (aks-recommendations R14).
func groupOf(n Node) (key, label string) {
	switch {
	case n.NodePool != "":
		return n.Cloud + "/np/" + n.NodePool, "NodePool " + n.NodePool
	case n.ManagedGroup != "" && isAzure(n):
		return n.Cloud + "/ng/" + n.ManagedGroup, "agent pool " + n.ManagedGroup
	case n.ManagedGroup != "":
		return n.Cloud + "/ng/" + n.ManagedGroup, "node group " + n.ManagedGroup
	}
	return n.Cloud + "/node/" + n.Name, n.Name
}

func splitType(t string) (family, size string) {
	family, size, _ = strings.Cut(t, ".")
	return family, size
}

// azureType tells an Azure size name; AWS types never start this way.
func azureType(t string) bool { return strings.HasPrefix(t, "Standard_") }

// familyKey is the family a type belongs to, keyed per cloud.
func familyKey(t string, s Spec) string {
	if azureType(t) {
		return "azure/" + s.Family
	}
	f, _ := splitType(t)
	return f
}

// byFamily indexes the catalog's types by family once per analysis.
func byFamily(in Input) map[string][]string {
	out := map[string][]string{}
	for _, t := range in.Types {
		s, ok := in.Spec(t)
		if !ok {
			continue
		}
		k := familyKey(t, s)
		out[k] = append(out[k], t)
	}
	return out
}

// counterpart is the Azure size of family with the same vCPU count and no
// less memory, the smallest such, or "" (aks-recommendations R8, R9).
func counterpart(in Input, idx map[string][]string, family string, spec Spec) string {
	best, bestMem := "", math.Inf(1)
	for _, t := range idx["azure/"+family] {
		s, _ := in.Spec(t)
		if s.Constrained || s.VCPU != spec.VCPU || s.MemGiB < spec.MemGiB {
			continue
		}
		if s.MemGiB < bestMem || (s.MemGiB == bestMem && t < best) {
			best, bestMem = t, s.MemGiB
		}
	}
	return best
}

// systemSizeOK tells whether an AKS system pool may use the size: at least 4
// vCPU and 4 GiB, not the B series, room for 30 pods (aks-recommendations R15).
func systemSizeOK(s Spec, pods float64) bool {
	return s.VCPU >= 4 && s.MemGiB >= 4 && !azureBSeries[s.Family] && pods >= 30
}

// poolNote says how an agent pool takes a new size of the same architecture
// (aks-recommendations R7, D7).
func poolNote(n Node, size string) string {
	if n.VMPool {
		return "the pool is a Virtual Machines pool: add " + size + " to its sizes"
	}
	return "the pool has one VM size: resize it in place (preview) only if every node fits, else move the workloads to a pool of that size"
}

func money(v float64) string { return fmt.Sprintf("$%.2f", v) }

func pct(used, alloc float64) float64 {
	if alloc <= 0 {
		return math.Inf(1)
	}
	return used / alloc
}

// idle lists each pool's or group's idle cost as information (R5).
func idle(in Input) []Rec {
	type acc struct {
		label string
		cost  float64
		nodes int
	}
	groups := map[string]*acc{}
	var order []string
	for _, n := range in.Nodes {
		if !n.PriceKnown || n.Equivalent || n.Capacity == Fargate {
			continue
		}
		r := n.requested()
		use := math.Max(pct(r.CPU, n.Allocatable.CPU), pct(r.Mem, n.Allocatable.Mem))
		cost := n.Price * math.Max(0, 1-math.Min(use, 1)) * HoursPerMonth
		key, label := groupOf(n)
		if _, ok := groups[key]; !ok {
			groups[key] = &acc{label: label}
			order = append(order, key)
		}
		groups[key].cost += cost
		groups[key].nodes++
	}
	var out []Rec
	for _, k := range order {
		g := groups[k]
		if g.cost < 0.01 {
			continue
		}
		out = append(out, Rec{Kind: Idle, Target: fmt.Sprintf("%s (%d nodes)", g.label, g.nodes), Info: true, Monthly: g.cost, Priced: true,
			Action: money(g.cost) + "/mo requested by nothing",
			Reason: "price × (1 − the larger of cpu and memory use), by requests"})
	}
	return out
}

// consolidate packs each managed node group's movable pods into as few nodes
// of its type as fit at the headroom (R6). Karpenter pools are information.
func consolidate(in Input) []Rec {
	type group struct {
		label     string
		karpenter bool
		nodes     []Node
	}
	groups := map[string]*group{}
	var order []string
	for _, n := range in.Nodes {
		if n.Capacity == Fargate || n.Auto || n.Virtual || (n.ManagedGroup == "" && n.NodePool == "") {
			continue
		}
		gk, label := groupOf(n)
		key := gk + "|" + n.Type
		if _, ok := groups[key]; !ok {
			groups[key] = &group{label: label, karpenter: n.NodePool != ""}
			order = append(order, key)
		}
		groups[key].nodes = append(groups[key].nodes, n)
	}
	var out []Rec
	for _, key := range order {
		g := groups[key]
		if len(g.nodes) < 2 {
			continue
		}
		need, steps := pack(g.nodes, in.Headroom)
		first := g.nodes[0]
		system := isAzure(first) && first.System
		if system && need < 2 {
			// AKS requires at least two nodes in a system pool (R6)
			need = 2
			steps = append(steps, "a system pool keeps at least 2 nodes; AKS recommends three")
		}
		if need >= len(g.nodes) {
			continue
		}
		removed := len(g.nodes) - need
		typ := first.Type
		r := Rec{Kind: Consolidate, Target: fmt.Sprintf("%s (%s ×%d)", g.label, typ, len(g.nodes)),
			Action: fmt.Sprintf("%d nodes → %d (pods fit at ≤%.0f%%)", len(g.nodes), need, in.Headroom*100),
			Reason: "first-fit decreasing by requests only (affinity, taints, spread, budgets and volumes not considered)",
			Math:   steps, Cloud: first.Cloud}
		if isAzure(first) && !g.karpenter {
			r.Action = fmt.Sprintf("scale %s to %d nodes (or lower its autoscaler minimum)", g.label, need)
		}
		if g.karpenter {
			r.Info = true
			r.Action = fmt.Sprintf("Karpenter consolidation should reach %d nodes; check the NodePool's consolidationPolicy", need)
			out = append(out, r)
			continue
		}
		// the least used nodes are the ones to remove
		nodes := append([]Node(nil), g.nodes...)
		sort.SliceStable(nodes, func(a, b int) bool {
			ra, rb := nodes[a].requested(), nodes[b].requested()
			return ra.CPU/nodes[a].Allocatable.CPU+ra.Mem/nodes[a].Allocatable.Mem < rb.CPU/nodes[b].Allocatable.CPU+rb.Mem/nodes[b].Allocatable.Mem
		})
		r.perNode = map[string]float64{}
		r.Priced = true
		for _, n := range nodes[:removed] {
			if !n.PriceKnown || n.Equivalent {
				r.Priced = false
				continue
			}
			s := n.Price * HoursPerMonth
			r.perNode[n.Name] = s
			r.Monthly += s
		}
		if !r.Priced {
			r.Monthly, r.perNode = 0, nil
		}
		r.Math = append(r.Math, fmt.Sprintf("saving: %d removed nodes × their price × %d h = %s", removed, HoursPerMonth, money(r.Monthly)))
		out = append(out, r)
	}
	return out
}

// pack returns how many nodes of the group's smallest allocatable the
// movable pods need at the headroom, each node carrying the group's largest
// per-node sum of fixed pods.
func pack(nodes []Node, headroom float64) (int, []string) {
	alloc := nodes[0].Allocatable
	var overhead Resources
	var pods []Pod
	for _, n := range nodes {
		if n.Allocatable.CPU < alloc.CPU {
			alloc.CPU = n.Allocatable.CPU
		}
		if n.Allocatable.Mem < alloc.Mem {
			alloc.Mem = n.Allocatable.Mem
		}
		if n.Allocatable.Pods < alloc.Pods || alloc.Pods == 0 {
			alloc.Pods = n.Allocatable.Pods
		}
		f := n.fixed()
		overhead.CPU = math.Max(overhead.CPU, f.CPU)
		overhead.Mem = math.Max(overhead.Mem, f.Mem)
		overhead.Pods = math.Max(overhead.Pods, f.Pods)
		for _, p := range n.Pods {
			if !p.Fixed {
				pods = append(pods, p)
			}
		}
	}
	room := Resources{alloc.CPU*headroom - overhead.CPU, alloc.Mem*headroom - overhead.Mem, alloc.Pods - overhead.Pods}
	weight := func(p Pod) float64 { return math.Max(p.Req.CPU/alloc.CPU, p.Req.Mem/alloc.Mem) }
	sort.SliceStable(pods, func(a, b int) bool { return weight(pods[a]) > weight(pods[b]) })
	var bins []Resources
	for _, p := range pods {
		placed := false
		for i := range bins {
			next := bins[i].add(p.Req)
			if next.CPU <= room.CPU+1e-9 && next.Mem <= room.Mem+1e-9 && (room.Pods <= 0 || next.Pods <= room.Pods) {
				bins[i], placed = next, true
				break
			}
		}
		if !placed {
			bins = append(bins, p.Req)
		}
	}
	need := max(len(bins), 1)
	return need, []string{
		fmt.Sprintf("%d movable pods; per node %.2f vCPU / %.2f GiB allocatable, %.0f%% headroom", len(pods), alloc.CPU, alloc.Mem, headroom*100),
		fmt.Sprintf("DaemonSet and static pods kept on every node: %.2f vCPU / %.2f GiB", overhead.CPU, overhead.Mem),
		fmt.Sprintf("first-fit decreasing needs %d nodes", need),
	}
}

// allocatableFor estimates a candidate type's allocatable (R7): a node of
// that type in the cluster, else catalog capacity × the family's
// allocatable/capacity ratio in the cluster, else 0.90 cpu / 0.85 memory.
func allocatableFor(in Input, typ string, spec Spec) (Resources, string) {
	for _, n := range in.Nodes {
		if n.Type == typ {
			return n.Allocatable, "allocatable of this cluster's " + typ + " nodes"
		}
	}
	family, _ := splitType(typ)
	var rc, rm float64
	count := 0
	for _, n := range in.Nodes {
		f, _ := splitType(n.Type)
		if f != family {
			continue
		}
		s, ok := in.Spec(n.Type)
		if !ok || s.VCPU == 0 || s.MemGiB == 0 {
			continue
		}
		rc += n.Allocatable.CPU / s.VCPU
		rm += n.Allocatable.Mem / s.MemGiB
		count++
	}
	if count > 0 {
		return Resources{CPU: spec.VCPU * rc / float64(count), Mem: spec.MemGiB * rm / float64(count)},
			"capacity × allocatable ratio of this cluster's " + family + " nodes"
	}
	return Resources{CPU: spec.VCPU * 0.90, Mem: spec.MemGiB * 0.85}, "capacity × 0.90 cpu / 0.85 memory (no " + family + " node to measure)"
}

// azureAllocatable estimates an Azure candidate's allocatable
// (aks-recommendations R7, D4): a node of that size in the cluster, else the
// candidate's catalog capacity minus node n's reservation, the larger of
// catalog − allocatable and status.capacity − allocatable. The estimate may
// understate; ok is false when it is 0 or less in any resource.
func azureAllocatable(in Input, n Node, nspec Spec, typ string, spec Spec) (alloc Resources, how string, ok bool) {
	for _, o := range in.Nodes {
		if o.Type == typ {
			return o.Allocatable, "allocatable of this cluster's " + typ + " nodes", true
		}
	}
	resCPU := math.Max(nspec.VCPU-n.Allocatable.CPU, n.NodeCapacity.CPU-n.Allocatable.CPU)
	resMem := math.Max(nspec.MemGiB-n.Allocatable.Mem, n.NodeCapacity.Mem-n.Allocatable.Mem)
	alloc = Resources{CPU: spec.VCPU - resCPU, Mem: spec.MemGiB - resMem}
	return alloc, "catalog capacity minus this node's reservation; may understate", alloc.CPU > 0 && alloc.Mem > 0
}

// listPrice is the price a node of the type would pay in its market: the
// on-demand list price, or the spot price in its zone (its region on Azure).
func listPrice(in Input, n Node, typ string) (float64, bool) {
	switch n.Capacity {
	case OnDemand:
		return in.OnDemand(n, typ)
	case Spot:
		return in.SpotIn(n, typ)
	}
	return 0, false
}

// currentPrice is what the node pays now: its own price, override included.
func currentPrice(n Node) (float64, bool, string) {
	if n.PriceKnown && !n.Equivalent {
		if n.Override {
			return n.Price, true, " (instance-price label)"
		}
		return n.Price, true, ""
	}
	return 0, false, ""
}

// rightsize recommends the cheapest smaller size of the same family and
// architecture the node's requests fit at the headroom (recommendations R7,
// aks-recommendations R7).
func rightsize(in Input) []Rec {
	// candidates come from the node's family only: index the catalog once
	idx := byFamily(in)
	var out []Rec
	for _, n := range in.Nodes {
		spec, ok := comparableSpec(in, n)
		if !ok || n.Capacity == Reserved {
			continue
		}
		cur, ok, note := currentPrice(n)
		if !ok {
			continue
		}
		azure := isAzure(n)
		req := n.requested()
		type cand struct {
			typ   string
			price float64
		}
		var cands []cand
		for _, t := range idx[familyKey(n.Type, spec)] {
			if t == n.Type {
				continue
			}
			s, ok := in.Spec(t)
			if !ok || s.Arch != spec.Arch || s.Constrained || (s.VCPU >= spec.VCPU && s.MemGiB >= spec.MemGiB) {
				continue
			}
			if azure && (!available(in, n, t) || (!n.System && !userSizeOK(s))) {
				continue
			}
			if p, ok := listPrice(in, n, t); ok && p < cur {
				cands = append(cands, cand{t, p})
			}
		}
		sort.Slice(cands, func(a, b int) bool {
			if cands[a].price != cands[b].price {
				return cands[a].price < cands[b].price
			}
			return cands[a].typ < cands[b].typ
		})
		var rejected []string
		for _, c := range cands {
			s, _ := in.Spec(c.typ)
			var alloc Resources
			var how string
			fits := true
			if azure {
				alloc, how, fits = azureAllocatable(in, n, spec, c.typ, s)
			} else {
				alloc, how = allocatableFor(in, c.typ, s)
			}
			podsNote := "pod capacity not checked (no node of this type)"
			podsOK := true
			podCap := math.NaN()
			if azure {
				// the pool's max-pods setting (R7)
				podCap, podsNote = n.Allocatable.Pods, fmt.Sprintf("pods %0.f of %0.f (this node's max-pods)", req.Pods, n.Allocatable.Pods)
			}
			for _, o := range in.Nodes {
				if o.Type == c.typ {
					podCap = o.Allocatable.Pods
					podsNote = fmt.Sprintf("pods %0.f of %0.f", req.Pods, o.Allocatable.Pods)
					if azure {
						podsNote += " (a " + c.typ + " node's)"
					}
					break
				}
			}
			if !math.IsNaN(podCap) {
				podsOK = req.Pods <= podCap
			}
			cpu, mem := math.Inf(1), math.Inf(1)
			if fits {
				cpu, mem = req.CPU/alloc.CPU, req.Mem/alloc.Mem
			}
			system := azure && n.System && !systemSizeOK(s, podCap)
			if !fits || cpu > in.Headroom || mem > in.Headroom || !podsOK || system {
				if azure {
					why := fmt.Sprintf("≈ %.2f vCPU / %.2f GiB (%s)", alloc.CPU, alloc.Mem, how)
					if system {
						why = "below a system pool's limits (4 vCPU, 4 GiB, no B series, 30 pods)"
					}
					rejected = append(rejected, fmt.Sprintf("%s doesn't fit: %s", c.typ, why))
				}
				continue
			}
			saving := (cur - c.price) * HoursPerMonth
			lines := []string{
				fmt.Sprintf("%s: %s → %s", n.Name, n.Type, c.typ),
				fmt.Sprintf("requests %.2f vCPU / %.2f GiB (DaemonSets %.2f / %.2f included)", req.CPU, req.Mem, n.fixed().CPU, n.fixed().Mem),
				fmt.Sprintf("%s allocatable ≈ %.2f vCPU / %.2f GiB (%s)", c.typ, alloc.CPU, alloc.Mem, how),
				fmt.Sprintf("after the move: cpu %.0f%%, memory %.0f%% ≤ %.0f%%; %s", cpu*100, mem*100, in.Headroom*100, podsNote),
			}
			if azure {
				lines = append(lines, liveSizes)
				lines = append(lines, rejected...)
				lines = append(lines, poolNote(n, c.typ))
			}
			lines = append(lines, fmt.Sprintf("saving: (%s%s − %s) × %d h = %s", money4(cur), note, money4(c.price), HoursPerMonth, money(saving)))
			out = append(out, Rec{Kind: Rightsize, Target: fmt.Sprintf("%s (%s)", n.Name, n.Type),
				Action:  fmt.Sprintf("→ %s (≈%.0f%% cpu, %.0f%% mem)", c.typ, cpu*100, mem*100),
				Monthly: saving, Priced: true, perNode: map[string]float64{n.Name: saving}, Cloud: n.Cloud,
				Reason: "requests fit a smaller size at the headroom", Math: lines})
			break
		}
	}
	return out
}

func money4(v float64) string { return fmt.Sprintf("$%.4f", v) }

// sameAzurePrice is the tolerance under which two Azure prices are the same
// (aks-recommendations R8).
const sameAzurePrice = 0.0001

// swapTarget is the family swap for a node: the type, or "" for none.
func swapTarget(in Input, idx map[string][]string, n Node, spec Spec, kind Kind) string {
	if isAzure(n) {
		switch kind {
		case Generation:
			if t := azureGeneration[spec.Family]; t != "" {
				return counterpart(in, idx, t, spec)
			}
		case Graviton:
			// Linux, not a system pool (R9, R15)
			if spec.Arch == "x86_64" && !n.Windows && !n.System {
				if t := azureArm[spec.Family]; t != "" {
					return counterpart(in, idx, t, spec)
				}
			}
		}
		return ""
	}
	family, size := splitType(n.Type)
	var target string
	switch kind {
	case Generation:
		if !spec.Current || xenFamilies[family] {
			target = generationTarget[family]
		}
	case Graviton:
		if spec.Arch == "x86_64" {
			target = gravitonTarget[family]
		}
	}
	if target == "" {
		return ""
	}
	cand := target + "." + size
	if _, ok := in.Spec(cand); !ok {
		return ""
	}
	return cand
}

// swapAction is the action and reason of a swap row.
func swapAction(n Node, kind Kind, cand string, info bool) (action, reason string) {
	if !isAzure(n) {
		action = "→ " + cand
		if n.NodePool != "" {
			action = "allow " + cand + " in the NodePool"
		}
		reason = "newer generation, Nitro, same class and size"
		if kind == Graviton {
			reason = "Graviton (arm64): images must run on arm64"
			action += " (arm64 images!)"
		}
		return action, reason
	}
	if kind == Graviton {
		action = "add a " + cand + " agent pool (arm64 images!)"
		if n.NodePool != "" {
			action = "allow " + cand + " in the NodePool (arm64 images!)"
		}
		return action, "Arm64: images must run on arm64"
	}
	switch {
	case n.NodePool != "":
		action = "allow " + cand + " in the NodePool"
	case n.System:
		action = "add a system-mode pool of " + cand
	default:
		action = "→ " + cand
	}
	if info {
		action += ": same price, newer hardware"
	}
	return action, "newer generation, same class, features and temp disk"
}

// swapFamily recommends a newer generation or the arm64 counterpart, per
// pool or group and type (recommendations R8, R9; aks-recommendations R8, R9).
func swapFamily(in Input, kind Kind) []Rec {
	type acc struct {
		rec   Rec
		nodes int
	}
	idx := byFamily(in)
	groups := map[string]*acc{}
	var order []string
	for _, n := range in.Nodes {
		spec, ok := comparableSpec(in, n)
		if !ok || n.Capacity == Reserved {
			continue
		}
		cand := swapTarget(in, idx, n, spec, kind)
		if cand == "" {
			continue
		}
		if isAzure(n) {
			s, _ := in.Spec(cand)
			if !available(in, n, cand) || !keepsTempDisk(spec, s) {
				continue
			}
			if n.System && !systemSizeOK(s, n.Allocatable.Pods) || !n.System && !userSizeOK(s) {
				continue
			}
		}
		curList, ok1 := listPrice(in, n, n.Type)
		newList, ok2 := listPrice(in, n, cand)
		if !ok1 || !ok2 {
			continue
		}
		// on Azure a generation at the same price is information (R8)
		info := isAzure(n) && kind == Generation && math.Abs(newList-curList) <= sameAzurePrice
		if !info && newList >= curList {
			continue
		}
		saving := (curList - newList) * HoursPerMonth
		gk, label := groupOf(n)
		key := gk + "|" + n.Type
		g, ok := groups[key]
		if !ok {
			action, reason := swapAction(n, kind, cand, info)
			g = &acc{rec: Rec{Kind: kind, Action: action, Priced: true, Info: info, Reason: reason, Cloud: n.Cloud, perNode: map[string]float64{}}}
			groups[key] = g
			order = append(order, key)
			g.rec.Math = append(g.rec.Math, fmt.Sprintf("%s list %s/h → %s list %s/h", n.Type, money4(curList), cand, money4(newList)))
			if isAzure(n) {
				g.rec.Math = append(g.rec.Math, liveSizes)
				if kind == Graviton {
					g.rec.Math = append(g.rec.Math, "the architecture can't change in place: a new pool of that size")
				} else {
					g.rec.Math = append(g.rec.Math, poolNote(n, cand))
				}
			}
		}
		g.nodes++
		g.rec.Target = fmt.Sprintf("%s (%s ×%d)", label, n.Type, g.nodes)
		if info {
			continue
		}
		g.rec.Monthly += saving
		g.rec.perNode[n.Name] = saving
	}
	var out []Rec
	for _, k := range order {
		g := groups[k]
		if g.rec.Info {
			g.rec.perNode = nil
			g.rec.Math = append(g.rec.Math, "same price within $0.0001/h: newer hardware for nothing; not in the total")
		} else {
			g.rec.Math = append(g.rec.Math, fmt.Sprintf("saving: %d nodes × difference × %d h = %s", g.nodes, HoursPerMonth, money(g.rec.Monthly)))
		}
		out = append(out, g.rec)
	}
	return out
}

// oldNodes recommends recycling nodes past the age limit (recommendations
// R10, aks-recommendations R10).
func oldNodes(in Input) []Rec {
	var out []Rec
	for _, n := range in.Nodes {
		if n.Created.IsZero() || n.Capacity == Fargate || n.Virtual {
			continue
		}
		age := in.Now.Sub(n.Created)
		if age <= in.OldAge {
			continue
		}
		r := Rec{Kind: OldNode, Target: fmt.Sprintf("%s (%d days)", n.Name, int(age.Hours()/24)), Cloud: n.Cloud,
			Action: "recycle for a fresh AMI and patches", Reason: fmt.Sprintf("older than %d days", int(in.OldAge.Hours()/24))}
		if isAzure(n) {
			r.Action = "upgrade the node image"
			r.Math = []string{"az aks nodepool upgrade --node-image-only recycles the pool's nodes"}
		}
		out = append(out, r)
	}
	return out
}

// spot recommends moving On-Demand groups to Spot, priced in each node's own
// zone, or region on Azure (recommendations R11, aks-recommendations R11).
// The screen lists it last.
func spot(in Input) []Rec {
	type acc struct {
		rec    Rec
		nodes  int
		priced bool
	}
	groups := map[string]*acc{}
	var order []string
	for _, n := range in.Nodes {
		azure := isAzure(n)
		if n.Capacity != OnDemand {
			continue
		}
		if azure && !sizesLoaded(in, n) {
			// sizes loading or not readable: Spot needs only the type and
			// prices (azure-live-skus R3); the other guards stay
			if n.Auto || n.Virtual || n.Type == "" {
				continue
			}
		} else if _, ok := comparableSpec(in, n); !ok || (azure && !available(in, n, n.Type)) {
			continue
		}
		if azure && n.System {
			// a Spot pool is only ever a secondary pool (R11)
			continue
		}
		gk, label := groupOf(n)
		key := gk + "|" + n.Type
		g, ok := groups[key]
		if !ok {
			action, warn, reason := "use Spot", " (interruptions!)", "workloads must tolerate interruptions"
			switch {
			case n.NodePool != "":
				action = "allow spot in the NodePool"
			case n.ManagedGroup != "" && azure:
				action = "add a Spot agent pool alongside it"
			case n.ManagedGroup != "":
				action = "add a Spot node group alongside it"
			}
			if azure {
				warn, reason = " (evictions!)", "workloads must tolerate evictions"
			}
			g = &acc{rec: Rec{Kind: SpotMove, Action: action + warn, Reason: reason, Cloud: n.Cloud}, priced: true}
			groups[key] = g
			order = append(order, key)
		}
		g.nodes++
		g.rec.Target = fmt.Sprintf("%s (On-Demand %s ×%d)", label, n.Type, g.nodes)
		where := n.Zone
		if azure {
			where = n.Region + " (regional price)"
		}
		od, ok1 := in.OnDemand(n, n.Type)
		sp, ok2 := in.SpotIn(n, n.Type)
		if !ok1 || !ok2 || sp >= od {
			g.priced = false
			if azure {
				g.rec.Math = append(g.rec.Math, fmt.Sprintf("%s in %s: no spot price", n.Name, where))
			} else {
				g.rec.Math = append(g.rec.Math, fmt.Sprintf("%s in %s: no spot price for this zone", n.Name, where))
			}
			continue
		}
		g.rec.Monthly += (od - sp) * HoursPerMonth
		g.rec.Math = append(g.rec.Math, fmt.Sprintf("%s in %s: %s → %s/h", n.Name, where, money4(od), money4(sp)))
	}
	var out []Rec
	for _, k := range order {
		g := groups[k]
		if !g.priced && g.rec.Cloud == CloudAzure {
			// an Azure row needs every node's price, else it is absent
			// (aks-recommendations R3, R11)
			continue
		}
		g.rec.Priced = g.priced
		if !g.priced {
			g.rec.Monthly = 0
		}
		out = append(out, g.rec)
	}
	return out
}
