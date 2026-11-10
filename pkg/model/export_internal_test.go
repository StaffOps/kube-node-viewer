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
	"encoding/csv"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func readCSV(t *testing.T, path string) [][]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func column(t *testing.T, rows [][]string, name string) int {
	t.Helper()
	for i, h := range rows[0] {
		if h == name {
			return i
		}
	}
	t.Fatalf("no column %q in %v", name, rows[0])
	return -1
}

// short-gke-context R3
func TestExportBaseName(t *testing.T) {
	at := time.Date(2026, 11, 2, 15, 30, 12, 0, time.UTC)
	for _, tc := range []struct{ context, want string }{
		{"arn:aws:eks:us-east-1:111122223333:cluster/prod", "prod"},
		{"gke_example-project_us-central1-a_demo-prod", "demo-prod"},
		{"staging", "staging"},
		{"team/a b", "team_a_b"},
		// not exactly an ARN: reduced as before, nothing shortened
		{"arn:aws:eks:us-east-1:11112222:cluster/prod", "arn_aws_eks_us-east-1_11112222_cluster_prod"},
		{"", "cluster"},
	} {
		want := "kube-node-viewer-" + tc.want + "-20261102-153012"
		if got := exportBase(tc.context, at); got != want {
			t.Errorf("exportBase(%q) = %s, want %s", tc.context, got, want)
		}
	}
}

// short-gke-context R3: the files and the footer notice carry no account or
// project id; the JSON keeps the full context
func TestExportFileNameIsShort(t *testing.T) {
	for _, tc := range []struct{ context, secret string }{
		{"arn:aws:eks:us-east-1:111122223333:cluster/demo-prod", "111122223333"},
		{"gke_example-project_us-central1-a_demo-prod", "example-project"},
	} {
		dir := t.TempDir()
		t.Chdir(dir)
		u := demoUI(t)
		u.SetClusterInfo(ClusterInfo{Context: tc.context, Server: "https://a.example"})
		doc := u.exportDoc(time.Now())
		msg := writeExport(doc, dir, "")
		if msg.err != nil {
			t.Fatal(msg.err)
		}
		u.exported(msg)
		for _, name := range []string{msg.csv, msg.json, u.currentNotice()} {
			if strings.Contains(name, tc.secret) || !strings.Contains(name, "kube-node-viewer-demo-prod-") {
				t.Errorf("%q: name or notice %q", tc.context, name)
			}
		}
		var got exportDoc
		data, err := os.ReadFile(msg.json)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatal(err)
		}
		if got.Context != tc.context {
			t.Errorf("JSON context %q, want the full %q", got.Context, tc.context)
		}
	}
}

// hide-cloud-ids R4 R10
func TestFleetNameInExportAndPopup(t *testing.T) {
	for _, context := range fleetContexts() {
		dir := t.TempDir()
		t.Chdir(dir)
		server := fleetServers[0].server
		u := demoUI(t)
		u.SetClusterInfo(ClusterInfo{Context: context, Server: server})
		msg := writeExport(u.exportDoc(time.Now()), dir, "")
		if msg.err != nil {
			t.Fatal(msg.err)
		}
		u.exported(msg)
		for _, name := range []string{msg.csv, msg.json, u.currentNotice()} {
			if strings.Contains(name, "example-project") || !strings.Contains(name, "kube-node-viewer-demo-prod-") {
				t.Errorf("%q: name or notice %q", context, name)
			}
		}
		var got exportDoc
		data, err := os.ReadFile(msg.json)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatal(err)
		}
		if got.Context != context || got.Server != server {
			t.Errorf("JSON context %q server %q, want the full %q and %q", got.Context, got.Server, context, server)
		}

		u.panel = panelContexts
		u.contextsLoaded(contextsMsg{names: []string{context, "staging"}, current: context})
		if rows := strings.Join(u.contextsPanel().rows, "\n"); !strings.Contains(rows, context) {
			t.Errorf("contexts popup rows:\n%s", rows)
		}
		u.contexts.switching = context
		if rows := strings.Join(u.contextsPanel().rows, "\n"); !strings.Contains(rows, "connecting to "+context) {
			t.Errorf("switching row:\n%s", rows)
		}
	}
}

