//go:build integration

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
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	helperResource "k8s.io/component-helpers/resource"
	"sigs.k8s.io/yaml"

	"github.com/staffops/kube-node-viewer/pkg/client"
	"github.com/staffops/kube-node-viewer/pkg/model"
	"github.com/staffops/kube-node-viewer/pkg/state"
)

const (
	// fixtureNamespace and fixturePods are testdata/kind/workload.yaml's
	fixtureNamespace = "knv-it"
	fixturePods      = 6
)

// fixtureLabels are the worker labels of testdata/kind/cluster.yaml, by the
// node name kind gives each worker.
var fixtureLabels = map[string]map[string]string{
	"-worker":  {"eks.amazonaws.com/nodegroup": "it-pool-a", v1.LabelTopologyZone: "it-zone-a"},
	"-worker2": {"eks.amazonaws.com/nodegroup": "it-pool-b", v1.LabelTopologyZone: "it-zone-b"},
}

// TestKindCluster connects to the cluster of scripts/kind-it.sh through the
// same sessions the binary uses (client, informers, store, view), without a
// terminal, and checks the screen against what the API server lists:
//
//	scripts/kind-it.sh   # creates the cluster, runs this test, deletes it
//
// KNV_IT_KUBECONFIG names the kubeconfig and KNV_IT_CONTEXT its context;
// without them the test is skipped, or fails when CI is set.
func TestKindCluster(t *testing.T) {
	kubeconfig, kubeContext := os.Getenv("KNV_IT_KUBECONFIG"), os.Getenv("KNV_IT_CONTEXT")
	if kubeconfig == "" || kubeContext == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("KNV_IT_KUBECONFIG and KNV_IT_CONTEXT are not set (scripts/kind-it.sh sets them)")
		}
		t.Skip("set KNV_IT_KUBECONFIG and KNV_IT_CONTEXT (scripts/kind-it.sh does)")
	}
	cs, err := client.NewKubernetes(kubeconfig, kubeContext)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	applyWorkload(ctx, t, cs)

	// what the API server says, read directly
	nodes, err := cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes.Items) != 3 {
		t.Fatalf("want the 3 nodes of testdata/kind/cluster.yaml, the API lists %d", len(nodes.Items))
	}
	checkFixtureLabels(t, nodes.Items)
	pods, err := cs.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	podsOn := map[string]int{}
	cpuOn := map[string]int64{} // milli-CPU requested, as the store sums it
	fixtureOn := map[string][]string{}
	total, fixture := 0, 0
	var cpuUsed int64
	for i := range pods.Items {
		p := &pods.Items[i]
		if p.Status.Phase == v1.PodSucceeded || p.Status.Phase == v1.PodFailed {
			continue
		}
		total++
		if p.Spec.NodeName == "" {
			continue
		}
		podsOn[p.Spec.NodeName]++
		req := helperResource.PodRequests(p, helperResource.PodResourcesOptions{})
		cpu := req[v1.ResourceCPU]
		cpuOn[p.Spec.NodeName] += cpu.MilliValue()
		cpuUsed += cpu.MilliValue()
		if p.Namespace == fixtureNamespace {
			fixture++
			fixtureOn[p.Spec.NodeName] = append(fixtureOn[p.Spec.NodeName], p.Name)
		}
	}
	if fixture != fixturePods {
		t.Fatalf("want the %d pods of testdata/kind/workload.yaml on nodes, the API lists %d", fixturePods, fixture)
	}

	u, gen := connect(t, kubeconfig, kubeContext, len(nodes.Items), total)
	if _, cmd := u.Update(model.ChangedMsg{Gen: gen}); cmd != nil {
		u.Update(cmd())
	}
	screen := u.View()
	t.Logf("screen:\n%s", screen)

	if want := fmt.Sprintf("Nodes:    %d of %d shown · %d pods", len(nodes.Items), len(nodes.Items), total); !strings.Contains(screen, want) {
		t.Errorf("header: want %q", want)
	}
	checkSummary(t, screen, nodes.Items, cpuUsed)

	// rows in screen order, so the cursor can reach each node below
	order := make([]v1.Node, len(nodes.Items))
	copy(order, nodes.Items)
	sort.Slice(order, func(i, j int) bool {
		return strings.Index(screen, order[i].Name+" ") < strings.Index(screen, order[j].Name+" ")
	})
	for i, n := range order {
		row := lineWith(screen, n.Name+" ")
		if row == "" {
			t.Errorf("node %s is not on the screen", n.Name)
			continue
		}
		maxPods := n.Status.Allocatable.Pods().Value()
		if !regexp.MustCompile(fmt.Sprintf(`\s%d/%d\s`, podsOn[n.Name], maxPods)).MatchString(row) {
			t.Errorf("node %s: want %d/%d pods in its row:\n%s", n.Name, podsOn[n.Name], maxPods, row)
		}
		alloc := n.Status.Allocatable[v1.ResourceCPU]
		pct := fmt.Sprintf("%3.0f%%", 100*float64(cpuOn[n.Name])/float64(alloc.MilliValue()))
		if !strings.Contains(row, pct) {
			t.Errorf("node %s: want cpu %s (%dm of %dm) in its row:\n%s", n.Name, pct, cpuOn[n.Name], alloc.MilliValue(), row)
		}
		for _, key := range []string{"eks.amazonaws.com/nodegroup", v1.LabelTopologyZone} {
			if want := n.Labels[key]; want != "" && !strings.Contains(row, want) {
				t.Errorf("node %s: want %s %s in its row:\n%s", n.Name, key, want, row)
			}
		}
		// the node's Pods tab lists the fixture pods scheduled there
		u.View()
		pressKeys(u, append(repeat(tea.KeyDown, i), tea.KeyEnter, tea.KeyTab)...)
		tab := u.View()
		for _, name := range fixtureOn[n.Name] {
			if !strings.Contains(tab, name) {
				t.Errorf("node %s: pod %s/%s is not in its Pods tab:\n%s", n.Name, fixtureNamespace, name, tab)
			}
		}
		pressKeys(u, tea.KeyEsc)
		pressKeys(u, repeat(tea.KeyUp, i)...)
	}
}

