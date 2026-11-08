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
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// firstHeaderLine is the first line inside the header box.
func firstHeaderLine(t *testing.T, view string) string {
	t.Helper()
	lines := strings.Split(view, "\n")
	if len(lines) < 2 || !strings.HasPrefix(lines[0], "╭") {
		t.Fatalf("no header box:\n%s", view)
	}
	return strings.TrimSpace(strings.Trim(lines[1], "│"))
}

// compact-header-no-url R1
func TestCompactHeaderHasNoServerURL(t *testing.T) {
	const server = "https://0A1B2C3D4E5F.gr7.us-east-1.eks.amazonaws.com"
	for _, tc := range []struct {
		name      string
		height    int
		ctrlE     bool
		region    string
		pending   bool
		noPricing bool
		want      string
	}{
		{"ctrl+e, region known", 40, true, "us-east-1", false, false, "ctx-a · us-east-1 · 2 of 2 shown · 0 pods · $0.000/month (global cluster)"},
		{"by height, region known", 20, false, "us-east-1", false, false, "ctx-a · us-east-1 · 2 of 2 shown · 0 pods · $0.000/month (global cluster)"},
		{"ctrl+e, no region", 40, true, "", false, false, "ctx-a · 2 of 2 shown · 0 pods · $0.000/month (global cluster)"},
		{"by height, no region", 20, false, "", false, false, "ctx-a · 2 of 2 shown · 0 pods · $0.000/month (global cluster)"},
		{"a pod not on a node yet", 20, false, "us-east-1", true, false, "ctx-a · us-east-1 · 2 of 2 shown · 0 pods (1 pending) · $0.000/month (global cluster)"},
		{"pricing off", 20, false, "us-east-1", false, true, "ctx-a · us-east-1 · 2 of 2 shown · 0 pods"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// capacityUI's nodes carry no region label, so "no region" has none at all
			u := capacityUI(t, 2)
			u.DisablePricing = tc.noPricing
			if tc.pending {
				u.cluster.AddPod(NewPod(&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "waiting", Namespace: "default"},
					Status:     v1.PodStatus{Phase: v1.PodPending},
				}))
				u.Refresh()
			}
			u.SetClusterInfo(ClusterInfo{Context: "ctx-a", Server: server, Region: tc.region})
			u.Update(tea.WindowSizeMsg{Width: 200, Height: tc.height})
			if tc.ctrlE {
				u.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
			}
			if u.headerSize() != headerCompact {
				t.Fatalf("header size %v, want compact", u.headerSize())
			}
			line := firstHeaderLine(t, u.View())
			// the counts and cost are the full panel's Nodes row, unchanged
			if line != tc.want {
				t.Errorf("first line %q, want %q", line, tc.want)
			}
			for _, part := range []string{server, "0A1B2C3D4E5F.gr7.us-east-1.eks.amazonaws.com", "https://"} {
				if strings.Contains(line, part) {
					t.Errorf("first line %q contains %q", line, part)
				}
			}
		})
	}
}

// compact-header-no-url R2
func TestFullHeaderKeepsServerURL(t *testing.T) {
	u := capacityUI(t, 2)
	u.SetClusterInfo(ClusterInfo{Context: "ctx-a", Server: "https://a.example", Region: "us-east-1"})
	u.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	if u.headerSize() != headerFull {
		t.Fatalf("header size %v, want full", u.headerSize())
	}
	if full := u.View(); !strings.Contains(full, "Cluster:") || !strings.Contains(full, "https://a.example") {
		t.Errorf("full panel lost the server URL:\n%s", full)
	}
	u.Update(tea.WindowSizeMsg{Width: 200, Height: 10})
	if first := strings.SplitN(u.View(), "\n", 2)[0]; strings.Contains(first, "a.example") {
		t.Errorf("one-line header shows the server URL: %q", first)
	}
}

