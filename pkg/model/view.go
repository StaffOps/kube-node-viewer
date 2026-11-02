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
	"math"
	"sort"
	"strings"

	v1 "k8s.io/api/core/v1"
)

// Field names a node fact the view can filter, group, sort or show on. It is a
// plain string so view state saves as readable JSON: one of the Field
// constants, or LabelField(key) for any node label (computed labels included).
type Field string

const (
	FieldCapacity     Field = "capacity"
	FieldNodeGroup    Field = "nodegroup"
	FieldZone         Field = "zone"
	FieldRegion       Field = "region"
	FieldInstanceType Field = "instance-type"
	FieldStatus       Field = "status"

	// sort and column only: they have a value per node rather than a few
	// shared values worth filtering or grouping on
	FieldCreation Field = "creation"
	FieldName     Field = "name"
	FieldIP       Field = "ip"
	FieldAge      Field = "age"
	FieldPrice    Field = "price"
	FieldPods     Field = "pods"

	labelFieldPrefix = "label:"
	usageFieldPrefix = "usage:"
)

// LabelField is the field for a node label.
func LabelField(key string) Field { return Field(labelFieldPrefix + key) }

// UsageField sorts on a resource's requests as a share of allocatable.
func UsageField(resource v1.ResourceName) Field { return Field(usageFieldPrefix + string(resource)) }

// Label is the label key of a label field, empty for any other field.
func (f Field) Label() string {
	key, ok := strings.CutPrefix(string(f), labelFieldPrefix)
	if !ok {
		return ""
	}
	return key
}

// FilterFields are the fields offered by the Filters and Group by panels
// besides labels (R3, R4).
var FilterFields = []Field{FieldCapacity, FieldNodeGroup, FieldZone, FieldRegion, FieldInstanceType, FieldStatus}

// Values is what a node has for the field. Status may give several flags; any
// other field gives one value, UngroupedValue when the node has none.
func (f Field) Values(n *Node) []string {
	if f == FieldStatus {
		return n.StatusFlags()
	}
	return []string{f.Value(n)}
}

// Value is the node's single value for the field, used for grouping, sorting
// and columns. Status flags are joined with "/".
func (f Field) Value(n *Node) string {
	var v string
	switch f {
	case FieldCapacity:
		v = string(n.CapacityType())
	case FieldNodeGroup:
		v = n.NodeGroup()
	case FieldZone:
		v = n.Zone()
	case FieldRegion:
		v = n.Region()
	case FieldInstanceType:
		v = string(n.InstanceType())
	case FieldStatus:
		v = strings.Join(n.StatusFlags(), "/")
	case FieldName:
		v = n.Name()
	case FieldIP:
		v = n.InternalIP()
	default:
		if key := f.Label(); key != "" {
			return n.LabelValue(key)
		}
	}
	if v == "" || v == "-" {
		return UngroupedValue
	}
	return v
}

// number is the node's value for fields that sort numerically; ok is false for
// fields that sort as text.
func (f Field) number(n *Node) (value float64, ok bool) {
	switch f {
	case FieldCreation, FieldAge:
		return float64(n.Created().UnixNano()), true
	case FieldPrice:
		return n.Price(), true
	case FieldPods:
		return float64(n.NumPods()), true
	}
	if res, isUsage := strings.CutPrefix(string(f), usageFieldPrefix); isUsage {
		allocatable := n.Allocatable()[v1.ResourceName(res)]
		if allocatable.AsApproximateFloat64() == 0 {
			return math.NaN(), true
		}
		used := n.Used()[v1.ResourceName(res)]
		return used.AsApproximateFloat64() / allocatable.AsApproximateFloat64(), true
	}
	return 0, false
}

// Filter keeps nodes whose field has any of Values (R3). A disabled filter is
// kept in the view state but filters nothing.
type Filter struct {
	Field   Field    `json:"field"`
	Values  []string `json:"values"`
	Enabled bool     `json:"enabled"`
}

// Matches tells whether the node passes the filter.
func (f Filter) Matches(n *Node) bool {
	if !f.Enabled || len(f.Values) == 0 {
		return true
	}
	for _, have := range f.Field.Values(n) {
		for _, want := range f.Values {
			if have == want {
				return true
			}
		}
	}
	return false
}

// SortSpec orders the node list.
type SortSpec struct {
	Field      Field `json:"field"`
	Descending bool  `json:"descending,omitempty"`
}

// MaxGroupLevels is how many group-by keys the view nests (R4).
const MaxGroupLevels = 3