// short-edge-context R4
func TestEdgeNameInExportAndPopup(t *testing.T) {
	for _, context := range edgeContexts() {
		dir := t.TempDir()
		t.Chdir(dir)
		u := demoUI(t)
		u.SetClusterInfo(ClusterInfo{Context: context, Server: "https://edge.example:443"})
		msg := writeExport(u.exportDoc(time.Now()), dir, "")
		if msg.err != nil {
			t.Fatal(msg.err)
		}
		u.exported(msg)
		for _, name := range []string{msg.csv, msg.json, u.currentNotice()} {
			if strings.Contains(name, "example-project") || strings.Contains(name, "ops_user") ||
				!strings.Contains(name, "kube-node-viewer-demo-prod-") {
				t.Errorf("%q: name or notice %q", context, name)
			}
		}
		var got exportDoc
		data, err := os.ReadFile(msg.json)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatal(err)
		}
		if got.Context != context {
			t.Errorf("JSON context %q, want the full %q", got.Context, context)
		}

		u.panel = panelContexts
		u.contextsLoaded(contextsMsg{names: []string{context, "staging"}, current: context})
		if rows := strings.Join(u.contextsPanel().rows, "\n"); !strings.Contains(rows, context) {
			t.Errorf("contexts popup rows:\n%s", rows)
		}
		u.contexts.switching = context
		if rows := strings.Join(u.contextsPanel().rows, "\n"); !strings.Contains(rows, "connecting to "+context) {
			t.Errorf("switching row:\n%s", rows)
		}
	}
}

// short-gke-context R3: the fallback directory gets the short name too
func TestExportFallbackUsesTheShortName(t *testing.T) {
	ro := t.TempDir()
	if err := os.Chmod(ro, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ro, 0o700) })
	fallback := filepath.Join(t.TempDir(), "state")
	doc := exportDoc{Context: "arn:aws:eks:us-east-1:111122223333:cluster/prod", Generated: time.Date(2026, 11, 2, 15, 30, 12, 0, time.UTC)}
	msg := writeExport(doc, ro, fallback)
	if msg.err != nil || !msg.fellBack {
		t.Fatalf("fallback: %+v", msg)
	}
	if got, want := filepath.Base(msg.csv), "kube-node-viewer-prod-20261102-153012.csv"; got != want || filepath.Dir(msg.csv) != fallback {
		t.Errorf("csv %s, want %s in %s", msg.csv, want, fallback)
	}
}

// short-gke-context R3: same-named clusters exported in the same second
func TestSameShortNameDoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	at := time.Date(2026, 11, 2, 15, 30, 12, 0, time.UTC)
	first := writeExport(exportDoc{Context: "arn:aws:eks:us-east-1:111122223333:cluster/prod", Generated: at}, dir, "")
	second := writeExport(exportDoc{Context: "gke_example-project_us-central1-a_prod", Generated: at}, dir, "")
	if first.err != nil || second.err != nil {
		t.Fatal(first.err, second.err)
	}
	if first.csv == second.csv || !strings.Contains(second.csv, "kube-node-viewer-prod-20261102-153012-2") {
		t.Errorf("csv files %s and %s", first.csv, second.csv)
	}
	data, err := os.ReadFile(first.json)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "111122223333") {
		t.Error("the first export was overwritten")
	}
}

