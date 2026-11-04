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
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"k8s.io/apimachinery/pkg/util/duration"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Render
	// column titles of the node list
	columnTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#875f00", Dark: "#ffd75f"}).Render
	// capacity types that change what a node costs
	spotStyle     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#d75f00", Dark: "#ffaf5f"}).Render
	reservedStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#af00af", Dark: "#d787d7"}).Render
	fargateStyle  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#005fd7", Dark: "#87afff"}).Render
	groupStyle    = lipgloss.NewStyle().Bold(true).Render
	// a footer notice
	noticeStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#005f5f", Dark: "#87d7af"}).Render
	// the cursor's mark in the left margin
	cursorStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#5f5fd7", Dark: "#a8a8f0"}).Render
	// group headings: the top level stands out, sub-levels hang off it
	topGroupStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#005f87", Dark: "#87d7ff"}).Render
	subGroupStyle = lipgloss.NewStyle().Bold(true).Render
	treeStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262")).Render
	// managedNodeGroupStyle tells EKS managed node groups from Karpenter
	// NodePools in the node group column
	managedNodeGroupStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#0087af", Dark: "#5fafff"}).Render
)

// A node's usage bar, percentage included, is at least nodeBarWidth wide and
// grows into width the columns leave free, up to maxNodeBarWidth. Columns are
// dropped before the bar shrinks.
const (
	nodeBarWidth    = 36
	maxNodeBarWidth = 60
)

// pathSeparator joins group-by values in group headings.
const pathSeparator = " › "

// contentMargin indents everything below the header box to line up with the
// text inside it (border plus padding).
const contentMargin = 2

// render draws the whole screen: header, cluster summary, one page of the
// node list with the open panel floating over it, then the footer.
func (u *UIModel) render() string {
	stats := u.stats
	nodes := u.view.Select(stats.Nodes)
	header := u.fit(u.header(stats, nodes))

	// below the box, lay out for the width left after the margin
	full := u.width
	if full > 0 {
		u.width = max(full-contentMargin, 1)
	}
	content := u.renderContent(stats, nodes, strings.Count(header, "\n"))
	u.width = full

	margin := strings.Repeat(" ", contentMargin)
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		// the selected row is marked in the margin, out of the way of its colours
		if u.panel == panelNone && i >= u.selFrom && i < u.selTo {
			lines[i] = cursorStyle("▌") + " " + line
		} else if line != "" {
			lines[i] = margin + line
		}
	}
	return header + strings.Join(lines, "\n")
}

// renderContent is everything below the header, headerLines tall.
func (u *UIModel) renderContent(stats Stats, nodes []*Node, headerLines int) string {
	var b strings.Builder
	b.WriteString(u.fit(u.clusterSummary(stats, nodes)))
	b.WriteString("\n")

	// what is left between the summary and the footer lines: two, plus
	// one always reserved on Overview with nodes on screen for the
	// DaemonSets line, so pages don't change as the cursor moves (node-pods
	// R10)
	dsLine := !u.onRecommendations() && stats.NumNodes > 0 && len(nodes) > 0
	footer := 2
	if dsLine {
		footer = 3
	}
	// an open panel also takes the reserved line, blank while it is open.
	// On a window too short to keep three list lines besides it, the line
	// is dropped: the list needs them to show a heading and the selected
	// node (R10, corrected 2026-11-03)
	avail, panelHeight := math.MaxInt, math.MaxInt
	if u.height > 0 {
		left := u.height - headerLines - strings.Count(b.String(), "\n")
		if dsLine && left-footer < 3 {
			dsLine, footer = false, 2
		}
		avail, panelHeight = max(left-footer, 3), max(left-2, 3)
	}

	u.selFrom, u.selTo = -1, -1
	bodyStart := strings.Count(b.String(), "\n")
	var body []string
	status := ""
	switch {
	case u.onRecommendations():
		var from, to int
		body, from, to = u.recommendationsBody(avail)
		u.selFrom, u.selTo = from, to
	case stats.NumNodes == 0:
		body = []string{"Waiting for update or no nodes found..."}
	case len(nodes) == 0:
		body = []string{"No nodes match the filters or search."}
	default:
		var pageCount, hidden int
		body, pageCount, hidden = u.currentPage(nodes, avail)
		u.paginator.PerPage = 1
		u.paginator.SetTotalPages(pageCount)
		u.paginator.Page = u.page
		if pageCount > 1 {
			status = u.paginator.View()
		}
		if hidden > 0 {
			noun := "columns"
			if hidden == 1 {
				noun = "column"
			}
			status += helpStyle(fmt.Sprintf("  %d %s hidden, widen the window", hidden, noun))
		}
	}
	if len(body) > avail {
		body = body[:avail]
	}
	if u.selFrom >= 0 {
		u.selFrom, u.selTo = bodyStart+u.selFrom, bodyStart+min(u.selTo, len(body))
	}
	for i := range body {
		body[i] = u.fit(body[i])
	}
	if p, ok := u.panelView(); ok {
		// the panel floats over the node list, which stays in view around it
		dsLine = false
		body = u.overlay(body, u.popup(p, panelHeight), panelHeight)
	}
	// footer priority: the search line, then a notice, then the page dots
	if notice := u.currentNotice(); notice != "" {
		status = noticeStyle(notice)
	}
	if u.searching {
		status = "/" + u.view.Search + "█"
	}
	if u.commanding {
		status = ":" + u.command + "█"
	}
	for _, line := range body {
		b.WriteString(line)
		b.WriteString("\n")
	}
	if dsLine {
		b.WriteString(u.fit(u.daemonSetFooter()))
		b.WriteString("\n")
	}
	b.WriteString(u.fit(status))
	b.WriteString("\n")
	if u.panel == panelNone {
		// an open panel shows its keys inside its box
		b.WriteString(u.fit(helpStyle(u.helpText())))
	}
	return b.String()
}

