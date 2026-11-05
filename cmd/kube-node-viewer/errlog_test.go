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
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/klog/v2"
)

// The standard logger's date and time are dropped too (fix-azure-retry R4).
// fix-azure-retry R4
func TestLastLine(t *testing.T) {
	cases := map[string]string{
		"E1102 12:00:01.123456   42 reflector.go:200] failed to list *v1.Node: connection refused\n": "failed to list *v1.Node: connection refused",
		"2026/11/02 12:00:01 Failed to create pricing client: no region\n":                           "Failed to create pricing client: no region",
		"2026/11/02 12:00:01.123456 with microseconds\n":                                             "with microseconds",
		"2026/11/02 not a time\n": "2026/11/02 not a time",
		"first\nsecond\n\n":       "second",
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

// fix-azure-retry R3
func TestResolvedLineClearsTheMatchingIssue(t *testing.T) {
	const fail = "2026/11/05 13:28:32 updating Azure pricing for eastus2, retail prices API: 429 Too Many Requests (no retry header)\n"
	write := func(l *errorLog, lines ...string) {
		for _, line := range lines {
			_, _ = l.Write([]byte(line))
		}
	}
	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"the matching failure clears", []string{fail, "2026/11/05 13:29:33 resolved: updating Azure pricing for eastus2\n"}, ""},
		{"a newer unrelated error stays", []string{fail, "E1105 13:29:00.1   1 r.go:1] watch failed\n", "2026/11/05 13:29:33 resolved: updating Azure pricing for eastus2\n"}, "watch failed"},
		{"another region does not clear", []string{fail, "2026/11/05 13:29:33 resolved: updating Azure pricing for westeurope\n"}, "updating Azure pricing for eastus2, retail prices API: 429 Too Many Requests (no retry header)"},
		{"a region prefix does not clear", []string{fail, "2026/11/05 13:29:33 resolved: updating Azure pricing for eastus\n"}, "updating Azure pricing for eastus2, retail prices API: 429 Too Many Requests (no retry header)"},
		{"a resolution alone is not kept", []string{"2026/11/05 13:29:33 resolved: updating Azure pricing for eastus2\n"}, ""},
	}
	for _, c := range cases {
		var file bytes.Buffer
		l := &errorLog{out: &file}
		write(l, c.lines...)
		if got, _ := l.Last(); got != c.want {
			t.Errorf("%s: last = %q, want %q", c.name, got, c.want)
		}
		if got := strings.Count(file.String(), "\n"); got != len(c.lines) {
			t.Errorf("%s: %d lines in the log file, want %d", c.name, got, len(c.lines))
		}
	}
}

// Through the real logger: log.Printf adds its date and time, the header
// still clears.
// fix-azure-retry R3
func TestResolvedThroughCaptureLogs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kube-node-viewer.log")
	logs, restore := captureLogs(path)
	log.Printf("updating Azure pricing for eastus2, retail prices API: 429 Too Many Requests (no retry header)")
	failed, _ := logs.Last()
	log.Printf("resolved: updating Azure pricing for eastus2")
	cleared, _ := logs.Last()
	restore()
	if failed != "updating Azure pricing for eastus2, retail prices API: 429 Too Many Requests (no retry header)" {
		t.Errorf("failure kept as %q", failed)
	}
	if cleared != "" {
		t.Errorf("after the resolution the header shows %q", cleared)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "resolved: updating Azure pricing for eastus2") {
		t.Errorf("the resolution is not in the log file:\n%s", data)
	}
}
