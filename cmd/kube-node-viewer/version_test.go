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
	"runtime/debug"
	"testing"
)

func TestResolveVersion(t *testing.T) {
	info := &debug.BuildInfo{
		Main:     debug.Module{Version: "v0.1.0"},
		Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "abc123"}},
	}
	devel := &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}

	cases := []struct {
		name        string
		version     string
		commit      string
		info        *debug.BuildInfo
		ok          bool
		wantVersion string
		wantCommit  string
	}{
		{"ldflags win", "v1.2.3", "def456", info, true, "v1.2.3", "def456"},
		{"go install", "dev", "", info, true, "v0.1.0", "abc123"},
		{"local devel build", "dev", "", devel, true, "dev", ""},
		{"no build info", "dev", "", nil, false, "dev", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v, cm := resolveVersion(c.version, c.commit, c.info, c.ok)
			if v != c.wantVersion || cm != c.wantCommit {
				t.Errorf("got (%q, %q), want (%q, %q)", v, cm, c.wantVersion, c.wantCommit)
			}
		})
	}
}
