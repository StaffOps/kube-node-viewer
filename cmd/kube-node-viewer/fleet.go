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

package main

import (
	"context"
	"slices"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/staffops/kube-node-viewer/pkg/client"
	"github.com/staffops/kube-node-viewer/pkg/model"
)

// maxStarting is how many members connect at once (clusters-screen D3).
const maxStarting = 4

// statusPoll is how often a starting member's status is read until its node
// watch synced or failed.
const statusPoll = 200 * time.Millisecond

// memberConn is what the fleet needs of a connection; *conn in production,
// a fake in tests.
type memberConn struct {
	store  *model.Cluster
	ctx    context.Context
	cancel context.CancelFunc
	status func() model.ConnStatus
	// startNodes and startPods run the watches; startPods once
	startNodes func()
	startPods  func()
}

// connector builds a member's connection with the visit's prices; notify
// hears its store's changes.
type connector func(name string, prices priceFunc, notify func(gen uint64)) (*memberConn, error)

// fleet is the Clusters screen's members: desired state under a mutex with
// an epoch, 4 starting at a time, a price set per visit (clusters-screen D3,
// D8, D11).
type fleet struct {
	connect connector
	send    func(tea.Msg)
	// guesses are the kubeconfig's guessed clouds, by context
	contexts func() ([]model.ContextInfo, error)
	// prices builds a visit's price set
	prices func(ctx context.Context) priceFunc

	mu       sync.Mutex
	epoch    int
	scope    string
	members  map[string]*member
	queue    []*member
	starting int
	// visit is the context of the current visit: the price set and the
	// change notifier live in it
	visit       context.Context
	stopVisit   context.CancelFunc
	visitPrices priceFunc
	changed     map[uint64]bool
	notifier    *client.Notifier
}

type member struct {
	name     string
	guess    string
	epoch    int
	c        *memberConn
	closed   bool
	wantPods bool
	pods     bool
	// released is set once the member gave its start place back
	released bool
	synced   bool
}

func newFleet(connect connector, send func(tea.Msg), contexts func() ([]model.ContextInfo, error), prices func(context.Context) priceFunc) *fleet {
	return &fleet{connect: connect, send: send, contexts: contexts, prices: prices, members: map[string]*member{}}
}

func (f *fleet) Contexts() ([]model.ContextInfo, error) {
	return f.contexts()
}

// SetWanted makes the members those contexts; nil closes them all and ends
// the visit. It never blocks on a connection.
func (f *fleet) SetWanted(contexts []string, scope string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if contexts == nil {
		f.closeAllLocked()
		return
	}
	if f.visit == nil {
		f.visit, f.stopVisit = context.WithCancel(context.Background())
		f.visitPrices = f.prices(f.visit)
		f.changed = map[uint64]bool{}
		f.notifier = client.NewNotifier(f.visit, changeInterval, f.fireChanged)
	}
	f.scope = scope
	for name, m := range f.members {
		if !slices.Contains(contexts, name) {
			f.closeLocked(m)
		}
	}
	guesses := map[string]string{}
	if infos, err := f.contexts(); err == nil {
		for _, i := range infos {
			guesses[i.Name] = i.Cloud
		}
	}
	for _, name := range contexts {
		if m, ok := f.members[name]; ok {
			// a synced member is judged again against the new scope
			if m.synced {
				f.decideLocked(m)
			}
			continue
		}
		m := &member{name: name, guess: guesses[name], epoch: f.epoch}
		f.members[name] = m
		f.queue = append(f.queue, m)
	}
	f.startLocked()
}

// WatchPods starts a member's full pod watch, now or once it is connected.
func (f *fleet) WatchPods(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, ok := f.members[name]
	if !ok {
		return
	}
	m.wantPods = true
	f.podsLocked(m)
}

func (f *fleet) podsLocked(m *member) {
	if m.wantPods && !m.pods && m.c != nil && !m.closed {
		m.pods = true
		go m.c.startPods()
	}
}

// Close ends the visit; quitting calls it.
func (f *fleet) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closeAllLocked()
}

func (f *fleet) closeAllLocked() {
	for _, m := range f.members {
		f.closeLocked(m)
	}
	f.queue = nil
	f.epoch++
	if f.stopVisit != nil {
		f.stopVisit()
	}
	f.visit, f.stopVisit, f.visitPrices, f.notifier, f.changed = nil, nil, nil, nil, nil
}

func (f *fleet) closeLocked(m *member) {
	m.closed = true
	if m.c != nil {
		m.c.cancel()
	}
	delete(f.members, m.name)
	f.queue = slices.DeleteFunc(f.queue, func(q *member) bool { return q == m })
}

