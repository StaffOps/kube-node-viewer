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

package model

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	v1 "k8s.io/api/core/v1"

	"github.com/staffops/kube-node-viewer/pkg/recommend"
)

const liveSub = "00000000-0000-0000-0000-000000000000"

// recordingAzure records the pairs of each Begin and when Types is read.
type recordingAzure struct {
	fakeAzureAdvisor
	pairs  *[][]recommend.Pair
	events *[]string
}

func (a recordingAzure) Begin(p []recommend.Pair) {
	*a.pairs = append(*a.pairs, p)
	*a.events = append(*a.events, "begin")
}

func (a recordingAzure) Types() []string {
	*a.events = append(*a.events, "types")
	return nil
}

func (recordingAzure) Available(recommend.Node, string) bool { return true }
func (recordingAzure) SizesLoaded(recommend.Node) bool       { return true }

// liveUI is aksUI with provider IDs in the zero subscription.
func liveUI(t *testing.T) *UIModel {
	u := aksUI(t)
	for _, n := range u.stats.Nodes {
		obj := n.Object().DeepCopy()
		obj.Spec.ProviderID = strings.Replace(obj.Spec.ProviderID, "/subscriptions/s/", "/subscriptions/"+liveSub+"/", 1)
		n.Update(obj)
	}
	u.Refresh()
	return u
}

// azure-live-skus R1, D2: the input gathers one pair per subscription and
// region of the comparable Azure nodes, with their pools, and passes it to
// Begin before reading the types; virtual and typeless nodes add none.
func TestPairsBeforeTypes(t *testing.T) {
	u := liveUI(t)
	var pairs [][]recommend.Pair
	var events []string
	u.advisor = AdvisorsByCloud{recommend.CloudAWS: fakeAdvisor{}, recommend.CloudAzure: recordingAzure{pairs: &pairs, events: &events}}
	u.recommendInput()
	if len(pairs) != 1 || len(pairs[0]) != 1 {
		t.Fatalf("pairs %v", pairs)
	}
	p := pairs[0][0]
	pools := append([]string(nil), p.AgentPools...)
	sort.Strings(pools)
	if p.Subscription != liveSub || p.Region != "eastus2" || strings.Join(pools, ",") != "system,workers" {
		t.Errorf("pair %+v", p)
	}
	if strings.Join(events, ",") != "begin,types" {
		t.Errorf("order %v, want Begin before Types", events)
	}
	for _, n := range u.stats.Nodes {
		obj := n.Object().DeepCopy()
		if strings.Contains(n.Name(), "system") {
			obj.Labels["type"] = "virtual-kubelet"
		} else {
			delete(obj.Labels, v1.LabelInstanceTypeStable)
		}
		n.Update(obj)
	}
	u.Refresh()
	pairs = nil
	u.recommendInput()
	if len(pairs) != 1 || len(pairs[0]) != 0 {
		t.Errorf("virtual and typeless nodes gave pairs: %v", pairs)
	}
}

// azure-live-skus R1: an AWS-only or empty view gives no pair, so no az run
// and no request.
func TestNoPairsWithoutAzureNodes(t *testing.T) {
	u := demoUI(t)
	var pairs [][]recommend.Pair
	var events []string
	u.advisor = AdvisorsByCloud{recommend.CloudAWS: fakeAdvisor{}, recommend.CloudAzure: recordingAzure{pairs: &pairs, events: &events}}
	u.recommendInput()
	u.view.Search = "no-such-node"
	u.recommendInput()
	for _, p := range pairs {
		if len(p) != 0 {
			t.Errorf("pairs without Azure nodes: %v", pairs)
		}
	}
	if got := subscriptionOf("aws:///us-east-1a/i-0123456789abcdef0"); got != "" {
		t.Errorf("an AWS provider ID gave subscription %q", got)
	}
	if got := subscriptionOf("azure:///subscriptions/" + liveSub + "/resourceGroups/example-rg/x"); got != liveSub {
		t.Errorf("subscriptionOf = %q", got)
	}
}

// azure-live-skus R8: the subscription id is nowhere on screen, matched by
// no search, and in neither export.
func TestSubscriptionNeverShown(t *testing.T) {
	u := liveUI(t)
	u.advisor = AdvisorsByCloud{recommend.CloudAWS: fakeAdvisor{}, recommend.CloudAzure: fakeAzureAdvisor{}}
	screens := []string{u.View()}
	pressAll(u, "tab")
	screens = append(screens, u.View())
	for _, s := range screens {
		if strings.Contains(s, liveSub) {
			t.Errorf("the subscription is on screen:\n%s", s)
		}
	}
	u.view.Search = liveSub
	if got := u.view.Select(u.stats.Nodes); len(got) != 0 {
		t.Errorf("a search for the subscription matched %d nodes", len(got))
	}
	u.view.Search = ""
	doc := u.exportDoc(timeNow())
	raw, _ := json.Marshal(doc)
	path := filepath.Join(t.TempDir(), "x.csv")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeCSV(path, doc); err != nil {
		t.Fatal(err)
	}
	csv, _ := os.ReadFile(path)
	if strings.Contains(string(raw), liveSub) || strings.Contains(string(csv), liveSub) {
		t.Error("the subscription is in the export")
	}
}
