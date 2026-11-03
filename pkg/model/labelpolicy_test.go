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
	"testing"

	"github.com/staffops/kube-node-viewer/pkg/model"
)

// TestLabelPatternMatching checks the pattern rules of label-visibility R4
// through an allow list of one pattern.
func TestLabelPatternMatching(t *testing.T) {
	tests := []struct {
		pattern, key string
		want         bool
	}{
		// R4 examples
		{"*.amazonaws.com/*", "eks.amazonaws.com/nodegroup", true},
		{"*.amazonaws.com/*", "amazonaws.com", false},
		{"karpenter.sh/*", "karpenter.sh/nodepool", true},
		{"karpenter", "karpenter.sh/nodepool", false},
		{"Team", "team", false},
		{"team", "Team", false},
		{"a.b/c", "aXb/c", false},
		{"a.b/c", "a.b/c", true},
		// whole key, no partial match
		{"team", "team-x", false},
		{"team", "my-team", false},
		{"karpenter.sh/*", "xkarpenter.sh/nodepool", false},
		// '*' matches any run, empty included, '/' and '.' included
		{"*", "", true},
		{"*", "a/b.c", true},
		{"karpenter.sh/*", "karpenter.sh/", true},
		{"a*b", "ab", true},
		{"a*b", "a/x.y/b", true},
		{"a*b", "a/x.y/bc", false},
		{"*-id", "app.example.com/internal-id", true},
		{"a*b*c", "abc", true},
		{"a*b*c", "aXbYc", true},
		{"a*b*c", "acb", false},
		{"a*b*b", "abb", true},
		{"a*b*b", "ab", false},
		{"ab*ba", "aba", false},
		{"a**b", "aXb", true},
		{"*x*", "abc", false},
		{"*x*", "axc", true},
		// other glob syntax is literal
		{"a?c", "abc", false},
		{"a?c", "a?c", true},
		{"[ab]", "a", false},
		{"[ab]", "[ab]", true},
	}
	for _, tt := range tests {
		t.Run(tt.pattern+"~"+tt.key, func(t *testing.T) {
			p, err := model.NewLabelPolicy([]string{tt.pattern}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if got := p.Shows(tt.key); got != tt.want {
				t.Errorf("allow %q, Shows(%q) = %v, want %v", tt.pattern, tt.key, got, tt.want)
			}
			// the same pattern as a deny list hides exactly what it matches
			d, err := model.NewLabelPolicy(nil, []string{tt.pattern})
			if err != nil {
				t.Fatal(err)
			}
			if got := d.Shows(tt.key); got == tt.want {
				t.Errorf("deny %q, Shows(%q) = %v, want %v", tt.pattern, tt.key, got, !tt.want)
			}
		})
	}
}

func TestLabelPolicyLists(t *testing.T) {
	tests := []struct {
		name        string
		allow, deny []string
		shown       []string
		hidden      []string
	}{
		{
			name:  "no lists shows everything (R3)",
			shown: []string{"team", "env", "kubernetes.io/os", "", "kube-node-viewer/instance-price"},
		},
		{
			name:  "empty lists show everything (R3)",
			allow: []string{}, deny: []string{},
			shown: []string{"team", "env", "kubernetes.io/os"},
		},
		{
			name:   "allow only (R1)",
			allow:  []string{"team"},
			shown:  []string{"team"},
			hidden: []string{"env", "kubernetes.io/os", "teams"},
		},
		{
			name:   "allow several, any match shows",
			allow:  []string{"karpenter.sh/*", "team"},
			shown:  []string{"karpenter.sh/nodepool", "karpenter.sh/capacity-type", "team"},
			hidden: []string{"env"},
		},
		{
			name:   "deny wins over allow (R2)",
			allow:  []string{"app.example.com/*"},
			deny:   []string{"app.example.com/internal-*"},
			shown:  []string{"app.example.com/owner"},
			hidden: []string{"app.example.com/internal-id", "team"},
		},
		{
			name:   "deny only (R2)",
			deny:   []string{"env"},
			shown:  []string{"team", "kubernetes.io/os", "environment"},
			hidden: []string{"env"},
		},
		{
			name:   "deny of an exact allowed key",
			allow:  []string{"team"},
			deny:   []string{"team"},
			hidden: []string{"team", "env"},
		},
		{
			name:   "README deny example",
			deny:   []string{"*.amazonaws.com/*", "beta.kubernetes.io/*", "kubernetes.io/hostname"},
			shown:  []string{"team", "kubernetes.io/os", "karpenter.sh/nodepool"},
			hidden: []string{"eks.amazonaws.com/nodegroup", "beta.kubernetes.io/arch", "kubernetes.io/hostname"},
		},
		{
			name:   "prefixed real label follows the lists (R1)",
			allow:  []string{"team"},
			hidden: []string{"kube-node-viewer/instance-price"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := model.NewLabelPolicy(tt.allow, tt.deny)
			if err != nil {
				t.Fatal(err)
			}
			for _, k := range tt.shown {
				if !p.Shows(k) {
					t.Errorf("Shows(%q) = false, want true", k)
				}
			}
			for _, k := range tt.hidden {
				if p.Shows(k) {
					t.Errorf("Shows(%q) = true, want false", k)
				}
			}
		})
	}
}

func TestLabelPolicyZeroValueShowsEverything(t *testing.T) {
	var p model.LabelPolicy
	for _, k := range []string{"", "team", "eks.amazonaws.com/nodegroup", "kube-node-viewer/node-age"} {
		if !p.Shows(k) {
			t.Errorf("zero policy hides %q", k)
		}
	}
}

func TestLabelPolicyRejectsEmptyPattern(t *testing.T) {
	tests := []struct {
		name        string
		allow, deny []string
	}{
		{"allow", []string{"team", ""}, nil},
		{"deny", nil, []string{""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := model.NewLabelPolicy(tt.allow, tt.deny); err == nil {
				t.Error("an empty pattern was accepted")
			}
		})
	}
}
