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

package recommend

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// The usage example of specs/aks-recommendations/requirements.md: a made-up
// AKS cluster in eastus, prices pinned from the Retail Prices API on
// 2026-11-10.

var azSpecs = map[string]Spec{
	"Standard_D2s_v5":   {VCPU: 2, MemGiB: 8, Arch: "x86_64", Family: "dsv5"},
	"Standard_D4s_v5":   {VCPU: 4, MemGiB: 16, Arch: "x86_64", Family: "dsv5"},
	"Standard_D8s_v5":   {VCPU: 8, MemGiB: 32, Arch: "x86_64", Family: "dsv5"},
	"Standard_D64s_v5":  {VCPU: 64, MemGiB: 256, Arch: "x86_64", Family: "dsv5"},
	"Standard_D2s_v3":   {VCPU: 2, MemGiB: 8, Arch: "x86_64", Family: "dsv3"},
	"Standard_D4s_v3":   {VCPU: 4, MemGiB: 16, Arch: "x86_64", Family: "dsv3"},
	"Standard_D4ds_v5":  {VCPU: 4, MemGiB: 16, Arch: "x86_64", Family: "ddsv5"},
	"Standard_D4ps_v5":  {VCPU: 4, MemGiB: 16, Arch: "arm64", Family: "dpsv5"},
	"Standard_D8ps_v5":  {VCPU: 8, MemGiB: 32, Arch: "arm64", Family: "dpsv5"},
	"Standard_D64ps_v5": {VCPU: 64, MemGiB: 208, Arch: "arm64", Family: "dpsv5"},
	"Standard_D4pds_v5": {VCPU: 4, MemGiB: 16, Arch: "arm64", Family: "dpdsv5"},
	"Standard_B2ms":     {VCPU: 2, MemGiB: 8, Arch: "x86_64", Family: "bs"},
	"Standard_B4ms":     {VCPU: 4, MemGiB: 16, Arch: "x86_64", Family: "bs"},
	// a constrained-vCPU size, cheaper than D4s_v5: never a candidate
	"Standard_D4-2s_v5": {VCPU: 4, MemGiB: 16, Arch: "x86_64", Family: "dsv5", Constrained: true},
	"Standard_D4s_v4":   {VCPU: 4, MemGiB: 16, Arch: "x86_64", Family: "dsv4"},
	"Standard_D2s_v4":   {VCPU: 2, MemGiB: 8, Arch: "x86_64", Family: "dsv4"},
}

var azPrices = map[string]float64{
	"Standard_D2s_v5": 0.096, "Standard_D4s_v5": 0.192, "Standard_D8s_v5": 0.384, "Standard_D64s_v5": 3.072,
	"Standard_D2s_v3": 0.096, "Standard_D4s_v3": 0.192, "Standard_D4ds_v5": 0.226,
	"Standard_D4ps_v5": 0.154, "Standard_D8ps_v5": 0.308, "Standard_D64ps_v5": 2.464, "Standard_D4pds_v5": 0.181,
	"Standard_B2ms": 0.0832, "Standard_B4ms": 0.166, "Standard_D4-2s_v5": 0.10,
	"Standard_D4s_v4": 0.20, "Standard_D2s_v4": 0.10,
}

var azSpot = map[string]float64{
	"Standard_D8s_v5": 0.081062, "Standard_D4s_v3": 0.038381, "Standard_D4s_v5": 0.040531,
}

// windows prices, for the meter tests
var azWindows = map[string]float64{"Standard_D4s_v5": 0.376, "Standard_D2s_v5": 0.188,
	// a cheaper Arm counterpart, so only the Windows rule keeps the Arm row out
	"Standard_D8s_v5": 0.752, "Standard_D8ps_v5": 0.5}
var azWindowsSpot = map[string]float64{"Standard_D4s_v5": 0.079374}

