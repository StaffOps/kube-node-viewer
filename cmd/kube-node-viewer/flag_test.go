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
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveConfigPath(t *testing.T) {
	cases := []struct {
		name     string
		files    []string
		want     string
		wantWarn bool
	}{
		{"new only", []string{configFileName}, configFileName, false},
		{"legacy only", []string{legacyConfigFileName}, legacyConfigFileName, true},
		{"both", []string{configFileName, legacyConfigFileName}, configFileName, false},
		{"neither", nil, configFileName, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			home := t.TempDir()
			for _, f := range c.files {
				if err := os.WriteFile(filepath.Join(home, f), []byte("style=#ff0000\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var warn bytes.Buffer
			got := resolveConfigPath(home, &warn)
			if got != filepath.Join(home, c.want) {
				t.Errorf("path = %q, want %q", got, filepath.Join(home, c.want))
			}
			if (warn.Len() > 0) != c.wantWarn {
				t.Errorf("warning = %q, want warning: %v", warn.String(), c.wantWarn)
			}
		})
	}
}

func TestLoadConfigFileLegacyContent(t *testing.T) {
	home := t.TempDir()
	legacy := filepath.Join(home, legacyConfigFileName)
	if err := os.WriteFile(legacy, []byte("node-sort=eks-node-viewer/node-cpu-usage=dsc\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfigFile(resolveConfigPath(home, &bytes.Buffer{}))
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.getValue("node-sort", ""); got != "eks-node-viewer/node-cpu-usage=dsc" {
		t.Errorf("node-sort = %q", got)
	}
}