// compact-header-no-url R1
func TestCompactHeaderNarrowAndIssue(t *testing.T) {
	u := capacityUI(t, 2)
	u.SetClusterInfo(ClusterInfo{Context: "ctx-a", Server: "https://0A1B2C3D4E5F.gr7.us-east-1.eks.amazonaws.com", Region: "us-east-1"})

	// a normal 80-column window: the line fits, starting with context and region
	u.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	view := u.View()
	if w := lipgloss.Width(strings.Split(view, "\n")[1]); w > 80 {
		t.Errorf("first header line is %d wide on 80 columns", w)
	}
	if line := firstHeaderLine(t, view); !strings.HasPrefix(line, "ctx-a · us-east-1 · ") {
		t.Errorf("first line %q", line)
	}

	// a recent error adds a third line to the box, unchanged
	u.SetIssues(func() (string, time.Time) { return "something failed", time.Now() }, "/tmp/log")
	u.Update(tea.WindowSizeMsg{Width: 200, Height: 20})
	lines := strings.Split(u.View(), "\n")
	if got := strings.TrimSpace(strings.Trim(lines[3], "│")); got != "⚠ something failed" {
		t.Errorf("third box line %q, want the warning", got)
	}
}

const demoARN = "arn:aws:eks:us-east-1:111122223333:cluster/demo-prod"