// daemonSetFooter is the DaemonSets line of the cursor's node (node-pods
// R10), or blank when the cursor is not on a node or the search or the
// command line is open. An open panel takes its line.
func (u *UIModel) daemonSetFooter() string {
	if u.searching || u.commanding {
		return ""
	}
	name, ok := strings.CutPrefix(u.cursorKey, "n:")
	if !ok {
		return ""
	}
	n, ok := u.findNode(name)
	if !ok {
		return ""
	}
	return DaemonSetLine(n)
}

// fit cuts every line of s to the window width. A line wider than the window
// wraps, the frame grows taller than the window and the terminal scrolls,
// leaving old lines behind.
func (u *UIModel) fit(s string) string {
	if u.width <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if textWidth(line) > u.width {
			lines[i] = ansi.Truncate(line, u.width, "…")
		}
	}
	return strings.Join(lines, "\n")
}

func (u *UIModel) helpText() string {
	if u.searching {
		if searchInvalid(u.view.Search) {
			return "type to search (regex) • invalid regex: as text • enter: keep • esc: clear"
		}
		return "type to search (regex) • enter: keep • esc: clear"
	}
	switch u.panel {
	case panelHelp:
		return "esc: close"
	case panelFilters:
		return u.filters.hint()
	case panelGroupBy:
		return u.group.hint()
	case panelSort:
		return u.sort.hint()
	case panelColumns:
		return u.columns.hint()
	case panelContexts:
		return u.contexts.hint()
	case panelViews:
		return u.viewsPanel.hint()
	case panelRecMath:
		return "esc: close"
	case panelNode:
		if u.detailsTab == detailsTabPods {
			if u.podsView == podsViewDaemonSets {
				return "↑/↓: move • d: all pods • s/S: sort • tab: details • esc: close"
			}
			return "↑/↓: move • d: daemonsets • s/S: sort • tab: details • esc: close"
		}
		return "↑/↓: scroll • home: top • tab: pods • esc: close"
	}
	if u.commanding {
		return "overview (ov) • recommendations (rec) • tab: complete • enter: run • esc: cancel"
	}
	if u.onRecommendations() {
		return "↑/↓: select • enter: why • tab/:ov: Overview • f: filter • /: search • c: context • v: views • e: export • ?: help • q: quit"
	}
	keys := "f: filter • g: group • s: sort • I: flip ↑/↓ • o: columns • /: search • r: resources • c: context • v: views • e: export • tab: recommendations"
	if len(u.view.GroupBy) > 0 {
		keys += " • enter: fold group • n: fold"
	}
	if u.canShowDetails() {
		keys += " • d: details"
	}
	return keys + " • ?: help • q: quit"
}

