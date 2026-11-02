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

	"github.com/charmbracelet/lipgloss"
)

// Screen heights below which the header shrinks, leaving rows to the nodes
// (R16): the full panel, then a two-line box, then one plain line.
const (
	headerPanelMinHeight   = 28
	headerCompactMinHeight = 14
)

var (
	headerBox     = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#626262")).Padding(0, 1)
	headerLabel   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#5f5f87", Dark: "#afafd7"}).Render
	headerValue   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#005f87", Dark: "#87d7ff"}).Render
	headerContext = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#005f87", Dark: "#87d7ff"}).Render
	// keys in bold yellow, what they do in plain grey, here and in popups
	headerKey    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#875f00", Dark: "#ffd75f"}).Render
	headerAction = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#4e4e4e", Dark: "#bcbcbc"}).Render
	// one colour per kind of header value
	countsValue = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#af5f00", Dark: "#ffaf5f"}).Render
	viewOption  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#8700af", Dark: "#d7afff"}).Render
	viewValue   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#005f00", Dark: "#87d787"}).Render
	viewSep     = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262")).Render
	// the cluster's cost, whatever the filters
	costValue = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#005f5f", Dark: "#87d7af"}).Render
)

// issueWindow is how long a logged error stays in the header.
const issueWindow = 2 * time.Minute

// recentIssue is the last logged error if it is recent, else empty.
func (u *UIModel) recentIssue() string {
	if u.issues == nil {
		return ""
	}
	msg, at := u.issues()
	if msg == "" || time.Since(at) > issueWindow {
		return ""
	}
	return msg
}

// clusterCost is the monthly cost of every watched node, whatever the
// filters, shown after the node and pod counts; empty with pricing disabled.
func (u *UIModel) clusterCost(stats Stats) string {
	if u.DisablePricing {
		return ""
	}
	if stats.NumNodes > 0 && stats.PricedNodes == 0 && stats.UnpricedReserved == 0 {
		return headerLabel("prices unknown")
	}
	cost := costValue(u.printer.Sprintf("$%0.3f/month", stats.TotalPrice*avgHoursPerMonth))
	if stats.PricedNodes < stats.NumNodes-stats.UnpricedReserved {
		// an unknown price somewhere, so this is a lower bound
		cost = costValue(">= ") + cost
	}
	if stats.UnpricedReserved > 0 {
		cost += costValue(u.printer.Sprintf(" (+%d reserved, not priced)", stats.UnpricedReserved))
	}
	return cost + headerLabel(" (global cluster)")
}

// countsWithCost is the node and pod counts, then the cluster's cost.
func (u *UIModel) countsWithCost(counts string, stats Stats) string {
	out := countsValue(counts)
	if cost := u.clusterCost(stats); cost != "" {
		out += countsValue(" · ") + cost
	}
	return out
}

// headerKeys are the main keys shown in the header panel, as key and action.
var headerKeys = [][2]string{
	{"f", "filters"}, {"g", "group by"},
	{"s", "sort"}, {"o", "columns"},
	{"/", "search"}, {"r", "resources"},
	{"c", "context"}, {"v", "views"},
	{"?", "help"}, {"^e", "compact"},
}

type headerSize int

const (
	headerLine headerSize = iota
	headerCompact
	headerFull
)

// headerSize is the header the screen shows: the user's pick from ctrl+e,
// else by height. A screen too short for a box always gets one line.
func (u *UIModel) headerSize() headerSize {
	if u.height > 0 && u.height < headerCompactMinHeight {
		return headerLine
	}
	switch u.view.Header {
	case HeaderFull:
		return headerFull
	case HeaderCompact:
		return headerCompact
	}
	if u.height > 0 && u.height < headerPanelMinHeight {
		return headerCompact
	}
	return headerFull
}

// toggleHeader switches between the full panel and the compact box (ctrl+e)
// and saves the pick with the view.
func (u *UIModel) toggleHeader() {
	if u.headerSize() == headerFull {
		u.view.Header = HeaderCompact
	} else {
		u.view.Header = HeaderFull
	}
	if u.onViewChange != nil {
		u.onViewChange(u.view)
	}
}

