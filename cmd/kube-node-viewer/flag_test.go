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
	"os"
	"path/filepath"
	"testing"
)

func TestConfigFile(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(configPath(home), []byte("# defaults\nnode-sort=kube-node-viewer/node-cpu-usage=dsc\nresources = cpu,memory\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfigFile(configPath(home))
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.getValue("node-sort", ""); got != "kube-node-viewer/node-cpu-usage=dsc" {
		t.Errorf("node-sort = %q", got)
	}
	if got := cfg.getValue("resources", ""); got != "cpu,memory" {
		t.Errorf("resources = %q", got)
	}
}

// rebrand R3, as corrected 2026-11-02: only ~/.kube-node-viewer is read.
func TestOnlyTheKubeNodeViewerConfigFileIsRead(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".eks-node-viewer"), []byte("style=#ff0000\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfigFile(configPath(home))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg) != 0 {
		t.Errorf("read another tool's config file: %v", cfg)
	}
}