func azInput(nodes []Node) Input {
	var types []string
	for t := range azSpecs {
		types = append(types, t)
	}
	return Input{
		Nodes: nodes, Now: now, Headroom: 0.8, OldAge: 30 * 24 * time.Hour, Types: types,
		Spec: func(t string) (Spec, bool) { s, ok := azSpecs[t]; return s, ok },
		OnDemand: func(n Node, t string) (float64, bool) {
			if n.Windows {
				p, ok := azWindows[t]
				return p, ok
			}
			p, ok := azPrices[t]
			return p, ok
		},
		SpotIn: func(n Node, t string) (float64, bool) {
			if n.Windows {
				p, ok := azWindowsSpot[t]
				return p, ok
			}
			p, ok := azSpot[t]
			return p, ok
		},
	}
}

func azNode(name, typ, pool string, system bool, alloc, capacity Resources, age time.Duration, pods ...Pod) Node {
	return Node{Name: name, Type: typ, Zone: "eastus-1", Region: "eastus", Cloud: CloudAzure, Capacity: OnDemand,
		ManagedGroup: pool, System: system, Allocatable: alloc, NodeCapacity: capacity,
		Price: azPrices[typ], PriceKnown: true, Created: now.Add(-age), Pods: pods}
}

var (
	d4Alloc = Resources{3.86, 11.9, 250}
	d4Cap   = Resources{4, 15.6, 250}
	d8Alloc = Resources{7.82, 27.2, 250}
	d8Cap   = Resources{8, 31.3, 250}
)

func azDS() []Pod { return []Pod{ds(0.125, 0.25), ds(0.125, 0.25)} }

func azFixtureNodes() []Node {
	day := 24 * time.Hour
	name := func(pool string, i int) string { return fmt.Sprintf("aks-%s-12345678-vmss%06d", pool, i) }
	var nodes []Node
	for i := 0; i < 3; i++ {
		nodes = append(nodes, azNode(name("system", i), "Standard_D4s_v5", "system", true, d4Alloc, d4Cap, 10*day,
			append(azDS(), app(1, 3), app(1, 3))...))
	}
	for i := 0; i < 3; i++ {
		pods := azDS()
		for j := 0; j < 6; j++ {
			pods = append(pods, app(1, 3.5))
		}
		nodes = append(nodes, azNode(name("userpool", i), "Standard_D8s_v5", "userpool", false, d8Alloc, d8Cap, 10*day, pods...))
	}
	nodes = append(nodes, azNode(name("userpool", 3), "Standard_D8s_v5", "userpool", false, d8Alloc, d8Cap, 10*day,
		append(azDS(), app(1.75, 5.5))...))
	for i := 0; i < 2; i++ {
		age := 10 * day
		if i == 1 {
			age = 40 * day
		}
		nodes = append(nodes, azNode(name("legacy", i), "Standard_D4s_v3", "legacy", false, d4Alloc, d4Cap, age,
			append(azDS(), app(1, 3), app(1, 3))...))
	}
	return nodes
}

func kinds(r Result, k Kind) []Rec {
	var out []Rec
	for _, x := range r.Recs {
		if x.Kind == k {
			out = append(out, x)
		}
	}
	return out
}