// connect runs the binary's sessions against the cluster in a headless
// program until the store holds what the API lists, and returns the model
// once the program has stopped, with the store's generation.
func connect(t *testing.T, kubeconfig, kubeContext string, wantNodes, wantPods int) (*model.UIModel, uint64) {
	t.Helper()
	style, err := model.ParseStyle("#04B575,#FFFF00,#FF0000")
	if err != nil {
		t.Fatal(err)
	}
	m := model.NewUIModel(style)
	m.DisablePricing = true
	program := tea.NewProgram(m, tea.WithInput(nil), tea.WithOutput(io.Discard),
		tea.WithoutRenderer(), tea.WithoutSignalHandler())
	states := state.Load(filepath.Join(t.TempDir(), "state.json"), io.Discard)
	saver := state.NewSaver(states, time.Millisecond, func(err error) { t.Error(err) })
	t.Cleanup(saver.Close)
	conn := &sessions{
		flags:        Flags{Kubeconfig: kubeconfig, Context: kubeContext, DisablePricing: true},
		nodeSelector: labels.Everything(),
		states:       states,
		saver:        saver,
		program:      program,
		starts:       newStartViews(nil, false),
	}
	t.Cleanup(conn.Close)
	first, err := conn.Switch(kubeContext)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	m.Connect(first)

	done := make(chan tea.Model, 1)
	go func() {
		final, err := program.Run()
		if err != nil {
			t.Error(err)
		}
		done <- final
	}()
	program.Send(tea.WindowSizeMsg{Width: 200, Height: 60})

	deadline := time.Now().Add(time.Minute)
	for {
		st := first.Cluster.Stats()
		if st.NumNodes == wantNodes && st.TotalPods == wantPods {
			break
		}
		if time.Now().After(deadline) {
			program.Quit()
			t.Fatalf("store has %d nodes and %d pods, the API %d and %d", st.NumNodes, st.TotalPods, wantNodes, wantPods)
		}
		time.Sleep(100 * time.Millisecond)
	}
	program.Quit()
	u, ok := (<-done).(*model.UIModel)
	if !ok {
		t.Fatal("the program did not return the *model.UIModel it ran")
	}
	// the last change may still be on its way to the stopped program: the
	// caller applies it with the generation
	return u, first.Cluster.Generation()
}