// block is lines that stay together on a page: a group heading or a node's
// lines. path is the group the block belongs to, or is, so a page that starts
// inside a group can repeat it (R14).
type block struct {
	path    []string
	heading bool
	lines   []string
	// node is set for a node's block, whose lines are built only when its
	// page is shown
	node  *Node
	count int
	// key names the row for the cursor: a group heading or a node
	key string
	// lead is how many leading lines (a blank spacer) the cursor skips
	lead int
}

func (b block) height() int {
	if b.node != nil {
		return b.count
	}
	return len(b.lines)
}

// currentPage lays the selected nodes out under their groups, cuts them into pages
// of at most height lines and returns the lines of the current page, the
// page count and how many columns did not fit the width.
func (u *UIModel) currentPage(nodes []*Node, height int) (lines []string, pageCount, hiddenColumns int) {
	var table *nodeTable
	var top []string
	if u.view.ShowNodes {
		indent := 0
		if len(u.view.GroupBy) > 0 {
			indent = 2 * len(u.view.GroupBy)
		}
		table = u.newNodeTable(nodes, indent)
		top = []string{table.title}
		hiddenColumns = table.hidden
	}

	var blocks []block
	groups := u.view.Groups(nodes)
	// with the node list hidden the groups carry their own usage bars
	// Folded, the deepest level shown carries the usage bars and the levels
	// above it are one-line summaries. With details on (d), the level below
	// the barred one follows as one-line rows.
	depth := u.view.VisibleDepth()
	shownDepth := depth
	if u.canShowDetails() && u.view.Details {
		shownDepth = depth + 1
	}
	tree := treeOf(groups, shownDepth, u.view.IsCollapsed)
	var barred map[*Group][]string
	if table == nil {
		barred = u.groupTable(groups, tree, depth)
	}
	// one-line headings line up their name, node and pod columns
	var cols groupLineWidths
	for g := range tree {
		if _, ok := barred[g]; !ok {
			cols.fit(u, g, tree[g])
		}
	}
	var walk func(groups []*Group)
	walk = func(groups []*Group) {
		for _, g := range groups {
			var lines []string
			if tree[g].spaced {
				lines = append(lines, "")
			}
			if _, shown := tree[g]; !shown {
				continue
			}
			lead := len(lines)
			if bars, ok := barred[g]; ok {
				lines = bars
				if tree[g].spaced {
					lead = 1
				}
			} else {
				lines = append(lines, u.groupLine(g, tree[g], cols))
			}
			blocks = append(blocks, block{path: g.Path, heading: true, lines: lines, key: groupCursorKey(g.Path), lead: lead})
			if u.view.IsCollapsed(g.Path) {
				continue
			}
			walk(g.Groups)
			if table != nil {
				for _, n := range g.Nodes {
					blocks = append(blocks, block{path: g.Path, node: n, count: len(u.cluster.resources), key: nodeCursorKey(n)})
				}
			}
		}
	}
	if groups != nil {
		walk(groups)
	} else {
		for _, n := range nodes {
			blocks = append(blocks, block{node: n, count: len(u.cluster.resources), key: nodeCursorKey(n)})
		}
	}
	pages := paginate(blocks, top, height)
	u.page = min(u.page, len(pages)-1)
	u.resolveCursor(pages)
	u.selFrom, u.selTo = -1, -1
	// nodes are set apart by the gap above their first bar, not a blank line
	for _, b := range pages[u.page] {
		var bl []string
		if b.node != nil {
			bl = table.render(b.node)
		} else {
			bl = b.lines
		}
		if b.key != "" && b.key == u.cursorKey {
			u.selFrom, u.selTo = len(lines)+b.lead, len(lines)+len(bl)
		}
		lines = append(lines, bl...)
	}
	return lines, len(pages), hiddenColumns
}

func groupCursorKey(path []string) string { return "g:" + strings.Join(path, pathSeparator) }
func nodeCursorKey(n *Node) string        { return "n:" + n.Name() }

