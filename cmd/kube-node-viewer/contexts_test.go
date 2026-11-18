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
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/staffops/kube-node-viewer/pkg/model"
	"github.com/staffops/kube-node-viewer/pkg/state"
)

const arnBilling = "arn:aws:eks:us-east-1:111122223333:cluster/billing"

// probeContexts are the context names used to probe a loaded policy.
var probeContexts = []string{"prod-us", "prod-sandbox", "staging", arnBilling, "kind-a"}

func shownContexts(p model.LabelPolicy) []string {
	var out []string
	for _, c := range probeContexts {
		if p.Shows(c) {
			out = append(out, c)
		}
	}
	return out
}

// context-visibility R4, R1, R2: no contexts key, empty lists, and lists
// read from the file.
func TestLoadSettingsContexts(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{name: "no contexts key", body: "labels:\n  allow: [team]\n", want: probeContexts},
		{name: "empty file", body: "", want: probeContexts},
		{name: "contexts null", body: "contexts:\n", want: probeContexts},
		{name: "empty allow", body: "contexts:\n  allow: []\n", want: probeContexts},
		{name: "empty both", body: "contexts:\n  allow: []\n  deny: []\n", want: probeContexts},
		{name: "deny only", body: "contexts: {deny: [\"kind-*\"]}\n", want: []string{"prod-us", "prod-sandbox", "staging", arnBilling}},
		{
			name: "allow and deny",
			body: "contexts:\n  allow:\n    - \"prod-*\"\n    - \"arn:aws:eks:*:111122223333:cluster/*\"\n  deny:\n    - \"*-sandbox\"\n",
			want: []string{"prod-us", arnBilling},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := loadSettings(writeSettings(t, tt.body), nil)
			if err != nil {
				t.Fatal(err)
			}
			if got := shownContexts(s.contexts); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("shown %v, want %v", got, tt.want)
			}
		})
	}
	t.Run("no file", func(t *testing.T) {
		s, err := loadSettings(filepath.Join(t.TempDir(), "absent.yaml"), nil)
		if err != nil {
			t.Fatal(err)
		}
		if got := shownContexts(s.contexts); !reflect.DeepEqual(got, probeContexts) {
			t.Errorf("shown %v, want every context", got)
		}
	})
}

// context-visibility R8: a contexts key the file does not allow stops the
// start with an error naming contexts….
func TestLoadSettingsContextsInvalid(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "not a map", body: "contexts: [a]\n"},
		{name: "unknown field", body: "contexts: {other: x}\n"},
		{name: "allow not a list", body: "contexts:\n  allow: prod\n"},
		{name: "deny not a list", body: "contexts:\n  deny: prod\n"},
		{name: "empty item", body: "contexts:\n  allow: [\"\"]\n"},
		{name: "number item", body: "contexts:\n  allow: [1]\n"},
		{name: "padded item", body: "contexts:\n  allow: [\" prod-*\"]\n"},
		{name: "padded deny item", body: "contexts:\n  deny: [\"kind-* \"]\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeSettings(t, tt.body)
			_, err := loadSettings(path, nil)
			if err == nil {
				t.Fatal("accepted")
			}
			msg := err.Error()
			if !strings.HasPrefix(msg, "config file "+path+": ") {
				t.Errorf("message %q does not name the file", msg)
			}
			if !strings.Contains(msg, "contexts") {
				t.Errorf("message %q does not name contexts…", msg)
			}
		})
	}
}

// visibilityKubeconfig has the contexts of the spec's example; prod-twin
// shares prod-us's server.
const visibilityKubeconfig = `apiVersion: v1
kind: Config
current-context: staging
clusters:
- name: a
  cluster:
    server: https://example-a.example.com
- name: b
  cluster:
    server: https://example-b.example.com
- name: c
  cluster:
    server: https://example-c.example.com
- name: d
  cluster:
    server: https://example-d.example.com
contexts:
- name: prod-us
  context: {cluster: a}
- name: prod-sandbox
  context: {cluster: b}
- name: staging
  context: {cluster: c}
- name: arn:aws:eks:us-east-1:111122223333:cluster/billing
  context: {cluster: d}
- name: hidden-twin
  context: {cluster: a}
users: []
`

func writeKubeconfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte(visibilityKubeconfig), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func mustPolicy(t *testing.T, allow, deny []string) model.LabelPolicy {
	t.Helper()
	p, err := model.NewLabelPolicy(allow, deny)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func fleetNames(t *testing.T, s *sessions) []string {
	t.Helper()
	infos, err := s.fleetContexts()
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, i := range infos {
		out = append(out, i.Name)
	}
	slices.Sort(out)
	return out
}

func pickerNames(t *testing.T, s *sessions) ([]string, string) {
	t.Helper()
	names, current, err := s.Contexts()
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	out = append(out, names...)
	slices.Sort(out)
	return out, current
}

// context-visibility R1, R2, R3, R4: both pickers' context lists follow
// contexts.allow and contexts.deny, matched on the full name.
func TestContextListsFilterPickers(t *testing.T) {
	path := writeKubeconfig(t)
	all := []string{arnBilling, "hidden-twin", "prod-sandbox", "prod-us", "staging"}
	tests := []struct {
		name        string
		allow, deny []string
		want        []string
	}{
		{name: "no lists", want: all},
		{name: "empty allow", allow: []string{}, want: all},
		{name: "allow prod", allow: []string{"prod-*"}, want: []string{"prod-sandbox", "prod-us"}},
		{
			name:  "spec example",
			allow: []string{"prod-*", "arn:aws:eks:*:111122223333:cluster/*"},
			deny:  []string{"*-sandbox"},
			want:  []string{arnBilling, "prod-us"},
		},
		{name: "deny only", deny: []string{"prod-*"}, want: []string{arnBilling, "hidden-twin", "staging"}},
		{name: "short name does not match the ARN", allow: []string{"billing"}, want: nil},
		{name: "case-sensitive", allow: []string{"Prod-*"}, want: nil},
		{name: "whole name", allow: []string{"prod"}, want: nil},
		{name: "star crosses : and /", allow: []string{"arn:*/billing"}, want: []string{arnBilling}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &sessions{flags: Flags{Kubeconfig: path}, contexts: mustPolicy(t, tt.allow, tt.deny)}
			got, current := pickerNames(t, s)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Contexts() = %v, want %v", got, tt.want)
			}
			if current != "staging" {
				t.Errorf("current = %q, want the kubeconfig's", current)
			}
			if got := fleetNames(t, s); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("fleetContexts() = %v, want %v", got, tt.want)
			}
		})
	}
}

// context-visibility R5, R6: only the context the run opens on is checked;
// when the lists hide it they are off for the run.
func TestEffectivePolicy(t *testing.T) {
	lists := mustPolicy(t, []string{"prod-*"}, []string{"*-sandbox"})
	denyAll := mustPolicy(t, nil, []string{"*"})
	tests := []struct {
		name    string
		start   string
		policy  model.LabelPolicy
		wantOff bool
	}{
		{name: "allowed start", start: "prod-us", policy: lists},
		{name: "hidden start, not in allow", start: "staging", policy: lists, wantOff: true},
		{name: "hidden start, denied", start: "prod-sandbox", policy: lists, wantOff: true},
		{name: "no context name", start: "", policy: lists},
		{name: "deny all", start: "prod-us", policy: denyAll, wantOff: true},
		{name: "deny all, no name", start: "", policy: denyAll},
		{name: "no lists", start: "staging", policy: model.LabelPolicy{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, off := effectivePolicy(tt.start, tt.policy)
			if off != tt.wantOff {
				t.Fatalf("off = %v, want %v", off, tt.wantOff)
			}
			want := shownContexts(tt.policy)
			if off {
				want = probeContexts // zero policy: everything listed
			}
			if got := shownContexts(got); !reflect.DeepEqual(got, want) {
				t.Errorf("shown %v, want %v", got, want)
			}
		})
	}

	// R5: started with an allowed --context while the kubeconfig's current
	// context is denied, the lists apply and the current one is not listed.
	path := writeKubeconfig(t)
	p, off := effectivePolicy("prod-us", lists)
	if off {
		t.Fatal("lists off for an allowed start")
	}
	got, _ := pickerNames(t, &sessions{flags: Flags{Kubeconfig: path}, contexts: p})
	if slices.Contains(got, "staging") {
		t.Errorf("the denied kubeconfig current context is listed: %v", got)
	}
	// R6: started with --context staging (hidden), every context is listed.
	p, off = effectivePolicy("staging", lists)
	if !off {
		t.Fatal("lists not off for a hidden start")
	}
	if got := fleetNames(t, &sessions{flags: Flags{Kubeconfig: path}, contexts: p}); len(got) != 5 {
		t.Errorf("lists off but fleet lists %v", got)
	}
}

