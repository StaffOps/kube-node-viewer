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

// Modified by the kube-node-viewer authors, 2026.
package main

import (
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	tea "github.com/charmbracelet/bubbletea"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/staffops/kube-node-viewer/pkg/aws"
	"github.com/staffops/kube-node-viewer/pkg/client"
	"github.com/staffops/kube-node-viewer/pkg/model"
	"github.com/staffops/kube-node-viewer/pkg/state"
)

//go:generate cp -r ../../ATTRIBUTION.md ./
//go:embed ATTRIBUTION.md
var attribution string

func main() {
	flags, err := ParseFlags()
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		log.Fatalf("cannot parse flags: %v", err)
	}

	if flags.ShowAttribution {
		fmt.Println(attribution)
		os.Exit(0)
	}

	if flags.Version {
		info, ok := debug.ReadBuildInfo()
		version, commit := resolveVersion(version, commit, info, ok)
		fmt.Printf("kube-node-viewer version %s\n", version)
		fmt.Printf("commit: %s\n", commit)
		fmt.Printf("built at: %s\n", date)
		fmt.Printf("built by: %s\n", builtBy)
		os.Exit(0)
	}

	cs, err := client.NewKubernetes(flags.Kubeconfig, flags.Context)
	if err != nil {
		log.Fatalf("creating client, %s", err)
	}
	nodeClaimClient, err := client.NewNodeClaims(flags.Kubeconfig, flags.Context)
	if err != nil {
		log.Fatalf("creating node claim client, %s", err)
	}
	ctx, cancel := context.WithCancel(context.Background())

	pprov := aws.NewStaticPricingProvider()
	style, err := model.ParseStyle(flags.Style)
	if err != nil {
		log.Fatalf("creating style, %s", err)
	}
	clusterKey, err := client.ClusterKey(flags.Kubeconfig, flags.Context)
	if err != nil {
		log.Fatalf("reading kubeconfig, %s", err)
	}
	// a bad state file is reported here, before the TUI takes the screen
	states := state.Load(state.DefaultPath(homeDir), os.Stderr)
	var saved *model.ViewState
	if v, ok := states.Get(clusterKey); ok {
		saved = &v
	}
	view, err := resolveView(flags.Config, saved, flags.Explicit)
	if err != nil {
		log.Fatalf("%s", err)
	}

	m := model.NewUIModel(style)
	m.DisablePricing = flags.DisablePricing
	m.SetView(view)
	saver := state.NewSaver(states, stateSaveDelay, func(error) {
		// the TUI has the screen; a failed save only loses the last choice
	})
	defer saver.Close()
	m.OnViewChange(func(v model.ViewState) { saver.Save(clusterKey, v) })

	nodeSelector, err := labels.Parse(flags.NodeSelector)
	if err != nil {
		log.Fatalf("parsing node selector: %s", err)
	}

	if !flags.DisablePricing {
		// Use AWS SDK Go v2 for configuration
		cfg, err := config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile(""))
		if err != nil {
			log.Fatalf("unable to load AWS SDK config: %s", err)
		}
		pprov = aws.NewPricingProvider(ctx, cfg)
	}
	controller := client.NewController(cs, nodeClaimClient, m, nodeSelector, pprov)

	// The program exists before the watches start: Send blocks until it runs,
	// and Start makes a blocking API call, so it runs in its own goroutine.
	program := tea.NewProgram(m, tea.WithAltScreen())
	cluster := m.Cluster()
	controller.SetNotifier(client.NewNotifier(ctx, changeInterval, func() {
		program.Send(model.ChangedMsg{Gen: cluster.Generation()})
	}))
	go controller.Start(ctx)

	_, err = program.Run()
	cancel()
	if err != nil {
		saver.Close()
		log.Fatalf("error running tea: %s", err)
	}
}

// stateSaveDelay batches view changes made in quick succession into one write.
const stateSaveDelay = 500 * time.Millisecond

// changeInterval is the most often cluster changes redraw the screen.
const changeInterval = 100 * time.Millisecond

// labelFilter is a --group-by value: 'label' or 'label=a,b,c'.
type labelFilter struct {
	label  string
	values []string
}

func parseLabelFilter(s string) (labelFilter, error) {
	label, rawValues, _ := strings.Cut(s, "=")
	filter := labelFilter{label: strings.TrimSpace(label)}
	if filter.label == "" {
		return labelFilter{}, fmt.Errorf("parsing %q: no label name given", s)
	}
	for _, v := range strings.Split(rawValues, ",") {
		if v = strings.TrimSpace(v); v != "" {
			filter.values = append(filter.values, v)
		}
	}
	return filter, nil
}