// resolveCursor applies pending cursor moves now that the order of the rows
// and their pages are known: a move follows the cursor to its page, a page
// change puts the cursor on that page's first row, and a cursor whose row
// went away (filtered, folded) goes to the first row of the page.
func (u *UIModel) resolveCursor(pages [][]block) {
	type row struct {
		page int
		key  string
		path []string
	}
	var rows []row
	for p, pg := range pages {
		for _, b := range pg {
			if b.key != "" {
				rows = append(rows, row{page: p, key: b.key, path: b.path})
			}
		}
	}
	if len(rows) == 0 {
		u.cursorKey, u.cursorPath, u.cursorDelta = "", nil, 0
		return
	}
	pos := -1
	for i, r := range rows {
		if r.key == u.cursorKey {
			pos = i
		}
	}
	switch {
	case u.cursorDelta != 0 && pos >= 0:
		pos = min(max(pos+u.cursorDelta, 0), len(rows)-1)
		u.page = rows[pos].page
	case pos < 0 || rows[pos].page != u.page:
		pos = 0
		for i, r := range rows {
			if r.page == u.page {
				pos = i
				break
			}
		}
	}
	u.cursorDelta = 0
	u.cursorKey = rows[pos].key
	u.cursorPath = nil
	if strings.HasPrefix(u.cursorKey, "g:") {
		u.cursorPath = rows[pos].path
	}
}

// paginate fills pages with whole blocks, each page starting with the top
// lines. A page that starts inside a group first repeats the group's path.
func paginate(blocks []block, top []string, height int) [][]block {
	var pages [][]block
	topBlock := block{lines: top}
	page, used := []block{topBlock}, len(top)
	started := false
	for i, b := range blocks {
		need := b.height()
		if b.heading && i+1 < len(blocks) {
			// a heading never ends a page apart from what it heads
			need += blocks[i+1].height()
		}
		if started && used+need > height {
			pages = append(pages, page)
			page, used = []block{topBlock}, len(top)
			path := b.path
			if b.heading {
				path = path[:len(path)-1]
			}
			if len(path) > 0 {
				page = append(page, block{lines: []string{groupStyle(strings.Join(path, pathSeparator)) + helpStyle(" (cont.)")}})
				used++
			}
		}
		page = append(page, b)
		used += b.height()
		started = true
	}
	return append(pages, page)
}

// groupLine is a group heading with its subtotals (R4).
// groupLineWidths are the column widths shared by one-line group headings.
type groupLineWidths struct {
	name, nodes, pods int
}

func (w *groupLineWidths) fit(u *UIModel, g *Group, t groupTree) {
	w.name = max(w.name, textWidth(t.branch)+textWidth(u.groupName(g)))
	w.nodes = max(w.nodes, len(u.printer.Sprintf("%d nodes", g.Stats.NumNodes)))
	w.pods = max(w.pods, len(u.printer.Sprintf("%d pods", g.Stats.NumPods)))
}

func (u *UIModel) groupLine(g *Group, t groupTree, w groupLineWidths) string {
	s := g.Stats
	var b strings.Builder
	b.WriteString(pad(treeStyle(t.branch)+u.groupName(g), w.name))
	b.WriteString("  " + pad(u.printer.Sprintf("%d nodes", s.NumNodes), w.nodes))
	b.WriteString("  " + pad(u.printer.Sprintf("%d pods", s.NumPods), w.pods))
	for _, res := range u.cluster.resources {
		pct := s.PercentUsed(res)
		fmt.Fprintf(&b, "  %s %s", res, u.colorizeUsage(pct, fmt.Sprintf("%0.0f%%", pct)))
	}
	if !u.DisablePricing {
		if s.PricedNodes > 0 {
			price := u.printer.Sprintf("$%0.3f/hour | $%0.3f/month", s.TotalPrice, s.TotalPrice*avgHoursPerMonth)
			if !s.FullyPriced() {
				// an unknown price somewhere, so this is a lower bound
				price = ">= " + price
			}
			b.WriteString("  " + price)
		}
		if s.ReservedNodes > 0 {
			u.printer.Fprintf(&b, " (+%d reserved, not priced)", s.ReservedNodes)
		}
		if s.IdlePrice >= 0.0005 {
			b.WriteString(helpStyle(u.printer.Sprintf("  idle $%0.3f/month", s.IdlePrice*avgHoursPerMonth)))
		}
	}
	return b.String()
}

// groupTree is how a group heading hangs in the tree: the connector before
// its name, what continues below it on its own extra lines, and whether a
// blank line sets it apart (top-level groups after the first).
type groupTree struct {
	branch, below string
	spaced        bool
}

