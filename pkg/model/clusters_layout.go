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
	"math"
	"slices"
	"sort"
	"strings"
)

// fieldPodsPending stands, in a group-by, for a field made from pods while
// the pods are not synced: every node groups under … (clusters-screen R15).
const fieldPodsPending Field = "pods-pending"

// pendingValue is how a figure made from pods reads before the pods synced.
const pendingValue = "…"

// podField tells the fields made from pods (clusters-screen R15): idle,
// pods, requests and the computed usage labels.
func podField(f Field) bool {
	switch f {
	case FieldIdle, FieldIdleMonthly, FieldPods:
		return true
	}
	if strings.HasPrefix(string(f), usageFieldPrefix) {
		return true
	}
	if key := f.Label(); key != "" && resourceLabelRe.MatchString(key) {
		return true
	}
	return false
}

// clusterSection is one cluster as the screen lays it out.
type clusterSection struct {
	s     *slot
	cloud string
	// nodes are the nodes kept by the view, in its order; total is the
	// cluster's node count
	nodes []*Node
	total int
	sum   GroupStats
	// blind: the pods are not synced; blindFilter: a filter on a figure
	// made from pods kept no node
	blind, blindFilter bool
	view               ViewState
}

// connected tells whether the cluster's nodes are in.
func (c *clusterSection) connected() bool {
	return c.s.ready && c.s.err == "" && c.s.st.Synced
}

// sectionView is the view a section is laid out with: the nodes listed,
// every group unfolded (clusters-screen D2), and while the pods are not
// synced no order, filter or group made from them (R15).
func (u *UIModel) sectionView(blind bool) (v ViewState, blindFilter bool) {
	v = u.view
	v.ShowNodes, v.GroupDepth, v.Details, v.Collapsed = true, 0, false, nil
	if !blind {
		return v, false
	}
	if podField(v.Sort.Field) {
		v.Sort = SortSpec{Field: FieldName}
	}
	v.GroupBy = slices.Clone(v.GroupBy)
	for i, f := range v.GroupBy {
		if podField(f) {
			v.GroupBy[i] = fieldPodsPending
		}
	}
	for _, f := range v.Filters {
		if f.Enabled && len(f.Ticked()) > 0 && podField(f.Field) {
			blindFilter = true
		}
	}
	return v, blindFilter
}

// clusterSections lays every slot out, ordered under its cloud
// (clusters-screen R12).
func (u *UIModel) clusterSections() []*clusterSection {
	u.refreshBorrowed()
	var out []*clusterSection
	for _, s := range u.fl.slots {
		c := &clusterSection{s: s, cloud: slotCloud(s), total: s.stats.NumNodes, blind: !s.podsReady(), sum: newGroupStats("")}
		c.view, c.blindFilter = u.sectionView(c.blind)
		if !c.blindFilter {
			c.nodes = c.view.Select(s.stats.Nodes)
		}
		for _, n := range c.nodes {
			c.sum.add(n)
		}
		out = append(out, c)
	}
	clouds := cloudTotals(out)
	rank := func(cloud string) (int, float64) {
		switch cloud {
		case cloudMixed:
			return 1, 0
		case cloudOther:
			return 2, 0
		case cloudUnknown:
			return 3, 0
		}
		return 0, clouds[cloud].sum.TotalPrice
	}
	sort.SliceStable(out, func(a, b int) bool {
		ra, ta := rank(out[a].cloud)
		rb, tb := rank(out[b].cloud)
		if ra != rb {
			return ra < rb
		}
		if out[a].cloud != out[b].cloud {
			if ta != tb {
				return ta > tb
			}
			return out[a].cloud < out[b].cloud
		}
		pa, pb := out[a].sum.PricedNodes > 0, out[b].sum.PricedNodes > 0
		if pa != pb {
			return pa
		}
		if out[a].sum.TotalPrice != out[b].sum.TotalPrice {
			return out[a].sum.TotalPrice > out[b].sum.TotalPrice
		}
		return naturalLess(out[a].s.name, out[b].s.name)
	})
	return out
}

// cloudTotal is a cloud's figures over its clusters (clusters-screen R4).
type cloudTotal struct {
	cloud                  string
	clusters, notConnected int
	total, blind           int
	sum                    GroupStats
	filtered               bool
}

