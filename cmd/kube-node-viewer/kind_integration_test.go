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
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/staffops/kube-node-viewer/pkg/client"
	"github.com/staffops/kube-node-viewer/pkg/model"
	"github.com/staffops/kube-node-viewer/pkg/state"
)

// TestKindCluster connects to the cluster of scripts/kind-it.sh through the
// same sessions the binary uses (client, informers, store, view), without a
// terminal, and checks the screen against what the API server lists:
//
//	scripts/kind-it.sh   # creates the cluster, runs this test, deletes it
//
// KNV_IT_KUBECONFIG names the kubeconfig and KNV_IT_CONTEXT its context; the
// test is skipped without them.
func TestKindCluster(t *testing.T) {
	kubeconfig, kubeContext := os.Getenv("KNV_IT_KUBECONFIG"), os.Getenv("KNV_IT_CONTEXT")
	if kubeconfig == "" || kubeContext == "" {
		t.Skip("set KNV_IT_KUBECONFIG and KNV_IT_CONTEXT (scripts/kind-it.sh does)")
	}

	// what the API server says, read the way the binary does not
	cs, err := client.NewKubernetes(kubeconfig, kubeContext)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	nodes, err := cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	pods, err := cs.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	podsOn := map[string]int{}
	total := 0
	for _, p := range pods.Items {
		if p.Status.Phase == v1.PodSucceeded || p.Status.Phase == v1.PodFailed {
			continue
		}
		total++
		if p.Spec.NodeName != "" {
			podsOn[p.Spec.NodeName]++
		}
	}
	if len(nodes.Items) != 3 {
		t.Fatalf("want the 3 nodes of testdata/kind/cluster.yaml, the API lists %d", len(nodes.Items))
	}

	style, err := model.ParseStyle("#04B575,#FFFF00,#FF0000")
	if err != nil {
		t.Fatal(err)
	}
	m := model.NewUIModel(style)
	m.DisablePricing = true
	program := tea.NewProgram(m, tea.WithInput(nil), tea.WithOutput(io.Discard),
		tea.WithoutRenderer(), tea.WithoutSignalHandler())
	dir := t.TempDir()
	states := state.Load(filepath.Join(dir, "state.json"), io.Discard)
	saver := state.NewSaver(states, time.Millisecond, func(err error) { t.Error(err) })
	defer saver.Close()
	conn := &sessions{
		flags:        Flags{Kubeconfig: kubeconfig, Context: kubeContext, DisablePricing: true},
		nodeSelector: labels.Everything(),
		states:       states,
		saver:        saver,
		program:      program,
		starts:       newStartViews(nil, false),
	}
	defer conn.Close()
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

	// the informers fill the store; wait until it holds what the API lists
	deadline := time.Now().Add(time.Minute)
	for {
		st := first.Cluster.Stats()
		if st.NumNodes == len(nodes.Items) && st.TotalPods == total {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("store has %d nodes and %d pods, the API %d and %d", st.NumNodes, st.TotalPods, len(nodes.Items), total)
		}
		time.Sleep(100 * time.Millisecond)
	}
	program.Quit()
	u := (<-done).(*model.UIModel)
	// the last change may still be on its way to the stopped program
	if _, cmd := u.Update(model.ChangedMsg{Gen: first.Cluster.Generation()}); cmd != nil {
		u.Update(cmd())
	}
	screen := u.View()
	t.Logf("screen:\n%s", screen)

	if want := fmt.Sprintf("Nodes:    %d of %d shown · %d pods", len(nodes.Items), len(nodes.Items), total); !strings.Contains(screen, want) {
		t.Errorf("header: want %q", want)
	}
	for _, n := range nodes.Items {
		row := lineWith(screen, n.Name+" ")
		if row == "" {
			t.Errorf("node %s is not on the screen", n.Name)
			continue
		}
		maxPods := n.Status.Allocatable.Pods().Value()
		if want := regexp.MustCompile(fmt.Sprintf(`\s%d/%d\s`, podsOn[n.Name], maxPods)); !want.MatchString(row) {
			t.Errorf("node %s: want %d/%d pods in its row:\n%s", n.Name, podsOn[n.Name], maxPods, row)
		}
		if pool := n.Labels["eks.amazonaws.com/nodegroup"]; pool != "" && !strings.Contains(row, pool) {
			t.Errorf("node %s: want node group %s in its row:\n%s", n.Name, pool, row)
		}
		if zone := n.Labels[v1.LabelTopologyZone]; zone != "" && !strings.Contains(row, zone) {
			t.Errorf("node %s: want zone %s in its row:\n%s", n.Name, zone, row)
		}
	}
	if podsOn[nodes.Items[0].Name]+podsOn[nodes.Items[1].Name]+podsOn[nodes.Items[2].Name] < 6 {
		t.Errorf("want the 6 pods of testdata/kind/workload.yaml on the nodes, got %v", podsOn)
	}
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