// aks-recommendations R5, R7, R9, R10, R11, R13: the usage example end to
// end, rows, order, labels and the $322.66 total.
func TestAKSFixture(t *testing.T) {
	r := Analyze(azInput(azFixtureNodes()))
	if !near(r.Total, 322.66) {
		t.Errorf("total %.2f, want 322.66", r.Total)
	}
	want := []struct {
		kind           Kind
		target, action string
		monthly        float64
		info           bool
	}{
		{Idle, "agent pool userpool (4 nodes)", "", 377.46, true},
		{Idle, "agent pool system (3 nodes)", "", 175.38, true},
		{Idle, "agent pool legacy (2 nodes)", "", 116.92, true},
		{Rightsize, "aks-userpool-12345678-vmss000003 (Standard_D8s_v5)", "→ Standard_D4s_v5 (≈52% cpu, 50% mem)", 140.16, false},
		{Graviton, "agent pool userpool (Standard_D8s_v5 ×4)", "add a Standard_D8ps_v5 agent pool (arm64 images!)", 221.92, false},
		{Graviton, "agent pool legacy (Standard_D4s_v3 ×2)", "add a Standard_D4pds_v5 agent pool (arm64 images!)", 16.06, false},
		{OldNode, "aks-legacy-12345678-vmss000001 (40 days)", "upgrade the node image", 0, false},
		{SpotMove, "agent pool userpool (On-Demand Standard_D8s_v5 ×4)", "add a Spot agent pool alongside it (evictions!)", 884.58, false},
		{SpotMove, "agent pool legacy (On-Demand Standard_D4s_v3 ×2)", "add a Spot agent pool alongside it (evictions!)", 224.28, false},
	}
	if len(r.Recs) != len(want) {
		for _, x := range r.Recs {
			t.Logf("%s | %s | %s | %.2f", x.Label(), x.Target, x.Action, x.Monthly)
		}
		t.Fatalf("%d rows, want %d", len(r.Recs), len(want))
	}
	for i, w := range want {
		got := r.Recs[i]
		if got.Kind != w.kind || got.Target != w.target || (w.action != "" && got.Action != w.action) || !near(got.Monthly, w.monthly) || got.Info != w.info {
			t.Errorf("row %d = %s | %s | %s | %.2f info %v, want %s | %s | %s | %.2f", i, got.Label(), got.Target, got.Action, got.Monthly, got.Info,
				w.kind, w.target, w.action, w.monthly)
		}
	}
	for _, x := range kinds(r, Graviton) {
		if x.Label() != "arm" {
			t.Errorf("Azure arm64 row labelled %q", x.Label())
		}
	}
	rs := find(t, r, Rightsize, "aks-userpool-12345678-vmss000003")
	m := strings.Join(rs.Math, "\n")
	for _, s := range []string{
		"Standard_D4s_v5 allocatable ≈ 3.86 vCPU / 11.90 GiB (allocatable of this cluster's Standard_D4s_v5 nodes)",
		"pods 3 of 250 (a Standard_D4s_v5 node's)",
		"Standard_D2s_v5 doesn't fit: ≈ 1.82 vCPU / 3.20 GiB (catalog capacity minus this node's reservation; may understate)",
		"the pool has one VM size: resize it in place (preview)",
		"saving: ($0.3840 − $0.1920) × 730 h = $140.16",
	} {
		if !strings.Contains(m, s) {
			t.Errorf("rightsize math lacks %q:\n%s", s, m)
		}
	}
	for _, x := range kinds(r, SpotMove) {
		if !strings.Contains(strings.Join(x.Math, "\n"), "eastus (regional price)") {
			t.Errorf("spot math doesn't say the price is regional: %v", x.Math)
		}
	}
	if old := find(t, r, OldNode, "aks-legacy"); !strings.Contains(strings.Join(old.Math, " "), "--node-image-only") {
		t.Errorf("old-node math: %v", old.Math)
	}
}

// aks-recommendations R4: agent pools are the managed groups, a NodePool
// label wins, and node with no system mode gets user-pool advice.
func TestAKSGrouping(t *testing.T) {
	n := azFixtureNodes()[3] // a userpool node
	nap := n
	nap.Name, nap.NodePool = "nap-1", "default"
	r := Analyze(azInput([]Node{n, nap}))
	find(t, r, Idle, "agent pool userpool (1 nodes)")
	find(t, r, Idle, "NodePool default (1 nodes)")
	find(t, r, SpotMove, "agent pool userpool")
	if sp := find(t, r, SpotMove, "NodePool default"); sp.Action != "allow spot in the NodePool (evictions!)" {
		t.Errorf("NodePool spot action %q", sp.Action)
	}
}

