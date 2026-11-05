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
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/staffops/kube-node-viewer/pkg/model"
	"github.com/staffops/kube-node-viewer/pkg/state"
)

func sampleView() model.ViewState {
	v := model.DefaultViewState()
	v.Filters = []model.Filter{{Field: model.FieldCapacity, Values: []string{"Spot"}, Enabled: true}}
	v.SetGroupBy(model.FieldNodeGroup, model.FieldZone)
	v.Sort = model.SortSpec{Field: model.FieldPrice, Descending: true}
	v.Search = "not saved"
	return v
}

// interactive-tui R1, R2: the view state saved and restored per cluster.
func TestRoundTripPerCluster(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "state.json")
	s := state.Load(path, os.Stderr)
	s.Put("a@https://a", sampleView())
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}

	loaded := state.Load(path, os.Stderr)
	got, ok := loaded.Get("a@https://a")
	if !ok {
		t.Fatal("state for cluster a not restored")
	}
	want := sampleView()
	want.Search = "" // search text is per session
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
	if _, ok := loaded.Get("b@https://b"); ok {
		t.Error("cluster b got cluster a's state")
	}
	if _, ok := loaded.Get("a@https://other"); ok {
		t.Error("same context name on another server got its state")
	}
}

func TestCorruptFileWarnsOnceAndStartsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	var warn bytes.Buffer
	s := state.Load(path, &warn)
	if strings.Count(warn.String(), "\n") != 1 || !strings.Contains(warn.String(), path) {
		t.Errorf("warning: %q", warn.String())
	}
	if _, ok := s.Get("a@x"); ok {
		t.Error("corrupt file produced state")
	}
}

func TestMissingFileIsSilent(t *testing.T) {
	var warn bytes.Buffer
	state.Load(filepath.Join(t.TempDir(), "none.json"), &warn)
	if warn.Len() != 0 {
		t.Errorf("unexpected warning %q", warn.String())
	}
}

func TestSaverWritesOnCloseAndLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	saver := state.NewSaver(state.Load(path, os.Stderr), time.Hour, func(err error) { t.Error(err) })
	for i := 0; i < 50; i++ {
		saver.Save("a@x", sampleView())
	}
	saver.Close()

	if _, ok := state.Load(path, os.Stderr).Get("a@x"); !ok {
		t.Fatal("pending state not written on close")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("files left: %v", entries)
	}
}

// interactive-tui R2: saving is debounced off the UI path.
func TestSaverDebounces(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	saver := state.NewSaver(state.Load(path, os.Stderr), 20*time.Millisecond, func(err error) { t.Error(err) })
	defer saver.Close()
	saver.Save("a@x", sampleView())
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("state never written")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestDefaultPath(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/xdg")
	if got := state.DefaultPath("/home/u"); got != "/xdg/kube-node-viewer/state.json" {
		t.Errorf("got %s", got)
	}
	t.Setenv("XDG_STATE_HOME", "")
	if got := state.DefaultPath("/home/u"); got != "/home/u/.local/state/kube-node-viewer/state.json" {
		t.Errorf("got %s", got)
	}
}

func TestNamedViewsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s := state.Load(path, os.Stderr)
	s.PutView("spot by zone", sampleView())
	s.PutView("a first", model.DefaultViewState())
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	loaded := state.Load(path, os.Stderr)
	if got := loaded.ViewNames(); !reflect.DeepEqual(got, []string{"a first", "spot by zone"}) {
		t.Errorf("names %v", got)
	}
	if _, ok := loaded.View("spot by zone"); !ok {
		t.Error("view lost")
	}
	loaded.DeleteView("a first")
	if got := loaded.ViewNames(); len(got) != 1 {
		t.Errorf("after delete %v", got)
	}
}

func TestViewsWithoutClustersAndOldFiles(t *testing.T) {
	dir := t.TempDir()
	onlyViews := filepath.Join(dir, "views.json")
	if err := os.WriteFile(onlyViews, []byte(`{"version":1,"views":{"v":{"sort":{"field":"price"},"showNodes":true}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := state.Load(onlyViews, os.Stderr).View("v"); !ok {
		t.Error("a file with views and no clusters lost its views")
	}
	old := filepath.Join(dir, "old.json")
	if err := os.WriteFile(old, []byte(`{"version":1,"clusters":{"a@x":{"sort":{"field":"name"},"showNodes":true}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	s := state.Load(old, os.Stderr)
	if len(s.ViewNames()) != 0 {
		t.Error("an old file grew views")
	}
	if _, ok := s.Get("a@x"); !ok {
		t.Error("an old file lost its cluster")
	}
}
