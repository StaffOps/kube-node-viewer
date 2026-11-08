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
	"strings"
	"testing"
)

// viewJSON is the "view" object of the export document, as written.
func viewJSON(t *testing.T, u *UIModel) string {
	t.Helper()
	raw, err := json.Marshal(u.exportDoc(timeNow()))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return string(doc["view"])
}

// fix-export-search R1: the search is written as view.search, as typed, after
// the saved view's keys.
func TestExportWritesTheSearch(t *testing.T) {
	for _, search := range []string{"c7g", "!c7g", "a(b", " c7g ", "!", "  "} {
		t.Run(search, func(t *testing.T) {
			u := demoUI(t)
			u.view.Search = search
			saved, _ := json.Marshal(u.view.forSaving())
			quoted, _ := json.Marshal(search)
			want := strings.TrimSuffix(string(saved), "}") + `,"search":` + string(quoted) + "}"
			if got := viewJSON(t, u); got != want {
				t.Errorf("view\n got %s\nwant %s", got, want)
			}
			raw, _ := json.Marshal(u.exportDoc(timeNow()))
			var doc exportDoc
			if err := json.Unmarshal(raw, &doc); err != nil {
				t.Fatal(err)
			}
			if doc.View.Search != search {
				t.Errorf("read back %q, want %q", doc.View.Search, search)
			}
		})
	}
}

// fix-export-search, unchanged behavior: which nodes are exported. A
// regression test: the nodes are still the ones the search keeps, a negated
// search included.
func TestExportWithANegatedSearchKeepsTheOtherNodes(t *testing.T) {
	u := demoUI(t)
	want := 0
	for _, n := range u.stats.Nodes {
		if !strings.HasPrefix(n.InstanceType(), "c7g") {
			want++
		}
	}
	u.view.Search = "!c7g"
	nodes := u.exportDoc(timeNow()).Nodes
	for _, n := range nodes {
		if strings.HasPrefix(n.InstanceType, "c7g") {
			t.Errorf("exported %s (%s) under !c7g", n.Name, n.InstanceType)
		}
	}
	if len(nodes) != want || want == 0 || want == len(u.stats.Nodes) {
		t.Errorf("exported %d nodes, want %d of %d", len(nodes), want, len(u.stats.Nodes))
	}
}

// fix-export-search R2: with no search the view is written as before.
func TestExportOmitsAnEmptySearch(t *testing.T) {
	u := demoUI(t)
	saved, _ := json.Marshal(u.view.forSaving())
	if got := viewJSON(t, u); got != string(saved) {
		t.Errorf("view\n got %s\nwant %s", got, saved)
	}
}
