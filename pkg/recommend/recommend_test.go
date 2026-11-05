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
	"math"
	"strings"
	"testing"
	"time"
)

func find(t *testing.T, r Result, kind Kind, target string) Rec {
	t.Helper()
	for _, x := range r.Recs {
		if x.Kind == kind && strings.HasPrefix(x.Target, target) {
			return x
		}
	}
	t.Fatalf("no %s row for %q in %v", kind, target, r.Recs)
	return Rec{}
}

func near(a, b float64) bool { return math.Abs(a-b) < 0.01 }

// The usage example of the spec, end to end:
// recommendations R3, R6, R7, R8, R9, R10, R11: the rows and their savings;
// recommendations R13: generation, Graviton and Spot per pool or group,
// rightsize and old node per node;
// recommendations R14: the total, Spot left out.
func TestSpecFixture(t *testing.T) {
	r := Analyze(fixtureInput(fixtureNodes()))
	want := []struct {
		kind    Kind
		target  string
		monthly float64
	}{
		{Consolidate, "node group system-ng", 60.74},
		{Rightsize, "ip-10-0-12-15", 70.08},
		{Generation, "node group legacy-ng", 14.02},
		{Graviton, "NodePool default", 84.10},
		{Graviton, "node group system-ng", 46.72},
		{SpotMove, "NodePool default", 336.38},
	}
	for _, w := range want {
		if got := find(t, r, w.kind, w.target); !got.Priced || !near(got.Monthly, w.monthly) {
			t.Errorf("%s %s: %+v, want %.2f", w.kind, w.target, got, w.monthly)
		}
	}
	if r.Total != 242.94 {
		t.Errorf("total %.2f, want 242.94", r.Total)
	}
	if !strings.Contains(find(t, r, Rightsize, "ip-10-0-12-15").Action, "m6i.large") {
		t.Error("rightsize target")
	}
	if old := find(t, r, OldNode, "ip-10-0-11-70"); old.Priced {
		t.Error("an old-node row has a saving")
	}
	for _, x := range r.Recs {
		if x.Kind == SpotMove && x.Target != "" && !strings.Contains(x.Target, "default") && x.Priced {
			t.Errorf("spot priced without a price in the node's zone: %+v", x)
		}
	}
}

// recommendations R5: idle cost is listed per group as info, with no action,
// and stays out of the total.
func TestIdleIsInfoOutOfTheTotal(t *testing.T) {
	r := Analyze(fixtureInput(fixtureNodes()))
	idle := 0
	for _, x := range r.Recs {
		if x.Kind != Idle {
			continue
		}
		idle++
		if !x.Info || x.Monthly <= 0 {
			t.Errorf("idle row not info or without its cost: %+v", x)
		}
		if len(x.perNode) != 0 {
			t.Errorf("idle row counts towards the total: %+v", x)
		}
	}
	if idle == 0 {
		t.Fatalf("the fixture has no idle row: %v", r.Recs)
	}
}

// recommendations R3 and R11: Spot rows come last.
func TestSpotIsAlwaysLast(t *testing.T) {
	r := Analyze(fixtureInput(fixtureNodes()))
	seenSpot := false
	for _, x := range r.Recs {
		if x.Kind == SpotMove {
			seenSpot = true
		} else if seenSpot {
			t.Fatalf("%s after a spot row", x.Kind)
		}
	}
	// its saving is the largest, yet it never enters the total
	if r.Total >= 336 {
		t.Error("spot counted in the total")
	}
}

func groupAt(share float64) []Node {
	alloc := Resources{2, 8, 30}
	var nodes []Node
	for i := 0; i < 4; i++ {
		// ten movable pods per node sharing its load
		var pods []Pod
		for j := 0; j < 10; j++ {
			pods = append(pods, app(2*share/10, 8*share/10))
		}
		nodes = append(nodes, node(fmt.Sprintf("n%d", i), "t3.large", "z", "", "g", 0.0832, alloc, time.Hour, pods...))
	}
	return nodes
}

// recommendations R6: consolidation packs by requests at the headroom.
func TestConsolidationByUse(t *testing.T) {
	for _, c := range []struct {
		share  float64
		remove int
	}{{0.35, 2}, {0.50, 1}, {0.70, 0}} {
		r := Analyze(fixtureInput(groupAt(c.share)))
		var got *Rec
		for i := range r.Recs {
			if r.Recs[i].Kind == Consolidate {
				got = &r.Recs[i]
			}
		}
		if c.remove == 0 {
			if got != nil {
				t.Errorf("%.0f%%: unexpected %+v", c.share*100, *got)
			}
			continue
		}
		if got == nil || !near(got.Monthly, float64(c.remove)*0.0832*HoursPerMonth) {
			t.Errorf("%.0f%%: %+v, want %d removed", c.share*100, got, c.remove)
		}
	}
}