// ViewState is everything the user chooses about what is on screen. It holds
// no cluster data, so it is cheap to copy, compare and save per cluster.
type ViewState struct {
	Filters   []Filter `json:"filters,omitempty"`
	GroupBy   []Field  `json:"groupBy,omitempty"`
	Sort      SortSpec `json:"sort"`
	Search    string   `json:"-"`
	Resources []string `json:"resources,omitempty"`
	Columns   []Field  `json:"columns,omitempty"`
	ShowNodes bool     `json:"showNodes"`
}

// DefaultViewState is the view with nothing saved and no flags: newest nodes
// first, the default columns, the node list shown.
func DefaultViewState() ViewState {
	return ViewState{
		Sort:      SortSpec{Field: FieldCreation, Descending: true},
		Resources: []string{string(v1.ResourceCPU)},
		Columns:   append([]Field(nil), DefaultColumns...),
		ShowNodes: true,
	}
}

// SetGroupBy replaces the group-by keys, keeping at most MaxGroupLevels and
// dropping repeats.
func (s *ViewState) SetGroupBy(fields ...Field) {
	s.GroupBy = nil
	seen := map[Field]bool{}
	for _, f := range fields {
		if f == "" || seen[f] || len(s.GroupBy) == MaxGroupLevels {
			continue
		}
		seen[f] = true
		s.GroupBy = append(s.GroupBy, f)
	}
}

// Select returns the nodes passing every enabled filter and the search text,
// sorted. The input slice is left untouched.
func (s ViewState) Select(nodes []*Node) []*Node {
	search := strings.ToLower(strings.TrimSpace(s.Search))
	out := make([]*Node, 0, len(nodes))
	for _, n := range nodes {
		if s.matches(n) && (search == "" || n.contains(search)) {
			out = append(out, n)
		}
	}
	s.Sort.apply(out)
	return out
}

func (s ViewState) matches(n *Node) bool {
	for _, f := range s.Filters {
		if !f.Matches(n) {
			return false
		}
	}
	return true
}

// contains tells whether the node's name, internal IP, instance type or any
// label value contains the lowercase text (R6).
func (n *Node) contains(text string) bool {
	if strings.Contains(strings.ToLower(n.Name()), text) ||
		strings.Contains(strings.ToLower(n.InternalIP()), text) ||
		strings.Contains(strings.ToLower(string(n.InstanceType())), text) {
		return true
	}
	n.mu.RLock()
	defer n.mu.RUnlock()
	for _, v := range n.node.Labels {
		if strings.Contains(strings.ToLower(v), text) {
			return true
		}
	}
	return false
}

type sortKey struct {
	node   *Node
	num    float64
	text   string
	isNum  bool
	tieKey string
}

// apply sorts nodes in place. Keys are built once per node: a key can lock the
// node or walk its resources, and there are O(n log n) comparisons. Nodes
// without a value (NaN, UngroupedValue) go last in either direction; ties
// break on name so the order is stable between redraws.
func (s SortSpec) apply(nodes []*Node) {
	field := s.Field
	if field == "" {
		field = FieldCreation
	}
	keys := make([]sortKey, len(nodes))
	for i, n := range nodes {
		k := sortKey{node: n, tieKey: n.Name()}
		if num, ok := field.number(n); ok {
			k.num, k.isNum = num, true
		} else {
			k.text = field.Value(n)
		}
		keys[i] = k
	}
	missing := func(k *sortKey) bool {
		if k.isNum {
			return math.IsNaN(k.num)
		}
		return k.text == UngroupedValue
	}
	sort.SliceStable(keys, func(a, b int) bool {
		lhs, rhs := &keys[a], &keys[b]
		if ml, mr := missing(lhs), missing(rhs); ml || mr {
			if ml && mr {
				return naturalLess(lhs.tieKey, rhs.tieKey)
			}
			return mr
		}
		less, equal := false, false
		if lhs.isNum {
			less, equal = lhs.num < rhs.num, lhs.num == rhs.num
		} else {
			less, equal = naturalLess(lhs.text, rhs.text), lhs.text == rhs.text
		}
		if equal {
			return naturalLess(lhs.tieKey, rhs.tieKey)
		}
		if s.Descending {
			return !less
		}
		return less
	})
	for i := range keys {
		nodes[i] = keys[i].node
	}
}

// Group is one level of the nested group-by: its path from the top level, its
// totals, and either sub-groups or, at the last level, its nodes in sort
// order.
type Group struct {
	Path   []string
	Stats  GroupStats
	Groups []*Group
	Nodes  []*Node
}

// Name is the group's own value, the last element of its path.
func (g *Group) Name() string { return g.Path[len(g.Path)-1] }

