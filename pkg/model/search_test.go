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

// search-regex R1, R2: which nodes a regex or a literal search keeps.
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
	for _, text := range []string{"nomatch", "nomatch|other", "^nomatch$", "nomatch\\.(a|b)"} {
		b.Run(text, func(b *testing.B) {
			s := model.ViewState{Sort: byName(), Search: text}
			for b.Loop() {
				s.Select(nodes)
			}
		})
	}
}