// treeOf lays out the group hierarchy: top-level groups by name, sub-groups
// under them with ├─ / └─ connectors, so each heading shows only its own
// value.
// Groups deeper than depth, and the contents of collapsed groups, are left
// out; depth 0 keeps every level.
func treeOf(groups []*Group, depth int, collapsed func([]string) bool) map[*Group]groupTree {
	out := map[*Group]groupTree{}
	var walk func(gs []*Group, prefix string, top bool)
	walk = func(gs []*Group, prefix string, top bool) {
		for i, g := range gs {
			last := i == len(gs)-1
			t := groupTree{spaced: top && i > 0}
			childPrefix := ""
			if !top {
				connector, rest := "├─ ", "│  "
				if last {
					connector, rest = "└─ ", "   "
				}
				t.branch = prefix + connector
				childPrefix = prefix + rest
			}
			shown := (depth == 0 || len(g.Path) < depth) && !collapsed(g.Path)
			t.below = childPrefix
			if len(g.Groups) > 0 && shown {
				t.below += "│"
			}
			out[g] = t
			if shown {
				walk(g.Groups, childPrefix, false)
			}
		}
	}
	walk(groups, "", true)
	return out
}

// groupName is the heading's own value, styled by level, with ▸ when the
// group is folded on its own.
func (u *UIModel) groupName(g *Group) string {
	name := g.Name()
	if u.view.IsCollapsed(g.Path) {
		name = "▸ " + name
	}
	if len(g.Path) == 1 {
		return topGroupStyle(name)
	}
	return subGroupStyle(name)
}

// groupTable lays out every group, nested ones included, for the collapsed
// view: name, node and pod counts, then a usage bar per resource, one line
// each, and the cost on the first line. Columns line up across all groups.
func (u *UIModel) groupTable(groups []*Group, tree map[*Group]groupTree, depth int) map[*Group][]string {
	// only the groups at the barred level; the levels around them are one line
	var all []*Group
	var collect func([]*Group)
	collect = func(gs []*Group) {
		for _, g := range gs {
			if len(g.Path) == depth {
				all = append(all, g)
			} else if len(g.Path) < depth {
				collect(g.Groups)
			}
		}
	}
	collect(groups)

	type groupRow struct {
		name, nodes, pods, price string
	}
	rows := make(map[*Group]groupRow, len(all))
	nameW, nodesW, podsW, resW := 0, 0, 0, 0
	for _, res := range u.cluster.resources {
		resW = max(resW, len(res))
	}
	for _, g := range all {
		s := g.Stats
		r := groupRow{
			name:  treeStyle(tree[g].branch) + u.groupName(g),
			nodes: u.printer.Sprintf("%d nodes", s.NumNodes),
			pods:  u.printer.Sprintf("%d pods", s.NumPods),
		}
		if !u.DisablePricing {
			if s.PricedNodes > 0 {
				r.price = u.printer.Sprintf("$%0.3f/hour | $%0.3f/month", s.TotalPrice, s.TotalPrice*avgHoursPerMonth)
				if !s.FullyPriced() {
					r.price = ">= " + r.price
				}
			}
			if s.ReservedNodes > 0 {
				r.price += u.printer.Sprintf(" (+%d reserved, not priced)", s.ReservedNodes)
			}
			if s.IdlePrice >= 0.0005 {
				r.price += helpStyle(u.printer.Sprintf("  idle $%0.3f/month", s.IdlePrice*avgHoursPerMonth))
			}
		}
		nameW = max(nameW, textWidth(r.name))
		nodesW = max(nodesW, len(r.nodes))
		podsW = max(podsW, len(r.pods))
		rows[g] = r
	}

	barW := nodeBarWidth
	if u.width > 0 {
		priceW := 0
		for _, r := range rows {
			priceW = max(priceW, textWidth(r.price))
		}
		spare := u.width - (nameW + 2 + nodesW + 2 + podsW + 2 + resW + 1) - 2 - priceW
		barW = min(max(spare, nodeBarWidth), maxNodeBarWidth)
	}
	out := make(map[*Group][]string, len(all))
	for _, g := range all {
		r := rows[g]
		var lines []string
		if tree[g].spaced {
			lines = append(lines, "")
		}
		for i, res := range u.cluster.resources {
			bar := pad(u.usageBar(g.Stats.PercentUsed(res)/100, barW, barRune(i)), barW)
			if i == 0 {
				line := pad(r.name, nameW) + "  " + pad(r.nodes, nodesW) + "  " + pad(r.pods, podsW) + "  " + pad(string(res), resW) + " " + bar
				if r.price != "" {
					line += "  " + r.price
				}
				lines = append(lines, line)
			} else {
				lines = append(lines, pad(treeStyle(tree[g].below), nameW+2+nodesW+2+podsW+2)+pad(string(res), resW)+" "+bar)
			}
		}
		out[g] = lines
	}
	return out
}

