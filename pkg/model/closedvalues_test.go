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
	"path/filepath"
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

// packageConsts parses the non-test files of package model and returns its
// string constants: those of type CapacityType (declared with the type, as
// CapacityType("x"), or inheriting it in a const block) and the others.
func packageConsts(t *testing.T) (capacities map[string]string, others map[string]string) {
	t.Helper()
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	capacities, others = map[string]string{}, map[string]string{}
	fset := token.NewFileSet()
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			capacityBlock := false // implicit repetition keeps the last type
			for _, spec := range gen.Specs {
				vs := spec.(*ast.ValueSpec)
				isCapacity := capacityBlock && vs.Type == nil && len(vs.Values) == 0
				if id, ok := vs.Type.(*ast.Ident); ok {
					isCapacity = id.Name == "CapacityType"
				}
				for i, name := range vs.Names {
					if i >= len(vs.Values) {
						continue // iota repetition: no literal to read
					}
					expr := vs.Values[i]
					if call, ok := expr.(*ast.CallExpr); ok && len(call.Args) == 1 {
						if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "CapacityType" {
							isCapacity, expr = true, call.Args[0]
						}
					}
					lit, ok := expr.(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						continue
					}
					value, _ := strconv.Unquote(lit.Value)
					if isCapacity {
						capacities[name.Name] = value
					} else {
						others[name.Name] = value
					}
				}
				if vs.Type != nil || len(vs.Values) > 0 {
					capacityBlock = isCapacity
				}
			}
		}
	}
	return capacities, others
}

// fix-config-filter-values R3: every CapacityType constant and every Status…
// string constant of package model is in its closed list, so a value added
// without the list fails here.
func TestClosedValueListsHoldEveryConstant(t *testing.T) {
	capacities, others := packageConsts(t)
	statuses := 0
	for name, value := range capacities {
		if !slices.Contains(model.CapacityValues, value) {
			t.Errorf("CapacityType %s = %q is not in CapacityValues %v", name, value, model.CapacityValues)
		}
	}
	for name, value := range others {
		if !strings.HasPrefix(name, "Status") {
			continue
		}
		statuses++
		if !slices.Contains(model.StatusValues, value) {
			t.Errorf("%s = %q is not in StatusValues %v", name, value, model.StatusValues)
		}
	}
	if len(capacities) == 0 || statuses == 0 {
		t.Fatalf("parsed no constants: %d capacity, %d status", len(capacities), statuses)
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
