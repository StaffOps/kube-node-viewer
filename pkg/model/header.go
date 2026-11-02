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

	v1 "k8s.io/api/core/v1"

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
)

// headerKeys are the main keys shown in the header panel, as key and action.
var headerKeys = [][2]string{
	{"f", "filters"}, {"g", "group by"},
	{"s", "sort"}, {"o", "columns"},
	{"/", "search"}, {"r", "resources"},
	{"c", "context"}, {"?", "help"},
	{"^e", "compact"},
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
	if pending := stats.PodsByPhase[v1.PodPending]; pending > 0 {
		counts += u.printer.Sprintf(" (%d pending)", pending)
	}
	context := u.info.Context
	if context == "" {
		context = "-"
	}

	if u.headerSize() == headerLine {
		sep := viewSep(" │ ")
		return headerContext(context) + sep + countsValue(counts) + sep + u.viewSummary() + "\n"
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
		where = append(where, countsValue(counts))
		return u.boxed(strings.Join(where, headerLabel(" · ")) + "\n" + u.viewSummary())
	}
	rows := [][2]string{
		{"Context:", headerValue(context)},
		{"Cluster:", headerValue(orDash(u.info.Server))},
		{"Region:", headerValue(orDash(region))},
		{"Nodes:", countsValue(counts)},
		{"View:", u.viewSummary()},
	}
	var left strings.Builder
	left.WriteString(titleStyle("kube-node-viewer"))
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
		if !f.Enabled || len(f.Values) == 0 {
			continue
		}
		if len(f.Values) == 1 {
			filters = append(filters, fieldName(f.Field)+"="+f.Values[0])
		} else {
			filters = append(filters, fieldName(f.Field)+"∈{"+strings.Join(f.Values, ",")+"}")
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
