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
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/staffops/kube-node-viewer/pkg/model"
)

// Tests for label-visibility R7-R9, written against
// specs/label-visibility/requirements.md.

func TestSettingsPath(t *testing.T) {
	home := t.TempDir()
	abs := t.TempDir()
	fallback := filepath.Join(home, ".config", "kube-node-viewer", "config.yaml")
	tests := []struct {
		name  string
		set   bool
		value string
		want  string
	}{
		{name: "set", set: true, value: abs, want: filepath.Join(abs, "kube-node-viewer", "config.yaml")},
		{name: "unset", want: fallback},
		{name: "empty", set: true, value: "", want: fallback},
		{name: "relative", set: true, value: "relative/config", want: fallback},
		{name: "dot relative", set: true, value: "./cfg", want: fallback},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// t.Setenv restores the variable; Unsetenv after it is undone too
			t.Setenv("XDG_CONFIG_HOME", tt.value)
			if !tt.set {
				if err := os.Unsetenv("XDG_CONFIG_HOME"); err != nil {
					t.Fatal(err)
				}
			}
			if got := settingsPath(home); got != tt.want {
				t.Errorf("settingsPath = %q, want %q", got, tt.want)
			}
		})
	}
}

// writeSettings writes a config file under a temp dir and returns its path.
func writeSettings(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "kube-node-viewer", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// keys are the label keys used to probe a loaded policy.
var keys = []string{"team", "env", "karpenter.sh/nodepool", "app.example.com/owner", "app.example.com/internal-id", "kubernetes.io/os"}

// shown is the subset of keys the policy shows.
func shown(p model.LabelPolicy) []string {
	var out []string
	for _, k := range keys {
		if p.Shows(k) {
			out = append(out, k)
		}
	}
	return out
}

func TestLoadSettingsValidFiles(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "allow and deny",
			body: "labels:\n  allow:\n    - team\n    - app.example.com/*\n  deny:\n    - app.example.com/internal-*\n",
			want: []string{"team", "app.example.com/owner"},
		},
		{
			name: "deny only",
			body: "labels:\n  deny:\n    - env\n",
			want: []string{"team", "karpenter.sh/nodepool", "app.example.com/owner", "app.example.com/internal-id", "kubernetes.io/os"},
		},
		{name: "allow null", body: "labels:\n  allow: null\n", want: keys},
		{name: "allow with no value", body: "labels:\n  allow:\n", want: keys},
		{name: "allow empty list", body: "labels:\n  allow: []\n", want: keys},
		{name: "no labels key", body: "# nothing yet\n", want: keys},
		{name: "empty file", body: "", want: keys},
		{name: "labels null", body: "labels:\n", want: keys},
		{name: "quoted star", body: "labels:\n  allow: [\"karpenter.sh/*\"]\n", want: []string{"karpenter.sh/nodepool"}},
		{name: "quoted yes is a string", body: "labels:\n  allow: [\"yes\", '123', team]\n", want: []string{"team"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := loadSettings(writeSettings(t, tt.body), nil)
			if err != nil {
				t.Fatal(err)
			}
			if got := shown(p.policy); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("shows %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadSettingsMissingFileOrDir(t *testing.T) {
	dir := t.TempDir()
	for name, path := range map[string]string{
		"missing file": filepath.Join(dir, "config.yaml"),
		"missing dir":  filepath.Join(dir, "no", "such", "dir", "config.yaml"),
	} {
		t.Run(name, func(t *testing.T) {
			p, err := loadSettings(path, nil)
			if err != nil {
				t.Fatalf("not an error (R8): %v", err)
			}
			if got := shown(p.policy); !reflect.DeepEqual(got, keys) {
				t.Errorf("shows %v, want every key", got)
			}
		})
	}
}

func TestLoadSettingsInvalidFiles(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		problem string
	}{
		{name: "invalid YAML", body: "labels:\n  allow: [team\n", problem: ""},
		{name: "invalid indentation", body: "labels:\n allow:\n   - a\n  deny: x\n", problem: ""},
		{name: "unknown field", body: "lables:\n  allow: [team]\n", problem: `unknown field "lables"`},
		{name: "unknown nested field", body: "labels:\n  alow: [team]\n", problem: `unknown field "alow"`},
		{name: "string instead of list", body: "labels:\n  allow: team\n", problem: "allow"},
		{name: "item 123", body: "labels:\n  allow: [a, b, 123]\n", problem: "123"},
		{name: "item yes", body: "labels:\n  allow:\n    - yes\n", problem: "labels.allow"},
		{name: "item ~", body: "labels:\n  deny:\n    - ~\n", problem: "labels.deny"},
		{name: "item empty string", body: "labels:\n  deny:\n    - ''\n", problem: "empty"},
		{name: "item a map", body: "labels:\n  deny:\n    - {a: b}\n", problem: "labels.deny"},
		{name: "labels a list", body: "labels: [a]\n", problem: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeSettings(t, tt.body)
			_, err := loadSettings(path, nil)
			if err == nil {
				t.Fatal("accepted (R8)")
			}
			msg := err.Error()
			if !strings.Contains(msg, path) {
				t.Errorf("message does not name the file (R8): %q", msg)
			}
			if !strings.HasPrefix(msg, "config file "+path+": ") {
				t.Errorf("message %q is not 'config file <path>: <problem>'", msg)
			}
			if tt.problem != "" && !strings.Contains(msg, tt.problem) {
				t.Errorf("message %q does not say %q", msg, tt.problem)
			}
			if strings.Contains(msg, "error unmarshaling JSON") {
				t.Errorf("library prefix kept: %q", msg)
			}
		})
	}
}

func TestLoadSettingsUnreadableFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode 000 file")
	}
	path := writeSettings(t, "labels:\n  allow: [team]\n")
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
	_, err := loadSettings(path, nil)
	if err == nil {
		t.Fatal("an unreadable file was accepted (R8)")
	}
	if !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("message %q does not name the file and the problem", err)
	}
}