// hasTotal tells the clouds whose clusters are added up (R4).
func hasTotal(cloud string) bool {
	return slices.Contains(ScopeClouds, cloud)
}

func cloudTotals(sections []*clusterSection) map[string]*cloudTotal {
	out := map[string]*cloudTotal{}
	for _, c := range sections {
		t, ok := out[c.cloud]
		if !ok {
			t = &cloudTotal{cloud: c.cloud, sum: newGroupStats(c.cloud)}
			out[c.cloud] = t
		}
		t.clusters++
		if !c.connected() {
			t.notConnected++
		}
		if c.blindFilter {
			t.blind++
		}
		t.total += c.total
		if len(c.nodes) != c.total {
			t.filtered = true
		}
		for _, n := range c.nodes {
			t.sum.add(n)
		}
	}
	return out
}

// cloudOrder is the clouds with a total, by total cost, highest first.
func cloudOrder(totals map[string]*cloudTotal) []string {
	var out []string
	for cloud := range totals {
		if hasTotal(cloud) {
			out = append(out, cloud)
		}
	}
	sort.Slice(out, func(a, b int) bool {
		if totals[out[a]].sum.TotalPrice != totals[out[b]].sum.TotalPrice {
			return totals[out[a]].sum.TotalPrice > totals[out[b]].sum.TotalPrice
		}
		return out[a] < out[b]
	})
	return out
}

// monthly is a cost in $/month with its marks: >= when it is a lower bound,
// reserved nodes, "prices unknown" when nothing is priced.
func (u *UIModel) monthly(s GroupStats, lowerBound bool) string {
	if s.NumNodes > 0 && s.PricedNodes == 0 && s.ReservedNodes == 0 {
		return helpStyle("prices unknown")
	}
	cost := u.printer.Sprintf("$%0.3f/month", s.TotalPrice*avgHoursPerMonth)
	if lowerBound || !s.FullyPriced() {
		cost = ">= " + cost
	}
	if s.ReservedNodes > 0 {
		cost += u.printer.Sprintf(" (+%d reserved, not priced)", s.ReservedNodes)
	}
	return cost
}

// cloudCost is a cloud's total with what makes it a lower bound.
func (u *UIModel) cloudCost(t *cloudTotal) string {
	if u.DisablePricing {
		return ""
	}
	cost := u.monthly(t.sum, t.notConnected > 0)
	var notes []string
	if t.notConnected > 0 {
		notes = append(notes, u.printer.Sprintf("%d %s not connected", t.notConnected, noun(t.notConnected, "cluster", "clusters")))
	}
	if unpriced := t.sum.NumNodes - t.sum.PricedNodes - t.sum.ReservedNodes; unpriced > 0 && t.sum.PricedNodes > 0 {
		notes = append(notes, u.printer.Sprintf("%d %s unpriced", unpriced, noun(unpriced, "node", "nodes")))
	}
	if len(notes) > 0 {
		cost += " (" + strings.Join(notes, ", ") + ")"
	}
	return cost
}

// cloudMarks is a cloud total's filter marks.
func (u *UIModel) cloudMarks(t *cloudTotal) string {
	var out string
	if t.filtered {
		out += helpStyle(u.printer.Sprintf("  (filtered: %d of %d nodes)", t.sum.NumNodes, t.total))
	}
	if t.blind > 0 {
		out += helpStyle(u.printer.Sprintf("  %d %s pods not loaded", t.blind, noun(t.blind, "cluster", "clusters")))
	}
	return out
}