// aks-recommendations R6: a system pool never goes below two nodes, and the
// row says AKS recommends three.
func TestAKSSystemPoolConsolidationFloor(t *testing.T) {
	var nodes []Node
	for i := 0; i < 3; i++ {
		nodes = append(nodes, azNode(fmt.Sprintf("sys-%d", i), "Standard_D4s_v5", "system", true, d4Alloc, d4Cap, time.Hour,
			append(azDS(), app(0.2, 0.5))...))
	}
	r := Analyze(azInput(nodes))
	c := find(t, r, Consolidate, "agent pool system")
	if c.Action != "scale agent pool system to 2 nodes (or lower its autoscaler minimum)" {
		t.Errorf("action %q", c.Action)
	}
	if !strings.Contains(strings.Join(c.Math, "\n"), "AKS recommends three") {
		t.Errorf("math: %v", c.Math)
	}
	if len(kinds(Analyze(azInput(azFixtureNodes())), Consolidate)) != 0 {
		t.Error("the fixture's pools are full: no consolidation row")
	}
}

// aks-recommendations R7: the estimate for a size the cluster doesn't run,
// the pod-capacity source and the pool wording.
func TestAKSRightsizeEstimate(t *testing.T) {
	n := azFixtureNodes()[6] // vmss000003, alone: no D4s_v5 node to measure
	r := Analyze(azInput([]Node{n}))
	rs := find(t, r, Rightsize, n.Name)
	m := strings.Join(rs.Math, "\n")
	for _, s := range []string{
		"Standard_D4s_v5 allocatable ≈ 3.82 vCPU / 11.20 GiB (catalog capacity minus this node's reservation; may understate)",
		"pods 3 of 250 (this node's max-pods)",
		"Standard_D2s_v5 doesn't fit",
	} {
		if !strings.Contains(m, s) {
			t.Errorf("math lacks %q:\n%s", s, m)
		}
	}
	vm := n
	vm.VMPool = true
	rs = find(t, Analyze(azInput([]Node{vm})), Rightsize, n.Name)
	if !strings.Contains(strings.Join(rs.Math, "\n"), "Virtual Machines pool: add Standard_D4s_v5 to its sizes") {
		t.Errorf("VMs pool math: %v", rs.Math)
	}
	// a reservation larger than the candidate leaves nothing: doesn't fit
	tiny := azNode("b", "Standard_B4ms", "burst", false, Resources{3.8, 2, 30}, Resources{4, 15.6, 30}, time.Hour, app(0.1, 0.1))
	for _, x := range Analyze(azInput([]Node{tiny})).Recs {
		if x.Kind == Rightsize {
			t.Errorf("rightsized with a reservation over the candidate's memory: %+v", x)
		}
	}
}

// aks-recommendations R8: the generation target keeps the temp disk, and a
// same price within $0.0001/h is an info row outside the total.
func TestAKSGenerationSamePrice(t *testing.T) {
	legacy := azFixtureNodes()[7:]
	if g := kinds(Analyze(azInput(legacy)), Generation); len(g) != 0 {
		t.Errorf("D4s_v3 → D4ds_v5 costs more ($0.226): %+v", g)
	}
	defer func() { azPrices["Standard_D4ds_v5"] = 0.226 }()
	for _, tt := range []struct {
		price      float64
		row, info  bool
		wantAction string
	}{
		{0.19205, true, true, "→ Standard_D4ds_v5: same price, newer hardware"},
		{0.1925, false, false, ""},
		{0.180, true, false, "→ Standard_D4ds_v5"},
	} {
		azPrices["Standard_D4ds_v5"] = tt.price
		r := Analyze(azInput(legacy))
		g := kinds(r, Generation)
		if !tt.row {
			if len(g) != 0 {
				t.Errorf("at $%v: %+v", tt.price, g)
			}
			continue
		}
		if len(g) != 1 || g[0].Info != tt.info || g[0].Action != tt.wantAction {
			t.Errorf("at $%v: %+v", tt.price, g)
			continue
		}
		if tt.info && !near(r.Total, 16.06) {
			t.Errorf("total %.2f, want the Arm rows' 16.06: the info row is not counted", r.Total)
		}
	}
}

