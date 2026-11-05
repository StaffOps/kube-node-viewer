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
	"sort"
	"strings"

	"github.com/charmbracelet/x/ansi"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// The Pods tab of the node details popup (node-pods): every pod bound to the
// node with its CPU and memory requests, the share of the node they take and
// what that share costs. DaemonSet pods are listed with the others, marked
// "ds"; `d` switches to a view of one row per DaemonSet under their Total.

// podSortField is what the Pods tab sorts by.
type podSortField int

const (
	podSortCost podSortField = iota
	podSortCPU
	podSortMemory
	podSortName
)

var podSortNames = map[podSortField]string{
	podSortCost: "cost", podSortCPU: "cpu", podSortMemory: "memory", podSortName: "name",
}

// podSort is the Pods tab sort: a field and its direction.
type podSort struct {
	field podSortField
	desc  bool
}

// defaultPodSort is cost, biggest first, or CPU with pricing disabled (R7, R4).
func defaultPodSort(disablePricing bool) podSort {
	if disablePricing {
		return podSort{field: podSortCPU, desc: true}
	}
	return podSort{field: podSortCost, desc: true}
}

// next is the following field of the cycle cost → cpu → memory → name,
// skipping cost with pricing disabled, in its starting direction: biggest
// first, A to Z for the name.
func (s podSort) next(disablePricing bool) podSort {
	f := (s.field + 1) % (podSortName + 1)
	if f == podSortCost && disablePricing {
		f = podSortCPU
	}
	return podSort{field: f, desc: f != podSortName}
}

func (s podSort) String() string {
	arrow := "↑"
	if s.desc {
		arrow = "↓"
	}
	return "sort: " + podSortNames[s.field] + " " + arrow
}

// podsView is the view of the Pods tab (R6, R9).
type podsView int

const (
	podsViewAll podsView = iota
	podsViewDaemonSets
)

func (v podsView) String() string {
	if v == podsViewDaemonSets {
		return "view: daemonsets"
	}
	return "view: all pods"
}

// unnamedDaemonSet labels the DaemonSet pods whose controller name is not
// recorded (R9).
const unnamedDaemonSet = "<unnamed>"

// podRow is a row of the Pods tab: a pod, one DaemonSet or the Total of the
// DaemonSets view. Percents, share and cost are fractions (0.25 is 25%).
type podRow struct {
	// key names the row for the cursor
	key string
	// name is namespace/name, or the Total's label
	name string
	// ds marks a DaemonSet pod in the all-pods view (R6)
	ds bool

	cpu, mem       resource.Quantity
	hasCPU, hasMem bool
	// noRequests is set when neither cpu nor memory is requested (R5)
	noRequests bool

	cpuPct, memPct     float64
	cpuPctOK, memPctOK bool
	share              float64
	shareOK            bool
	cost               float64
	costOK             bool
	costEquivalent     bool
}

// podPrice is the node price the costs come from.
type podPrice struct {
	price      float64
	known      bool
	equivalent bool
}

// newPodRow fills the percents, share and cost of a row from its requests.
// The share is the larger of the CPU and memory percents of allocatable; a
// percent whose allocatable is zero or missing is left out (R3).
func newPodRow(key, name string, req v1.ResourceList, alloc v1.ResourceList, price podPrice) podRow {
	r := podRow{key: key, name: name}
	r.cpu, r.hasCPU = req[v1.ResourceCPU]
	r.mem, r.hasMem = req[v1.ResourceMemory]
	r.hasCPU = r.hasCPU && !r.cpu.IsZero()
	r.hasMem = r.hasMem && !r.mem.IsZero()
	r.noRequests = !r.hasCPU && !r.hasMem
	if r.noRequests {
		return r
	}
	ratio := func(q resource.Quantity, res v1.ResourceName) (float64, bool) {
		a, ok := alloc[res]
		if !ok || a.IsZero() {
			return 0, false
		}
		return q.AsApproximateFloat64() / a.AsApproximateFloat64(), true
	}
	r.cpuPct, r.cpuPctOK = ratio(r.cpu, v1.ResourceCPU)
	r.memPct, r.memPctOK = ratio(r.mem, v1.ResourceMemory)
	if r.cpuPctOK {
		r.share, r.shareOK = r.cpuPct, true
	}
	if r.memPctOK {
		r.share, r.shareOK = max(r.share, r.memPct), true
	}
	if r.shareOK && price.known {
		r.cost, r.costOK, r.costEquivalent = r.share*price.price, true, price.equivalent
	}
	return r
}