// views-and-export R3: e writes every kept node to CSV and JSON.
func TestExportWritesTheFilteredNodes(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	u := demoUI(t)
	// one node without a price, one reserved (equivalent price)
	// stats.Nodes comes from a map: pick a node by name, not by position
	for _, n := range u.stats.Nodes {
		if n.Name() == "ip-192-0-2-24.ec2.internal" {
			n.SetPrice(math.NaN())
		}
	}
	v := u.view
	v.Filters = []Filter{{Field: FieldZone, Values: []string{"us-east-1b"}, Enabled: true}}
	v.SetGroupBy(FieldNodeGroup)
	v.Columns = append(v.Columns, LabelField("karpenter.sh/nodepool"))
	u.SetView(v)

	apply(u, press(u, "e"))
	if !strings.Contains(u.View(), "exported 4 nodes to ./kube-node-viewer-demo-prod-") {
		t.Fatalf("footer:\n%s", u.View())
	}
	csvs, _ := filepath.Glob(filepath.Join(dir, "*.csv"))
	jsons, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	if len(csvs) != 1 || len(jsons) != 1 {
		t.Fatalf("files: %v %v", csvs, jsons)
	}
	rows := readCSV(t, csvs[0])
	if len(rows) != 5 {
		t.Fatalf("want header + 4 rows, got %d", len(rows))
	}
	kinds := map[string]bool{}
	for _, r := range rows[1:] {
		kinds[r[column(t, rows, "price_kind")]] = true
		if r[column(t, rows, "zone")] != "us-east-1b" {
			t.Errorf("a filtered-out node was exported: %v", r)
		}
		if r[column(t, rows, "group")] == "" {
			t.Errorf("no group path: %v", r)
		}
	}
	for _, k := range []string{"actual", "equivalent"} {
		if !kinds[k] {
			t.Errorf("price kind %s missing: %v", k, kinds)
		}
	}
	column(t, rows, "cpu_percent")
	column(t, rows, "karpenter.sh/nodepool")

	var doc exportDoc
	data, _ := os.ReadFile(jsons[0])
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Shown.Nodes != 4 || doc.Cluster.Nodes != 10 || doc.Context != "demo-prod" || len(doc.Nodes) != 4 {
		t.Errorf("json: shown %d cluster %d context %q nodes %d", doc.Shown.Nodes, doc.Cluster.Nodes, doc.Context, len(doc.Nodes))
	}
}

func TestExportUnknownAndDisabledPrices(t *testing.T) {
	u := demoUI(t)
	n := u.stats.Nodes[0]
	n.SetPrice(math.NaN())
	at := timeNow()
	if got := u.exportNode(n, nil, at); got.PriceKind != "unknown" || got.PricePerHour != nil {
		t.Errorf("unknown: %+v", got)
	}
	u.DisablePricing = true
	if got := u.exportNode(u.stats.Nodes[1], nil, at); got.PriceKind != "disabled" || got.PricePerHour != nil {
		t.Errorf("disabled: %+v", got)
	}
}

func TestExportFallsBackWhenTheDirectoryIsReadOnly(t *testing.T) {
	ro := t.TempDir()
	if err := os.Chmod(ro, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ro, 0o700) })
	fallback := filepath.Join(t.TempDir(), "state")
	doc := exportDoc{Context: "c", Generated: time.Now()}
	msg := writeExport(doc, ro, fallback)
	if msg.err != nil || !msg.fellBack || filepath.Dir(msg.csv) != fallback {
		t.Errorf("fallback: %+v", msg)
	}
}

func TestExportNamesNeverCollide(t *testing.T) {
	dir := t.TempDir()
	doc := exportDoc{Context: "c", Generated: time.Date(2026, 11, 2, 1, 2, 3, 0, time.UTC)}
	first := writeExport(doc, dir, "")
	second := writeExport(doc, dir, "")
	if first.err != nil || second.err != nil || first.csv == second.csv || !strings.HasSuffix(second.csv, "-2.csv") {
		t.Errorf("first %+v second %+v", first, second)
	}
}

func TestCSVCellsAreNotFormulas(t *testing.T) {
	for in, want := range map[string]string{"=SUM(A1)": "'=SUM(A1)", "+1": "'+1", "-x": "'-x", "@me": "'@me", "ok": "ok", "": ""} {
		if got := cell(in); got != want {
			t.Errorf("cell(%q) = %q, want %q", in, got, want)
		}
	}
}
