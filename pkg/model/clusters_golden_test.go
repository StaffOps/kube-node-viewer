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
	"slices"
	"testing"
)

// fakeFleet records what the UI asks of the Clusters screen's fleet.
type fakeFleet struct {
	infos  []ContextInfo
	wanted [][]string
	scopes []string
	pods   []string
}

func (f *fakeFleet) Contexts() ([]ContextInfo, error) { return f.infos, nil }
func (f *fakeFleet) SetWanted(c []string, scope string) {
	f.wanted = append(f.wanted, slices.Clone(c))
	f.scopes = append(f.scopes, scope)
}
func (f *fakeFleet) WatchPods(name string) { f.pods = append(f.pods, name) }

// last is the last set of wanted members.
func (f *fakeFleet) last() []string {
	if len(f.wanted) == 0 {
		return nil
	}
	return f.wanted[len(f.wanted)-1]
}

type fakeFleetStore struct {
	contexts []string
	cloud    string
	ok       bool
	saves    int
}

func (s *fakeFleetStore) Fleet() ([]string, string, bool) { return s.contexts, s.cloud, s.ok }
func (s *fakeFleetStore) SaveFleet(c []string, cloud string) {
	s.contexts, s.cloud, s.ok = slices.Clone(c), cloud, true
	s.saves++
}

// clustersUI is the made-up EKS cluster on screen, with the made-up AKS
// cluster as a connected member whose pods are not watched yet.
func clustersUI(t *testing.T) *UIModel {
	u := demoUI(t)
	aks := aksUI(t)
	f := &fakeFleet{infos: []ContextInfo{
		{Name: "demo-prod", Server: "https://0A1B2C3D4E5F.gr7.us-east-1.eks.amazonaws.com", Cloud: "aws"},
		{Name: "aks", Server: "https://example-aks.hcp.eastus2.azmk8s.io:443", Cloud: "azure"},
	}}
	u.SetFleet(f, &fakeFleetStore{contexts: []string{"demo-prod", "aks"}, ok: true})
	return withMember(u, "aks", aks.cluster, ConnStatus{Synced: true})
}

// withMember opens the Clusters screen and connects one member.
func withMember(u *UIModel, name string, c *Cluster, st ConnStatus) *UIModel {
	pressAll(u, ":", "c", "l", "enter")
	run(u, MemberReadyMsg{Context: name, Gen: c.Generation(), Cluster: c, Status: func() ConnStatus { return st }})
	return u
}

func init() {
	goldenScenes = append(goldenScenes,
		struct {
			name          string
			width, height int
			keys          func(u *UIModel)
		}{"clusters", 168, 38, func(u *UIModel) {}},
		struct {
			name          string
			width, height int
			keys          func(u *UIModel)
		}{"clusters-unfolded", 168, 40, func(u *UIModel) {
			v := u.view
			v.SetGroupBy(FieldNodeGroup)
			u.SetView(v)
			u.View()
			u.cursorKey = "c:demo-prod"
			press(u, " ")
		}},
		// clusters-screen R15: an unfolded member whose pods are not synced
		struct {
			name          string
			width, height int
			keys          func(u *UIModel)
		}{"clusters-pending", 168, 30, func(u *UIModel) {
			v := u.view
			v.SetGroupBy(FieldNodeGroup)
			u.SetView(v)
			u.View()
			u.cursorKey = "c:aks"
			press(u, " ")
		}},
		struct {
			name          string
			width, height int
			keys          func(u *UIModel)
		}{"clusters-picker", 168, 38, func(u *UIModel) { press(u, "c") }},
	)
	goldenSetup["clusters"] = clustersUI
	goldenSetup["clusters-unfolded"] = clustersUI
	goldenSetup["clusters-picker"] = clustersUI
	goldenSetup["clusters-pending"] = clustersUI
}
