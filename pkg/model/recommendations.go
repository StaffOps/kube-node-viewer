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
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	v1 "k8s.io/api/core/v1"

	"github.com/staffops/kube-node-viewer/pkg/recommend"
)

// Advisor gives the recommendations what they need from the cloud: the
// instance catalog and prices (recommendations D2).
type Advisor interface {
	Spec(instanceType string) (recommend.Spec, bool)
	OnDemand(instanceType string) (float64, bool)
	SpotIn(instanceType, zone string) (float64, bool)
	Types() []string
}

// RecommendOptions are the config-file settings of the rules (R12).
type RecommendOptions struct {
	Headroom float64
	OldAge   time.Duration
}

// SetRecommendOptions sets the rules' config-file settings (R12).
func (u *UIModel) SetRecommendOptions(opts RecommendOptions) {
	u.recOpts = opts
}

type recsMsg struct {
	key    string
	result recommend.Result
}

// recKey names the input of an analysis: the snapshot and the view's
// selection; a new key means a new analysis.
func (u *UIModel) recKey() string {
	return fmt.Sprintf("%d|%d|%v|%q|%v", u.gen, u.snapSeq, u.view.Filters, u.view.Search, u.view.Resources)
}

// maybeAnalyze starts an analysis when Recommendations is shown and its
// input changed. The input is built here, on the UI goroutine, from the
// snapshot; the rules run in the command (R3).
func (u *UIModel) maybeAnalyze() tea.Cmd {
	if !u.onRecommendations() || u.advisor == nil || u.recPending || u.shownCloud() != "aws" {
		return nil
	}
	key := u.recKey()
	if key == u.recFor {
		return nil
	}
	u.recPending = true
	in := u.recommendInput()
	return func() tea.Msg {
		return recsMsg{key: key, result: recommend.Analyze(in)}
	}
}

func (u *UIModel) recommendInput() recommend.Input {
	a := u.advisor
	in := recommend.Input{
		Now: time.Now(), Headroom: u.recOpts.Headroom, OldAge: u.recOpts.OldAge,
		PendingPods: u.stats.Unscheduled,
		Spec:        a.Spec, OnDemand: a.OnDemand, SpotIn: a.SpotIn, Types: a.Types(),
	}
	for _, n := range u.view.Select(u.stats.Nodes) {
		in.Nodes = append(in.Nodes, recommendNode(n))
	}
	return in
}

func recommendNode(n *Node) recommend.Node {
	res := func(l v1.ResourceList) recommend.Resources {
		cpu, mem, pods := l[v1.ResourceCPU], l[v1.ResourceMemory], l[v1.ResourcePods]
		return recommend.Resources{CPU: cpu.AsApproximateFloat64(), Mem: mem.AsApproximateFloat64() / (1 << 30), Pods: pods.AsApproximateFloat64()}
	}
	rn := recommend.Node{
		Name: n.Name(), Type: string(n.InstanceType()), Zone: n.Zone(), Capacity: string(n.CapacityType()),
		Auto:        n.LabelValue("eks.amazonaws.com/compute-type") == "auto",
		Allocatable: res(n.Allocatable()),
		PriceKnown:  n.HasPrice(),
		Equivalent:  n.PriceIsEquivalent(),
		Price:       n.Price(),
		Override:    n.LabelValue(LabelPrefix+"instance-price") != UngroupedValue,
		Created:     n.Created(),
	}
	if v := n.LabelValue("karpenter.sh/nodepool"); v != UngroupedValue {
		rn.NodePool = v
	}
	if v := n.LabelValue("eks.amazonaws.com/nodegroup"); v != UngroupedValue && rn.NodePool == "" {
		rn.ManagedGroup = v
	}
	for _, p := range n.PodsCopy() {
		r := res(p.Requested())
		r.Pods = 1
		rn.Pods = append(rn.Pods, recommend.Pod{Req: r, Fixed: p.StaysOnItsNode()})
	}
	return rn
}

func (u *UIModel) recsDone(msg recsMsg) {
	u.recPending = false
	u.recFor = msg.key
	u.recs = msg.result
	if u.recCursor >= len(u.recs.Recs) {
		u.recCursor = max(len(u.recs.Recs)-1, 0)
	}
}