func TestDaemonSetsAreNotMoved(t *testing.T) {
	nodes := groupAt(0.30)
	// a big DaemonSet on every node leaves no room to pack into
	for i := range nodes {
		nodes[i].Pods = append(nodes[i].Pods, ds(1.0, 4.0))
	}
	r := Analyze(fixtureInput(nodes))
	for _, x := range r.Recs {
		if x.Kind == Consolidate {
			t.Errorf("packed DaemonSet pods: %+v", x)
		}
	}
}

func TestPendingPodsBlockPacking(t *testing.T) {
	in := fixtureInput(groupAt(0.35))
	in.PendingPods = 3
	r := Analyze(in)
	for _, x := range r.Recs {
		if x.Kind == Consolidate || x.Kind == Rightsize {
			t.Errorf("packing with pending pods: %+v", x)
		}
	}
	if len(r.Notes) == 0 {
		t.Error("no note about pending pods")
	}
}

func TestKarpenterConsolidationIsInfo(t *testing.T) {
	nodes := groupAt(0.35)
	for i := range nodes {
		nodes[i].ManagedGroup, nodes[i].NodePool = "", "default"
	}
	r := Analyze(fixtureInput(nodes))
	c := find(t, r, Consolidate, "NodePool default")
	if !c.Info || c.perNode != nil {
		t.Errorf("Karpenter consolidation counted: %+v", c)
	}
}

// recommendations R15: nodes that can't be compared get no catalog advice.
func TestExcludedNodesGetNoCatalogAdvice(t *testing.T) {
	base := node("x", "t3.large", "z", "", "g", 0.0832, Resources{2, 8, 30}, time.Hour, app(0.1, 0.5))
	fargate, auto, untyped := base, base, base
	fargate.Capacity = Fargate
	auto.Auto = true
	untyped.Type = ""
	for name, n := range map[string]Node{"fargate": fargate, "auto": auto, "no type": untyped} {
		r := Analyze(fixtureInput([]Node{n}))
		for _, x := range r.Recs {
			if x.Kind == Rightsize || x.Kind == Generation || x.Kind == Graviton || x.Kind == SpotMove {
				t.Errorf("%s got %s advice", name, x.Kind)
			}
		}
	}
}

func TestSwapsNeedTheSizeAndALowerPrice(t *testing.T) {
	// t2.small → t3.small exists in the fixture specs but t3.small's price is
	// missing → no row; t2.medium → t3.medium cheaper → row
	n1 := node("a", "t2.small", "z", "", "g", 0.023, Resources{1, 2, 10}, time.Hour, app(0.5, 1.2))
	n2 := node("b", "t2.medium", "z", "", "h", 0.0464, Resources{2, 4, 17}, time.Hour, app(1.5, 3))
	in := fixtureInput([]Node{n1, n2})
	delete(fixturePrices, "t3.small")
	defer func() { fixturePrices["t3.small"] = 0.0208 }()
	r := Analyze(in)
	for _, x := range r.Recs {
		if x.Kind == Generation && strings.HasPrefix(x.Target, "node group g ") {
			t.Errorf("swap without a price: %+v", x)
		}
	}
	find(t, r, Generation, "node group h")
}

// recommendations R15: an instance-price label is used and said in the math.
func TestOverridePriceShowsInTheMath(t *testing.T) {
	n := node("o", "m6i.xlarge", "z", "", "", 0.30, Resources{3.92, 14, 58}, time.Hour, app(1, 4))
	n.Override = true
	r := Analyze(fixtureInput([]Node{n}))
	rs := find(t, r, Rightsize, "o")
	if !strings.Contains(strings.Join(rs.Math, "\n"), "instance-price label") {
		t.Errorf("math: %v", rs.Math)
	}
}

// 500 nodes, 15 000 pods, across 10 groups and the fixture's types.
func BenchmarkAnalyze(b *testing.B) {
	types := []string{"m6i.xlarge", "t3.large", "t2.large", "m6i.2xlarge"}
	var nodes []Node
	for i := 0; i < 500; i++ {
		typ := types[i%len(types)]
		s := fixtureSpecs[typ]
		var pods []Pod
		pods = append(pods, ds(0.1, 0.2))
		for j := 0; j < 29; j++ {
			pods = append(pods, app(s.VCPU*0.02, s.MemGiB*0.02))
		}
		nodes = append(nodes, node(fmt.Sprintf("n%d", i), typ, "us-east-1a", "", fmt.Sprintf("g%d", i%10), fixturePrices[typ],
			Resources{s.VCPU * 0.95, s.MemGiB * 0.88, 58}, time.Hour, pods...))
	}
	in := fixtureInput(nodes)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Analyze(in)
	}
}
