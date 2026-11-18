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
	"net/url"
	"slices"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// The Clusters screen (clusters-screen): several contexts at once, one
// section per cluster, a total per cloud.

// ScopeClouds are the clouds `:clusters <cloud>` accepts (clusters-screen R14).
var ScopeClouds = []string{"aws", "azure", "gce"}

// Cloud buckets with no total (clusters-screen R3, R4).
const (
	cloudMixed   = "mixed"
	cloudOther   = "other"
	cloudUnknown = "-"
)

// ContextInfo is a kubeconfig context, its API server and the cloud guessed
// from the kubeconfig alone ("" when unknown).
type ContextInfo struct {
	Name   string
	Server string
	Cloud  string
}

// Fleet is the Clusters screen's connections (clusters-screen D3). Every
// call is made on the UI goroutine and must not block.
type Fleet interface {
	// Contexts lists the kubeconfig's contexts; it reads the kubeconfig,
	// once per visit as the screen opens
	Contexts() ([]ContextInfo, error)
	// SetWanted is the desired set of members and the cloud scope ("" for
	// every cloud); nil closes them all
	SetWanted(contexts []string, scope string)
	// WatchPods starts the full pod watch of a member, now or once it is
	// connected (clusters-screen D10)
	WatchPods(context string)
	// Live tells whether serial is the member the fleet runs for the
	// context now, so a late message of a closed one is dropped
	Live(context string, serial uint64) bool
}

// FleetStore keeps the Clusters screen's choice (clusters-screen R8, R14).
type FleetStore interface {
	Fleet() (contexts []string, cloud string, ok bool)
	SaveFleet(contexts []string, cloud string)
}

// MemberReadyMsg tells that a member connected, or failed to (Err).
type MemberReadyMsg struct {
	Context string
	// Serial names the member among those the fleet ran for the context
	Serial  uint64
	Gen     uint64
	Cluster *Cluster
	Status  func() ConnStatus
	Err     string
}

// MembersChangedMsg tells that the stores of these generations changed; the
// fleet sends one per change interval for all its members.
type MembersChangedMsg struct {
	Gens []uint64
}

// MemberLeftMsg tells that the fleet closed a member whose cloud, learned
// from its nodes, is outside the scope ("" for a cluster with no node and no
// guess) (clusters-screen R14).
type MemberLeftMsg struct {
	Context string
	Cloud   string
}

type memberSnapshotMsg struct {
	context string
	gen     uint64
	st      ConnStatus
	stats   Stats
}

// membersSnapshotMsg carries the snapshots of every member that changed,
// so one frame shows them all (clusters-screen D3).
type membersSnapshotMsg []memberSnapshotMsg

// slot is one section's cluster: a member, or Overview's own store, borrowed.
type slot struct {
	name     string
	server   string
	guess    string
	borrowed bool
	ready    bool
	gen      uint64
	cluster  *Cluster
	status   func() ConnStatus
	st       ConnStatus
	stats    Stats
	err      string
}

// fleetState is the Clusters screen's UI state; nothing here is saved but
// the choice and the scope.
type fleetState struct {
	fleet Fleet
	store FleetStore
	infos []ContextInfo
	// chosen and scope are the saved choice; saved is false until the
	// picker's enter (clusters-screen R2, R8)
	chosen []string
	scope  string
	saved  bool
	// asked is set once the first-use picker opened in this visit
	asked bool
	slots map[string]*slot
	// left holds the learned cloud of members closed by the scope
	left      map[string]string
	unfolded  map[string]bool
	podsAsked map[string]bool
	// switching is the context `enter` is opening (clusters-screen R6)
	switching string
	// overview reads the connection of the context on screen; ovStatus is
	// what it said with Overview's last snapshot (clusters-screen D10)
	overview func() ConnStatus
	ovStatus ConnStatus
	picker   clusterPicker
	// frame is the layout of the frame being drawn, computed once
	frame *clustersOverview
	// paged is set by a page key: the cursor does not follow its row
	paged bool
	// issuesFrom and issuesTo bound the last visit: Overview's header does
	// not show log lines of that time, which the members wrote into the same
	// log (clusters-screen R7)
	issuesFrom, issuesTo time.Time
}