func TestLoadSettingsDirectoryInPlaceOfFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	_, err := loadSettings(path, nil)
	if err == nil {
		t.Fatal("a directory at the file's path was accepted")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("message %q does not name the file", err)
	}
}

func TestLabelFlagsReplaceTheFileLists(t *testing.T) {
	file := "labels:\n  allow:\n    - team\n    - app.example.com/*\n  deny:\n    - app.example.com/internal-*\n"
	tests := []struct {
		name     string
		explicit map[string]string
		want     []string
	}{
		{
			name: "no flags: the file", explicit: map[string]string{},
			want: []string{"team", "app.example.com/owner"},
		},
		{
			name: "allow replaced, deny kept", explicit: map[string]string{"labels-allow": "karpenter.sh/*"},
			want: []string{"karpenter.sh/nodepool"},
		},
		{
			name: "allow replaced, deny still wins", explicit: map[string]string{"labels-allow": "app.example.com/*"},
			want: []string{"app.example.com/owner"},
		},
		{
			name: "allow cleared", explicit: map[string]string{"labels-allow": ""},
			want: []string{"team", "env", "karpenter.sh/nodepool", "app.example.com/owner", "kubernetes.io/os"},
		},
		{
			name: "deny cleared", explicit: map[string]string{"labels-deny": ""},
			want: []string{"team", "app.example.com/owner", "app.example.com/internal-id"},
		},
		{
			name: "deny replaced", explicit: map[string]string{"labels-deny": "team"},
			want: []string{"app.example.com/owner", "app.example.com/internal-id"},
		},
		{
			name: "both, comma separated with spaces", explicit: map[string]string{"labels-allow": "karpenter.sh/*, team ,env", "labels-deny": "env"},
			want: []string{"team", "karpenter.sh/nodepool"},
		},
		{
			name: "both cleared", explicit: map[string]string{"labels-allow": "", "labels-deny": ""},
			want: keys,
		},
		{
			name: "other flags do not touch the lists", explicit: map[string]string{"extra-labels": "env", "group-by": "env"},
			want: []string{"team", "app.example.com/owner"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeSettings(t, file)
			p, err := loadSettings(path, tt.explicit)
			if err != nil {
				t.Fatal(err)
			}
			if got := shown(p.policy); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("shows %v, want %v", got, tt.want)
			}
			// the run does not write the config file (R7)
			if data, _ := os.ReadFile(path); string(data) != file {
				t.Errorf("config file changed:\n%s", data)
			}
		})
	}
}

func TestLabelFlagsWithoutAConfigFile(t *testing.T) {
	p, err := loadSettings(filepath.Join(t.TempDir(), "absent.yaml"), map[string]string{"labels-deny": "env,team"})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := shown(p.policy), []string{"karpenter.sh/nodepool", "app.example.com/owner", "app.example.com/internal-id", "kubernetes.io/os"}; !reflect.DeepEqual(got, want) {
		t.Errorf("shows %v, want %v", got, want)
	}
}

// TestLabelFlagsAreNotViewState checks R7: the flags do not reach the view
// state that is saved per cluster.
func TestLabelFlagsAreNotViewState(t *testing.T) {
	explicit := map[string]string{"labels-allow": "team", "labels-deny": "env"}
	got, err := resolveView(configFile{}, nil, nil, explicit)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, model.DefaultViewState()) {
		t.Errorf("label flags changed the view: %+v", got)
	}
	data, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"labels-allow", "labels-deny", "team"} {
		if strings.Contains(string(data), s) {
			t.Errorf("saved view holds %q: %s", s, data)
		}
	}
}

