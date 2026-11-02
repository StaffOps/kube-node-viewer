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
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/klog/v2"
)

func TestLastLine(t *testing.T) {
	cases := map[string]string{
		"E1102 12:00:01.123456   42 reflector.go:200] failed to list *v1.Node: connection refused\n": "failed to list *v1.Node: connection refused",
		"2026/11/02 12:00:01 Failed to create pricing client: no region\n":                           "2026/11/02 12:00:01 Failed to create pricing client: no region",
		"first\nsecond\n\n": "second",
	}
	for in, want := range cases {
		if got := lastLine(in); got != want {
			t.Errorf("lastLine(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCaptureLogsKeepsTheTerminalClean(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "kube-node-viewer.log")
	logs, restore := captureLogs(path)
	klog.Errorf("watch failed: %s", "connection refused")
	log.Printf("pricing: %s", "throttled")
	klog.Flush()
	msg, at := logs.Last()
	restore()

	if msg != "pricing: throttled" && !strings.HasSuffix(msg, "pricing: throttled") {
		t.Errorf("last = %q", msg)
	}
	if at.IsZero() {
		t.Error("no time recorded")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "watch failed: connection refused") || !strings.Contains(string(data), "pricing: throttled") {
		t.Errorf("log file:\n%s", data)
	}
}
