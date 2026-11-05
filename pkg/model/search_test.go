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

package model_test

import (
	"fmt"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/staffops/kube-node-viewer/pkg/model"
)

// Tests for specs/search-regex R1 and R2: which nodes a search keeps.

func searchNodes() []*model.Node {
	specs := []viewNode{
		{name: "node-1", pool: "search", ip: "203.0.113.5", itype: "m7g.large"},
		{name: "node-12", pool: "batch", ip: "203.0.113.12", itype: "m7g.xlarge"},
		{name: "ip-203-0-113-5.example.com", pool: "web", ip: "203.0.113.50", itype: "m7g.medium"},
	}
	var nodes []*model.Node
	for _, s := range specs {
		nodes = append(nodes, s.build())
	}
	return nodes
}

func TestSearchRegex(t *testing.T) {
	cases := []struct {
		text string
		want []string
	}{
		{"search|batch", []string{"node-1", "node-12"}},                   // R1: either word
		{"^node-1$", []string{"node-1"}},                                  // R1: anchored
		{"node-1", []string{"node-1", "node-12"}},                         // plain text, as before
		{"m7g\\.(large|xl)", []string{"node-1", "node-12"}},               // R1: group
		{"SEARCH|zz", []string{"node-1"}},                                 // R1: any case
		{"(?-i)SEARCH|zz", nil},                                           // R1: case on request
		{"203.0.113.5", []string{"ip-203-0-113-5.example.com", "node-1"}}, // R2: literal `.`; .50 contains .5
		{"203.0.113.5$", []string{"node-1"}},                              // a regex: anchored at the end
		{"a(b", nil},                                                      // R3: literal, nothing contains it
		{"   ", []string{"ip-203-0-113-5.example.com", "node-1", "node-12"}},
	}
	for _, c := range cases {
		s := model.ViewState{Sort: byName(), Search: c.text}
		if got := names(s.Select(searchNodes())); fmt.Sprint(got) != fmt.Sprint(c.want) {
			t.Errorf("search %q: got %v, want %v", c.text, got, c.want)
		}
	}
}

// BenchmarkSelectSearch measures Select at 5,000 nodes x 30 labels with a
// substring and regex searches (search-regex D3: words joined by | and
// anchored patterns within 2x of the substring, a group within about 6x).
func BenchmarkSelectSearch(b *testing.B) {
	nodes := make([]*model.Node, 0, 5000)
	for i := range 5000 {
		labels := map[string]string{}
		for j := range 30 {
			labels[fmt.Sprintf("example.com/key-%d", j)] = fmt.Sprintf("value-%d-%d", i, j)
		}
		nodes = append(nodes, model.NewNode(&v1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("node-%d", i), Labels: labels},
			Status:     v1.NodeStatus{Allocatable: v1.ResourceList{v1.ResourceCPU: resource.MustParse("4")}},
		}))
	}
	// nomatch=x: a label-pair search (search-negation, Performance: within 3x
	// of nomatch)
	for _, text := range []string{"nomatch", "nomatch|other", "^nomatch$", "nomatch\\.(a|b)", "nomatch=x", "!nomatch", "nomatch\\.(a|b)=", "^nomatch=x$"} {
		b.Run(text, func(b *testing.B) {
			s := model.ViewState{Sort: byName(), Search: text}
			for b.Loop() {
				s.Select(nodes)
			}
		})
	}
}

// Tests for specs/search-negation R1, R2, R3, R4: negation and label pairs.
// search-negation R1 search-negation R2 search-negation R3 search-negation R4

func negationNodes() []*model.Node {
	specs := []struct {
		name   string
		labels map[string]string
	}{
		{"node-a", map[string]string{"workload": "search", "topology.kubernetes.io/zone": "az-1", "team": "a"}},
		{"node-b", map[string]string{"workload": "batch", "topology.kubernetes.io/zone": "az-1", "example.com/gpu": ""}},
		{"node-c", map[string]string{"workload": "batch-2", "topology.kubernetes.io/zone": "az-2", "example.com/gpu": "t4"}},
		{"node-d", map[string]string{"workload": "web", "sub.example.com/gpu": "x", "note": "a(b"}},
		{"node-e", nil},
	}
	var nodes []*model.Node
	for _, s := range specs {
		n := testNode(s.name)
		n.Labels = s.labels
		n.Status.Allocatable = v1.ResourceList{v1.ResourceCPU: resource.MustParse("4")}
		nodes = append(nodes, model.NewNode(n))
	}
	return nodes
}

func TestSearchNegationAndPairs(t *testing.T) {
	all := []string{"node-a", "node-b", "node-c", "node-d", "node-e"}
	cases := []struct {
		text string
		want []string
	}{
		{"!search", []string{"node-b", "node-c", "node-d", "node-e"}},     // R1
		{"!search|batch", []string{"node-d", "node-e"}},                   // R1: negated regex (words)
		{"! search", []string{"node-b", "node-c", "node-d", "node-e"}},    // R1: space after !
		{"  !search  ", []string{"node-b", "node-c", "node-d", "node-e"}}, // R1: trimmed
		{"!^batch$", []string{"node-a", "node-c", "node-d", "node-e"}},    // R1: compiled regex
		{"!a(b", []string{"node-a", "node-b", "node-c", "node-e"}},        // R1: invalid, as text, negated
		{"workload=batch", []string{"node-b", "node-c"}},                  // R2: substring of the pair
		{"^workload=batch$", []string{"node-b"}},                          // R2: exact pair
		{"example.com/gpu=", []string{"node-b", "node-c", "node-d"}},      // R2: has the key; sub.example.com/gpu too
		{"^example\\.com/gpu=", []string{"node-b", "node-c"}},             // R2: exact key
		{"!example.com/gpu=", []string{"node-a", "node-e"}},               // R2: without the key; no labels kept
		{"!^example\\.com/gpu=", []string{"node-a", "node-d", "node-e"}},  // R2
		{"team=a|batch", []string{"node-a", "node-b", "node-c"}},          // R2: pair or value
		{"zone", nil},             // R3: keys not searched without =
		{"!", all},                // R4
		{"!   ", all},             // R4
		{"!!search", all},         // only one ! negates: literal "!search"
		{"a!b", nil},              // ! inside is text
		{"workload!=batch", nil},  // kubectl not-equal: literal
		{"!example.com/gpu", all}, // kubectl "absent": values only, negated
		{"=", []string{"node-a", "node-b", "node-c", "node-d"}}, // every labelled node
		{"!=", []string{"node-e"}},                              // only the node with no labels
		{"\\!search", nil},                                      // a literal leading !
	}
	for _, c := range cases {
		s := model.ViewState{Sort: byName(), Search: c.text}
		if got := names(s.Select(negationNodes())); fmt.Sprint(got) != fmt.Sprint(c.want) {
			t.Errorf("search %q: got %v, want %v", c.text, got, c.want)
		}
	}
	// search-negation R1: filters still apply, the search only removes nodes
	f := model.Filter{Field: model.LabelField("workload"), Values: []string{"batch", "batch-2"}, Enabled: true}
	s := model.ViewState{Sort: byName(), Filters: []model.Filter{f}, Search: "!^batch$"}
	if got := names(s.Select(negationNodes())); fmt.Sprint(got) != "[node-c]" {
		t.Errorf("filter workload in {batch, batch-2} with !^batch$: got %v, want [node-c]", got)
	}
	if got := (model.ViewState{Search: "!x"}).Select(nil); len(got) != 0 {
		t.Errorf("empty node list: got %v", got)
	}
}