// podRows lists the sorted rows of the Pods tab in a view (R2, R5, R6, R7,
// R9): one row per pod, DaemonSet pods marked, or one row per DaemonSet. pods
// are the pods the node's totals count.
func podRows(pods []*Pod, alloc v1.ResourceList, price podPrice, s podSort, view podsView) []podRow {
	var rows []podRow
	if view == podsViewDaemonSets {
		rows, _ = daemonSetRows(pods, alloc, price)
	} else {
		for _, p := range pods {
			id := p.Namespace() + "/" + p.Name()
			r := newPodRow("p:"+id, id, p.Requested(), alloc, price)
			r.ds = isDaemonSetPod(p)
			rows = append(rows, r)
		}
	}
	sortPodRows(rows, s)
	return rows
}

func isDaemonSetPod(p *Pod) bool {
	kind, _ := p.Owner()
	return kind == "DaemonSet"
}

// daemonSetTotal is the summed requests of a node's DaemonSet pods.
type daemonSetTotal struct {
	req        v1.ResourceList
	pods, sets int
}

// daemonSetRows is one unsorted row per DaemonSet of the pods, by
// namespace/name, the pods with no recorded name under one <unnamed> row
// that sets does not count, and their summed requests (R9).
func daemonSetRows(pods []*Pod, alloc v1.ResourceList, price podPrice) ([]podRow, daemonSetTotal) {
	byDS := map[string]v1.ResourceList{}
	var order []string
	total := daemonSetTotal{req: v1.ResourceList{}}
	for _, p := range pods {
		kind, owner := p.Owner()
		if kind != "DaemonSet" {
			continue
		}
		id := unnamedDaemonSet
		if owner != "" {
			id = p.Namespace() + "/" + owner
		}
		req, ok := byDS[id]
		if !ok {
			req = v1.ResourceList{}
			byDS[id] = req
			order = append(order, id)
			if owner != "" {
				total.sets++
			}
		}
		addResources(req, p.Requested())
		addResources(total.req, p.Requested())
		total.pods++
	}
	rows := make([]podRow, 0, len(order))
	for _, id := range order {
		rows = append(rows, newPodRow("ds:"+id, id, byDS[id], alloc, price))
	}
	return rows, total
}

// daemonSetTotalRow is the fixed Total line of the DaemonSets view, from the
// summed requests, not the rows' shares (R9).
func daemonSetTotalRow(t daemonSetTotal, alloc v1.ResourceList, price podPrice) podRow {
	label := fmt.Sprintf("Total (%s, %s)", plural(t.pods, "pod"), plural(t.sets, "set"))
	return newPodRow("", label, t.req, alloc, price)
}

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// sortPodRows orders rows by the sort; rows with no requests go last and
// rows without a share after those with one, in either direction. Ties break
// on namespace/name (R5, R7).
func sortPodRows(rows []podRow, s podSort) {
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.noRequests != b.noRequests {
			return !a.noRequests
		}
		if !a.noRequests {
			// a row without the sorted value goes last in both directions
			if has, hasB := sortedValueOK(a, s.field), sortedValueOK(b, s.field); has != hasB {
				return has
			}
			if c := comparePodRows(a, b, s.field); c != 0 {
				if s.desc {
					return c > 0
				}
				return c < 0
			}
		} else if s.field == podSortName && s.desc && a.name != b.name {
			return a.name > b.name
		}
		return a.name < b.name
	})
}

// sortedValueOK reports whether the row has the value the sort orders by: a
// share for cost, a CPU or memory request for those fields; names always.
func sortedValueOK(r podRow, field podSortField) bool {
	switch field {
	case podSortCost:
		return r.shareOK
	case podSortCPU:
		return r.hasCPU
	case podSortMemory:
		return r.hasMem
	}
	return true
}

// comparePodRows is -1, 0 or 1 as a's value is less than, equal to or more
// than b's.
func comparePodRows(a, b podRow, field podSortField) int {
	switch field {
	case podSortCost:
		// every row of a node has the same price, so cost orders like the
		// share, also without a known price
		switch {
		case a.share < b.share:
			return -1
		case a.share > b.share:
			return 1
		}
		return 0
	case podSortCPU:
		return a.cpu.Cmp(b.cpu)
	case podSortMemory:
		return a.mem.Cmp(b.mem)
	case podSortName:
		return strings.Compare(a.name, b.name)
	}
	return 0
}

