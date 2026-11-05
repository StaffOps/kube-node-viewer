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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	v1 "k8s.io/api/core/v1"
)

// SetExportFallback sets the directory exports go to when the current one is
// not writable (views-and-export R3), the state directory.
func (u *UIModel) SetExportFallback(dir string) {
	u.exportFallback = dir
}

// exportResource is one resource of a node: requested, allocatable, percent.
type exportResource struct {
	Requested   string  `json:"requested"`
	Allocatable string  `json:"allocatable"`
	Percent     float64 `json:"percent"`
}

// exportNode is a node as exported, every field whatever columns are shown.
type exportNode struct {
	Name         string                    `json:"name"`
	Group        []string                  `json:"group,omitempty"`
	InstanceType string                    `json:"instanceType"`
	Capacity     string                    `json:"capacity"`
	NodeGroup    string                    `json:"nodeGroup"`
	Zone         string                    `json:"zone"`
	Region       string                    `json:"region"`
	IP           string                    `json:"ip"`
	Status       string                    `json:"status"`
	Created      time.Time                 `json:"created"`
	Age          string                    `json:"age"`
	Pods         int                       `json:"pods"`
	MaxPods      int64                     `json:"maxPods"`
	Resources    map[string]exportResource `json:"resources"`
	PricePerHour *float64                  `json:"pricePerHour,omitempty"`
	PriceKind    string                    `json:"priceKind"`
	Labels       map[string]string         `json:"labels,omitempty"`
}

// exportTotals is usage and cost summed over a set of nodes.
type exportTotals struct {
	Nodes             int                       `json:"nodes"`
	Resources         map[string]exportResource `json:"resources"`
	PricePerHour      *float64                  `json:"pricePerHour,omitempty"`
	PricePerMonth     *float64                  `json:"pricePerMonth,omitempty"`
	ReservedNotPriced int                       `json:"reservedNotPriced"`
	// UnknownPrice counts nodes with no price: the totals leave them out
	UnknownPrice int `json:"unknownPrice"`
}

// exportFocus is one level of the focus: a field and the value kept.
type exportFocus struct {
	Field string `json:"field"`
	Value string `json:"value"`
}

// exportView is the view as the JSON export writes it: the saved view plus
// the search text, which the state file never saves (fix-export-search).
type exportView struct {
	ViewState
	Search string `json:"search,omitempty"`
}

// exportDoc is the JSON export; its nodes are also the CSV rows.
type exportDoc struct {
	Context   string       `json:"context"`
	Server    string       `json:"server"`
	Generated time.Time    `json:"generated"`
	View      exportView   `json:"view"`
	Shown     exportTotals `json:"shown"`
	Cluster   exportTotals `json:"cluster"`
	// Focus is the group focused on Overview (group-focus R8)
	Focus     []exportFocus `json:"focus,omitempty"`
	Labels    []string      `json:"labelColumns,omitempty"`
	Resources []string      `json:"resources"`
	Nodes     []exportNode  `json:"nodes"`
}

type exportDoneMsg struct {
	csv, json string
	nodes     int
	fellBack  bool
	err       error
}

// export starts writing the nodes the filters and search keep, every page and
// folded group included. The rows are built here, on the UI goroutine, from
// the frame's data; only plain values go to the command.
func (u *UIModel) export() tea.Cmd {
	doc := u.exportDoc(time.Now())
	fallback := u.exportFallback
	return func() tea.Msg {
		dir, err := os.Getwd()
		if err != nil {
			dir = fallback
		}
		return writeExport(doc, dir, fallback)
	}
}

