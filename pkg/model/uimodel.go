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

// Modified by the kube-node-viewer authors, 2026.

package model

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/paginator"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/duration"

	"github.com/staffops/kube-node-viewer/pkg/text"
)

var (
	helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262")).Render
	// white / black
	activeDot = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "235", Dark: "252"}).Render("•")
	// black / white
	inactiveDot = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "250", Dark: "238"}).Render("•")
)

// avgHoursPerMonth turns an hourly price into a monthly one
const avgHoursPerMonth = (365 * 24) / 12

type UIModel struct {
	progress   progress.Model
	cluster    *Cluster
	paginator  paginator.Model
	height     int
	style      *Style
	view       ViewState
	showGroups bool
	// onViewChange receives the view state after the user changes it, to save
	// it for the cluster. It must not block.
	onViewChange func(ViewState)
	// printer formats numbers with commas, built once as the view is redrawn often
	printer        *message.Printer
	bars           map[barKey]string
	DisablePricing bool

	// gen is the cluster generation on screen, kept here so handling a message
	// never takes the cluster lock
	gen uint64
	// stats is the last snapshot of the cluster. View renders from it and never
	// takes the cluster lock, so keys stay responsive while events are applied.
	stats Stats
	// frame is the last rendered view, reused until something changes
	frame string
	dirty bool
}

// ChangedMsg tells the UI that cluster data or prices changed. Gen is the
// cluster generation the change belongs to; changes from a cluster the UI no
// longer shows are dropped.
type ChangedMsg struct {
	Gen uint64
}

// snapshotMsg carries a snapshot taken off the UI goroutine.
type snapshotMsg struct {
	gen   uint64
	stats Stats
}

type barKey struct {
	// fill is the level in thousandths, finer than the bar's own cells
	fill           int
	width          int
	showPercentage bool
}

// every fill at or above 100% draws the same full bar
const maxCachedBarFill = 2000

// progressBar renders the bar for a 0..1 fill level, cached because every cell of
// the gradient is a colour interpolated in Luv space and a frame draws dozens of
// bars. Rounding the level keeps the cache bounded and is finer than what the bar
// and its percentage can show. The cache needs no lock: only the view renders.
func (u *UIModel) progressBar(fill float64) string {
	if fill < 0 || math.IsNaN(fill) {
		fill = 0
	}
	rounded := int(math.Round(fill * 1000))
	if rounded > maxCachedBarFill {
		return u.progress.ViewAs(fill)
	}
	key := barKey{fill: rounded, width: u.progress.Width, showPercentage: u.progress.ShowPercentage}
	if bar, ok := u.bars[key]; ok {
		return bar
	}
	bar := u.progress.ViewAs(float64(rounded) / 1000)
	u.bars[key] = bar
	return bar
}

// NewUIModel starts with the default view state; SetView replaces it.
func NewUIModel(style *Style) *UIModel {
	pager := paginator.New()
	pager.Type = paginator.Dots
	pager.ActiveDot = activeDot
	pager.InactiveDot = inactiveDot
	cluster := NewCluster()
	return &UIModel{
		gen: cluster.Generation(),
		// red to green
		progress:  progress.New(style.gradient),
		cluster:   cluster,
		paginator: pager,
		style:     style,
		view:      DefaultViewState(),
		printer:   message.NewPrinter(language.English),
		bars:      map[barKey]string{},
		dirty:     true,
	}
}

func (u *UIModel) Cluster() *Cluster {
	return u.cluster
}

// SetView replaces the view state, as loaded at start or for a new cluster.
func (u *UIModel) SetView(v ViewState) {
	u.view = v
	if len(u.view.Resources) == 0 {
		u.view.Resources = []string{string(v1.ResourceCPU)}
	}
	u.SetResources(u.view.Resources)
	u.showGroups = len(u.view.GroupBy) > 0
	if !u.showGroups {
		u.view.ShowNodes = true
	}
	u.paginator.Page = 0
	u.dirty = true
}

// View state as it is now.
func (u *UIModel) ViewState() ViewState {
	return u.view
}

// OnViewChange sets the function told about view changes made from the TUI.
func (u *UIModel) OnViewChange(f func(ViewState)) {
	u.onViewChange = f
}

func (u *UIModel) viewChanged() {
	if u.onViewChange != nil {
		u.onViewChange(u.view)
	}
}

// toggleGroups and toggleNodes never leave both sections hidden, there would be
// nothing left to look at.
func (u *UIModel) toggleGroups() {
	if len(u.view.GroupBy) == 0 || (u.showGroups && !u.view.ShowNodes) {
		return
	}
	u.showGroups = !u.showGroups
	u.paginator.Page = 0
}

