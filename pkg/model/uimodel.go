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
	"fmt"
	"io"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/paginator"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	v1 "k8s.io/api/core/v1"

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

// ClusterInfo names the cluster on screen, for the header (R16).
type ClusterInfo struct {
	Context string
	Server  string
	// Region is the pricing region; empty to take it from the nodes
	Region string
}

type UIModel struct {
	progress  progress.Model
	cluster   *Cluster
	paginator paginator.Model
	width     int
	height    int
	page      int
	style     *Style
	view      ViewState
	info      ClusterInfo
	// onViewChange receives the view state after the user changes it, to save
	// it for the cluster. It must not block.
	onViewChange func(ViewState)

	// panel is the open panel, if any; it gets the keys (R12)
	panel    panelKind
	filters  filtersPanel
	group    groupPanel
	sort     sortPanel
	columns  columnsPanel
	contexts contextsPanel
	switcher Switcher
	// searching is true while the search text takes the keys
	searching bool
	// customResources is a resource list other than cpu, memory or both,
	// from --resources or saved state: the fourth step of r (R9)
	customResources []string

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

// barCell fills the usage bars: a block 7/8 high leaves a thin gap above it,
// so bars stacked on consecutive lines (cpu, memory) read apart. The empty
// part is the same block in dark grey, keeping the gap along the whole bar.
const (
	barCell       = '▇'
	barEmptyColor = "#3a3a3a"
)

// newProgress builds the usage bar, red to green.
func newProgress(style *Style, opts ...progress.Option) progress.Model {
	p := progress.New(append([]progress.Option{style.gradient, progress.WithFillCharacters(barCell, barCell)}, opts...)...)
	p.EmptyColor = barEmptyColor
	return p
}

// NewUIModel starts with the default view state; SetView replaces it.
func NewUIModel(style *Style) *UIModel {
	pager := paginator.New()
	pager.Type = paginator.Dots
	pager.ActiveDot = activeDot
	pager.InactiveDot = inactiveDot
	cluster := NewCluster()
	return &UIModel{
		gen:       cluster.Generation(),
		progress:  newProgress(style),
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

// SetClusterInfo names the cluster in the header.
func (u *UIModel) SetClusterInfo(info ClusterInfo) {
	u.info = info
	u.dirty = true
}

// SetView replaces the view state, as loaded at start or for a new cluster.
func (u *UIModel) SetView(v ViewState) {
	u.view = v
	if len(u.view.Resources) == 0 {
		u.view.Resources = []string{string(v1.ResourceCPU)}
	}
	u.SetResources(u.view.Resources)
	u.customResources = nil
	if !slices.ContainsFunc(u.resourceCycle(), func(r []string) bool { return slices.Equal(r, u.view.Resources) }) {
		u.customResources = slices.Clone(u.view.Resources)
	}
	if len(u.view.GroupBy) == 0 {
		u.view.ShowNodes = true
	}
	u.page = 0
	u.dirty = true
}

// ViewState is the view as it is now.
func (u *UIModel) ViewState() ViewState {
	return u.view
}

// OnViewChange sets the function told about view changes made from the TUI.
func (u *UIModel) OnViewChange(f func(ViewState)) {
	u.onViewChange = f
}

// setView applies a change the user made and reports it for saving.
func (u *UIModel) setView(v ViewState) {
	u.view = v
	if len(u.view.GroupBy) == 0 {
		u.view.ShowNodes = true
	}
	u.page = 0
	if u.onViewChange != nil {
		u.onViewChange(u.view)
	}
}

// toggleNodes folds the list one level per press: first the nodes go, then
// the deepest group level, until only the top level is left; the next press
// shows everything again. Without a group-by there is nothing to fold to
// (R13).
func (u *UIModel) toggleNodes() {
	if len(u.view.GroupBy) == 0 {
		return
	}
	v := u.view
	switch depth := v.VisibleDepth(); {
	case depth == 0:
		v.ShowNodes, v.GroupDepth = false, len(v.GroupBy)
	case depth > 1:
		v.GroupDepth = depth - 1
	default:
		v.ShowNodes, v.GroupDepth = true, 0
	}
	u.setView(v)
}

// canShowDetails tells whether a level is hidden below the barred one, which
// d can list as one-line rows.
func (u *UIModel) canShowDetails() bool {
	depth := u.view.VisibleDepth()
	return depth > 0 && depth < len(u.view.GroupBy)
}

func (u *UIModel) toggleDetails() {
	if !u.canShowDetails() {
		return
	}
	v := u.view
	v.Details = !v.Details
	u.setView(v)
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

// colorizeUsage styles by how full a resource is, fuller being better for
// cost: above 80% green, 60-80% yellow, below 60% red.
func (u *UIModel) colorizeUsage(pctUsed float64, label string) string {
	if pctUsed > 80 {
		return u.style.green(label)
	} else if pctUsed >= 60 {
		return u.style.yellow(label)
	}
	return u.style.red(label)
}

// usageBar is a bar of the given total width followed by its percentage,
// coloured like colorizeUsage.
func (u *UIModel) usageBar(fill float64, width int) string {
	defer func(w int, pct bool) { u.progress.Width, u.progress.ShowPercentage = w, pct }(u.progress.Width, u.progress.ShowPercentage)
	u.progress.Width, u.progress.ShowPercentage = max(width-5, 1), false
	pct := fill * 100
	if math.IsNaN(pct) || pct < 0 {
		pct = 0
	}
	return u.progressBar(fill) + " " + u.colorizeUsage(pct, fmt.Sprintf("%3.0f%%", pct))
}

// clusterSummary is the cluster-wide usage and cost, one line per resource.
// It covers every watched node, whatever the filters; node and pod counts are
// in the header.
func (u *UIModel) clusterSummary(stats Stats) string {
	var b strings.Builder
	ctw := text.NewColorTabWriter(&b, 0, 8, 1)
	u.writeClusterSummary(u.cluster.resources, stats, ctw)
	ctw.Flush()
	return b.String()
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
			u.printer.Fprintf(w, "%s/%s\t%s\t%s\t%s\t%s\n",
				formatQuantity(res, used), formatQuantity(res, allocatable), pctUsedStr, res, u.progressBar(pctUsed/100.0), clusterPrice)
		} else {
			u.printer.Fprintf(w, "%s/%s\t%s\t%s\t%s\t\n",
				formatQuantity(res, used), formatQuantity(res, allocatable), pctUsedStr, res, u.progressBar(pctUsed/100.0))
		}
		firstLine = false
	}
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
		u.width, u.height = msg.Width, msg.Height
		// the terminal has already reflowed the old frame for the new size;
		// redraw from a clean screen or its leftovers stay below the new one
		return u, tea.ClearScreen
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
		return u, u.handleKey(msg)
	case contextsMsg:
		u.contextsLoaded(msg)
		return u, nil
	case switchedMsg:
		return u, u.switched(msg)
	case tickMsg:
		return u, tickCmd()
	}
	return u, nil
}

// handleKey routes a key to the open panel, or acts on it in the main view
// (R12). ctrl+c quits from anywhere.
func (u *UIModel) handleKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()
	switch key {
	case "ctrl+c":
		return tea.Quit
	case "ctrl+e":
		// not text in any panel, so it works everywhere like ctrl+c
		u.toggleHeader()
		return nil
	}
	if u.searching {
		u.searchKey(key)
		return nil
	}
	if u.panel != panelNone {
		return u.panelKey(key)
	}
	switch key {
	case "q":
		return tea.Quit
	case "?":
		u.panel = panelHelp
	case "f":
		u.openFilters()
	case "g":
		u.openGroupBy()
	case "s":
		u.openSort()
	case "o":
		u.openColumns()
	case "/":
		u.searching = true
	case "r":
		u.cycleResources()
	case "c":
		return u.openContexts()
	case "esc":
		// esc in the main view clears a kept search
		u.view.Search = ""
	case "n":
		u.toggleNodes()
	case "d":
		u.toggleDetails()
	case "right", "l", "pgdown":
		u.page++
	case "left", "h", "pgup":
		if u.page > 0 {
			u.page--
		}
	case "home":
		u.page = 0
	}
	return nil
}

func (u *UIModel) SetResources(resources []string) {
	u.cluster.resources = nil
	for _, r := range resources {
		u.cluster.resources = append(u.cluster.resources, v1.ResourceName(r))
	}
}
