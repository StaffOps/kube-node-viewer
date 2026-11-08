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

package model

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// The node list's AGE column reads the package clock.
func TestAgeColumnUsesTheClock(t *testing.T) {
	u := demoUI(t)
	v := u.view
	v.Columns = []Field{FieldAge}
	u.SetView(v)
	u.Refresh()
	u.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	// ip-192-0-2-10 was created 52 hours before demoNow
	if out := u.View(); !strings.Contains(out, "2d4h") {
		t.Errorf("no node shows the age 2d4h:\n%s", out)
	}
}

// The computed node-age label reads the package clock.
func TestNodeAgeLabelUsesTheClock(t *testing.T) {
	pinDemoClock(t)
	n := NewNode(&v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "n", CreationTimestamp: metav1.NewTime(demoNow.Add(-3 * time.Hour))}})
	if got := n.ComputeLabel(LabelPrefix + "node-age"); got != "3h" {
		t.Errorf("node-age = %q, want 3h", got)
	}
}