// memberLogGrace is how long after leaving the screen the closing members'
// last log lines are still left out of Overview's header.
const memberLogGrace = 5 * time.Second

// memberLine tells a log line written during the last visit of the Clusters
// screen, off it; Overview's own lines of that time are left out too, and
// come back as its watches log again.
func (fs *fleetState) memberLine(at time.Time) bool {
	return !fs.issuesTo.IsZero() && !at.Before(fs.issuesFrom) && at.Before(fs.issuesTo)
}

// SetContextListsOff marks the pickers' titles: contexts.allow/deny are not
// applied in this run (context-visibility R6).
func (u *UIModel) SetContextListsOff() {
	u.contextListsOff = true
}

// pickerTitle is a picker's title, marked when the context lists are off.
func (u *UIModel) pickerTitle(title string) string {
	if u.contextListsOff {
		return title + " (lists off)"
	}
	return title
}

// SetFleet enables the Clusters screen; call it before the program runs.
func (u *UIModel) SetFleet(f Fleet, store FleetStore) {
	u.fl.fleet, u.fl.store = f, store
	if store != nil {
		u.fl.chosen, u.fl.scope, u.fl.saved = store.Fleet()
	}
}

func (u *UIModel) onClusters() bool {
	return u.view.Screen == ScreenClusters
}

// fleetSync tells the fleet which members are wanted now: the chosen
// contexts in the scope, less those sharing an API server with an earlier one
// (clusters-screen D3, D11, D12). Off the screen it closes them all.
func (u *UIModel) fleetSync() {
	f := u.fl.fleet
	if f == nil {
		return
	}
	if !u.onClusters() {
		if u.fl.slots != nil {
			u.fl.issuesTo = time.Now().Add(memberLogGrace)
		}
		u.fl.infos, u.fl.slots, u.fl.left, u.fl.unfolded, u.fl.podsAsked = nil, nil, nil, nil, nil
		u.fl.asked = false
		f.SetWanted(nil, "")
		return
	}
	if u.fl.infos == nil {
		infos, err := f.Contexts()
		if err != nil {
			u.notify("kubeconfig: " + err.Error())
		}
		u.fl.infos = infos
		if u.fl.infos == nil {
			u.fl.infos = []ContextInfo{}
		}
	}
	if u.fl.slots == nil {
		// a line of an earlier visit may still be recent: keep its start
		if u.fl.issuesTo.IsZero() || time.Since(u.fl.issuesTo) > issueWindow {
			u.fl.issuesFrom = time.Now()
		}
		u.fl.slots, u.fl.left, u.fl.unfolded, u.fl.podsAsked = map[string]*slot{}, map[string]string{}, map[string]bool{}, map[string]bool{}
	}
	if !u.fl.saved && !u.fl.asked {
		u.fl.asked = true
		u.openClusterPicker()
	}

	wanted := u.wantedMembers()
	keep := map[string]bool{}
	for _, name := range wanted {
		keep[name] = true
		if _, ok := u.fl.slots[name]; !ok {
			info := u.contextInfo(name)
			u.fl.slots[name] = &slot{name: name, server: info.Server, guess: info.Cloud}
		}
	}
	cur := u.info.Context
	for name, s := range u.fl.slots {
		if !keep[name] && !s.borrowed {
			delete(u.fl.slots, name)
			delete(u.fl.unfolded, name)
			delete(u.fl.podsAsked, name)
		}
	}
	// the context on screen is always a section, unless the scope leaves
	// its cloud out (clusters-screen R2)
	if old, ok := u.fl.slots[cur]; ok && !old.borrowed {
		delete(u.fl.slots, cur)
	}
	for name, s := range u.fl.slots {
		if s.borrowed && name != cur {
			delete(u.fl.slots, name)
		}
	}
	if cur != "" {
		s, ok := u.fl.slots[cur]
		if !ok {
			info := u.contextInfo(cur)
			s = &slot{name: cur, server: u.info.Server, guess: info.Cloud, borrowed: true, ready: true}
			u.fl.slots[cur] = s
		}
		u.refreshBorrowed()
		if u.fl.scope != "" {
			if c := slotCloud(s); c != cloudUnknown && c != u.fl.scope {
				delete(u.fl.slots, cur)
			}
		}
	}
	f.SetWanted(wanted, u.fl.scope)
}

