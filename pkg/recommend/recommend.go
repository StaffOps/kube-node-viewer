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
// fixed rules and arithmetic (specs/recommendations): no AI, no I/O, no
// Kubernetes or AWS types. The same input always gives the same result.
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

// Node is what the rules know about a node.
type Node struct {
	Name, Type, Zone, Capacity string
	// Auto marks EKS Auto Mode nodes
	Auto bool
	// NodePool is the Karpenter NodePool; ManagedGroup the EKS managed node group
	NodePool, ManagedGroup string
	Allocatable            Resources
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
	// Spec, OnDemand and SpotIn answer from the catalog and prices; SpotIn is
	// the price in that very zone
	Spec     func(instanceType string) (Spec, bool)
	OnDemand func(instanceType string) (float64, bool)
	SpotIn   func(instanceType, zone string) (float64, bool)
	// Types lists the catalog's instance types, the rightsize candidates
	Types []string
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
	// perNode is each node's share of the saving, for the total
	perNode map[string]float64
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

// comparable tells whether catalog-based advice applies to the node (R15).
func comparableSpec(in Input, n Node) (Spec, bool) {
	if n.Capacity == Fargate || n.Auto || n.Type == "" {
		return Spec{}, false
	}
	return in.Spec(n.Type)
}

// groupOf names the pool or group a node belongs to, for targets (R13).
func groupOf(n Node) (key, label string) {
	switch {
	case n.NodePool != "":
		return "np/" + n.NodePool, "NodePool " + n.NodePool
	case n.ManagedGroup != "":
		return "ng/" + n.ManagedGroup, "node group " + n.ManagedGroup
	}
	return "node/" + n.Name, n.Name
}

func splitType(t string) (family, size string) {
	family, size, _ = strings.Cut(t, ".")
	return family, size
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
		if n.Capacity == Fargate || n.Auto || (n.ManagedGroup == "" && n.NodePool == "") {
			continue
		}
		_, label := groupOf(n)
		key := label + "|" + n.Type
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
		if need >= len(g.nodes) {
			continue
		}
		removed := len(g.nodes) - need
		typ := g.nodes[0].Type
		r := Rec{Kind: Consolidate, Target: fmt.Sprintf("%s (%s ×%d)", g.label, typ, len(g.nodes)),
			Action: fmt.Sprintf("%d nodes → %d (pods fit at ≤%.0f%%)", len(g.nodes), need, in.Headroom*100),
			Reason: "first-fit decreasing by requests only (affinity, taints, spread, budgets and volumes not considered)",
			Math:   steps}
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

// listPrice is the price a node of the type would pay in its market: the
// on-demand list price, or the spot price in its zone.
func listPrice(in Input, n Node, typ string) (float64, bool) {
	switch n.Capacity {
	case OnDemand:
		return in.OnDemand(typ)
	case Spot:
		return in.SpotIn(typ, n.Zone)
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
// architecture the node's requests fit at the headroom (R7).
func rightsize(in Input) []Rec {
	// candidates come from the node's family only: index the catalog once
	byFamily := map[string][]string{}
	for _, t := range in.Types {
		f, _ := splitType(t)
		byFamily[f] = append(byFamily[f], t)
	}
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
		family, _ := splitType(n.Type)
		req := n.requested()
		type cand struct {
			typ   string
			price float64
		}
		var cands []cand
		for _, t := range byFamily[family] {
			if t == n.Type {
				continue
			}
			s, ok := in.Spec(t)
			if !ok || s.Arch != spec.Arch || (s.VCPU >= spec.VCPU && s.MemGiB >= spec.MemGiB) {
				continue
			}
			if p, ok := listPrice(in, n, t); ok && p < cur {
				cands = append(cands, cand{t, p})
			}
		}
		sort.Slice(cands, func(a, b int) bool { return cands[a].price < cands[b].price })
		for _, c := range cands {
			s, _ := in.Spec(c.typ)
			alloc, how := allocatableFor(in, c.typ, s)
			cpu, mem := req.CPU/alloc.CPU, req.Mem/alloc.Mem
			podsNote := "pod capacity not checked (no node of this type)"
			podsOK := true
			for _, o := range in.Nodes {
				if o.Type == c.typ {
					podsOK, podsNote = req.Pods <= o.Allocatable.Pods, fmt.Sprintf("pods %0.f of %0.f", req.Pods, o.Allocatable.Pods)
					break
				}
			}
			if cpu > in.Headroom || mem > in.Headroom || !podsOK {
				continue
			}
			saving := (cur - c.price) * HoursPerMonth
			out = append(out, Rec{Kind: Rightsize, Target: fmt.Sprintf("%s (%s)", n.Name, n.Type),
				Action:  fmt.Sprintf("→ %s (≈%.0f%% cpu, %.0f%% mem)", c.typ, cpu*100, mem*100),
				Monthly: saving, Priced: true, perNode: map[string]float64{n.Name: saving},
				Reason: "requests fit a smaller size at the headroom",
				Math: []string{
					fmt.Sprintf("%s: %s → %s", n.Name, n.Type, c.typ),
					fmt.Sprintf("requests %.2f vCPU / %.2f GiB (DaemonSets %.2f / %.2f included)", req.CPU, req.Mem, n.fixed().CPU, n.fixed().Mem),
					fmt.Sprintf("%s allocatable ≈ %.2f vCPU / %.2f GiB (%s)", c.typ, alloc.CPU, alloc.Mem, how),
					fmt.Sprintf("after the move: cpu %.0f%%, memory %.0f%% ≤ %.0f%%; %s", cpu*100, mem*100, in.Headroom*100, podsNote),
					fmt.Sprintf("saving: (%s%s − %s) × %d h = %s", money4(cur), note, money4(c.price), HoursPerMonth, money(saving)),
				}})
			break
		}
	}
	return out
}

func money4(v float64) string { return fmt.Sprintf("$%.4f", v) }

// swapFamily recommends a newer generation or the Graviton counterpart in
// the same size, per pool or group and type (R8, R9).
func swapFamily(in Input, kind Kind) []Rec {
	type acc struct {
		rec   Rec
		nodes int
	}
	groups := map[string]*acc{}
	var order []string
	for _, n := range in.Nodes {
		spec, ok := comparableSpec(in, n)
		if !ok || n.Capacity == Reserved {
			continue
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
			continue
		}
		cand := target + "." + size
		if _, ok := in.Spec(cand); !ok {
			continue
		}
		curList, ok1 := listPrice(in, n, n.Type)
		newList, ok2 := listPrice(in, n, cand)
		if !ok1 || !ok2 || newList >= curList {
			continue
		}
		saving := (curList - newList) * HoursPerMonth
		_, label := groupOf(n)
		key := label + "|" + n.Type
		g, ok := groups[key]
		if !ok {
			action := "→ " + cand
			if n.NodePool != "" {
				action = "allow " + cand + " in the NodePool"
			}
			reason := "newer generation, Nitro, same class and size"
			if kind == Graviton {
				reason = "Graviton (arm64): images must run on arm64"
				action += " (arm64 images!)"
			}
			g = &acc{rec: Rec{Kind: kind, Action: action, Priced: true, Reason: reason, perNode: map[string]float64{}}}
			groups[key] = g
			order = append(order, key)
			g.rec.Math = append(g.rec.Math, fmt.Sprintf("%s list %s/h → %s list %s/h", n.Type, money4(curList), cand, money4(newList)))
		}
		g.nodes++
		g.rec.Target = fmt.Sprintf("%s (%s ×%d)", label, n.Type, g.nodes)
		g.rec.Monthly += saving
		g.rec.perNode[n.Name] = saving
	}
	var out []Rec
	for _, k := range order {
		g := groups[k]
		g.rec.Math = append(g.rec.Math, fmt.Sprintf("saving: %d nodes × difference × %d h = %s", g.nodes, HoursPerMonth, money(g.rec.Monthly)))
		out = append(out, g.rec)
	}
	return out
}

// oldNodes recommends recycling nodes past the age limit (R10).
func oldNodes(in Input) []Rec {
	var out []Rec
	for _, n := range in.Nodes {
		if n.Created.IsZero() || n.Capacity == Fargate {
			continue
		}
		age := in.Now.Sub(n.Created)
		if age <= in.OldAge {
			continue
		}
		out = append(out, Rec{Kind: OldNode, Target: fmt.Sprintf("%s (%d days)", n.Name, int(age.Hours()/24)),
			Action: "recycle for a fresh AMI and patches", Reason: fmt.Sprintf("older than %d days", int(in.OldAge.Hours()/24))})
	}
	return out
}

// spot recommends moving On-Demand groups to Spot, priced in each node's own
// zone (R11). The screen lists it last.
func spot(in Input) []Rec {
	type acc struct {
		rec    Rec
		nodes  int
		priced bool
	}
	groups := map[string]*acc{}
	var order []string
	for _, n := range in.Nodes {
		if _, ok := comparableSpec(in, n); !ok || n.Capacity != OnDemand {
			continue
		}
		_, label := groupOf(n)
		key := label + "|" + n.Type
		g, ok := groups[key]
		if !ok {
			action := "use Spot"
			switch {
			case n.NodePool != "":
				action = "allow spot in the NodePool"
			case n.ManagedGroup != "":
				action = "add a Spot node group alongside it"
			}
			g = &acc{rec: Rec{Kind: SpotMove, Action: action + " (interruptions!)", Reason: "workloads must tolerate interruptions"}, priced: true}
			groups[key] = g
			order = append(order, key)
		}
		g.nodes++
		g.rec.Target = fmt.Sprintf("%s (On-Demand %s ×%d)", label, n.Type, g.nodes)
		od, ok1 := in.OnDemand(n.Type)
		sp, ok2 := in.SpotIn(n.Type, n.Zone)
		if !ok1 || !ok2 || sp >= od {
			g.priced = false
			g.rec.Math = append(g.rec.Math, fmt.Sprintf("%s in %s: no spot price for this zone", n.Name, n.Zone))
			continue
		}
		g.rec.Monthly += (od - sp) * HoursPerMonth
		g.rec.Math = append(g.rec.Math, fmt.Sprintf("%s in %s: %s → %s/h", n.Name, n.Zone, money4(od), money4(sp)))
	}
	var out []Rec
	for _, k := range order {
		g := groups[k]
		g.rec.Priced = g.priced
		if !g.priced {
			g.rec.Monthly = 0
		}
		out = append(out, g.rec)
	}
	return out
}
