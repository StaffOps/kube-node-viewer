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
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"k8s.io/klog/v2"

	"github.com/staffops/kube-node-viewer/pkg/model"
)

// errorLog takes what client-go (klog) and the standard logger print while
// the TUI owns the terminal, where it would scramble the screen. It writes it
// to a file and keeps the last line for the header.
type errorLog struct {
	mu   sync.Mutex
	out  io.Writer
	last string
	at   time.Time
}

func (l *errorLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.out != nil {
		_, _ = l.out.Write(p)
	}
	line := lastLine(string(p))
	if text, ok := strings.CutPrefix(line, model.ResolvedPrefix); ok {
		// a resolution: clear the kept error it names, never keep it
		// (fix-azure-retry R3); text must end at the end or at a comma, so
		// "eastus" does not resolve "eastus2"
		if text != "" && (l.last == text || strings.HasPrefix(l.last, text+",")) {
			l.last, l.at = "", time.Time{}
		}
		return len(p), nil
	}
	if line != "" {
		l.last, l.at = line, time.Now()
	}
	return len(p), nil
}

// Last is the last line logged and when, for the header.
func (l *errorLog) Last() (string, time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.last, l.at
}

// stdLogTime is the date and time the standard logger puts first.
var stdLogTime = regexp.MustCompile(`^\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}(\.\d+)? `)

// lastLine is the last non-empty line, without klog's header (severity,
// timestamp, pid, file:line) or the standard logger's date and time when it
// has one (fix-azure-retry R4).
func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	line := strings.TrimSpace(lines[len(lines)-1])
	if i := strings.Index(line, "] "); i > 0 && len(line) > 0 && strings.ContainsRune("IWEF", rune(line[0])) {
		line = line[i+2:]
	}
	return stdLogTime.ReplaceAllString(line, "")
}

// captureLogs sends klog and the standard logger to path (truncated) and to
// the returned errorLog, until restore is called. If the file can't be opened
// lines are only kept for the header.
func captureLogs(path string) (l *errorLog, restore func()) {
	l = &errorLog{}
	var f *os.File
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err == nil {
		f, _ = os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	}
	if f != nil {
		l.out = f
	}
	klog.LogToStderr(false)
	klog.SetOutput(l)
	stdOut := log.Writer()
	log.SetOutput(l)
	return l, func() {
		klog.Flush()
		klog.SetOutput(os.Stderr)
		klog.LogToStderr(true)
		log.SetOutput(stdOut)
		if f != nil {
			_ = f.Close()
		}
	}
}
