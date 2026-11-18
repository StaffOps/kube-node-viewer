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

package state_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/staffops/kube-node-viewer/pkg/state"
)

// clusters-screen R8, R14: a state file written before the Clusters screen
// has no fleet; it loads with its views and no choice.
func TestFleetMissingInOldFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	old := `{"version":1,"clusters":{"a@https://a.example.com":{}},"views":{"spot":{}}}`
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	var warn bytes.Buffer
	s := state.Load(path, &warn)
	if warn.Len() != 0 {
		t.Errorf("old file warned: %q", warn.String())
	}
	if ctxs, cloud, ok := s.Fleet(); ok || ctxs != nil || cloud != "" {
		t.Errorf("Fleet() = %v, %q, %v; want nil, \"\", false", ctxs, cloud, ok)
	}
	if _, ok := s.Get("a@https://a.example.com"); !ok {
		t.Error("old file's cluster view lost")
	}
	if _, ok := s.View("spot"); !ok {
		t.Error("old file's named view lost")
	}
}

// clusters-screen R8, R14: the chosen contexts and the scope round-trip
// through the file; an empty choice is still a choice.
func TestFleetRoundTrip(t *testing.T) {
	cases := []struct {
		name     string
		contexts []string
		cloud    string
	}{
		{"aws scope", []string{"eks-a", "eks-b"}, "aws"},
		{"every cloud", []string{"aks", "eks-a"}, ""},
		{"nothing ticked", []string{}, "azure"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state.json")
			s := state.Load(path, os.Stderr)
			s.PutFleet(tc.contexts, tc.cloud)
			if err := s.Flush(); err != nil {
				t.Fatal(err)
			}
			loaded := state.Load(path, os.Stderr)
			ctxs, cloud, ok := loaded.Fleet()
			if !ok {
				t.Fatal("Fleet() ok = false after a PutFleet")
			}
			if len(ctxs) != len(tc.contexts) || (len(ctxs) > 0 && !reflect.DeepEqual(ctxs, tc.contexts)) {
				t.Errorf("contexts = %v, want %v", ctxs, tc.contexts)
			}
			if cloud != tc.cloud {
				t.Errorf("cloud = %q, want %q", cloud, tc.cloud)
			}
		})
	}
}

// clusters-screen R8: the store keeps its own copy of the choice, in and out.
func TestFleetCopies(t *testing.T) {
	s := state.Load(filepath.Join(t.TempDir(), "state.json"), os.Stderr)
	in := []string{"a", "b"}
	s.PutFleet(in, "aws")
	in[0] = "changed"
	out, _, _ := s.Fleet()
	if out[0] != "a" {
		t.Errorf("PutFleet kept the caller's slice: %v", out)
	}
	out[1] = "changed"
	again, _, _ := s.Fleet()
	if again[1] != "b" {
		t.Errorf("Fleet returned the store's slice: %v", again)
	}
}

// clusters-screen R8, R14: Saver.SaveFleet writes the choice to the file
// (without Close, after its delay; and on Close).
func TestSaverSaveFleet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s := state.Load(path, os.Stderr)
	saver := state.NewSaver(s, 10*time.Millisecond, func(err error) { t.Error(err) })
	saver.SaveFleet([]string{"eks-a"}, "aws")

	deadline := time.Now().Add(5 * time.Second)
	for {
		data, err := os.ReadFile(path)
		if err == nil {
			var f struct {
				Fleet *struct {
					Contexts []string `json:"contexts"`
					Cloud    string   `json:"cloud"`
				} `json:"fleet"`
			}
			if err := json.Unmarshal(data, &f); err != nil {
				t.Fatal(err)
			}
			if f.Fleet != nil && reflect.DeepEqual(f.Fleet.Contexts, []string{"eks-a"}) && f.Fleet.Cloud == "aws" {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("fleet not written by the saver's delay: %s, %v", data, err)
		}
		time.Sleep(5 * time.Millisecond)
	}

	saver.SaveFleet([]string{"aks"}, "")
	saver.Close()
	ctxs, cloud, ok := state.Load(path, os.Stderr).Fleet()
	if !ok || !reflect.DeepEqual(ctxs, []string{"aks"}) || cloud != "" {
		t.Errorf("after Close: %v, %q, %v; want [aks], \"\", true", ctxs, cloud, ok)
	}
}