// nodeTable is the node list laid out in aligned columns: name, a bar per
// resource, pods, then the view's columns that fit the width. Widths come from
// every selected node so columns don't shift between pages; lines are built
// only for the nodes shown.
type nodeTable struct {
	u      *UIModel
	title  string
	rows   map[*Node]*tableRow
	shown  []Field
	hidden int
	indent string
	nameW  int
	resW   int
	barW   int
	podsW  int
	colW   map[Field]int
}

type tableRow struct {
	name, pods string
	// fills are the usage of each resource, drawn as bars per page
	fills []float64
	cells map[Field]string
}

func columnTitle(f Field) string {
	switch f {
	case FieldInstanceType:
		return "INSTANCE"
	case FieldPrice:
		return "PRICE/H"
	case FieldIdle:
		return "IDLE/H"
	case FieldPriceMonthly:
		return "PRICE/M"
	case FieldIdleMonthly:
		return "IDLE/M"
	}
	if key := f.Label(); key != "" {
		return strings.ToUpper(key[strings.LastIndex(key, "/")+1:])
	}
	return strings.ToUpper(string(f))
}

// textWidth is the screen width of s: its length for plain ASCII, as most
// cells are, else what lipgloss measures.
func textWidth(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 || s[i] == 0x1b {
			return lipgloss.Width(s)
		}
	}
	return len(s)
}

