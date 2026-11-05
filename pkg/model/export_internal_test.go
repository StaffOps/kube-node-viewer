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

func TestExportBaseName(t *testing.T) {
	at := time.Date(2026, 11, 2, 15, 30, 12, 0, time.UTC)
	got := exportBase("arn:aws:eks:us-east-1:111122223333:cluster/prod", at)
	if got != "kube-node-viewer-arn_aws_eks_us-east-1_111122223333_cluster_prod-20261102-153012" {
		t.Errorf("got %s", got)
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
		if n.Name() == "ip-10-0-12-15.ec2.internal" {
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
	at := time.Now()
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
