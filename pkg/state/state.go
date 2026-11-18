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

// Package state keeps the view state per cluster in a file of its own,
// separate from the user's config file (design D6).
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/staffops/kube-node-viewer/pkg/model"
)

// fileVersion lets a later release read or migrate older files.
const fileVersion = 1

type file struct {
	Version  int                        `json:"version"`
	Clusters map[string]model.ViewState `json:"clusters"`
	// Views are the named views, shared by every cluster
	Views map[string]model.ViewState `json:"views,omitempty"`
	// Fleet is the Clusters screen's chosen contexts and cloud scope
	// (clusters-screen R8, R14); missing in older files
	Fleet *fleet `json:"fleet,omitempty"`
}

type fleet struct {
	Contexts []string `json:"contexts"`
	Cloud    string   `json:"cloud,omitempty"`
}

// DefaultPath is $XDG_STATE_HOME/kube-node-viewer/state.json, falling back to
// ~/.local/state when XDG_STATE_HOME is unset.
func DefaultPath(home string) string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		dir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(dir, "kube-node-viewer", "state.json")
}

// Store holds the saved view states and writes them back to the file.
type Store struct {
	path string

	mu       sync.Mutex
	clusters map[string]model.ViewState
	views    map[string]model.ViewState
	fleet    *fleet
}

// Load reads the state file. A missing file is an empty store; a corrupt or
// unreadable one is ignored with one warning on w, and the store starts empty
// (R2). Saving then replaces the bad file.
func Load(path string, w io.Writer) *Store {
	s := &Store{path: path, clusters: map[string]model.ViewState{}, views: map[string]model.ViewState{}}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s
	}
	if err == nil {
		var f file
		if err = json.Unmarshal(data, &f); err == nil {
			// either may be missing: a file from v0.8.x has no views
			if f.Clusters != nil {
				s.clusters = f.Clusters
			}
			if f.Views != nil {
				s.views = f.Views
			}
			s.fleet = f.Fleet
		}
	}
	if err != nil {
		fmt.Fprintf(w, "kube-node-viewer: ignoring saved view state %s: %s\n", path, err)
	}
	return s
}

// Get returns the view state saved for the cluster.
func (s *Store) Get(cluster string) (model.ViewState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.clusters[cluster]
	return v, ok
}

// Put records the cluster's view state in memory; Flush writes it.
func (s *Store) Put(cluster string, v model.ViewState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clusters[cluster] = v
}

// Fleet is the Clusters screen's saved contexts and scope; ok is false when
// none was ever chosen (clusters-screen R2, R8).
func (s *Store) Fleet() (contexts []string, cloud string, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fleet == nil {
		return nil, "", false
	}
	return append([]string(nil), s.fleet.Contexts...), s.fleet.Cloud, true
}

// PutFleet records the Clusters screen's choice; Flush writes it.
func (s *Store) PutFleet(contexts []string, cloud string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fleet = &fleet{Contexts: append([]string{}, contexts...), Cloud: cloud}
}

// ViewNames lists the named views, sorted.
func (s *Store) ViewNames() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	names := make([]string, 0, len(s.views))
	for name := range s.views {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// View returns the named view.
func (s *Store) View(name string) (model.ViewState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.views[name]
	return v, ok
}

// PutView records a named view, replacing one of the same name; Flush writes
// it.
func (s *Store) PutView(name string, v model.ViewState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.views[name] = v
}

// DeleteView forgets a named view; Flush writes it.
func (s *Store) DeleteView(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.views, name)
}

// Flush writes every saved state to the file atomically: a temporary file in
// the same directory, then a rename, so a killed process never leaves half a
// file behind.
func (s *Store) Flush() error {
	s.mu.Lock()
	data, err := json.MarshalIndent(file{Version: fileVersion, Clusters: s.clusters, Views: s.views, Fleet: s.fleet}, "", "  ")
	s.mu.Unlock()
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".state-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.path)
}

// Saver writes view changes in the background, at most once per delay, so the
// UI never waits on the disk (R2).
type Saver struct {
	store  *Store
	delay  time.Duration
	onErr  func(error)
	notify chan struct{}
	done   chan struct{}
	close  sync.Once
}

// NewSaver starts the background writer. onErr receives write failures; the
// UI has the screen, so it should not print them.
func NewSaver(store *Store, delay time.Duration, onErr func(error)) *Saver {
	s := &Saver{
		store:  store,
		delay:  delay,
		onErr:  onErr,
		notify: make(chan struct{}, 1),
		done:   make(chan struct{}),
	}
	go s.run()
	return s
}

// Save records the cluster's view state and schedules a write. It never
// blocks.
func (s *Saver) Save(cluster string, v model.ViewState) {
	s.store.Put(cluster, v)
	select {
	case s.notify <- struct{}{}:
	default:
	}
}

// SaveView records a named view and schedules a write.
func (s *Saver) SaveView(name string, v model.ViewState) {
	s.store.PutView(name, v)
	s.schedule()
}

// SaveFleet records the Clusters screen's choice and schedules a write.
func (s *Saver) SaveFleet(contexts []string, cloud string) {
	s.store.PutFleet(contexts, cloud)
	s.schedule()
}

// DeleteView forgets a named view and schedules a write.
func (s *Saver) DeleteView(name string) {
	s.store.DeleteView(name)
	s.schedule()
}

func (s *Saver) schedule() {
	select {
	case s.notify <- struct{}{}:
	default:
	}
}

// Close writes anything pending and stops the writer. Save must not be
// called after Close; Close may be called more than once.
func (s *Saver) Close() {
	s.close.Do(func() { close(s.notify) })
	<-s.done
}

func (s *Saver) run() {
	defer close(s.done)
	for range s.notify {
		timer := time.NewTimer(s.delay)
		closed := false
	wait:
		for {
			select {
			case _, ok := <-s.notify:
				if !ok {
					closed = true
					break wait
				}
			case <-timer.C:
				break wait
			}
		}
		timer.Stop()
		if err := s.store.Flush(); err != nil && s.onErr != nil {
			s.onErr(err)
		}
		if closed {
			return
		}
	}
}