// aks-recommendations R9: no arm64 row when the counterpart has less
// memory, for a Windows node or for a system pool; AWS keeps "graviton".
func TestAKSArmLimits(t *testing.T) {
	big := azNode("big", "Standard_D64s_v5", "big", false, Resources{63.5, 240, 250}, Resources{64, 251, 250}, time.Hour, app(60, 200))
	win := azFixtureNodes()[3]
	win.Windows = true
	sys := azFixtureNodes()[0]
	for name, n := range map[string]Node{"D64 (208 GiB counterpart)": big, "windows": win, "system": sys} {
		if g := kinds(Analyze(azInput([]Node{n})), Graviton); len(g) != 0 {
			t.Errorf("%s got %+v", name, g)
		}
	}
	aws := Analyze(fixtureInput(fixtureNodes()))
	for _, x := range kinds(aws, Graviton) {
		if x.Label() != "graviton" {
			t.Errorf("AWS row labelled %q", x.Label())
		}
	}
}

// aks-recommendations R10: the age limit, and no old-node row for an ACI
// virtual node.
func TestAKSOldNodes(t *testing.T) {
	day := 24 * time.Hour
	young := azNode("young", "Standard_D4s_v5", "p", false, d4Alloc, d4Cap, 10*day)
	old := azNode("old", "Standard_D4s_v5", "p", false, d4Alloc, d4Cap, 40*day)
	virtual := Node{Name: "virtual-node-aci-linux", Cloud: CloudAzure, Virtual: true, Created: now.Add(-40 * day), Capacity: OnDemand}
	r := Analyze(azInput([]Node{young, old, virtual}))
	o := kinds(r, OldNode)
	if len(o) != 1 || !strings.HasPrefix(o[0].Target, "old ") {
		t.Errorf("old-node rows %+v", o)
	}
}

// aks-recommendations R11: regional Spot, none for a system pool, the
// Windows Spot meter.
func TestAKSSpot(t *testing.T) {
	sys := azFixtureNodes()[0]
	if s := kinds(Analyze(azInput([]Node{sys})), SpotMove); len(s) != 0 {
		t.Errorf("Spot row for a system pool: %+v", s)
	}
	user := sys
	user.System, user.ManagedGroup = false, "user"
	user.Windows = true
	s := find(t, Analyze(azInput([]Node{user})), SpotMove, "agent pool user")
	if !near(s.Monthly, (0.376-0.079374)*730) {
		t.Errorf("Windows Spot saving %.2f, want the Windows meters'", s.Monthly)
	}
}

// aks-recommendations R12: a virtual node gets idle only; a size missing
// from the catalog gets idle and old node; the instance-price label is used.
func TestAKSExclusions(t *testing.T) {
	day := 24 * time.Hour
	virtual := Node{Name: "virtual-node-aci-linux", Cloud: CloudAzure, Region: "eastus", Virtual: true, Capacity: OnDemand,
		Allocatable: Resources{10, 100, 500}, Price: 1, PriceKnown: true, Created: now.Add(-40 * day), Pods: []Pod{app(1, 1)}}
	unknown := azNode("unknown", "Standard_Made_up_v9", "p", false, d4Alloc, d4Cap, 40*day, app(0.1, 0.1))
	unknown.Price, unknown.PriceKnown = 0.2, true
	for name, tt := range map[string]struct {
		n     Node
		kinds []Kind
	}{
		"virtual": {virtual, []Kind{Idle}},
		"unknown": {unknown, []Kind{Idle, OldNode}},
	} {
		r := Analyze(azInput([]Node{tt.n}))
		var got []Kind
		for _, x := range r.Recs {
			got = append(got, x.Kind)
		}
		if fmt.Sprint(got) != fmt.Sprint(tt.kinds) {
			t.Errorf("%s: kinds %v, want %v", name, got, tt.kinds)
		}
	}
	labelled := azFixtureNodes()[6]
	labelled.Price, labelled.Override = 0.30, true
	rs := find(t, Analyze(azInput([]Node{labelled})), Rightsize, labelled.Name)
	if !strings.Contains(strings.Join(rs.Math, "\n"), "$0.3000 (instance-price label)") {
		t.Errorf("math: %v", rs.Math)
	}
}