// Groups nests already selected and sorted nodes under the view's group-by
// keys (R4). Groups sort by value with UngroupedValue last; nodes keep their
// order. With no group-by it returns nil.
func (s ViewState) Groups(nodes []*Node) []*Group {
	if len(s.GroupBy) == 0 {
		return nil
	}
	return groupLevel(nodes, s.GroupBy, nil)
}

func groupLevel(nodes []*Node, fields []Field, parent []string) []*Group {
	field := fields[0]
	byValue := map[string]*Group{}
	var order []*Group
	for _, n := range nodes {
		value := field.Value(n)
		g, ok := byValue[value]
		if !ok {
			path := append(append([]string(nil), parent...), value)
			g = &Group{Path: path, Stats: newGroupStats(value)}
			byValue[value] = g
			order = append(order, g)
		}
		g.Stats.add(n)
		g.Nodes = append(g.Nodes, n)
	}
	sortGroups(order, func(g *Group) string { return g.Name() })
	if len(fields) > 1 {
		for _, g := range order {
			g.Groups = groupLevel(g.Nodes, fields[1:], g.Path)
			g.Nodes = nil
		}
	}
	return order
}

func sortGroups[T any](groups []T, name func(T) string) {
	sort.SliceStable(groups, func(a, b int) bool {
		na, nb := name(groups[a]), name(groups[b])
		if (na == UngroupedValue) != (nb == UngroupedValue) {
			return nb == UngroupedValue
		}
		return naturalLess(na, nb)
	})
}

// ValueCount is a field value and how many nodes have it.
type ValueCount struct {
	Value string
	Count int
}

// ValueCounts lists the values the nodes have for the field with the number of
// nodes for each, for the filter value picker (R3). Capacity and status list
// every possible value, with zero counts, so the choices don't move around.
func ValueCounts(nodes []*Node, field Field) []ValueCount {
	counts := map[string]int{}
	switch field {
	case FieldCapacity:
		for _, c := range []CapacityType{CapacityOnDemand, CapacitySpot, CapacityReserved, CapacityFargate, CapacityUnknown} {
			counts[string(c)] = 0
		}
	case FieldStatus:
		for _, s := range []string{StatusReady, StatusNotReady, StatusCordoned, StatusDeleting, StatusLaunching} {
			counts[s] = 0
		}
	}
	for _, n := range nodes {
		for _, v := range field.Values(n) {
			counts[v]++
		}
	}
	out := make([]ValueCount, 0, len(counts))
	for v, c := range counts {
		out = append(out, ValueCount{Value: v, Count: c})
	}
	if field == FieldCapacity || field == FieldStatus {
		fixed := map[string]int{}
		for i, v := range []string{
			string(CapacityOnDemand), string(CapacitySpot), string(CapacityReserved), string(CapacityFargate), string(CapacityUnknown),
			StatusReady, StatusNotReady, StatusCordoned, StatusDeleting, StatusLaunching,
		} {
			fixed[v] = i
		}
		sort.Slice(out, func(a, b int) bool { return fixed[out[a].Value] < fixed[out[b].Value] })
		return out
	}
	sortGroups(out, func(v ValueCount) string { return v.Value })
	return out
}

// DefaultColumns are shown when nothing is saved, in screen order.
var DefaultColumns = []Field{FieldInstanceType, FieldCapacity, FieldNodeGroup, FieldZone, FieldIP, FieldPrice, FieldStatus}

// columnPriority ranks columns by what to keep when the screen is narrow, most
// important first (R14). Labels and anything unlisted go after these.
var columnPriority = []Field{FieldStatus, FieldPrice, FieldCapacity, FieldInstanceType, FieldNodeGroup, FieldZone, FieldIP, FieldAge, FieldRegion}

// FitColumns keeps the columns, in their given order, that fit in width,
// dropping the lowest-priority ones first, and says how many were dropped.
// widthOf is the width a column takes on screen, separator included.
func FitColumns(columns []Field, width int, widthOf func(Field) int) (shown []Field, hidden int) {
	rank := func(f Field) int {
		for i, p := range columnPriority {
			if p == f {
				return i
			}
		}
		return len(columnPriority)
	}
	byPriority := append([]Field(nil), columns...)
	sort.SliceStable(byPriority, func(a, b int) bool { return rank(byPriority[a]) < rank(byPriority[b]) })

	keep := map[Field]bool{}
	used := 0
	for _, f := range byPriority {
		w := widthOf(f)
		if used+w > width {
			// a lower-priority column never takes the place of a dropped one
			break
		}
		used += w
		keep[f] = true
	}
	for _, f := range columns {
		if keep[f] {
			shown = append(shown, f)
		}
	}
	return shown, len(columns) - len(shown)
}