// header shows which cluster this is, how many nodes the view keeps and the
// view itself (R16): a panel when the screen is tall enough, else one line.
func (u *UIModel) header(stats Stats, nodes []*Node) string {
	pods := 0
	for _, n := range nodes {
		pods += n.NumPods()
	}
	counts := u.printer.Sprintf("%d of %d shown · %d pods", len(nodes), stats.NumNodes, pods)
	// pods not on a node yet belong to no filtered set; say so when there are any
	if pending := stats.Unscheduled; pending > 0 {
		counts += u.printer.Sprintf(" (%d pending)", pending)
	}
	context := u.info.Context
	if context == "" {
		context = "-"
	}

	issue := u.recentIssue()
	if u.headerSize() == headerLine {
		sep := viewSep(" │ ")
		line := headerContext(context) + sep + u.countsWithCost(counts, stats) + sep + u.viewSummary()
		if issue != "" {
			line += sep + u.style.red("⚠ "+issue)
		}
		return line + "\n"
	}

	region := u.info.Region
	if region == "" && len(stats.Nodes) > 0 {
		region = stats.Nodes[0].Region()
	}
	if u.headerSize() == headerCompact {
		where := []string{headerContext(context)}
		for _, s := range []string{u.info.Server, region} {
			if s != "" {
				where = append(where, headerValue(s))
			}
		}
		where = append(where, u.countsWithCost(counts, stats))
		body := strings.Join(where, headerLabel(" · ")) + "\n" + u.viewSummary()
		if issue != "" {
			body += "\n" + u.style.red("⚠ "+issue)
		}
		return u.boxed(body)
	}
	rows := [][2]string{
		{"Context:", headerValue(context)},
		{"Cluster:", headerValue(orDash(u.info.Server))},
		{"Region:", headerValue(orDash(region))},
		{"Nodes:", u.countsWithCost(counts, stats)},
		{"View:", u.viewSummary()},
	}
	if issue != "" {
		rows = append(rows, [2]string{"Issue:", u.style.red(issue) + headerLabel("  full log: "+u.issueLog)})
	}
	var left strings.Builder
	left.WriteString(titleStyle("kube-node-viewer") + headerLabel(" · ") + headerContext(u.screenTitle()))
	for _, r := range rows {
		left.WriteString("\n" + headerLabel(fmt.Sprintf("%-9s", r[0])) + " " + r[1])
	}

	var right strings.Builder
	for i := 0; i < len(headerKeys); i += 2 {
		right.WriteString("\n")
		for _, k := range headerKeys[i:min(i+2, len(headerKeys))] {
			right.WriteString(fmt.Sprintf("%s %s", headerKey(fmt.Sprintf("%3s", k[0])), headerAction(fmt.Sprintf("%-10s", k[1]))))
		}
	}

	return u.boxed(left.String(), right.String())
}

// boxed draws the header box around a block of lines and, when it fits, the
// keys on the right.
func (u *UIModel) boxed(leftBlock string, rightBlocks ...string) string {
	rightBlock := strings.Join(rightBlocks, "")
	box := headerBox
	if u.width > 0 {
		inner := u.width - 4 // border and padding
		gap := inner - lipgloss.Width(leftBlock) - lipgloss.Width(rightBlock)
		if gap < 2 {
			// too narrow for the keys; they are in the footer and in help too
			rightBlock, gap = "", 0
		}
		leftBlock = lipgloss.NewStyle().MaxWidth(inner).Render(leftBlock)
		gap = max(gap, 0)
		box = box.Width(u.width - 2)
		return box.Render(lipgloss.JoinHorizontal(lipgloss.Top, leftBlock, strings.Repeat(" ", gap), rightBlock)) + "\n"
	}
	return box.Render(lipgloss.JoinHorizontal(lipgloss.Top, leftBlock, "   ", rightBlock)) + "\n"
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// fieldName is how a field reads on screen: a label by its key.
func fieldName(f Field) string {
	if key := f.Label(); key != "" {
		return key
	}
	if res, ok := strings.CutPrefix(string(f), usageFieldPrefix); ok {
		return res + " usage"
	}
	return string(f)
}

// viewSummary is the view in one line: filters │ group │ sort │ resources.
func (u *UIModel) viewSummary() string {
	var filters []string
	for _, f := range u.view.Filters {
		values := f.Ticked()
		if !f.Enabled || len(values) == 0 {
			continue
		}
		eq, in := "=", "∈"
		if f.Excludes() {
			eq, in = "≠", "∉"
		}
		if len(values) == 1 {
			filters = append(filters, fieldName(f.Field)+eq+values[0])
		} else {
			filters = append(filters, fieldName(f.Field)+in+"{"+strings.Join(values, ",")+"}")
		}
	}
	filterText := "none"
	if len(filters) > 0 {
		filterText = strings.Join(filters, " · ")
	}
	if u.view.Search != "" {
		filterText += fmt.Sprintf(" · /%s", u.view.Search)
	}

	var group []string
	for _, f := range u.view.GroupBy {
		group = append(group, fieldName(f))
	}
	groupText := "none"
	if len(group) > 0 {
		groupText = strings.Join(group, pathSeparator)
	}

	sortField := u.view.Sort.Field
	if sortField == "" {
		sortField = FieldCreation
	}
	arrow := "↑"
	if u.view.Sort.Descending {
		arrow = "↓"
	}

	sep := viewSep(" │ ")
	return viewOption("Filters:") + " " + viewValue(filterText) + sep +
		viewOption("Group:") + " " + viewValue(groupText) + sep +
		viewOption("Sort:") + " " + viewValue(fieldName(sortField)+" "+arrow) + sep +
		viewValue(strings.Join(u.view.Resources, "+"))
}