// aks-recommendations R14: AWS and Azure nodes together give both
// fixtures' rows, groups never mix, and one total.
func TestMixedClouds(t *testing.T) {
	awsNodes := fixtureNodes()
	for i := range awsNodes {
		awsNodes[i].Cloud = CloudAWS
	}
	in := azInput(append(awsNodes, azFixtureNodes()...))
	awsIn := fixtureInput(nil)
	azIn := azInput(nil)
	in.Types = append(awsIn.Types, azIn.Types...)
	in.Spec = func(t string) (Spec, bool) {
		if azureType(t) {
			return azIn.Spec(t)
		}
		return awsIn.Spec(t)
	}
	route := func(aws, az func(Node, string) (float64, bool)) func(Node, string) (float64, bool) {
		return func(n Node, t string) (float64, bool) {
			if isAzure(n) {
				return az(n, t)
			}
			return aws(n, t)
		}
	}
	in.OnDemand = route(awsIn.OnDemand, azIn.OnDemand)
	in.SpotIn = route(awsIn.SpotIn, azIn.SpotIn)
	r := Analyze(in)
	if !near(r.Total, 242.94+322.66) {
		t.Errorf("total %.2f, want 565.60", r.Total)
	}
	a, z := Analyze(fixtureInput(fixtureNodes())), Analyze(azInput(azFixtureNodes()))
	if len(r.Recs) != len(a.Recs)+len(z.Recs) {
		t.Errorf("%d rows, want %d + %d", len(r.Recs), len(a.Recs), len(z.Recs))
	}
}

// aks-recommendations R15: a system pool's size keeps 4 vCPU, 4 GiB, no B
// series and 30 pods.
func TestAKSSystemPoolLimits(t *testing.T) {
	sys := azNode("sys", "Standard_D4s_v5", "system", true, d4Alloc, d4Cap, time.Hour, append(azDS(), app(0.5, 1))...)
	d2 := azNode("d2", "Standard_D2s_v5", "user", false, Resources{1.9, 5.4, 250}, Resources{2, 7.7, 250}, time.Hour)
	for _, x := range Analyze(azInput([]Node{sys, d2})).Recs {
		if x.Kind == Rightsize && strings.HasPrefix(x.Target, "sys ") {
			t.Errorf("system node rightsized below 4 vCPU: %+v", x)
		}
	}
	if systemSizeOK(Spec{VCPU: 4, MemGiB: 16, Family: "bsv2"}, 250) {
		t.Error("a B-series size passes for a system pool")
	}
	if systemSizeOK(Spec{VCPU: 4, MemGiB: 16, Family: "dsv5"}, 20) {
		t.Error("a pod capacity of 20 passes for a system pool")
	}
	if !systemSizeOK(Spec{VCPU: 4, MemGiB: 16, Family: "dsv5"}, 30) {
		t.Error("a 4 vCPU / 16 GiB / 30 pods size refused")
	}
	// a system node whose pool keeps max-pods 20 (no node of the candidate
	// size to measure): no rightsize even where it would fit
	bigSys := azNode("bigsys", "Standard_D8s_v5", "system", true, Resources{7.82, 27.2, 20}, d8Cap, time.Hour, app(0.5, 1))
	for _, x := range Analyze(azInput([]Node{bigSys})).Recs {
		if x.Kind == Rightsize && strings.HasPrefix(x.Target, "bigsys ") {
			t.Errorf("system node with 20 pods rightsized: %+v", x)
		}
	}
}