func noun(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// clustersOverview is what the header and the cloud lines say: the clouds
// with a total, and the clusters in no cloud's total.
type clustersOverview struct {
	sections            []*clusterSection
	totals              map[string]*cloudTotal
	clouds              []string
	nodes               int
	noTotal, noTotalOff int
}

func (u *UIModel) clustersOverview() clustersOverview {
	o := clustersOverview{sections: u.clusterSections()}
	o.totals = cloudTotals(o.sections)
	o.clouds = cloudOrder(o.totals)
	for _, c := range o.sections {
		o.nodes += len(c.nodes)
		if !hasTotal(c.cloud) {
			o.noTotal++
			if !c.connected() {
				o.noTotalOff++
			}
		}
	}
	return o
}

// clustersHeader is the header of the Clusters screen in its three sizes
// (clusters-screen R1): no pod count, a cost only with one cloud on screen.
func (u *UIModel) clustersHeader() string {
	o := u.clustersOverview()
	scope := "all clouds"
	if u.fl.scope != "" {
		scope = u.fl.scope
	}
	clusters := u.printer.Sprintf("%d %s", len(o.sections), noun(len(o.sections), "cluster", "clusters"))
	nodes := u.printer.Sprintf("%d nodes", o.nodes)
	cost := ""
	if len(o.clouds) == 1 && !u.DisablePricing {
		t := o.totals[o.clouds[0]]
		cost = u.cloudCost(t) + u.cloudMarks(t)
	}
	unknown := ""
	if o.noTotal > 0 {
		unknown = u.printer.Sprintf("%d %s of unknown or mixed cloud", o.noTotal, noun(o.noTotal, "cluster", "clusters"))
		if o.noTotalOff > 0 {
			unknown += u.printer.Sprintf(" (%d not connected)", o.noTotalOff)
		}
	}
	parts := []string{headerContext("Clusters")}
	if u.fl.scope != "" {
		parts = append(parts, headerValue(u.fl.scope))
	}
	parts = append(parts, countsValue(clusters), countsValue(nodes))
	if cost != "" {
		parts = append(parts, costValue(cost))
	}
	if unknown != "" {
		parts = append(parts, headerLabel(unknown))
	}
	switch u.headerSize() {
	case headerLine:
		return strings.Join(parts, viewSep(" · ")) + viewSep(" │ ") + u.viewSummary() + "\n"
	case headerCompact:
		return u.boxed(strings.Join(parts, headerLabel(" · ")) + "\n" + u.viewSummary())
	}
	rows := [][2]string{
		{"Scope:", headerValue(scope)},
		{"Clusters:", countsValue(clusters)},
		{"Nodes:", countsValue(nodes)},
	}
	if cost != "" {
		rows = append(rows, [2]string{"Cost:", costValue(cost)})
	}
	if unknown != "" {
		rows = append(rows, [2]string{"Unknown:", headerLabel(unknown)})
	}
	rows = append(rows, [2]string{"View:", u.viewSummary()})
	var left strings.Builder
	left.WriteString(titleStyle("kube-node-viewer") + headerLabel(" · ") + headerContext(u.screenTitle()))
	for _, r := range rows {
		left.WriteString("\n" + headerLabel(fmt.Sprintf("%-9s", r[0])) + " " + r[1])
	}
	keys := slices.Clone(headerKeys)
	for i, k := range keys {
		if k[0] == "c" {
			keys[i][1] = "clusters"
		}
	}
	var right strings.Builder
	for i := 0; i < len(keys); i += 2 {
		right.WriteString("\n")
		for _, k := range keys[i:min(i+2, len(keys))] {
			fmt.Fprintf(&right, "%s %s", headerKey(fmt.Sprintf("%3s", k[0])), headerAction(fmt.Sprintf("%-10s", k[1])))
		}
	}
	return u.boxed(left.String(), right.String())
}

// cloudLines are the totals of each cloud, shown above the sections when
// more than one cloud is on screen (clusters-screen R4).
func (u *UIModel) cloudLines(o clustersOverview) []string {
	if len(o.clouds) < 2 {
		return nil
	}
	nameW, clW, nodesW := 0, 0, 0
	type row struct{ name, clusters, nodes string }
	rows := make([]row, 0, len(o.clouds))
	for _, cloud := range o.clouds {
		t := o.totals[cloud]
		r := row{cloud, u.printer.Sprintf("%d %s", t.clusters, noun(t.clusters, "cluster", "clusters")), u.printer.Sprintf("%d nodes", t.sum.NumNodes)}
		nameW, clW, nodesW = max(nameW, len(r.name)), max(clW, len(r.clusters)), max(nodesW, len(r.nodes))
		rows = append(rows, r)
	}
	var out []string
	for i, cloud := range o.clouds {
		t, r := o.totals[cloud], rows[i]
		line := "  " + pad(topGroupStyle(r.name), nameW) + "  " + pad(r.clusters, clW) + "  " + pad(r.nodes, nodesW)
		if cost := u.cloudCost(t); cost != "" {
			line += "  " + cost
		}
		out = append(out, line+u.cloudMarks(t))
	}
	return out
}

// cblock is a group of lines that stays together on a page of the Clusters
// screen; sec is its section's context, set on everything but cloud lines.
type cblock struct {
	lines []string
	key   string
	sec   string
	// heading is the section's own line; title is the section's column
	// titles, repeated when a page starts inside the section
	heading bool
	title   string
	label   string
}

// sectionHeading is a section's line (clusters-screen R3).
type headingWidths struct{ name, cloud, nodes, cost int }

func (u *UIModel) sectionParts(c *clusterSection) (name, cloud, nodes, cost, rest string) {
	mark := "▸ "
	if u.fl.unfolded[c.s.name] {
		mark = "▾ "
	}
	name = mark + shortContext(c.s.name)
	cloud = c.cloud
	nodes = u.printer.Sprintf("%d nodes", len(c.nodes))
	if !c.connected() {
		nodes = "—"
	}
	if !u.DisablePricing && c.connected() {
		cost = u.monthly(c.sum, false)
		idle := pendingValue
		if !c.blind {
			idle = u.printer.Sprintf("$%0.3f/month", c.sum.IdlePrice*avgHoursPerMonth)
		}
		rest = helpStyle("idle " + idle)
	}
	if c.connected() && len(c.nodes) != c.total && !c.blindFilter {
		rest += helpStyle(u.printer.Sprintf("  (filtered: %d of %d nodes)", len(c.nodes), c.total))
	}
	if c.blindFilter {
		rest += helpStyle("  (filtered: … pods not loaded)")
	}
	status := u.style.green("ok")
	switch {
	case c.s.err != "":
		status = u.style.red(c.s.err)
	case c.s.st.Err != "":
		status = u.style.red(c.s.st.Err)
	case !c.s.ready || !c.s.st.Synced:
		status = helpStyle("connecting…")
	}
	rest += "  " + status
	return name, cloud, nodes, cost, rest
}

// clustersBody lays the sections out and returns the current page, where the
// cursor's row is on it, and the page count (clusters-screen D6).
func (u *UIModel) clustersBody(o clustersOverview, height int) (lines []string, from, to, pageCount int) {
	from, to = -1, -1
	if len(o.sections) == 0 {
		return []string{"No clusters: c picks them."}, from, to, 1
	}
	var w headingWidths
	parts := make(map[*clusterSection][5]string, len(o.sections))
	for _, c := range o.sections {
		name, cloud, nodes, cost, rest := u.sectionParts(c)
		parts[c] = [5]string{name, cloud, nodes, cost, rest}
		w.name, w.cloud = max(w.name, textWidth(name)), max(w.cloud, textWidth(cloud))
		w.nodes, w.cost = max(w.nodes, textWidth(nodes)), max(w.cost, textWidth(cost))
	}
	buckets := 0
	for i, c := range o.sections {
		if i == 0 || c.cloud != o.sections[i-1].cloud {
			buckets++
		}
	}

	var blocks []cblock
	for i, c := range o.sections {
		if buckets > 1 && (i == 0 || c.cloud != o.sections[i-1].cloud) {
			lines := []string{topGroupStyle(c.cloud)}
			if i > 0 {
				lines = append([]string{""}, lines...)
			}
			blocks = append(blocks, cblock{lines: lines})
		}
		p := parts[c]
		head := pad(groupStyle(p[0]), w.name) + "  " + pad(p[1], w.cloud) + "  " + pad(p[2], w.nodes) + "  " + pad(p[3], w.cost) + "  " + p[4]
		label := shortContext(c.s.name)
		if !u.fl.unfolded[c.s.name] || len(c.nodes) == 0 {
			blocks = append(blocks, cblock{lines: []string{strings.TrimRight(head, " ")}, key: "c:" + c.s.name, sec: c.s.name, heading: true, label: label})
			continue
		}
		inner := u.sectionBlocks(c)
		title := ""
		if len(inner) > 0 {
			title = inner[0].title
		}
		blocks = append(blocks, cblock{lines: []string{strings.TrimRight(head, " "), title}, key: "c:" + c.s.name, sec: c.s.name, heading: true, title: title, label: label})
		for _, b := range inner {
			b.sec, b.label = c.s.name, label
			blocks = append(blocks, b)
		}
	}
	pages := paginateClusters(blocks, height)
	u.page = min(u.page, len(pages)-1)
	asBlocks := make([][]block, len(pages))
	for i, pg := range pages {
		for _, b := range pg {
			asBlocks[i] = append(asBlocks[i], block{key: b.key})
		}
	}
	u.resolveCursor(asBlocks)
	u.cursorPath = nil
	for _, b := range pages[u.page] {
		if b.key != "" && b.key == u.cursorKey {
			n := 1
			if !b.heading {
				n = len(b.lines)
			}
			from, to = len(lines), len(lines)+n
		}
		lines = append(lines, b.lines...)
	}
	return lines, from, to, len(pages)
}

// sectionBlocks are an unfolded section's groups and nodes, indented under
// its heading; the first block's title is the section's column titles.
func (u *UIModel) sectionBlocks(c *clusterSection) []cblock {
	const indent = "  "
	saved, width := u.view, u.width
	u.view, u.podsBlind = c.view, c.blind
	if width > 0 {
		// the section's rows sit under its heading, indented
		u.width = max(width-len(indent), 1)
	}
	defer func() { u.view, u.podsBlind, u.width = saved, false, width }()

	groups := u.view.Groups(c.nodes)
	tableIndent := 0
	if groups != nil {
		tableIndent = 2 * len(u.view.GroupBy)
	}
	table := u.newNodeTable(c.nodes, tableIndent)
	title := indent + table.title
	node := func(n *Node) cblock {
		lines := table.render(n)
		for i := range lines {
			lines[i] = indent + lines[i]
		}
		return cblock{lines: lines, key: "n:" + c.s.name + "/" + n.Name(), title: title}
	}
	var out []cblock
	if groups == nil {
		for _, n := range c.nodes {
			out = append(out, node(n))
		}
		return out
	}
	tree := treeOf(groups, 0, func([]string) bool { return false })
	var cols groupLineWidths
	for g := range tree {
		cols.fit(u, g, tree[g])
	}
	var walk func([]*Group)
	walk = func(gs []*Group) {
		for _, g := range gs {
			out = append(out, cblock{lines: []string{indent + u.groupLine(g, tree[g], cols)}, key: "g:" + c.s.name + "/" + strings.Join(g.Path, pathSeparator), title: title})
			walk(g.Groups)
			for _, n := range g.Nodes {
				out = append(out, node(n))
			}
		}
	}
	walk(groups)
	return out
}

// paginateClusters fills pages with whole blocks; a section heading stays
// with what follows it, and a page that starts inside a section repeats its
// name and column titles.
func paginateClusters(blocks []cblock, height int) [][]cblock {
	var pages [][]cblock
	var page []cblock
	used := 0
	for i, b := range blocks {
		need := len(b.lines)
		if b.heading && i+1 < len(blocks) && blocks[i+1].sec == b.sec {
			need += len(blocks[i+1].lines)
		}
		if len(page) > 0 && used+need > height {
			pages = append(pages, page)
			page, used = nil, 0
			if b.sec != "" && !b.heading {
				cont := []string{groupStyle(b.label) + helpStyle(" (cont.)")}
				if b.title != "" {
					cont = append(cont, b.title)
				}
				page = append(page, cblock{lines: cont})
				used += len(cont)
			}
		}
		page = append(page, b)
		used += len(b.lines)
	}
	return append(pages, page)
}

// pendingBar is a usage bar before the pods synced: empty, then ….
func (u *UIModel) pendingBar(width int, cell rune) string {
	defer func(w int, pct bool) { u.progress.Width, u.progress.ShowPercentage = w, pct }(u.progress.Width, u.progress.ShowPercentage)
	defer u.setBarCell(u.progress.Full)
	u.progress.Width, u.progress.ShowPercentage = max(width-5, 1), false
	u.setBarCell(cell)
	return u.progressBar(0) + " " + helpStyle(fmt.Sprintf("%4s", pendingValue))
}

// isPending tells a fill that stands for "pods not synced".
func isPending(fill float64) bool { return math.IsInf(fill, -1) }