// applyWorkload creates testdata/kind/workload.yaml's objects and waits for
// its pods to run.
func applyWorkload(ctx context.Context, t *testing.T, cs *kubernetes.Clientset) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "kind", "workload.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, doc := range bytes.Split(raw, []byte("\n---\n")) {
		var meta metav1.TypeMeta
		if err := yaml.Unmarshal(doc, &meta); err != nil {
			t.Fatal(err)
		}
		switch meta.Kind {
		case "Namespace":
			var ns v1.Namespace
			if err := yaml.UnmarshalStrict(doc, &ns); err != nil {
				t.Fatal(err)
			}
			if _, err := cs.CoreV1().Namespaces().Create(ctx, &ns, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
				t.Fatal(err)
			}
		case "Deployment":
			var d appsv1.Deployment
			if err := yaml.UnmarshalStrict(doc, &d); err != nil {
				t.Fatal(err)
			}
			if _, err := cs.AppsV1().Deployments(d.Namespace).Create(ctx, &d, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
				t.Fatal(err)
			}
		default:
			t.Fatalf("workload.yaml: unexpected kind %q", meta.Kind)
		}
	}
	for {
		pods, err := cs.CoreV1().Pods(fixtureNamespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			t.Fatal(err)
		}
		running := 0
		for _, p := range pods.Items {
			if p.Status.Phase == v1.PodRunning && p.Spec.NodeName != "" && p.DeletionTimestamp == nil {
				running++
			}
		}
		if running == fixturePods && len(pods.Items) == fixturePods {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("%d of %d fixture pods running: %v", running, fixturePods, ctx.Err())
		case <-time.After(time.Second):
		}
	}
}

// checkFixtureLabels fails unless the two workers carry exactly the labels of
// testdata/kind/cluster.yaml, so the row checks below do not pass on empty
// labels.
func checkFixtureLabels(t *testing.T, nodes []v1.Node) {
	t.Helper()
	found := 0
	for _, n := range nodes {
		for suffix, want := range fixtureLabels {
			if !strings.HasSuffix(n.Name, suffix) {
				continue
			}
			found++
			for k, v := range want {
				if n.Labels[k] != v {
					t.Fatalf("node %s: label %s is %q, want %q (testdata/kind/cluster.yaml)", n.Name, k, n.Labels[k], v)
				}
			}
		}
	}
	if found != len(fixtureLabels) {
		t.Fatalf("want %d workers named *-worker and *-worker2, found %d", len(fixtureLabels), found)
	}
}

// checkSummary checks the cluster cpu line: requested and allocatable cpu and
// their percentage.
func checkSummary(t *testing.T, screen string, nodes []v1.Node, cpuUsed int64) {
	t.Helper()
	var alloc resource.Quantity
	for _, n := range nodes {
		alloc.Add(n.Status.Allocatable[v1.ResourceCPU])
	}
	m := regexp.MustCompile(`(\d+)m/(\S+)\s+([\d.]+)% cpu`).FindStringSubmatch(screen)
	if m == nil {
		t.Errorf("no cpu summary line on the screen")
		return
	}
	if want := fmt.Sprint(cpuUsed); m[1] != want {
		t.Errorf("cpu summary: requested %sm, want %sm", m[1], want)
	}
	if q, err := resource.ParseQuantity(m[2]); err != nil || q.Cmp(alloc) != 0 {
		t.Errorf("cpu summary: allocatable %s, want %s", m[2], alloc.String())
	}
	if want := fmt.Sprintf("%.1f", 100*float64(cpuUsed)/float64(alloc.MilliValue())); m[3] != want {
		t.Errorf("cpu summary: %s%%, want %s%%", m[3], want)
	}
}

func pressKeys(u *model.UIModel, types ...tea.KeyType) {
	for _, k := range types {
		u.Update(tea.KeyMsg{Type: k})
	}
}

func repeat(k tea.KeyType, n int) []tea.KeyType {
	out := make([]tea.KeyType, n)
	for i := range out {
		out[i] = k
	}
	return out
}

// lineWith returns the first line of s that contains sub, or "".
func lineWith(s, sub string) string {
	for _, l := range strings.Split(s, "\n") {
		if strings.Contains(l, sub) {
			return l
		}
	}
	return ""
}
