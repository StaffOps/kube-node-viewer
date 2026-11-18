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
	"slices"
	"strings"
	"testing"
)

// context-visibility R6: while the lists are off, the title of both pickers
// says so; otherwise it does not.
func TestPickerTitlesSayListsOff(t *testing.T) {
	for _, off := range []bool{false, true} {
		name := "lists on"
		if off {
			name = "lists off"
		}
		t.Run(name, func(t *testing.T) {
			// the context picker, c on Overview
			u, _, _ := switchUI(t)
			if off {
				u.SetContextListsOff()
			}
			apply(u, press(u, "c"))
			if u.panel != panelContexts {
				t.Fatalf("c did not open the context picker: panel %v", u.panel)
			}
			out := u.View()
			if got := strings.Contains(out, "Context (lists off)"); got != off {
				t.Errorf("context picker title marked=%v, want %v:\n%s", got, off, out)
			}
			if !strings.Contains(out, "Context") {
				t.Errorf("context picker has no title:\n%s", out)
			}

			// the Clusters screen's picker, c on Clusters
			c := clustersUI(t)
			if off {
				c.SetContextListsOff()
			}
			apply(c, press(c, "c"))
			out = c.View()
			if !strings.Contains(out, "Clusters") {
				t.Fatalf("Clusters picker not open:\n%s", out)
			}
			if got := strings.Contains(out, "Clusters (lists off)"); got != off {
				t.Errorf("Clusters picker title marked=%v, want %v:\n%s", got, off, out)
			}
		})
	}
}

// context-visibility R7: with the hidden context left out of the infos and
// the saved choice (as the cmd side hands them over), the allowed one sharing
// its server is wanted and no "same cluster as" mark appears.
func TestHiddenTwinLeavesNoSameClusterMark(t *testing.T) {
	e := newFleetEnv(t, 40, []ContextInfo{
		{Name: "prod-us", Server: "https://example-a.example.com", Cloud: "aws"},
	}, []string{"demo-prod", "prod-us"})
	u := e.u
	command(u, "cl")
	if len(e.f.wanted) == 0 || !slices.Contains(e.f.wanted[len(e.f.wanted)-1], "prod-us") {
		t.Fatalf("prod-us not wanted: %v", e.f.wanted)
	}
	apply(u, press(u, "c"))
	out := u.View()
	if !strings.Contains(out, "prod-us") {
		t.Fatalf("prod-us not in the picker:\n%s", out)
	}
	if strings.Contains(out, "same cluster as") {
		t.Errorf("a same cluster mark with only allowed contexts:\n%s", out)
	}
	if strings.Contains(out, "missing") {
		t.Errorf("a context shown as missing:\n%s", out)
	}
}