func (u *UIModel) toggleNodes() {
	if u.view.ShowNodes && !u.showGroups {
		return
	}
	u.view.ShowNodes = !u.view.ShowNodes
	u.paginator.Page = 0
	u.viewChanged()
}

func (u *UIModel) Init() tea.Cmd {
	return tea.Batch(tickCmd(), u.snapshotCmd())
}

// Refresh takes a snapshot of the cluster now, on the calling goroutine.
func (u *UIModel) Refresh() {
	u.stats = u.cluster.Stats()
	u.dirty = true
}

// snapshotCmd takes the snapshot off the UI goroutine, which may wait for the
// cluster lock while events are applied.
func (u *UIModel) snapshotCmd() tea.Cmd {
	cluster, gen := u.cluster, u.gen
	return func() tea.Msg {
		return snapshotMsg{gen: gen, stats: cluster.Stats()}
	}
}

func (u *UIModel) View() string {
	if !u.dirty {
		return u.frame
	}
	u.frame = u.render()
	u.dirty = false
	return u.frame
}

func (u *UIModel) render() string {
	b := strings.Builder{}

	// cluster totals stay whole; the lists below show the selected nodes
	stats := u.stats
	nodes := u.view.Select(stats.Nodes)

	ctw := text.NewColorTabWriter(&b, 0, 8, 1)
	u.writeClusterSummary(u.cluster.resources, stats, ctw)
	ctw.Flush()
	u.printer.Fprintf(&b, "%d pods (%d pending %d running %d bound)\n", stats.TotalPods,
		stats.PodsByPhase[v1.PodPending], stats.PodsByPhase[v1.PodRunning], stats.BoundPodCount)

	if stats.NumNodes == 0 {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "Waiting for update or no nodes found...")
		fmt.Fprintln(&b, u.paginator.View())
		fmt.Fprintln(&b, helpStyle(u.helpText()))
		return b.String()
	}

	if u.showGroups && len(u.view.GroupBy) > 0 {
		var groups []GroupStats
		for _, g := range u.view.Groups(nodes) {
			groups = append(groups, g.Stats)
		}
		fmt.Fprintln(&b)
		if u.view.ShowNodes {
			u.writeGroupSummary(groups, ctw)
		} else {
			// with no node list, the groups are the paginated content
			u.paginator.PerPage = u.itemsPerPage(&b, len(u.cluster.resources))
			u.paginator.SetTotalPages(len(groups))
			u.clampPage(len(groups))
			if start, end := u.paginator.GetSliceBounds(len(groups)); start >= 0 && end >= start {
				u.writeGroupSummary(groups[start:end], ctw)
			}
		}
		ctw.Flush()
	}

	if !u.view.ShowNodes {
		fmt.Fprintln(&b, u.paginator.View())
		fmt.Fprintln(&b, helpStyle(u.helpText()))
		return b.String()
	}

	fmt.Fprintln(&b)
	u.progress.ShowPercentage = true
	if len(nodes) == 0 {
		fmt.Fprintln(&b, "No nodes match the filters.")
		fmt.Fprintln(&b, helpStyle(u.helpText()))
		return b.String()
	}
	u.paginator.PerPage = u.computeItemsPerPage(nodes, &b)
	u.paginator.SetTotalPages(len(nodes))
	u.clampPage(len(nodes))
	start, end := u.paginator.GetSliceBounds(len(nodes))
	if start >= 0 && end >= start {
		for _, n := range nodes[start:end] {
			u.writeNodeInfo(n, ctw, u.cluster.resources)
		}
	}
	ctw.Flush()

	fmt.Fprintln(&b, u.paginator.View())
	fmt.Fprintln(&b, helpStyle(u.helpText()))
	return b.String()
}

// clampPage keeps the page in bounds as the item count shrinks underneath us.
func (u *UIModel) clampPage(totalItems int) {
	if u.paginator.PerPage > 0 && u.paginator.Page*u.paginator.PerPage > totalItems {
		u.paginator.Page = u.paginator.TotalPages - 1
	}
	if u.paginator.Page < 0 {
		u.paginator.Page = 0
	}
}

func (u *UIModel) helpText() string {
	if len(u.view.GroupBy) == 0 {
		return "←/→ page • q: quit"
	}
	groups, nodes := "hide", "hide"
	if !u.showGroups {
		groups = "show"
	}
	if !u.view.ShowNodes {
		nodes = "show"
	}
	return fmt.Sprintf("←/→ page • g: %s groups • n: %s nodes • q: quit", groups, nodes)
}