// TestLabelListsNotReadFromTheOldConfigFile checks the counter-example: the
// lists are never read from ~/.kube-node-viewer.
func TestLabelListsNotReadFromTheOldConfigFile(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(configPath(home), []byte("labels-allow=team\nlabels-deny=env\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	oldHome, oldArgs := homeDir, os.Args
	t.Cleanup(func() { homeDir, os.Args = oldHome, oldArgs })
	homeDir = home
	os.Args = []string{"kube-node-viewer"}
	flags, err := ParseFlags()
	if err != nil {
		t.Fatal(err)
	}
	if flags.LabelsAllow != "" || flags.LabelsDeny != "" {
		t.Errorf("lists read from ~/.kube-node-viewer: allow %q deny %q", flags.LabelsAllow, flags.LabelsDeny)
	}
	if _, ok := flags.Explicit["labels-allow"]; ok {
		t.Error("labels-allow counted as given on the command line")
	}
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	p, err := loadSettings(settingsPath(home), flags.Explicit)
	if err != nil {
		t.Fatal(err)
	}
	if got := shown(p.policy); !reflect.DeepEqual(got, keys) {
		t.Errorf("shows %v, want every key", got)
	}
}

func TestLabelFlagsParsed(t *testing.T) {
	home := t.TempDir()
	oldHome, oldArgs := homeDir, os.Args
	t.Cleanup(func() { homeDir, os.Args = oldHome, oldArgs })
	homeDir = home
	os.Args = []string{"kube-node-viewer", "--labels-allow", "karpenter.sh/*,team", "--labels-deny", ""}
	flags, err := ParseFlags()
	if err != nil {
		t.Fatal(err)
	}
	if flags.Explicit["labels-allow"] != "karpenter.sh/*,team" {
		t.Errorf("explicit labels-allow %q", flags.Explicit["labels-allow"])
	}
	if v, ok := flags.Explicit["labels-deny"]; !ok || v != "" {
		t.Errorf("explicit labels-deny %q, %v", v, ok)
	}
}

// helpText is the -h output of ParseFlags with the given home and
// XDG_CONFIG_HOME (unset when xdg is nil).
func helpText(t *testing.T, home string, xdg *string) string {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", "")
	if xdg == nil {
		if err := os.Unsetenv("XDG_CONFIG_HOME"); err != nil {
			t.Fatal(err)
		}
	} else {
		t.Setenv("XDG_CONFIG_HOME", *xdg)
	}
	oldHome, oldArgs, oldStderr := homeDir, os.Args, os.Stderr
	t.Cleanup(func() { homeDir, os.Args, os.Stderr = oldHome, oldArgs, oldStderr })
	homeDir = home
	os.Args = []string{"kube-node-viewer", "-h"}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	_, perr := ParseFlags()
	os.Stderr = oldStderr
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if !errors.Is(perr, flag.ErrHelp) {
		t.Fatalf("ParseFlags -h: %v, want flag.ErrHelp", perr)
	}
	return string(out)
}

// flagHelp is the usage text of one flag in the -h output.
func flagHelp(t *testing.T, help, name string) string {
	t.Helper()
	_, rest, ok := strings.Cut(help, "  -"+name+" ")
	if !ok {
		t.Fatalf("no -%s in the help:\n%s", name, help)
	}
	if end := strings.Index(rest, "\n  -"); end >= 0 {
		rest = rest[:end]
	}
	return rest
}

func TestLabelFlagsHelpNamesTheFileTheRunReads(t *testing.T) {
	home := t.TempDir()
	abs := t.TempDir()
	relative := "relative/config"
	fallback := filepath.Join(home, ".config", "kube-node-viewer", "config.yaml")
	tests := []struct {
		name string
		xdg  *string
		want string
	}{
		{name: "absolute XDG_CONFIG_HOME", xdg: &abs, want: filepath.Join(abs, "kube-node-viewer", "config.yaml")},
		{name: "unset", want: fallback},
		{name: "relative", xdg: &relative, want: fallback},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			help := helpText(t, home, tt.xdg)
			for _, name := range []string{"labels-allow", "labels-deny"} {
				usage := flagHelp(t, help, name)
				if !strings.Contains(usage, tt.want) {
					t.Errorf("-%s help does not name %s:\n%s", name, tt.want, usage)
				}
				if tt.want != fallback && strings.Contains(usage, fallback) {
					t.Errorf("-%s help names the fallback %s with XDG_CONFIG_HOME set:\n%s", name, fallback, usage)
				}
			}
		})
	}
}