// Column widths of the Pods tab, two spaces apart.
const (
	podsCPUW    = 5
	podsPctW    = 4
	podsMemW    = 6
	podsShareW  = 5
	podsCostW   = 8
	podsMinPodW = 12
	podsGap     = "  "
	podsMarkerW = 2
)

// podsTable lays out the Pods tab for an inner width (0: no limit).
type podsTable struct {
	podW     int
	percents bool
	// values shows the CPU and MEMORY request columns; only the narrowest
	// windows drop them, after the percents, keeping POD, SHARE and COST/H.
	values bool
	cost   bool
}

func newPodsTable(rows []podRow, title string, avail int, pricing bool) podsTable {
	t := podsTable{percents: true, values: true, cost: pricing}
	natural := textWidth(title)
	for _, r := range rows {
		natural = max(natural, textWidth(r.label()))
	}
	t.podW = natural
	if avail <= 0 {
		return t
	}
	need := min(natural, podsMinPodW)
	if avail-podsMarkerW-t.others() < need {
		t.percents = false
	}
	if avail-podsMarkerW-t.others() < need {
		t.values = false
	}
	t.podW = max(min(natural, avail-podsMarkerW-t.others()), need)
	return t
}

// others is the width of every column but POD and the marker.
func (t podsTable) others() int {
	w := len(podsGap) + podsShareW
	if t.values {
		w += 2*len(podsGap) + podsCPUW + podsMemW
	}
	if t.percents {
		w += 2 * (len(podsGap) + podsPctW)
	}
	if t.cost {
		w += len(podsGap) + podsCostW
	}
	return w
}

func rightAlign(s string, w int) string {
	if gap := w - textWidth(s); gap > 0 {
		return strings.Repeat(" ", gap) + s
	}
	return s
}

func (t podsTable) line(marker, name string, cells [6]string) string {
	var b strings.Builder
	b.WriteString(pad(marker, podsMarkerW))
	b.WriteString(pad(ansi.Truncate(name, t.podW, "…"), t.podW))
	add := func(s string, w int) {
		b.WriteString(podsGap)
		b.WriteString(rightAlign(s, w))
	}
	if t.values {
		add(cells[0], podsCPUW)
	}
	if t.percents {
		add(cells[1], podsPctW)
	}
	if t.values {
		add(cells[2], podsMemW)
	}
	if t.percents {
		add(cells[3], podsPctW)
	}
	add(cells[4], podsShareW)
	if t.cost {
		add(cells[5], podsCostW)
	}
	return b.String()
}

func (t podsTable) header(title string) string {
	return columnTitleStyle(t.line("", title, [6]string{"CPU", "CPU%", "MEMORY", "MEM%", "SHARE", "COST/H"}))
}

func (t podsTable) row(r podRow) string {
	marker := ""
	if r.noRequests {
		return pad(marker, podsMarkerW) + pad(t.podCell(r), t.podW) + podsGap + helpStyle("no requests")
	}
	cells := [6]string{"-", "-", "-", "-", "-", "-"}
	if r.hasCPU {
		cells[0] = cpuCores(r.cpu.AsApproximateFloat64())
		if r.cpuPctOK {
			cells[1] = podPercent(r.cpuPct)
		}
	}
	if r.hasMem {
		cells[2] = memBytes(r.mem.AsApproximateFloat64())
		if r.memPctOK {
			cells[3] = podPercent(r.memPct)
		}
	}
	if r.shareOK {
		cells[4] = podPercent(r.share)
	}
	if r.costOK {
		cells[5] = fmt.Sprintf("$%0.4f", r.cost)
		if r.costEquivalent {
			cells[5] = "~" + cells[5]
		}
	}
	return t.line(marker, t.podCell(r), cells)
}

// podCell is the row's text in the POD column, at most podW wide: on a
// narrow window the name is cut with … and the ds mark of a DaemonSet pod
// stays after it (R6).
func (t podsTable) podCell(r podRow) string {
	if !r.ds {
		return ansi.Truncate(r.name, t.podW, "…")
	}
	return ansi.Truncate(r.name, max(t.podW-len(podsDSMark), 1), "…") + podsDSMark
}

// podsDSMark follows the name of a DaemonSet pod in the all-pods view (R6).
const podsDSMark = " ds"

// label is the row's natural text in the POD column, used to size the
// column: the name, with the ds mark of a DaemonSet pod after it.
func (r podRow) label() string {
	if r.ds {
		return r.name + podsDSMark
	}
	return r.name
}

func podPercent(f float64) string { return fmt.Sprintf("%.0f%%", 100*f) }

