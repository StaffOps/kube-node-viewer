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
	"time"
)

var now = time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)

// prices and specs from pkg/aws static tables (us-east-1)
var fixturePrices = map[string]float64{
	"m6i.large": 0.096, "m6i.xlarge": 0.192, "m6i.2xlarge": 0.384, "m7g.xlarge": 0.1632, "m7g.large": 0.0816,
	"t3.large": 0.0832, "t3.medium": 0.0416, "t3.small": 0.0208, "t4g.large": 0.0672, "t4g.medium": 0.0336,
	"t2.large": 0.0928, "t2.medium": 0.0464, "t2.small": 0.023,
}

var fixtureSpecs = map[string]Spec{
	"m6i.large": {2, 8, "x86_64", "General purpose", true}, "m6i.xlarge": {4, 16, "x86_64", "General purpose", true},
	"m6i.2xlarge": {8, 32, "x86_64", "General purpose", true},
	"m7g.large":   {2, 8, "arm64", "General purpose", true}, "m7g.xlarge": {4, 16, "arm64", "General purpose", true},
	"t3.large": {2, 8, "x86_64", "General purpose", true}, "t3.medium": {2, 4, "x86_64", "General purpose", true},
	"t3.small":  {2, 2, "x86_64", "General purpose", true},
	"t4g.large": {2, 8, "arm64", "General purpose", true}, "t4g.medium": {2, 4, "arm64", "General purpose", true},
	"t2.large": {2, 8, "x86_64", "General purpose", true}, "t2.medium": {2, 4, "x86_64", "General purpose", true},
	"t2.small": {1, 2, "x86_64", "General purpose", true},
}

func fixtureInput(nodes []Node) Input {
	var types []string
	for t := range fixtureSpecs {
		types = append(types, t)
	}
	return Input{
		Nodes: nodes, Now: now, Headroom: 0.8, OldAge: 30 * 24 * time.Hour, Types: types,
		Spec:     func(t string) (Spec, bool) { s, ok := fixtureSpecs[t]; return s, ok },
		OnDemand: func(t string) (float64, bool) { p, ok := fixturePrices[t]; return p, ok },
		SpotIn: func(t, zone string) (float64, bool) {
			if t == "m6i.xlarge" {
				return 0.0768, true
			}
			return 0, false
		},
	}
}

func ds(cpu, mem float64) Pod  { return Pod{Req: Resources{cpu, mem, 1}, Fixed: true} }
func app(cpu, mem float64) Pod { return Pod{Req: Resources{cpu, mem, 1}} }

func node(name, typ, zone, pool, group string, price float64, alloc Resources, age time.Duration, pods ...Pod) Node {
	return Node{Name: name, Type: typ, Zone: zone, Capacity: OnDemand, NodePool: pool, ManagedGroup: group,
		Allocatable: alloc, Price: price, PriceKnown: true, Created: now.Add(-age), Pods: pods}
}

// the usage example of specs/recommendations/requirements.md
func fixtureNodes() []Node {
	xl := Resources{3.92, 14, 58}
	t := Resources{1.93, 7.1, 35}
	day := 24 * time.Hour
	var nodes []Node
	nodes = append(nodes, node("ip-10-0-12-15", "m6i.xlarge", "us-east-1b", "default", "", 0.192, xl, 3*day, ds(0.25, 0.4), app(1.2, 5)))
	for i, z := range []string{"us-east-1a", "us-east-1b", "us-east-1c"} {
		nodes = append(nodes, node(fmt.Sprintf("ip-10-0-1%d-1", i), "m6i.xlarge", z, "default", "", 0.192, xl, 3*day, ds(0.25, 0.4), app(2.9, 10)))
	}
	for i := 0; i < 4; i++ {
		nodes = append(nodes, node(fmt.Sprintf("sys-%d", i), "t3.large", "us-east-1a", "", "system-ng", 0.0832, t, 5*day,
			ds(0.1, 0.2), app(0.2, 1.3), app(0.2, 1.3), app(0.2, 1.3)))
	}
	for i := 0; i < 2; i++ {
		nodes = append(nodes, node(fmt.Sprintf("legacy-%d", i), "t2.large", "us-east-1a", "", "legacy-ng", 0.0928, t, 10*day,
			ds(0.1, 0.2), app(1.2, 4.5)))
	}
	nodes[len(nodes)-1].Created = now.Add(-40 * day)
	nodes[len(nodes)-1].Name = "ip-10-0-11-70"
	return nodes
}