func (u *UIModel) exportDoc(now time.Time) exportDoc {
	nodes := u.view.Select(u.stats.Nodes)
	var focus []exportFocus
	if !u.onRecommendations() {
		nodes = u.focusNodes(nodes)
		for _, l := range u.focusLevels() {
			focus = append(focus, exportFocus{Field: fieldName(l.Field), Value: l.Value})
		}
	}
	var labels []string
	for _, c := range u.view.Columns {
		if key := c.Label(); key != "" {
			labels = append(labels, key)
		}
	}
	doc := exportDoc{
		Context: u.info.Context, Server: u.info.Server, Generated: now,
		View:   exportView{ViewState: u.view.forSaving(), Search: u.view.Search},
		Labels: labels, Focus: focus,
	}
	for _, r := range u.cluster.resources {
		doc.Resources = append(doc.Resources, string(r))
	}
	shown := newGroupStats("")
	for _, n := range nodes {
		shown.add(n)
		doc.Nodes = append(doc.Nodes, u.exportNode(n, labels, now))
	}
	doc.Shown = u.exportTotals(shown.NumNodes, shown.AllocatableResources, shown.UsedResources, shown.TotalPrice, shown.PricedNodes, shown.ReservedNodes)
	s := u.stats
	doc.Cluster = u.exportTotals(s.NumNodes, s.AllocatableResources, s.UsedResources, s.TotalPrice, s.PricedNodes, s.UnpricedReserved)
	return doc
}

func (u *UIModel) exportTotals(nodes int, allocatable, used v1.ResourceList, price float64, priced, reserved int) exportTotals {
	t := exportTotals{Nodes: nodes, Resources: map[string]exportResource{}, ReservedNotPriced: reserved}
	for _, r := range u.cluster.resources {
		a, us := allocatable[r], used[r]
		t.Resources[string(r)] = exportResource{Requested: us.String(), Allocatable: a.String(), Percent: percent(us.AsApproximateFloat64(), a.AsApproximateFloat64())}
	}
	if u.DisablePricing {
		return t
	}
	t.UnknownPrice = nodes - priced - reserved
	// no node priced, as on a cloud without a price source: no total, not $0
	if priced > 0 {
		month := price * avgHoursPerMonth
		t.PricePerHour, t.PricePerMonth = &price, &month
	}
	return t
}

func percent(used, allocatable float64) float64 {
	if allocatable == 0 {
		return 0
	}
	return float64(int(used/allocatable*1000+0.5)) / 10
}

func (u *UIModel) exportNode(n *Node, labels []string, now time.Time) exportNode {
	allocatable, used := n.Allocatable(), n.Used()
	maxPods, _ := allocatable.Pods().AsInt64()
	e := exportNode{
		Name:         n.Name(),
		InstanceType: n.InstanceType(),
		Capacity:     string(n.CapacityType()),
		NodeGroup:    n.NodeGroup(),
		Zone:         n.Zone(),
		Region:       n.Region(),
		IP:           n.InternalIP(),
		Status:       strings.Join(n.StatusFlags(), "/"),
		Created:      n.Created(),
		Age:          now.Sub(n.Created()).Round(time.Second).String(),
		Pods:         n.NumPods(),
		MaxPods:      maxPods,
		Resources:    map[string]exportResource{},
	}
	for _, f := range u.view.GroupBy {
		e.Group = append(e.Group, f.Value(n))
	}
	for _, r := range u.cluster.resources {
		a, us := allocatable[r], used[r]
		e.Resources[string(r)] = exportResource{Requested: us.String(), Allocatable: a.String(), Percent: percent(us.AsApproximateFloat64(), a.AsApproximateFloat64())}
	}
	switch {
	case u.DisablePricing:
		e.PriceKind = "disabled"
	case !n.HasPrice():
		e.PriceKind = "unknown"
	case n.PriceIsEquivalent():
		e.PriceKind = "equivalent"
	default:
		e.PriceKind = "actual"
	}
	if e.PriceKind == "actual" || e.PriceKind == "equivalent" {
		p := n.Price()
		e.PricePerHour = &p
	}
	if len(labels) > 0 {
		e.Labels = map[string]string{}
		for _, l := range labels {
			v := n.LabelValue(l)
			if v == UngroupedValue {
				v = ""
			}
			e.Labels[l] = v
		}
	}
	return e
}

var unsafeNameChars = regexp.MustCompile(`[^A-Za-z0-9._-]`)

// exportBase is kube-node-viewer-<context>-<YYYYMMDD-HHMMSS>, the context
// reduced to characters every filesystem takes.
func exportBase(context string, now time.Time) string {
	if context == "" {
		context = "cluster"
	}
	return "kube-node-viewer-" + unsafeNameChars.ReplaceAllString(context, "_") + "-" + now.Format("20060102-150405")
}