// extraLabels are the label columns, after the fixed ones.
func (u *UIModel) extraLabels() []string {
	var labels []string
	for _, c := range u.view.Columns {
		if key := c.Label(); key != "" {
			labels = append(labels, key)
		}
	}
	return labels
}

func (u *UIModel) writeNodeInfo(n *Node, w io.Writer, resources []v1.ResourceName) {
	extraLabels := u.extraLabels()
	allocatable := n.Allocatable()
	used := n.Used()
	firstLine := true
	resNameLen := 0
	for _, res := range resources {
		if len(res) > resNameLen {
			resNameLen = len(res)
		}
	}
	for _, res := range resources {
		usedRes := used[res]
		allocatableRes := allocatable[res]
		pct := usedRes.AsApproximateFloat64() / allocatableRes.AsApproximateFloat64()
		if allocatableRes.AsApproximateFloat64() == 0 {
			pct = 0
		}

		if firstLine {
			priceLabel := fmt.Sprintf("/$%0.4f", n.Price())
			if n.PriceIsEquivalent() {
				// not what the node costs, see R8
				priceLabel = fmt.Sprintf("/~$%0.4f", n.Price())
			}
			if !n.HasPrice() || u.DisablePricing {
				priceLabel = ""
			}
			maxPods, _ := allocatable.Pods().AsInt64()
			fmt.Fprintf(w, "%s\t%s\t%s\t(%d/%d pods)\t%s%s", n.Name(), res, u.progressBar(pct), n.NumPods(), maxPods, n.InstanceType(), priceLabel)

			// node compute type
			if n.IsOnDemand() {
				fmt.Fprintf(w, "\tOn-Demand")
			} else if n.IsSpot() {
				fmt.Fprintf(w, "\tSpot")
			} else if n.IsFargate() {
				fmt.Fprintf(w, "\tFargate")
			} else {
				fmt.Fprintf(w, "\t-")
			}

			if n.IsAuto() {
				fmt.Fprintf(w, "/Auto")
			}

			// node status
			if n.Cordoned() && n.Deleting() {
				fmt.Fprintf(w, "\tCordoned/Deleting")
			} else if n.Deleting() {
				fmt.Fprintf(w, "\tDeleting")
			} else if n.Cordoned() {
				fmt.Fprintf(w, "\tCordoned")
			} else {
				fmt.Fprintf(w, "\t-")
			}

			// node readiness or time we've been waiting for it to be ready
			if n.Ready() {
				fmt.Fprintf(w, "\tReady")
			} else {
				fmt.Fprintf(w, "\tNotReady/%s", duration.HumanDuration(time.Since(n.NotReadyTime())))
			}

			for _, label := range extraLabels {
				labelValue, ok := n.node.Labels[label]
				if !ok {
					// support computed label values
					labelValue = n.ComputeLabel(label)
				}
				fmt.Fprintf(w, "\t%s", labelValue)
			}

		} else {
			fmt.Fprintf(w, " \t%s\t%s\t\t\t\t\t", res, u.progressBar(pct))
			for range extraLabels {
				fmt.Fprintf(w, "\t")
			}
		}
		fmt.Fprintln(w)
		firstLine = false
	}
}

// writeGroupSummary writes a block per group: nodes, pods, usage and cost.
func (u *UIModel) writeGroupSummary(groups []GroupStats, w io.Writer) {
	u.progress.ShowPercentage = false

	for _, g := range groups {
		firstLine := true
		for _, res := range u.cluster.resources {
			allocatable := g.AllocatableResources[res]
			used := g.UsedResources[res]
			pctUsed := g.PercentUsed(res)
			pctUsedStr := u.colorizeUsage(pctUsed, fmt.Sprintf("%0.1f%%", pctUsed))

			if firstLine {
				groupPrice := ""
				if !u.DisablePricing && g.PricedNodes > 0 {
					groupPrice = u.printer.Sprintf("$%0.3f/hour | $%0.3f/month", g.TotalPrice, g.TotalPrice*avgHoursPerMonth)
					if !g.FullyPriced() {
						// an unknown price somewhere, so this is a lower bound
						groupPrice = ">= " + groupPrice
					}
				}
				if !u.DisablePricing && g.ReservedNodes > 0 {
					groupPrice += u.printer.Sprintf(" (+%d reserved, not priced)", g.ReservedNodes)
				}
				u.printer.Fprintf(w, "%s\t%d nodes\t%d pods\t(%s/%s)\t%s\t%s\t%s\t%s\n",
					g.Name, g.NumNodes, g.NumPods, formatQuantity(res, used), formatQuantity(res, allocatable),
					pctUsedStr, res, u.progressBar(pctUsed/100.0), groupPrice)
			} else {
				u.printer.Fprintf(w, " \t\t\t(%s/%s)\t%s\t%s\t%s\t\n",
					formatQuantity(res, used), formatQuantity(res, allocatable), pctUsedStr, res, u.progressBar(pctUsed/100.0))
			}
			firstLine = false
		}
	}
}

