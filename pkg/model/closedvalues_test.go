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
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	"github.com/staffops/kube-node-viewer/pkg/model"
)

// fix-config-filter-values R3: every CapacityType constant and every Status…
// constant declared in node.go is in its closed list, so a value added there
// without the list fails here.
func TestClosedValueListsHoldEveryConstant(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "node.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var capacities, statuses []string
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs := spec.(*ast.ValueSpec)
			for i, name := range vs.Names {
				lit, ok := vs.Values[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				value, _ := strconv.Unquote(lit.Value)
				typ, _ := vs.Type.(*ast.Ident)
				switch {
				case typ != nil && typ.Name == "CapacityType":
					capacities = append(capacities, value)
				case strings.HasPrefix(name.Name, "Status"):
					statuses = append(statuses, value)
				}
			}
		}
	}
	if len(capacities) == 0 || len(statuses) == 0 {
		t.Fatalf("parsed no constants: capacity %v, status %v", capacities, statuses)
	}
	for _, c := range capacities {
		if !slices.Contains(model.CapacityValues, c) {
			t.Errorf("CapacityType %q is not in CapacityValues %v", c, model.CapacityValues)
		}
	}
	for _, s := range statuses {
		if !slices.Contains(model.StatusValues, s) {
			t.Errorf("status %q is not in StatusValues %v", s, model.StatusValues)
		}
	}
}

// fix-config-filter-values R3: every branch of CapacityType and every flag of
// StatusFlags gives a listed value, and the panel's choices are the lists.
func TestClosedValueListsCoverEveryBranch(t *testing.T) {
	node := func(providerID string, labels map[string]string) *v1.Node {
		return &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "n", Labels: labels}, Spec: v1.NodeSpec{ProviderID: providerID}}
	}
	capacity := map[string]*v1.Node{
		"fargate":        node("aws:///z/i", map[string]string{"eks.amazonaws.com/compute-type": "fargate"}),
		"karpenter od":   node("aws:///z/i", map[string]string{"karpenter.sh/capacity-type": "on-demand"}),
		"karpenter spot": node("aws:///z/i", map[string]string{"karpenter.sh/capacity-type": "spot"}),
		"karpenter rsv":  node("aws:///z/i", map[string]string{"karpenter.sh/capacity-type": "reserved"}),
		"eks od":         node("aws:///z/i", map[string]string{"eks.amazonaws.com/capacityType": "ON_DEMAND"}),
		"eks spot":       node("aws:///z/i", map[string]string{"eks.amazonaws.com/capacityType": "SPOT"}),
		"eks block":      node("aws:///z/i", map[string]string{"eks.amazonaws.com/capacityType": "CAPACITY_BLOCK"}),
		"aks spot":       node("azure:///vm", map[string]string{"kubernetes.azure.com/scalesetpriority": "spot"}),
		"aks regular":    node("azure:///vm", nil),
		"none":           node("", nil),
	}
	seen := map[string]bool{}
	for name, n := range capacity {
		got := string(model.NewNode(n).CapacityType())
		seen[got] = true
		if !slices.Contains(model.CapacityValues, got) {
			t.Errorf("%s: CapacityType %q is not listed", name, got)
		}
	}
	for _, want := range []string{"OnDemand", "Spot", "Reserved", "Fargate", "Unknown"} {
		if !seen[want] {
			t.Errorf("no branch gave %s", want)
		}
	}

	ready := node("", nil)
	ready.Status.Conditions = []v1.NodeCondition{{Type: v1.NodeReady, Status: v1.ConditionTrue}}
	cordoned := node("", nil)
	cordoned.Spec.Unschedulable = true
	deleting := node("", nil)
	deleting.DeletionTimestamp = &metav1.Time{Time: time.Unix(1, 0)}
	launching := model.NewNodeFromNodeClaim(&karpv1.NodeClaim{ObjectMeta: metav1.ObjectMeta{Name: "c"}})
	flags := map[string]bool{}
	for _, n := range []*model.Node{model.NewNode(ready), model.NewNode(node("", nil)), model.NewNode(cordoned), model.NewNode(deleting), launching} {
		for _, f := range n.StatusFlags() {
			flags[f] = true
			if !slices.Contains(model.StatusValues, f) {
				t.Errorf("status flag %q is not listed", f)
			}
		}
	}
	for _, want := range model.StatusValues {
		if !flags[want] {
			t.Errorf("no node gave the status %s", want)
		}
	}

	for field, want := range map[model.Field][]string{model.FieldCapacity: model.CapacityValues, model.FieldStatus: model.StatusValues} {
		var got []string
		for _, vc := range model.ValueCounts(nil, field) {
			got = append(got, vc.Value)
		}
		if !slices.Equal(got, want) {
			t.Errorf("ValueCounts(%s) offers %v, want %v", field, got, want)
		}
	}
}