// aks-recommendations R3, R11: an Azure Spot row needs every node's price;
// a Windows node without a Windows Spot price, or prices off, give none.
func TestAKSSpotNeedsPrices(t *testing.T) {
	win := azFixtureNodes()[3]
	win.Windows = true
	delete(azWindowsSpot, "Standard_D8s_v5")
	if s := kinds(Analyze(azInput([]Node{win})), SpotMove); len(s) != 0 {
		t.Errorf("Windows node without a Windows Spot price got %+v", s)
	}
	// pricing off: no node price, no list price
	off := azInput(nil)
	off.OnDemand = func(Node, string) (float64, bool) { return 0, false }
	off.SpotIn = off.OnDemand
	for _, n := range azFixtureNodes() {
		n.Price, n.PriceKnown = 0, false
		off.Nodes = append(off.Nodes, n)
	}
	var got []Kind
	for _, x := range Analyze(off).Recs {
		got = append(got, x.Kind)
	}
	if fmt.Sprint(got) != fmt.Sprint([]Kind{OldNode}) {
		t.Errorf("prices off: kinds %v, want [old node]", got)
	}
	// a pool that packs still gets its consolidation row, saving unknown
	off.Nodes = nil
	for i := 0; i < 3; i++ {
		n := azNode(fmt.Sprintf("sys-%d", i), "Standard_D4s_v5", "system", true, d4Alloc, d4Cap, time.Hour, append(azDS(), app(0.2, 0.5))...)
		n.Price, n.PriceKnown = 0, false
		off.Nodes = append(off.Nodes, n)
	}
	r := Analyze(off)
	if len(r.Recs) != 1 || r.Recs[0].Kind != Consolidate || r.Recs[0].Priced {
		t.Errorf("prices off, packing pool: %+v", r.Recs)
	}
}

// aks-recommendations R14: an AWS node group and an Azure agent pool of the
// same name stay two groups.
func TestCloudsNeverShareAGroup(t *testing.T) {
	a := node("ip-192-0-2-10", "m6i.xlarge", "us-east-1a", "", "shared", 0.192, Resources{3.92, 14, 58}, time.Hour, app(1, 4))
	a.Cloud = CloudAWS
	z := azFixtureNodes()[3]
	z.ManagedGroup = "shared"
	in := azInput([]Node{a, z})
	awsIn := fixtureInput(nil)
	in.Spec = func(t string) (Spec, bool) {
		if azureType(t) {
			return azSpecs[t], true
		}
		return awsIn.Spec(t)
	}
	idle := kinds(Analyze(in), Idle)
	if len(idle) != 2 {
		t.Fatalf("idle rows %+v, want one per cloud", idle)
	}
	find(t, Analyze(in), Idle, "node group shared (1 nodes)")
	find(t, Analyze(in), Idle, "agent pool shared (1 nodes)")
}

// aks-recommendations R2, R7: a constrained-vCPU size is never recommended,
// though cheaper.
func TestConstrainedSizesNeverRecommended(t *testing.T) {
	r := Analyze(azInput(azFixtureNodes()))
	for _, x := range r.Recs {
		if strings.Contains(x.Action, "Standard_D4-2s_v5") {
			t.Errorf("constrained size recommended: %+v", x)
		}
	}
}