// colorizeUsage styles by how full a resource is, fuller being better.
func (u *UIModel) colorizeUsage(pctUsed float64, label string) string {
	if pctUsed > 90 {
		return u.style.green(label)
	} else if pctUsed > 60 {
		return u.style.yellow(label)
	}
	return u.style.red(label)
}

func (u *UIModel) writeClusterSummary(resources []v1.ResourceName, stats Stats, w io.Writer) {
	firstLine := true

	for _, res := range resources {
		allocatable := stats.AllocatableResources[res]
		used := stats.UsedResources[res]
		pctUsed := 0.0
		if allocatable.AsApproximateFloat64() != 0 {
			pctUsed = 100 * (used.AsApproximateFloat64() / allocatable.AsApproximateFloat64())
		}
		pctUsedStr := u.colorizeUsage(pctUsed, fmt.Sprintf("%0.1f%%", pctUsed))

		u.progress.ShowPercentage = false
		monthlyPrice := stats.TotalPrice * avgHoursPerMonth
		clusterPrice := u.printer.Sprintf("$%0.3f/hour | $%0.3f/month", stats.TotalPrice, monthlyPrice)
		if stats.UnpricedReserved > 0 {
			clusterPrice += u.printer.Sprintf(" (+%d reserved, not priced)", stats.UnpricedReserved)
		}
		if u.DisablePricing {
			clusterPrice = ""
		}
		if firstLine {
			u.printer.Fprintf(w, "%d nodes\t(%10s/%s)\t%s\t%s\t%s\t%s\n",
				stats.NumNodes, formatQuantity(res, used), formatQuantity(res, allocatable), pctUsedStr, res, u.progressBar(pctUsed/100.0), clusterPrice)
		} else {
			u.printer.Fprintf(w, " \t%s/%s\t%s\t%s\t%s\t\n",
				formatQuantity(res, used), formatQuantity(res, allocatable), pctUsedStr, res, u.progressBar(pctUsed/100.0))
		}
		firstLine = false
	}
}

// computeItemsPerPage dynamically calculates the number of nodes we can fit per page
// taking into account header and footer text
func (u *UIModel) computeItemsPerPage(nodes []*Node, b *strings.Builder) int {
	var buf bytes.Buffer
	u.writeNodeInfo(nodes[0], &buf, u.cluster.resources)
	return u.itemsPerPage(b, strings.Count(buf.String(), "\n"))
}

// itemsPerPage fits items of linesPerItem lines below the header written so far.
// Never returns zero: the paginator divides by it.
func (u *UIModel) itemsPerPage(header *strings.Builder, linesPerItem int) int {
	if linesPerItem < 1 {
		linesPerItem = 1
	}
	headerLines := strings.Count(header.String(), "\n") + 2
	perPage := ((u.height - headerLines) / linesPerItem) - 1
	if perPage < 1 {
		return 1
	}
	return perPage
}

type tickMsg time.Time

// tickInterval only keeps node ages and NotReady durations moving; cluster
// changes arrive as ChangedMsg.
const tickInterval = time.Second

func tickCmd() tea.Cmd {
	return tea.Tick(tickInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (u *UIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// anything reaching Update may change what is on screen
	u.dirty = true
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		u.height = msg.Height
		return u, nil
	case ChangedMsg:
		if msg.Gen != u.gen {
			return u, nil
		}
		return u, u.snapshotCmd()
	case snapshotMsg:
		if msg.gen == u.gen {
			u.stats = msg.stats
		}
		return u, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return u, tea.Quit
		case "g":
			u.toggleGroups()
			return u, nil
		case "n":
			u.toggleNodes()
			return u, nil
		}
	case tickMsg:
		return u, tickCmd()
	}
	var cmd tea.Cmd
	u.paginator, cmd = u.paginator.Update(msg)
	return u, cmd
}

func (u *UIModel) SetResources(resources []string) {
	u.cluster.resources = nil
	for _, r := range resources {
		u.cluster.resources = append(u.cluster.resources, v1.ResourceName(r))
	}
}