// wantedMembers is the chosen contexts to connect: in the kubeconfig, not the
// context on screen, one per API server, in the scope by their guessed or
// learned cloud.
func (u *UIModel) wantedMembers() []string {
	cur := u.info.Context
	seen := map[string]bool{}
	if s := normalizeServer(u.info.Server); s != "" {
		seen[s] = true
	}
	chosen := slices.Clone(u.fl.chosen)
	sort.Strings(chosen)
	// never nil on the screen: nil is "off the screen" to the fleet
	out := []string{}
	for _, name := range chosen {
		info, ok := u.knownContext(name)
		if !ok || name == cur {
			continue
		}
		if s := normalizeServer(info.Server); s != "" {
			if seen[s] {
				continue
			}
			seen[s] = true
		}
		if learned, ok := u.fl.left[name]; ok {
			if u.fl.scope != "" && learned != u.fl.scope {
				continue
			}
			delete(u.fl.left, name)
		} else if u.fl.scope != "" && info.Cloud != "" && info.Cloud != u.fl.scope {
			continue
		}
		out = append(out, name)
	}
	return out
}

func (u *UIModel) knownContext(name string) (ContextInfo, bool) {
	for _, i := range u.fl.infos {
		if i.Name == name {
			return i, true
		}
	}
	return ContextInfo{}, false
}

func (u *UIModel) contextInfo(name string) ContextInfo {
	info, _ := u.knownContext(name)
	return info
}

// normalizeServer compares API servers: host in lower case, no trailing /,
// no default :443 (clusters-screen R17).
func normalizeServer(server string) string {
	if server == "" {
		return ""
	}
	p, err := url.Parse(server)
	if err != nil || p.Host == "" {
		return strings.TrimRight(strings.ToLower(server), "/")
	}
	host := strings.ToLower(p.Hostname())
	if port := p.Port(); port != "" && (port != "443" || p.Scheme != "https") {
		host += ":" + port
	}
	return p.Scheme + "://" + host + strings.TrimRight(p.Path, "/")
}

// refreshBorrowed copies Overview's snapshot into its slot.
func (u *UIModel) refreshBorrowed() {
	s, ok := u.fl.slots[u.info.Context]
	if !ok || !s.borrowed {
		return
	}
	s.cluster, s.gen, s.stats = u.cluster, u.gen, u.stats
	s.st = ConnStatus{Synced: true, PodsSynced: true}
	if u.fl.overview != nil {
		s.st = u.fl.ovStatus
	}
}

// slotCloud is the section's cloud: the provider-ID scheme all its nodes
// share once the node watch synced, else the guess (clusters-screen R3).
func slotCloud(s *slot) string {
	guess := s.guess
	if guess == "" {
		guess = cloudUnknown
	}
	if !s.st.Synced || len(s.stats.Nodes) == 0 {
		return guess
	}
	cloud := ""
	for _, n := range s.stats.Nodes {
		c := n.Cloud()
		if cloud == "" {
			cloud = c
		} else if c != cloud {
			return cloudMixed
		}
	}
	if slices.Contains(ScopeClouds, cloud) {
		return cloud
	}
	return cloudOther
}

// podsReady tells whether figures made from pods can be shown for the slot
// (clusters-screen R15).
func (s *slot) podsReady() bool {
	return s.ready && s.st.PodsSynced
}