// recommendationsBody draws the screen below the header: the total, notes,
// then the rows, the selected one marked in the margin.
func (u *UIModel) recommendationsBody(height int) (lines []string, selFrom, selTo int) {
	selFrom, selTo = -1, -1
	if u.advisor == nil {
		return []string{"Recommendations need the instance catalog; not available here."}, selFrom, selTo
	}
	if cloud := u.shownCloud(); cloud != "aws" {
		return []string{"Recommendations cover EKS nodes only for now; the nodes shown run on " + cloudName(cloud) + "."}, selFrom, selTo
	}
	if u.recFor == "" {
		return []string{"Working out recommendations…"}, selFrom, selTo
	}
	r := u.recs
	saving := "—"
	if !u.DisablePricing && u.stats.PricedNodes > 0 {
		saving = u.printer.Sprintf("$%0.2f/month", r.Total)
	}
	lines = append(lines,
		titleStyle("Savings: ")+costValue(saving)+helpStyle("  best non-Spot option per node; estimates from requests and list prices, by requests only"))
	for _, n := range r.Notes {
		lines = append(lines, u.style.yellow("⚠ "+n))
	}
	lines = append(lines, "")
	if len(r.Recs) == 0 {
		return append(lines, "Nothing to recommend for the nodes shown."), selFrom, selTo
	}
	rows := [][4]string{{"KIND", "TARGET", "ACTION", "SAVES/MO"}}
	for _, rec := range r.Recs {
		saves := "—"
		switch {
		case rec.Info:
			saves = "info"
		case rec.Priced && !u.DisablePricing:
			saves = u.printer.Sprintf("$%0.2f", rec.Monthly)
		}
		rows = append(rows, [4]string{rec.Kind.String(), rec.Target, rec.Action, saves})
	}
	var w [4]int
	for _, row := range rows {
		for i, c := range row {
			w[i] = max(w[i], textWidth(c))
		}
	}
	format := func(row [4]string) string {
		return pad(row[0], w[0]) + "  " + pad(row[1], w[1]) + "  " + pad(row[2], w[2]) + "  " + fmt.Sprintf("%*s", w[3], row[3])
	}
	lines = append(lines, columnTitleStyle(format(rows[0])))
	// rows that fit, scrolled to keep the cursor in view
	room := max(height-len(lines), 1)
	start, end := window(len(rows)-1, u.recCursor, room)
	for i := start; i < end; i++ {
		if i == u.recCursor {
			selFrom, selTo = len(lines), len(lines)+1
		}
		line := format(rows[i+1])
		if r.Recs[i].Kind == recommend.SpotMove {
			line = spotStyle(line)
		} else if r.Recs[i].Info {
			line = helpStyle(line)
		}
		lines = append(lines, line)
	}
	return lines, selFrom, selTo
}

// shownCloud is the cloud of the nodes the view keeps: "aws" when any runs
// on AWS or none is shown, else the first node's.
func (u *UIModel) shownCloud() string {
	nodes := u.view.Select(u.stats.Nodes)
	if len(nodes) == 0 {
		return "aws" // nothing shown: the screen says so as usual
	}
	for _, n := range nodes {
		if n.Cloud() == "aws" {
			return "aws"
		}
	}
	return nodes[0].Cloud()
}

func cloudName(scheme string) string {
	switch scheme {
	case "azure":
		return "Azure"
	case "gce":
		return "Google Cloud"
	case "":
		return "an unknown provider"
	}
	return scheme
}

// recsKey handles keys on the Recommendations screen; Overview-only keys do
// nothing there (R3).
func (u *UIModel) recsKey(key string) {
	if u.shownCloud() != "aws" {
		// the screen only says the rules cover EKS: nothing to select
		return
	}
	switch key {
	case "up", "k":
		if u.recCursor > 0 {
			u.recCursor--
		}
	case "down", "j":
		if u.recCursor < len(u.recs.Recs)-1 {
			u.recCursor++
		}
	case "enter":
		if u.recCursor < len(u.recs.Recs) {
			u.panel = panelRecMath
		}
	}
}

func (u *UIModel) recMathPanel() panelView {
	if u.recCursor >= len(u.recs.Recs) {
		return panelView{title: "Why", cursor: -1}
	}
	rec := u.recs.Recs[u.recCursor]
	rows := append([]string{rec.Kind.String() + ": " + rec.Target, rec.Action, ""}, rec.Math...)
	if rec.Reason != "" {
		rows = append(rows, "", helpStyle(rec.Reason))
	}
	return panelView{title: "Why: " + strings.TrimSpace(rec.Kind.String()), rows: rows, cursor: -1}
}