// recommendations R7, R11; aks-recommendations R1: the AWS math is exactly
// what it was (no Azure lines, wording or pod source on AWS rows).
func TestAWSMathUnchanged(t *testing.T) {
	r := Analyze(fixtureInput(fixtureNodes()))
	rs := find(t, r, Rightsize, "ip-10-0-12-15")
	want := []string{
		"ip-10-0-12-15: m6i.xlarge → m6i.large",
		"requests 1.45 vCPU / 5.40 GiB (DaemonSets 0.25 / 0.40 included)",
		"m6i.large allocatable ≈ 1.96 vCPU / 7.00 GiB (capacity × allocatable ratio of this cluster's m6i nodes)",
		"after the move: cpu 74%, memory 77% ≤ 80%; pod capacity not checked (no node of this type)",
		"saving: ($0.1920 − $0.0960) × 730 h = $70.08",
	}
	if strings.Join(rs.Math, "\n") != strings.Join(want, "\n") {
		t.Errorf("AWS rightsize math changed:\n%s", strings.Join(rs.Math, "\n"))
	}
	sp := find(t, r, SpotMove, "NodePool default")
	if sp.Action != "allow spot in the NodePool (interruptions!)" || sp.Math[0] != "ip-10-0-12-15 in us-east-1b: $0.1920 → $0.0768/h" {
		t.Errorf("AWS spot row changed: %q %q", sp.Action, sp.Math)
	}
	ng := find(t, r, SpotMove, "node group system-ng")
	if ng.Action != "add a Spot node group alongside it (interruptions!)" || ng.Math[0] != "sys-0 in us-east-1a: no spot price for this zone" || ng.Priced {
		t.Errorf("AWS unpriced spot row changed: %q %q", ng.Action, ng.Math)
	}
}

// aks-recommendations R12: a virtual node is excluded even with an
// instance-type label.
func TestVirtualNodeWithASize(t *testing.T) {
	v := azFixtureNodes()[3]
	v.Virtual = true
	for _, x := range Analyze(azInput([]Node{v})).Recs {
		if x.Kind != Idle {
			t.Errorf("virtual node got %s advice", x.Kind)
		}
	}
}

// aks-recommendations R7: the reservation takes status.capacity when it is
// the larger (a table truncated to whole GB, as for DS1_v2), and a user pool
// checks the node's own max-pods.
func TestAzureReservationAndPods(t *testing.T) {
	n := Node{Type: "Standard_DS1_v2", Allocatable: Resources{0.9, 2.0, 30}, NodeCapacity: Resources{1, 3.4, 30}}
	nspec := Spec{VCPU: 1, MemGiB: 3} // truncated from 3.5
	alloc, _, ok := azureAllocatable(Input{}, n, nspec, "Standard_DS2_v2", Spec{VCPU: 2, MemGiB: 7})
	if !ok || !near(alloc.Mem, 7-1.4) || !near(alloc.CPU, 2-0.1) {
		t.Errorf("estimate %+v, want memory 7 − (3.4 − 2.0)", alloc)
	}
	busy := azFixtureNodes()[6]
	for i := 0; i < 40; i++ {
		busy.Pods = append(busy.Pods, app(0.01, 0.01))
	}
	busy.Allocatable.Pods = 30 // the pool's max-pods; no D4s_v5 node to measure
	for _, x := range Analyze(azInput([]Node{busy})).Recs {
		if x.Kind == Rightsize {
			t.Errorf("43 pods rightsized onto a 30-pod pool: %+v", x)
		}
	}
}

// aks-recommendations R8, R15: a system pool's generation row reads "add a
// system-mode pool", and a target under the system limits gives none.
func TestAKSSystemPoolSwaps(t *testing.T) {
	sys := azNode("sys", "Standard_D4s_v4", "system", true, d4Alloc, d4Cap, time.Hour, app(1, 3))
	g := find(t, Analyze(azInput([]Node{sys})), Generation, "agent pool system")
	if g.Action != "add a system-mode pool of Standard_D4s_v5" {
		t.Errorf("action %q", g.Action)
	}
	small := azNode("small", "Standard_D2s_v4", "system", true, Resources{1.9, 5.4, 250}, Resources{2, 7.7, 250}, time.Hour, app(0.5, 1))
	if x := kinds(Analyze(azInput([]Node{small})), Generation); len(x) != 0 {
		t.Errorf("system pool offered a 2-vCPU size: %+v", x)
	}
}
