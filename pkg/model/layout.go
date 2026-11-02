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
	"k8s.io/apimachinery/pkg/util/duration"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Render
	groupStyle = lipgloss.NewStyle().Bold(true).Render
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

// render draws the whole screen: header, cluster summary, then the open panel
// or one page of the node list, then the footer.
func (u *UIModel) render() string {
	stats := u.stats
	nodes := u.view.Select(stats.Nodes)

	var b strings.Builder
	b.WriteString(u.header(stats, nodes))
	b.WriteString(u.clusterSummary(stats))
	b.WriteString("\n")

	// what is left between the summary and the two footer lines
	avail := math.MaxInt
	if u.height > 0 {
		avail = max(u.height-strings.Count(b.String(), "\n")-2, 3)
	}

	var body []string
	status := ""
	switch {
	case u.panel == panelHelp:
		body = u.helpLines()
	case u.panel == panelFilters:
		body = u.filterLines(avail)
	case u.panel == panelGroupBy:
		body = u.groupByLines(avail)
	case u.panel == panelSort:
		body = u.sortLines(avail)
	case u.panel == panelColumns:
		body = u.columnLines(avail)
	case u.panel == panelContexts:
		body = u.contextLines(avail)
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
			status += helpStyle(fmt.Sprintf("  %d columns hidden, widen the window", hidden))
		}
	}
	if len(body) > avail {
		body = body[:avail]
	}
	if u.searching {
		status = "/" + u.view.Search + "█"
	}
	for _, line := range body {
		b.WriteString(line)
		b.WriteString("\n")
	}
	b.WriteString(status)
	b.WriteString("\n")
	b.WriteString(helpStyle(u.helpText()))
	return b.String()
}

func (u *UIModel) helpText() string {
	if u.searching {
		return "type to search • enter: keep • esc: clear"
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
	}
	keys := "f: filter • g: group • s: sort • o: columns • /: search • r: resources • c: context"
	if len(u.view.GroupBy) > 0 {
		keys += " • n: nodes"
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
	var walk func(groups []*Group)
	walk = func(groups []*Group) {
		for _, g := range groups {
			blocks = append(blocks, block{path: g.Path, heading: true, lines: []string{u.groupLine(g)}})
			walk(g.Groups)
			if table != nil {
				for _, n := range g.Nodes {
					blocks = append(blocks, block{path: g.Path, node: n, count: len(u.cluster.resources)})
				}
			}
		}
	}
	if groups := u.view.Groups(nodes); groups != nil {
		walk(groups)
	} else {
		for _, n := range nodes {
			blocks = append(blocks, block{node: n, count: len(u.cluster.resources)})
		}
	}
	pages := paginate(blocks, top, height)
	u.page = min(u.page, len(pages)-1)
	for _, b := range pages[u.page] {
		if b.node != nil {
			lines = append(lines, table.render(b.node)...)
		} else {
			lines = append(lines, b.lines...)
		}
	}
	return lines, len(pages), hiddenColumns
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
func (u *UIModel) groupLine(g *Group) string {
	s := g.Stats
	var b strings.Builder
	b.WriteString(strings.Repeat("  ", len(g.Path)-1))
	b.WriteString(groupStyle(strings.Join(g.Path, pathSeparator)))
	u.printer.Fprintf(&b, "  %d nodes  %d pods", s.NumNodes, s.NumPods)
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
	}
	return b.String()
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
		if c == FieldPrice && u.DisablePricing {
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
	t.title = titleStyle(t.indent + pad("NAME", t.nameW) + " " + pad("", t.resW) + " " + pad("", t.barW) + " " + pad("PODS", t.podsW) + t.tail(titles))
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
	defer func(width int, pct bool) { u.progress.Width, u.progress.ShowPercentage = width, pct }(u.progress.Width, u.progress.ShowPercentage)
	u.progress.Width, u.progress.ShowPercentage = t.barW, true

	lines := make([]string, 0, len(r.fills))
	for j, res := range u.cluster.resources {
		bar := pad(u.progressBar(r.fills[j]), t.barW)
		if j == 0 {
			lines = append(lines, t.indent+pad(r.name, t.nameW)+" "+pad(string(res), t.resW)+" "+bar+" "+pad(r.pods, t.podsW)+t.tail(r.cells))
		} else {
			lines = append(lines, strings.TrimRight(t.indent+pad("", t.nameW)+" "+pad(string(res), t.resW)+" "+bar, " "))
		}
	}
	return lines
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
	case FieldStatus:
		flags := n.StatusFlags()
		if flags[0] == StatusNotReady {
			flags[0] += "/" + duration.HumanDuration(now.Sub(n.NotReadyTime()))
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
		c := string(n.CapacityType())
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