// startLocked starts queued members while fewer than maxStarting are.
func (f *fleet) startLocked() {
	for f.starting < maxStarting && len(f.queue) > 0 {
		m := f.queue[0]
		f.queue = f.queue[1:]
		f.starting++
		go f.run(m, f.visitPrices)
	}
}

// release gives a member's start place back, once: on its node sync, its
// first recorded error, or when it is closed.
func (f *fleet) release(m *member) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if m.released {
		return
	}
	m.released = true
	f.starting--
	f.startLocked()
}

func (f *fleet) run(m *member, prices priceFunc) {
	defer f.release(m)
	c, err := f.connect(m.name, prices, f.memberChanged)
	f.mu.Lock()
	if m.closed || m.epoch != f.epoch {
		f.mu.Unlock()
		if c != nil {
			c.cancel() // a result of an older visit is closed at once
		}
		return
	}
	if err != nil {
		f.mu.Unlock()
		f.send(model.MemberReadyMsg{Context: m.name, Err: client.Sanitize(err)})
		return
	}
	m.c = c
	f.podsLocked(m)
	f.mu.Unlock()
	go c.startNodes()
	f.send(model.MemberReadyMsg{Context: m.name, Gen: c.store.Generation(), Cluster: c.store, Status: c.status})

	ticker := time.NewTicker(statusPoll)
	defer ticker.Stop()
	for {
		st := c.status()
		if st.Synced {
			f.mu.Lock()
			m.synced = true
			f.decideLocked(m)
			f.mu.Unlock()
			return
		}
		if st.Err != "" {
			return
		}
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// decideLocked closes a synced member whose cloud is not the scope's
// (clusters-screen R14, D11): the provider-ID scheme all its nodes share; a
// cluster with no node keeps a known guess.
func (f *fleet) decideLocked(m *member) {
	if f.scope == "" || m.closed || m.c == nil {
		return
	}
	cloud := memberCloud(m.c.store, m.guess)
	if cloud == f.scope {
		return
	}
	f.closeLocked(m)
	name := m.name
	go f.send(model.MemberLeftMsg{Context: name, Cloud: cloud})
}

// memberCloud is the cloud a synced cluster's nodes say, "" for an empty
// cluster of unknown guess.
func memberCloud(store *model.Cluster, guess string) string {
	nodes := store.Stats().Nodes
	if len(nodes) == 0 {
		return guess
	}
	cloud := ""
	for _, n := range nodes {
		c := n.Cloud()
		if cloud == "" {
			cloud = c
		} else if c != cloud {
			return "mixed"
		}
	}
	if slices.Contains(model.ScopeClouds, cloud) {
		return cloud
	}
	return "other"
}

// memberChanged folds the members' change notices into one message per
// interval (clusters-screen D3).
func (f *fleet) memberChanged(gen uint64) {
	f.mu.Lock()
	n := f.notifier
	if f.changed != nil {
		f.changed[gen] = true
	}
	f.mu.Unlock()
	n.Changed()
}

func (f *fleet) fireChanged() {
	f.mu.Lock()
	gens := make([]uint64, 0, len(f.changed))
	for g := range f.changed {
		gens = append(gens, g)
	}
	clear(f.changed)
	f.mu.Unlock()
	if len(gens) > 0 {
		f.send(model.MembersChangedMsg{Gens: gens})
	}
}

// fleetContexts lists the kubeconfig's contexts with their guessed cloud.
func (s *sessions) fleetContexts() ([]model.ContextInfo, error) {
	list, _, err := client.ContextServers(s.flags.Kubeconfig)
	if err != nil {
		return nil, err
	}
	out := make([]model.ContextInfo, 0, len(list))
	for _, c := range list {
		out = append(out, model.ContextInfo{Name: c.Name, Server: c.Server, Cloud: client.GuessCloud(c.Name, c.Server)})
	}
	return out, nil
}

// memberConnect is the production connector: a member has no Azure sizes
// and only its node watch until WatchPods (clusters-screen D10).
func (s *sessions) memberConnect(name string, prices priceFunc, notify func(gen uint64)) (*memberConn, error) {
	c, err := s.connect(name, prices, notify)
	if err != nil {
		return nil, err
	}
	c.controller.SetMember(!s.flags.DisablePricing)
	return &memberConn{
		store:      c.store,
		ctx:        c.ctx,
		cancel:     c.cancel,
		status:     connStatus(c.controller),
		startNodes: func() { c.controller.StartNodes(c.ctx) },
		startPods:  func() { c.controller.StartPods(c.ctx) },
	}, nil
}