// memberReady fills a member's slot (clusters-screen R2, R7).
func (u *UIModel) memberReady(msg MemberReadyMsg) tea.Cmd {
	s, ok := u.fl.slots[msg.Context]
	if !ok || s.borrowed || !u.fl.fleet.Live(msg.Context, msg.Serial) {
		return nil
	}
	s.ready, s.err = true, msg.Err
	if msg.Err != "" {
		return nil
	}
	s.gen, s.cluster, s.status = msg.Gen, msg.Cluster, msg.Status
	return u.memberSnapshot(s)
}

// memberSnapshot reads the status first, then the stats, so a frame never
// pairs "pods synced" with stats from before them (clusters-screen D10).
func (u *UIModel) memberSnapshot(s *slot) tea.Cmd {
	name, gen, cluster, status := s.name, s.gen, s.cluster, s.status
	if cluster == nil {
		return nil
	}
	return func() tea.Msg {
		return takeSnapshot(name, gen, cluster, status)
	}
}

func takeSnapshot(name string, gen uint64, cluster *Cluster, status func() ConnStatus) memberSnapshotMsg {
	st := ConnStatus{Synced: true, PodsSynced: true}
	if status != nil {
		st = status()
	}
	return memberSnapshotMsg{context: name, gen: gen, st: st, stats: cluster.Stats()}
}

// membersChanged takes the changed members' snapshots in one command.
func (u *UIModel) membersChanged(msg MembersChangedMsg) tea.Cmd {
	type src struct {
		name    string
		gen     uint64
		cluster *Cluster
		status  func() ConnStatus
	}
	var todo []src
	for _, s := range u.fl.slots {
		if s.borrowed || !s.ready || s.cluster == nil {
			continue
		}
		if slices.Contains(msg.Gens, s.gen) {
			todo = append(todo, src{s.name, s.gen, s.cluster, s.status})
		}
	}
	if len(todo) == 0 {
		return nil
	}
	return func() tea.Msg {
		out := make(membersSnapshotMsg, 0, len(todo))
		for _, s := range todo {
			out = append(out, takeSnapshot(s.name, s.gen, s.cluster, s.status))
		}
		return out
	}
}

func (u *UIModel) memberSnapshotDone(msg memberSnapshotMsg) {
	s, ok := u.fl.slots[msg.context]
	if !ok || s.borrowed || s.gen != msg.gen {
		return
	}
	s.st, s.stats = msg.st, msg.stats
}

func (u *UIModel) memberLeft(msg MemberLeftMsg) {
	if u.fl.slots == nil {
		return
	}
	if s, ok := u.fl.slots[msg.Context]; ok && !s.borrowed {
		delete(u.fl.slots, msg.Context)
		delete(u.fl.unfolded, msg.Context)
		delete(u.fl.podsAsked, msg.Context)
		u.fl.left[msg.Context] = msg.Cloud
	}
}

// fleetTick refreshes the members' errors and node sync, as errors fire no
// change (clusters-screen R7). The pods' sync is taken only with a snapshot,
// so it never pairs with stats from before the pods (D10).
func (u *UIModel) fleetTick() tea.Cmd {
	var cmds []tea.Cmd
	for _, s := range u.fl.slots {
		if s.borrowed || !s.ready || s.status == nil {
			continue
		}
		live := s.status()
		s.st.Err, s.st.Synced = live.Err, live.Synced
		if live.PodsSynced && !s.st.PodsSynced {
			cmds = append(cmds, u.memberSnapshot(s))
		}
	}
	if u.fl.overview != nil {
		live := u.fl.overview()
		u.fl.ovStatus.Err, u.fl.ovStatus.Synced = live.Err, live.Synced
		if live.PodsSynced && !u.fl.ovStatus.PodsSynced {
			cmds = append(cmds, u.snapshotCmd())
		}
	}
	return tea.Batch(cmds...)
}