func newFleetStore(t *testing.T, path string, p model.LabelPolicy) (fleetStore, *state.Saver) {
	t.Helper()
	store := state.Load(path, io.Discard)
	saver := state.NewSaver(store, time.Millisecond, func(err error) { t.Errorf("save: %v", err) })
	return fleetStore{store: store, saver: saver, contexts: p}, saver
}

// context-visibility R7: a hidden saved context is not handed to the screen
// and stays in the saved choice; R6: with the lists off it is handed over.
func TestFleetStoreKeepsHiddenContexts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	deny := mustPolicy(t, nil, []string{"staging"})

	// the saved choice from an earlier run
	f, saver := newFleetStore(t, path, deny)
	f.store.PutFleet([]string{"prod-us", "staging"}, "")
	saver.Close()
	if err := f.store.Flush(); err != nil {
		t.Fatal(err)
	}

	f, saver = newFleetStore(t, path, deny)
	got, _, ok := f.Fleet()
	if !ok || !reflect.DeepEqual(got, []string{"prod-us"}) {
		t.Fatalf("Fleet() = %v %v, want [prod-us]", got, ok)
	}
	f.SaveFleet([]string{"prod-us", arnBilling}, "aws")
	saver.Close()

	saved, cloud, ok := state.Load(path, io.Discard).Fleet()
	slices.Sort(saved)
	want := []string{arnBilling, "prod-us", "staging"}
	if !ok || cloud != "aws" || !reflect.DeepEqual(saved, want) {
		t.Errorf("saved %v %q %v, want %v aws", saved, cloud, ok, want)
	}

	// a hidden context unticked stays; an allowed one unticked goes
	f, saver = newFleetStore(t, path, deny)
	f.SaveFleet([]string{"prod-us"}, "")
	saver.Close()
	saved, _, _ = state.Load(path, io.Discard).Fleet()
	slices.Sort(saved)
	if want := []string{"prod-us", "staging"}; !reflect.DeepEqual(saved, want) {
		t.Errorf("after unticking: saved %v, want %v", saved, want)
	}

	// R6: lists off (zero policy), the hidden one is handed over
	f, saver = newFleetStore(t, path, model.LabelPolicy{})
	defer saver.Close()
	got, _, _ = f.Fleet()
	if !slices.Contains(got, "staging") {
		t.Errorf("lists off: Fleet() = %v, want staging in it", got)
	}
}

// context-visibility R7: a hidden saved context sharing its API server with
// an allowed chosen one is in neither the fleet's contexts nor its choice.
func TestHiddenTwinNotInFleet(t *testing.T) {
	kube := writeKubeconfig(t)
	p := mustPolicy(t, nil, []string{"hidden-*"})
	s := &sessions{flags: Flags{Kubeconfig: kube}, contexts: p}
	names := fleetNames(t, s)
	if slices.Contains(names, "hidden-twin") || !slices.Contains(names, "prod-us") {
		t.Errorf("fleetContexts = %v", names)
	}

	path := filepath.Join(t.TempDir(), "state.json")
	f, saver := newFleetStore(t, path, p)
	defer saver.Close()
	f.store.PutFleet([]string{"hidden-twin", "prod-us"}, "")
	got, _, _ := f.Fleet()
	if !reflect.DeepEqual(got, []string{"prod-us"}) {
		t.Errorf("Fleet() = %v, want [prod-us]", got)
	}
}