func pad(s string, w int) string {
	if gap := w - textWidth(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}

func (u *UIModel) newNodeTable(nodes []*Node, indent int) *nodeTable {
	now := time.Now()
	columns := make([]Field, 0, len(u.view.Columns))
	for _, c := range u.view.Columns {
		if u.DisablePricing && isPriceColumn(c) {
			continue
		}
		columns = append(columns, c)
	}

	t := &nodeTable{
		u:      u,
		rows:   make(map[*Node]*tableRow, len(nodes)),
		indent: strings.Repeat(" ", indent),
		nameW:  len("NAME"),
		podsW:  len("PODS"),
		colW:   map[Field]int{},
	}
	for _, c := range columns {
		t.colW[c] = textWidth(columnTitle(c))
	}
	for _, res := range u.cluster.resources {
		t.resW = max(t.resW, len(res))
	}

	for _, n := range nodes {
		allocatable, used := n.Allocatable(), n.Used()
		maxPods, _ := allocatable.Pods().AsInt64()
		r := &tableRow{name: n.Name(), pods: fmt.Sprintf("%d/%d", n.NumPods(), maxPods), cells: make(map[Field]string, len(columns))}
		for _, res := range u.cluster.resources {
			pct := 0.0
			a, used := allocatable[res], used[res]
			if a.AsApproximateFloat64() != 0 {
				pct = used.AsApproximateFloat64() / a.AsApproximateFloat64()
			}
			r.fills = append(r.fills, pct)
		}
		for _, c := range columns {
			cell := u.cell(n, c, now)
			r.cells[c] = cell
			t.colW[c] = max(t.colW[c], textWidth(cell))
		}
		t.nameW = max(t.nameW, textWidth(r.name))
		t.podsW = max(t.podsW, len(r.pods))
		t.rows[n] = r
	}
	t.barW = nodeBarWidth
	fixed := indent + t.nameW + 1 + t.resW + 1 + t.barW + 1 + t.podsW
	t.shown, t.hidden = columns, 0
	if u.width > 0 {
		t.shown, t.hidden = FitColumns(columns, u.width-fixed, func(f Field) int { return t.colW[f] + 2 })
		spare := u.width - fixed
		for _, c := range t.shown {
			spare -= t.colW[c] + 2
		}
		t.barW += min(max(spare, 0), maxNodeBarWidth-nodeBarWidth)
	}

	titles := make(map[Field]string, len(t.shown))
	for _, c := range t.shown {
		titles[c] = columnTitle(c)
	}
	t.title = columnTitleStyle(t.indent + pad("NAME", t.nameW) + " " + pad("", t.resW) + " " + pad("", t.barW) + " " + pad("PODS", t.podsW) + t.tail(titles))
	return t
}

func (t *nodeTable) tail(cells map[Field]string) string {
	var b strings.Builder
	for _, c := range t.shown {
		b.WriteString("  ")
		b.WriteString(pad(cells[c], t.colW[c]))
	}
	return strings.TrimRight(b.String(), " ")
}

// render is the node's lines: one per resource, the first with its columns.
func (t *nodeTable) render(n *Node) []string {
	u, r := t.u, t.rows[n]

	lines := make([]string, 0, len(r.fills))
	for j, res := range u.cluster.resources {
		bar := pad(u.usageBar(r.fills[j], t.barW, barRune(j)), t.barW)
		if j == 0 {
			lines = append(lines, t.indent+pad(r.name, t.nameW)+" "+pad(string(res), t.resW)+" "+bar+" "+pad(r.pods, t.podsW)+t.tail(r.cells))
		} else {
			lines = append(lines, strings.TrimRight(t.indent+pad("", t.nameW)+" "+pad(string(res), t.resW)+" "+bar, " "))
		}
	}
	return lines
}

// isPriceColumn tells the columns that show a price, left out when pricing
// is disabled.
func isPriceColumn(f Field) bool {
	switch f {
	case FieldPrice, FieldPriceMonthly, FieldIdle, FieldIdleMonthly:
		return true
	}
	return false
}

// cell is the node's value in a column.
func (u *UIModel) cell(n *Node, f Field, now time.Time) string {
	switch f {
	case FieldPrice:
		switch {
		case !n.HasPrice():
			return "-"
		case n.PriceIsEquivalent():
			// not what the node costs, see R8
			return fmt.Sprintf("~$%0.4f", n.Price())
		default:
			return fmt.Sprintf("$%0.4f", n.Price())
		}
	case FieldIdle:
		if idle, ok := n.IdleHourly(); ok {
			return fmt.Sprintf("$%0.4f", idle)
		}
		return "-"
	case FieldPriceMonthly:
		switch {
		case !n.HasPrice():
			return "-"
		case n.PriceIsEquivalent():
			return u.printer.Sprintf("~$%0.2f", n.Price()*avgHoursPerMonth)
		default:
			return u.printer.Sprintf("$%0.2f", n.Price()*avgHoursPerMonth)
		}
	case FieldIdleMonthly:
		if idle, ok := n.IdleHourly(); ok {
			return u.printer.Sprintf("$%0.2f", idle*avgHoursPerMonth)
		}
		return "-"
	case FieldStatus:
		flags := n.StatusFlags()
		for i, f := range flags {
			switch f {
			case StatusReady:
				flags[i] = u.style.green(f)
			case StatusNotReady:
				flags[i] = u.style.red(f + "/" + duration.HumanDuration(now.Sub(n.NotReadyTime())))
			case StatusDeleting:
				flags[i] = u.style.red(f)
			default: // Cordoned, Launching
				flags[i] = u.style.yellow(f)
			}
		}
		return strings.Join(flags, "/")
	case FieldAge:
		return duration.HumanDuration(now.Sub(n.Created()))
	case FieldNodeGroup:
		if n.IsManagedNodeGroup() {
			return managedNodeGroupStyle(n.NodeGroup())
		}
		return n.NodeGroup()
	case FieldCapacity:
		capacity := n.CapacityType()
		c := string(capacity)
		switch capacity {
		case CapacitySpot:
			c = spotStyle(c)
		case CapacityReserved:
			c = reservedStyle(c)
		case CapacityFargate:
			c = fargateStyle(c)
		}
		if n.LabelValue("eks.amazonaws.com/compute-type") == "auto" {
			c += "/Auto"
		}
		return c
	}
	if v := f.Value(n); v != UngroupedValue {
		return v
	}
	return "-"
}