// toggleSection unfolds or folds a section; the first unfold starts the
// cluster's pod watch (clusters-screen R11, R15).
func (u *UIModel) toggleSection(name string) {
	s, ok := u.fl.slots[name]
	if !ok {
		return
	}
	u.fl.unfolded[name] = !u.fl.unfolded[name]
	if u.fl.unfolded[name] && !s.borrowed && !u.fl.podsAsked[name] {
		u.fl.podsAsked[name] = true
		u.fl.fleet.WatchPods(name)
	}
}

// setScope sets and saves the cloud scope (clusters-screen R14).
func (u *UIModel) setScope(cloud string) {
	u.fl.scope = cloud
	if u.fl.saved && u.fl.store != nil {
		u.fl.store.SaveFleet(u.fl.chosen, cloud)
	}
}

// keySep parts a row key's context from the rest: a context name may hold
// "/" and ":" (EKS ARNs), never a control character.
const keySep = "\x1f"

// sectionRowKey names a node ("n") or group ("g") row of a section.
func sectionRowKey(kind, context, rest string) string {
	return kind + ":" + context + keySep + rest
}

// clusterCursorContext is the context of the cursor's row on the screen.
func clusterCursorContext(key string) (string, bool) {
	switch {
	case strings.HasPrefix(key, "c:"):
		return key[2:], true
	case strings.HasPrefix(key, "n:"), strings.HasPrefix(key, "g:"):
		context, _, ok := strings.Cut(key[2:], keySep)
		return context, ok
	}
	return "", false
}

// clustersKey handles the keys of the Clusters screen that differ from
// Overview's (clusters-screen R13); handled reports whether it took the key.
func (u *UIModel) clustersKey(key string) (cmd tea.Cmd, handled bool) {
	switch key {
	case "up", "k":
		u.cursorDelta--
	case "down", "j":
		u.cursorDelta++
	case "right", "l", "pgdown":
		u.page++
		u.fl.paged = true
	case "left", "h", "pgup":
		if u.page > 0 {
			u.page--
		}
		u.fl.paged = true
	case "home":
		u.page = 0
		u.fl.paged = true
	case "tab":
		u.setScreen(ScreenOverview)
	case "c":
		u.openClusterPicker()
	case "n", "d", "e":
		// fold levels, details and export are Overview's
	case "esc":
		u.view.Search = ""
	case " ":
		u.layoutPending()
		if name, ok := strings.CutPrefix(u.cursorKey, "c:"); ok {
			u.toggleSection(name)
		}
	case "enter":
		u.layoutPending()
		if name, ok := clusterCursorContext(u.cursorKey); ok {
			return u.openCluster(name), true
		}
	default:
		return nil, false
	}
	return nil, true
}

// layoutPending lays out a pending cursor move before acting on the row.
func (u *UIModel) layoutPending() {
	if u.cursorDelta != 0 || u.cursorKey == "" {
		u.dirty = true
		u.View()
	}
}

// openCluster opens a context in Overview, as c would (clusters-screen R6).
func (u *UIModel) openCluster(name string) tea.Cmd {
	if u.fl.switching != "" {
		return nil
	}
	if name == u.info.Context {
		u.setScreen(ScreenOverview)
		return nil
	}
	if u.switcher == nil {
		return nil
	}
	u.fl.switching = name
	switcher := u.switcher
	return func() tea.Msg {
		s, err := switcher.Switch(name)
		return switchedMsg{session: s, err: err, context: name, fromClusters: true}
	}
}

// panelNodes are the nodes the Filters and Group by panels offer values of:
// every section's on the Clusters screen (clusters-screen R5).
func (u *UIModel) panelNodes() []*Node {
	if !u.onClusters() || len(u.fl.slots) == 0 {
		return u.stats.Nodes
	}
	u.refreshBorrowed()
	names := make([]string, 0, len(u.fl.slots))
	for name := range u.fl.slots {
		names = append(names, name)
	}
	sort.Strings(names)
	var out []*Node
	for _, name := range names {
		out = append(out, u.fl.slots[name].stats.Nodes...)
	}
	return out
}
