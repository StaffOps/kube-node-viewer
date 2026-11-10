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
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/staffops/kube-node-viewer/pkg/azure"
	"github.com/staffops/kube-node-viewer/pkg/model"
	"github.com/staffops/kube-node-viewer/pkg/recommend"
)

const liveSubscription = "00000000-0000-0000-0000-000000000000"

// listPrices is a PriceLister with fixed regular prices.
type listPrices map[string]float64

func (l listPrices) ListPrice(_, size string, _, spot bool) (float64, bool, bool) {
	if spot {
		return 0, false, false
	}
	p, ok := l[size]
	return p, ok, false
}
func (listPrices) Generation() uint64 { return 0 }

type awsNone struct{}

func (awsNone) Spec(string) (recommend.Spec, bool)              { return recommend.Spec{}, false }
func (awsNone) OnDemand(recommend.Node, string) (float64, bool) { return 0, false }
func (awsNone) SpotIn(recommend.Node, string) (float64, bool)   { return 0, false }
func (awsNone) Types() []string                                 { return nil }

// pump sends msg and runs the commands it returns, for at most half a
// second each (ticks are left behind).
func pump(u *model.UIModel, msg tea.Msg) {
	var run func(tea.Cmd)
	run = func(cmd tea.Cmd) {
		if cmd == nil {
			return
		}
		ch := make(chan tea.Msg, 1)
		go func() { ch <- cmd() }()
		select {
		case m := <-ch:
			if b, ok := m.(tea.BatchMsg); ok {
				for _, c := range b {
					run(c)
				}
				return
			}
			if m != nil {
				_, next := u.Update(m)
				run(next)
			}
		case <-time.After(500 * time.Millisecond):
		}
	}
	_, cmd := u.Update(msg)
	run(cmd)
}

// azure-live-skus R1, D2: with the real Azure advisor, a fake az and a fake
// Resource Manager, nothing is asked before Recommendations is shown; then
// the screen says the sizes are loading and, with no change to the cluster,
// shows the rightsize row once they arrive (the advisor's generation moves).
func TestLiveSizesOnAStaticCluster(t *testing.T) {
	raw, err := os.ReadFile("../azure/testdata/skus-eastus.json")
	if err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	arm := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release
		_, _ = w.Write(raw)
	}))
	defer arm.Close()
	var runs atomic.Int32
	az := func(_ context.Context, args ...string) ([]byte, []byte, error) {
		runs.Add(1)
		if args[1] == "list" {
			return []byte(`[{"cloudName":"AzureCloud","id":"` + liveSubscription + `","state":"Enabled"}]`), nil, nil
		}
		return []byte(fmt.Sprintf(`{"accessToken":"t","expires_on":%d}`, time.Now().Add(time.Hour).Unix())), nil, nil
	}
	sizes := azure.NewSizes(context.Background(), az, arm.URL, arm.Client().Transport)
	defer sizes.Close()

	c := model.NewCluster()
	n := model.NewNode(&v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "aks-legacy-12345678-vmss000000", CreationTimestamp: metav1.NewTime(time.Now().Add(-time.Hour)),
			Labels: map[string]string{v1.LabelTopologyRegion: "EastUS", v1.LabelTopologyZone: "0", v1.LabelInstanceTypeStable: "Standard_D4s_v3",
				"kubernetes.azure.com/agentpool": "legacy", v1.LabelOSStable: "linux"}},
		Spec: v1.NodeSpec{ProviderID: "azure:///subscriptions/" + liveSubscription + "/resourceGroups/MC_example-rg_example_eastus/providers/Microsoft.Compute/virtualMachineScaleSets/aks-legacy-12345678-vmss/virtualMachines/0"},
		Status: v1.NodeStatus{
			Capacity:    v1.ResourceList{v1.ResourceCPU: resource.MustParse("4"), v1.ResourceMemory: resource.MustParse("15974Mi"), v1.ResourcePods: resource.MustParse("250")},
			Allocatable: v1.ResourceList{v1.ResourceCPU: resource.MustParse("3860m"), v1.ResourceMemory: resource.MustParse("12185Mi"), v1.ResourcePods: resource.MustParse("250")},
			Conditions:  []v1.NodeCondition{{Type: v1.NodeReady, Status: v1.ConditionTrue}},
		},
	})
	c.AddNode(n).Show()
	n.SetPrice(0.192)

	style, err := model.ParseStyle("#04B575,#FFFF00,#FF0000")
	if err != nil {
		t.Fatal(err)
	}
	u := model.NewUIModel(style)
	u.Connect(model.Session{Cluster: c, Advisor: model.AdvisorsByCloud{
		recommend.CloudAWS:   awsNone{},
		recommend.CloudAzure: azure.NewAdvisor(listPrices{"Standard_D4s_v3": 0.192, "Standard_D2s_v3": 0.096}, false, sizes),
	}})
	pump(u, tea.WindowSizeMsg{Width: 200, Height: 50})
	pump(u, model.ChangedMsg{Gen: c.Generation()})
	for range 5 {
		pump(u, tea.WindowSizeMsg{Width: 200, Height: 50})
	}
	if runs.Load() != 0 {
		t.Fatalf("az ran %d times before Recommendations was shown", runs.Load())
	}
	pump(u, tea.KeyMsg{Type: tea.KeyTab})
	if out := u.View(); !strings.Contains(out, "Azure VM sizes: loading") {
		t.Errorf("no loading note:\n%s", out)
	}
	close(release)
	deadline := time.Now().Add(5 * time.Second)
	var out string
	for time.Now().Before(deadline) {
		pump(u, tea.WindowSizeMsg{Width: 200, Height: 50})
		if out = u.View(); strings.Contains(out, "→ Standard_D2s_v3") {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !strings.Contains(out, "→ Standard_D2s_v3") || strings.Contains(out, "Azure VM sizes") {
		t.Errorf("no rightsize row after the sizes arrived:\n%s", out)
	}
	if strings.Contains(out, liveSubscription) {
		t.Error("the subscription is on screen")
	}
	// another screen asks for nothing
	before := runs.Load()
	pump(u, tea.KeyMsg{Type: tea.KeyTab})
	for range 5 {
		pump(u, tea.WindowSizeMsg{Width: 200, Height: 50})
	}
	if runs.Load() != before {
		t.Errorf("az ran while another screen showed")
	}
}