// cpuCores shows cores with two decimals, fewer when they don't fit.
func cpuCores(cores float64) string {
	for _, format := range []string{"%.2f", "%.1f"} {
		if s := fmt.Sprintf(format, cores); len(s) <= podsCPUW {
			return s
		}
	}
	return fmt.Sprintf("%.0f", cores)
}

// memBytes shows bytes in Gi with one decimal, or Mi below 1Gi.
func memBytes(b float64) string {
	switch gi := b / (1 << 30); {
	case gi >= 100:
		return fmt.Sprintf("%.0fGi", gi)
	case gi >= 1:
		return fmt.Sprintf("%.1fGi", gi)
	case b >= 1<<20:
		return fmt.Sprintf("%.0fMi", math.Floor(b/(1<<20)))
	}
	return fmt.Sprintf("%.0fKi", b/(1<<10))
}

// nodePodPrice is the node's price for the costs.
func nodePodPrice(n *Node, disablePricing bool) podPrice {
	if disablePricing || !n.HasPrice() {
		return podPrice{}
	}
	return podPrice{price: n.Price(), known: true, equivalent: n.PriceIsEquivalent()}
}

// nodePodRows is the Pods tab rows of the node in the current view and sort.
func (u *UIModel) nodePodRows(n *Node) []podRow {
	return podRows(n.PodsCopy(), n.Allocatable(), nodePodPrice(n, u.DisablePricing), u.podsSort, u.podsView)
}

// podsCursorIndex finds the cursor's row: its key, else the row now at its
// last position, else the last row; -1 with no rows (R8). It records the
// result so the fallback sticks. The view calls it while drawing, so a frame
// can move the cursor, as detailsRows does for the Details tab.
func (u *UIModel) podsCursorIndex(rows []podRow) int {
	if len(rows) == 0 {
		return -1
	}
	i := -1
	for j, r := range rows {
		if r.key == u.podsCursor {
			i = j
			break
		}
	}
	if i < 0 {
		i = min(max(u.podsIndex, 0), len(rows)-1)
	}
	u.podsCursor, u.podsIndex = rows[i].key, i
	return i
}

// nodePodsKey handles the keys of the Pods tab (R7, R8, R9). enter and space
// do nothing here.
func (u *UIModel) nodePodsKey(key string) {
	switch key {
	case "esc":
		u.panel = panelNone
		return
	case "tab":
		u.detailsTab = detailsTabDetails
		return
	case "d":
		if u.podsView == podsViewDaemonSets {
			u.podsView = podsViewAll
		} else {
			u.podsView = podsViewDaemonSets
		}
		u.podsCursor, u.podsIndex = "", 0
		return
	case "s":
		u.podsSort = u.podsSort.next(u.DisablePricing)
		return
	case "S":
		u.podsSort.desc = !u.podsSort.desc
		return
	}
	n, ok := u.findNode(u.detailsNode)
	if !ok {
		return
	}
	rows := u.nodePodRows(n)
	i := u.podsCursorIndex(rows)
	if i < 0 {
		return
	}
	move := func(to int) {
		to = min(max(to, 0), len(rows)-1)
		u.podsCursor, u.podsIndex = rows[to].key, to
	}
	switch key {
	case "up", "k":
		move(i - 1)
	case "down", "j":
		move(i + 1)
	case "pgup":
		move(i - 10)
	case "pgdown":
		move(i + 10)
	case "home":
		move(0)
	}
}

// nodePodsPanel is the Pods tab, rebuilt each frame so it follows the live
// node (R2, R9).
func (u *UIModel) nodePodsPanel(n *Node) panelView {
	rows := u.nodePodRows(n)
	avail := 0
	if u.width > 0 {
		avail = u.width - 6
	}
	title, empty := "POD", "no pods"
	sized := rows
	var total podRow
	if u.podsView == podsViewDaemonSets {
		title, empty = "DAEMONSET", "no DaemonSets"
		_, sum := daemonSetRows(n.PodsCopy(), nil, podPrice{})
		total = daemonSetTotalRow(sum, n.Allocatable(), nodePodPrice(n, u.DisablePricing))
		sized = append(append([]podRow(nil), rows...), total)
	}
	t := newPodsTable(sized, title, avail, !u.DisablePricing)
	fixed := []string{"Details  [Pods]   " + u.podsView.String() + "\t" + u.podsSort.String(), u.nodePodsLine(n), t.header(title)}
	if u.podsView == podsViewDaemonSets && len(rows) > 0 {
		fixed = append(fixed, t.row(total))
	}
	if len(rows) == 0 {
		return panelView{title: n.Name(), fixed: fixed, rows: []string{helpStyle(empty)}, cursor: -1}
	}
	lines := make([]string, len(rows))
	for i, r := range rows {
		lines[i] = t.row(r)
	}
	return panelView{title: n.Name(), fixed: fixed, rows: lines, cursor: u.podsCursorIndex(rows)}
}