// short-arn-context R1 R2
func TestShortContextName(t *testing.T) {
	long := strings.Repeat("a", 100)
	for _, tc := range []struct{ in, want string }{
		// R1: EKS cluster ARNs show their cluster name
		{demoARN, "demo-prod"},
		{"arn:aws-cn:eks:cn-north-1:111122223333:cluster/demo", "demo"},
		{"arn:aws-us-gov:eks:us-gov-west-1:111122223333:cluster/demo", "demo"},
		{"arn:aws-iso-b:eks:us-isob-east-1:111122223333:cluster/demo", "demo"},
		{"arn:aws-eusc:eks:eusc-de-east-1:111122223333:cluster/demo", "demo"},
		{"arn:aws:eks:us-east-1:111122223333:cluster/my_cluster-2", "my_cluster-2"},
		{"arn:aws:eks:us-east-1:111122223333:cluster/x", "x"},
		{"arn:aws:eks:us-east-1:111122223333:cluster/" + long, long},
		// R2: anything else is shown as it is
		{"demo-prod", "demo-prod"},
		{"kind-knv", "kind-knv"},
		{"", ""},
		{"arn:aws:iam::111122223333:role/demo", "arn:aws:iam::111122223333:role/demo"},
		{"arn:aws:eks:us-east-1:111122223333:nodegroup/demo/ng/1", "arn:aws:eks:us-east-1:111122223333:nodegroup/demo/ng/1"},
		{"arn:aws:eks:us-east-1:11112222333:cluster/demo", "arn:aws:eks:us-east-1:11112222333:cluster/demo"},
		{"arn:aws:eks:us-east-1:1111222233334:cluster/demo", "arn:aws:eks:us-east-1:1111222233334:cluster/demo"},
		{"arn:aws:eks:us-east-1:111122223333:cluster/", "arn:aws:eks:us-east-1:111122223333:cluster/"},
		{"arn:aws:eks:us-east-1:111122223333:cluster/" + long + "a", "arn:aws:eks:us-east-1:111122223333:cluster/" + long + "a"},
		{"arn:aws:eks:us-east-1:111122223333:cluster/a/b", "arn:aws:eks:us-east-1:111122223333:cluster/a/b"},
		{"arn:aws:eks:us-east-1:111122223333:cluster/a:b", "arn:aws:eks:us-east-1:111122223333:cluster/a:b"},
		{"arn:aws:eks:us-east-1:111122223333:cluster/a b", "arn:aws:eks:us-east-1:111122223333:cluster/a b"},
		{"arn:aws:eks:us-east-1:111122223333:cluster/-demo", "arn:aws:eks:us-east-1:111122223333:cluster/-demo"},
		{"arn:aws:eks::111122223333:cluster/demo", "arn:aws:eks::111122223333:cluster/demo"},
		{"arn:aws:eks:us-east-1::cluster/demo", "arn:aws:eks:us-east-1::cluster/demo"},
		{"x arn:aws:eks:us-east-1:111122223333:cluster/demo", "x arn:aws:eks:us-east-1:111122223333:cluster/demo"},
		{demoARN + " ", demoARN + " "},
		{"arn:azure:eks:us-east-1:111122223333:cluster/demo", "arn:azure:eks:us-east-1:111122223333:cluster/demo"},
		{"arn:aws:ecs:us-east-1:111122223333:cluster/demo", "arn:aws:ecs:us-east-1:111122223333:cluster/demo"},
		{"arn:aws:eks:us-east-1:111122223333:addon/demo", "arn:aws:eks:us-east-1:111122223333:addon/demo"},
		{"ARN:AWS:EKS:US-EAST-1:111122223333:cluster/demo", "ARN:AWS:EKS:US-EAST-1:111122223333:cluster/demo"},
		{"arn:aws:eks:US-EAST-1:111122223333:cluster/demo", "arn:aws:eks:US-EAST-1:111122223333:cluster/demo"},
		{"arn:aws:eks:useast-1:111122223333:cluster/demo", "arn:aws:eks:useast-1:111122223333:cluster/demo"},
		{"arn:aws:eks:us-east-1:111122223333:cluster/_demo", "arn:aws:eks:us-east-1:111122223333:cluster/_demo"},
		{demoARN + "\n", demoARN + "\n"},
		{"arn:aws:eks:us-east-1:111122223333:cluster/d\u0435mo", "arn:aws:eks:us-east-1:111122223333:cluster/d\u0435mo"},
	} {
		if got := shortContext(tc.in); got != tc.want {
			t.Errorf("shortContext(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

const gkeDemo = "gke_example-project_us-central1-a_demo-prod"

// headerOf is the view above the node list: nodes of equal age have no fixed
// order, so two renders can differ below it.
func headerOf(view string) string { return strings.SplitN(view, "NAME", 2)[0] }

// short-gke-context R1 R2
func TestShortGKEContextName(t *testing.T) {
	// boundary literals content.sh would flag are built by concatenation
	gke := func(project string) string { return "gke_" + project + "_us-central1-a_demo" }
	at := func(location string) string { return "gke_example-project_" + location + "_demo" }
	named := func(cluster string) string { return "gke_example-project_us-central1-a_" + cluster }
	for _, tc := range []struct{ in, want string }{
		// R1: the cluster name
		{gkeDemo, "demo-prod"},
		{at("us-central1"), "demo"},
		{at("europe-west4-b"), "demo"},
		{at("northamerica-northeast1"), "demo"},
		{at("us-central1-ai1a"), "demo"},
		{gke("abcdef"), "demo"},                      // project of 6
		{gke("a" + strings.Repeat("b", 29)), "demo"}, // project of 30
		{named("a"), "a"},
		{named("a" + strings.Repeat("b", 39)), "a" + strings.Repeat("b", 39)}, // cluster of 40
		{"gke_example.com:example-project_us-central1-a_demo", "demo"},
		// R2: anything else is shown as it is
		{gke("abcde"), ""},                       // project of 5
		{gke("a" + strings.Repeat("b", 30)), ""}, // project of 31
		{gke("1example-project"), ""},
		{gke("example-project-"), ""},
		{gke("Example-Project"), ""},
		{at("uscentral1"), ""},
		{at("-central1"), ""},
		{at("us-central"), ""},
		{at("u-france-east1"), ""},
		{at("US-CENTRAL1"), ""},
		{named(""), ""},
		{named("a" + strings.Repeat("b", 40)), ""}, // cluster of 41
		{named("1demo"), ""},
		{named("demo-"), ""},
		{named("Demo"), ""},
		{named("my_demo"), ""},   // an extra field
		{named("d\u0435mo"), ""}, // Cyrillic e
		{"gke_example-project_demo", ""},
		{"gke_" + "aws_example-project_us-west1_demo", ""}, // GKE multicloud
		{"connectgateway_" + "example-project_us-central1_demo", ""},
		{gkeDemo + "\n", ""},
		{gkeDemo + " ", ""},
		{"x " + gkeDemo, ""},
		{"gke_" + "examplecom:example-project_us-central1-a_demo", ""},   // no dot in the domain
		{"gke_" + "example..com:example-project_us-central1-a_demo", ""}, // an empty label
		{"gke_" + ".com:example-project_us-central1-a_demo", ""},
		{"gke_example-project_us-central1-a_demo_extra", ""},
	} {
		want := tc.want
		if want == "" {
			want = tc.in
		}
		if got := shortContext(tc.in); got != want {
			t.Errorf("shortContext(%q) = %q, want %q", tc.in, got, want)
		}
	}
}

// short-gke-context R1
func TestHeadersShowTheShortGKEName(t *testing.T) {
	for _, tc := range []struct {
		name   string
		height int
		ctrlE  bool
	}{
		{"full panel", 40, false},
		{"compact box", 40, true},
		{"one line", 10, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			render := func(context string) string {
				u := capacityUI(t, 2)
				// a host unrelated to the project's domain: example.com in
				// the output could only come from the context
				u.SetClusterInfo(ClusterInfo{Context: context, Server: "https://a.example", Region: "us-central1"})
				u.Update(tea.WindowSizeMsg{Width: 200, Height: tc.height})
				if tc.ctrlE {
					u.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
				}
				return u.View()
			}
			for _, context := range []string{gkeDemo, "gke_example.com:example-project_us-central1-a_demo-prod"} {
				got := render(context)
				if !strings.Contains(got, "demo-prod") || strings.Contains(got, "example-project") ||
					strings.Contains(got, "example.com") || strings.Contains(got, "gke_") {
					t.Errorf("%s header:\n%s", context, got)
				}
				if plain := render("demo-prod"); headerOf(plain) != headerOf(got) {
					t.Errorf("plain-name header differs:\n%s\nwant\n%s", headerOf(plain), headerOf(got))
				}
			}
		})
	}
}

// short-gke-context R4
func TestShortGKENameOnlyInTheHeaders(t *testing.T) {
	u := capacityUI(t, 2)
	u.SetClusterInfo(ClusterInfo{Context: gkeDemo, Server: "https://a.example"})
	u.Update(tea.WindowSizeMsg{Width: 200, Height: 40})

	u.panel = panelContexts
	u.contextsLoaded(contextsMsg{names: []string{gkeDemo, "staging"}, current: gkeDemo})
	rows := strings.Join(u.contextsPanel().rows, "\n")
	if !strings.Contains(rows, gkeDemo) || !strings.Contains(rows, "(current)") {
		t.Errorf("contexts popup rows:\n%s", rows)
	}
	u.contexts.switching = gkeDemo
	if rows := strings.Join(u.contextsPanel().rows, "\n"); !strings.Contains(rows, "connecting to "+gkeDemo) {
		t.Errorf("switching row:\n%s", rows)
	}
	if doc := u.exportDoc(time.Now()); doc.Context != gkeDemo {
		t.Errorf("export context %q, want the full name", doc.Context)
	}
}

// short-gke-context R1: the GKE location is not the pricing region
func TestGKELocationIsNotThePricingRegion(t *testing.T) {
	for _, ctrlE := range []bool{false, true} {
		u := newTestUI(t)
		u.SetClusterInfo(ClusterInfo{Context: gkeDemo, Server: "https://a.example"})
		u.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
		if ctrlE {
			u.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
		}
		h := u.View()
		if strings.Contains(h, "us-central1") {
			t.Errorf("ctrl+e %v: header shows the GKE location:\n%s", ctrlE, h)
		}
		if !ctrlE && !strings.Contains(h, "Region:   -") {
			t.Errorf("full panel Region row is not -:\n%s", h)
		}
	}
}

// short-arn-context R1 R2
func TestHeadersShowTheShortContextName(t *testing.T) {
	for _, tc := range []struct {
		name   string
		height int
		ctrlE  bool
		size   headerSize
	}{
		{"full panel", 40, false, headerFull},
		{"compact box", 40, true, headerCompact},
		{"one line", 10, false, headerLine},
	} {
		t.Run(tc.name, func(t *testing.T) {
			render := func(context string) string {
				u := capacityUI(t, 2)
				u.SetClusterInfo(ClusterInfo{Context: context, Server: "https://example.com", Region: "us-east-1"})
				u.Update(tea.WindowSizeMsg{Width: 200, Height: tc.height})
				if tc.ctrlE {
					u.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
				}
				if u.headerSize() != tc.size {
					t.Fatalf("header size %v, want %v", u.headerSize(), tc.size)
				}
				return u.View()
			}
			arn := render(demoARN)
			if !strings.Contains(arn, "demo-prod") || strings.Contains(arn, "111122223333") || strings.Contains(arn, "arn:") {
				t.Errorf("ARN context header:\n%s", arn)
			}
			// a plain name renders exactly as a short ARN does
			if plain := render("demo-prod"); headerOf(plain) != headerOf(arn) {
				t.Errorf("plain-name header differs:\n%s\nwant\n%s", headerOf(plain), headerOf(arn))
			}
		})
	}
}

// short-arn-context R3
func TestShortNameOnlyInTheHeaders(t *testing.T) {
	u := capacityUI(t, 2)
	u.SetClusterInfo(ClusterInfo{Context: demoARN, Server: "https://example.com"})
	u.Update(tea.WindowSizeMsg{Width: 200, Height: 40})

	u.panel = panelContexts
	u.contextsLoaded(contextsMsg{names: []string{demoARN, "staging"}, current: demoARN})
	rows := strings.Join(u.contextsPanel().rows, "\n")
	if !strings.Contains(rows, demoARN) || !strings.Contains(rows, "(current)") {
		t.Errorf("contexts popup rows:\n%s", rows)
	}

	u.contexts.switching = demoARN
	if rows := strings.Join(u.contextsPanel().rows, "\n"); !strings.Contains(rows, "connecting to "+demoARN) {
		t.Errorf("switching row:\n%s", rows)
	}
	u.switched(switchedMsg{context: demoARN, err: errors.New("unreachable")})
	if rows := strings.Join(u.contextsPanel().rows, "\n"); !strings.Contains(rows, demoARN+": unreachable") {
		t.Errorf("switch error row:\n%s", rows)
	}

	if doc := u.exportDoc(time.Now()); doc.Context != demoARN {
		t.Errorf("export context %q, want the full ARN", doc.Context)
	}
}

// short-arn-context R1: the ARN's region is not the pricing region
func TestARNRegionIsNotThePricingRegion(t *testing.T) {
	const arn = "arn:aws:eks:eu-west-1:111122223333:cluster/demo"
	for _, ctrlE := range []bool{false, true} {
		u := newTestUI(t)
		u.SetClusterInfo(ClusterInfo{Context: arn, Server: "https://example.com"})
		u.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
		if ctrlE {
			u.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
		}
		h := u.View()
		if strings.Contains(h, "eu-west-1") {
			t.Errorf("ctrl+e %v: header shows the ARN's region:\n%s", ctrlE, h)
		}
		if !ctrlE && !strings.Contains(h, "Region:   -") {
			t.Errorf("full panel Region row is not -:\n%s", h)
		}
	}
}