// writeExport writes the CSV and JSON files in dir, or in fallback when dir
// is not writable. Files are created exclusively; a name in use gets -2, -3…
func writeExport(doc exportDoc, dir, fallback string) exportDoneMsg {
	msg := exportDoneMsg{nodes: len(doc.Nodes)}
	csvPath, jsonPath, err := createPair(dir, exportBase(doc.Context, doc.Generated))
	if err != nil && fallback != "" && fallback != dir && !errors.Is(err, os.ErrExist) {
		msg.fellBack = true
		if mkErr := os.MkdirAll(fallback, 0o700); mkErr == nil {
			csvPath, jsonPath, err = createPair(fallback, exportBase(doc.Context, doc.Generated))
		}
	}
	if err != nil {
		msg.err = err
		return msg
	}
	if err := writeCSV(csvPath, doc); err != nil {
		msg.err = err
		return msg
	}
	if err := writeJSON(jsonPath, doc); err != nil {
		msg.err = err
		return msg
	}
	msg.csv, msg.json = csvPath, jsonPath
	return msg
}

// createPair creates base.csv and base.json in dir, both new, trying base-2,
// base-3… while either name is taken.
func createPair(dir, base string) (string, string, error) {
	for i := 1; i < 100; i++ {
		name := base
		if i > 1 {
			name = fmt.Sprintf("%s-%d", base, i)
		}
		csvPath, jsonPath := filepath.Join(dir, name+".csv"), filepath.Join(dir, name+".json")
		c, err := os.OpenFile(csvPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", "", err
		}
		c.Close()
		j, err := os.OpenFile(jsonPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if errors.Is(err, os.ErrExist) {
			os.Remove(csvPath)
			continue
		}
		if err != nil {
			os.Remove(csvPath)
			return "", "", err
		}
		j.Close()
		return csvPath, jsonPath, nil
	}
	return "", "", fmt.Errorf("no free file name for %s in %s", base, dir)
}

// cell keeps spreadsheets from running a value as a formula.
func cell(s string) string {
	if s != "" && strings.ContainsRune("=+-@", rune(s[0])) {
		return "'" + s
	}
	return s
}

func writeCSV(path string, doc exportDoc) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	header := []string{"name", "group", "instance_type", "capacity", "node_group", "zone", "region", "ip", "status", "age", "pods", "max_pods"}
	for _, r := range doc.Resources {
		header = append(header, r+"_requested", r+"_allocatable", r+"_percent")
	}
	header = append(header, "price_per_hour", "price_kind")
	header = append(header, doc.Labels...)
	if err := w.Write(header); err != nil {
		return err
	}
	for _, n := range doc.Nodes {
		row := []string{n.Name, strings.Join(n.Group, " › "), n.InstanceType, n.Capacity, n.NodeGroup, n.Zone, n.Region, n.IP, n.Status, n.Age,
			strconv.Itoa(n.Pods), strconv.FormatInt(n.MaxPods, 10)}
		for _, r := range doc.Resources {
			res := n.Resources[r]
			row = append(row, res.Requested, res.Allocatable, strconv.FormatFloat(res.Percent, 'f', 1, 64))
		}
		price := ""
		if n.PricePerHour != nil {
			price = strconv.FormatFloat(*n.PricePerHour, 'f', 4, 64)
		}
		row = append(row, price, n.PriceKind)
		for _, l := range doc.Labels {
			row = append(row, n.Labels[l])
		}
		for i := range row {
			row[i] = cell(row[i])
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func writeJSON(path string, doc exportDoc) error {
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	// an export is a file for the user's spreadsheet or report, readable like
	// any other they create
	return os.WriteFile(path, append(data, '\n'), 0o644) //nolint:gosec // G306: see above
}

// exported tells, in the footer, where the files went or why they didn't.
func (u *UIModel) exported(msg exportDoneMsg) {
	if msg.err != nil {
		u.notify("export failed: " + msg.err.Error())
		return
	}
	where := msg.csv
	if wd, err := os.Getwd(); err == nil && !msg.fellBack {
		if rel, err := filepath.Rel(wd, msg.csv); err == nil {
			where = "./" + rel
		}
	}
	note := ""
	if msg.fellBack {
		note = " (current directory not writable)"
	}
	u.notify(fmt.Sprintf("exported %d nodes to %s and .json%s", msg.nodes, where, note))
}