// daemonSetDetailRows is the DaemonSets section of the Details tab: what the
// node's DaemonSet pods request together, absolute and as a percent of
// allocatable, per resource (node-pods R11).
func daemonSetDetailRows(n *Node) []string {
	_, sum := daemonSetRows(n.PodsCopy(), nil, podPrice{})
	if sum.pods == 0 {
		return []string{"", titleStyle("DaemonSets") + helpStyle("  none on this node")}
	}
	r := newPodRow("", "", sum.req, n.Allocatable(), podPrice{})
	head := "  " + plural(sum.pods, "pod") + ", " + plural(sum.sets, "set")
	pct := func(f float64, ok bool) string {
		if !ok {
			return "-"
		}
		return podPercent(f)
	}
	cpu, mem := "0m", "0"
	if r.hasCPU {
		cpu = fmt.Sprintf("%dm", r.cpu.MilliValue())
	}
	if r.hasMem {
		mem = memBytes(float64(r.mem.Value()))
	}
	return []string{
		"", titleStyle("DaemonSets") + helpStyle("  requested (of allocatable)"),
		head,
		fmt.Sprintf("  %-6s  %s (%s)", "cpu", cpu, pct(r.cpuPct, r.cpuPctOK)),
		fmt.Sprintf("  %-6s  %s (%s)", "memory", mem, pct(r.memPct, r.memPctOK)),
	}
}

// DaemonSetLine is the footer line of the node list for the cursor's node:
// what the node's DaemonSet pods request, absolute and as a share of the
// node, with no cost (R10). It
// walks that node's bound pods only.
func DaemonSetLine(n *Node) string {
	_, sum := daemonSetRows(n.PodsCopy(), nil, podPrice{})
	if sum.pods == 0 {
		return n.Name() + " · no DaemonSets"
	}
	r := newPodRow("", "", sum.req, n.Allocatable(), podPrice{})
	pct := func(f float64, ok bool) string {
		if !ok {
			return "-"
		}
		return podPercent(f)
	}
	share := "-"
	if r.shareOK {
		share = podPercent(r.share)
	}
	cpu, mem := "0m", "0"
	if r.hasCPU {
		cpu = fmt.Sprintf("%dm", r.cpu.MilliValue())
	}
	if r.hasMem {
		mem = memBytes(float64(r.mem.Value()))
	}
	return fmt.Sprintf("%s · DaemonSets: %s of the node (cpu %s %s · mem %s %s)", n.Name(), share,
		cpu, pct(r.cpuPct, r.cpuPctOK), mem, pct(r.memPct, r.memPctOK))
}

// nodePodsLine is the node line above the pods: instance type, price, pod
// count and requested / allocatable CPU and memory (R2).
func (u *UIModel) nodePodsLine(n *Node) string {
	head := orDash(n.InstanceType())
	if !u.DisablePricing {
		price := "-"
		if p := nodePodPrice(n, false); p.known {
			price = fmt.Sprintf("$%0.4f/h", p.price)
			if p.equivalent {
				price = "~" + price
			}
		}
		head += " " + price
	}
	used, alloc := n.Used(), n.Allocatable()
	usage := func(res v1.ResourceName, format func(float64) string) string {
		q, a := used[res], alloc[res]
		text := format(q.AsApproximateFloat64()) + "/" + format(a.AsApproximateFloat64())
		if a.IsZero() {
			return text + " -"
		}
		return text + " " + podPercent(q.AsApproximateFloat64()/a.AsApproximateFloat64())
	}
	cpu := usage(v1.ResourceCPU, func(f float64) string { return fmt.Sprintf("%.2f", f) })
	mem := usage(v1.ResourceMemory, func(f float64) string { return fmt.Sprintf("%.1f", f/(1<<30)) })
	if i := strings.LastIndex(mem, " "); i >= 0 {
		mem = mem[:i] + "Gi" + mem[i:]
	}
	return strings.Join([]string{head, plural(n.NumPods(), "pod"), "cpu " + cpu, "mem " + mem}, " · ")
}
