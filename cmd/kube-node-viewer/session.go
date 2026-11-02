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
	"sync"

	"github.com/aws/aws-sdk-go-v2/config"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	tea "github.com/charmbracelet/bubbletea"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/staffops/kube-node-viewer/pkg/aws"
	"github.com/staffops/kube-node-viewer/pkg/client"
	"github.com/staffops/kube-node-viewer/pkg/model"
	"github.com/staffops/kube-node-viewer/pkg/pricing"
	"github.com/staffops/kube-node-viewer/pkg/recommend"
	"github.com/staffops/kube-node-viewer/pkg/state"
)

// sessions connects to one kubeconfig context at a time (R10). Each
// connection gets its own store, watches and pricing, all stopped when the
// next one starts; the UI drops messages from an older store's generation.
type sessions struct {
	flags        Flags
	nodeSelector labels.Selector
	states       *state.Store
	saver        *state.Saver
	program      *tea.Program

	mu     sync.Mutex
	cancel context.CancelFunc
	// explicit holds the command-line flags until the first connection; they
	// apply to the cluster the run started on (R2)
	explicit map[string]string
}

func (s *sessions) Contexts() ([]string, string, error) {
	return client.Contexts(s.flags.Kubeconfig)
}

// Switch connects to the context, then stops the previous connection.
func (s *sessions) Switch(name string) (model.Session, error) {
	cluster, err := client.ResolveCluster(s.flags.Kubeconfig, name)
	if err != nil {
		return model.Session{}, err
	}
	cs, err := client.NewKubernetes(s.flags.Kubeconfig, cluster.Context)
	if err != nil {
		return model.Session{}, fmt.Errorf("creating client: %w", err)
	}
	nodeClaims, err := client.NewNodeClaims(s.flags.Kubeconfig, cluster.Context)
	if err != nil {
		return model.Session{}, fmt.Errorf("creating node claim client: %w", err)
	}

	s.mu.Lock()
	explicit := s.explicit
	s.explicit = nil
	s.mu.Unlock()
	var saved *model.ViewState
	if v, ok := s.states.Get(cluster.Key()); ok {
		saved = &v
	}
	view, err := resolveView(s.flags.Config, saved, explicit)
	if err != nil {
		return model.Session{}, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	info := model.ClusterInfo{Context: cluster.Context, Server: cluster.Server}
	prices, region, err := s.pricing(ctx, cluster.Server)
	if err != nil {
		cancel()
		return model.Session{}, err
	}
	info.Region = region

	store := model.NewCluster()
	gen := store.Generation()
	controller := client.NewController(cs, nodeClaims, store, s.nodeSelector, prices)
	controller.SetNotifier(client.NewNotifier(ctx, changeInterval, func() {
		s.program.Send(model.ChangedMsg{Gen: gen})
	}))

	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	s.cancel = cancel
	s.mu.Unlock()
	// Start makes a blocking API call
	go controller.Start(ctx)

	key := cluster.Key()
	return model.Session{
		Cluster: store,
		Info:    info,
		View:    view,
		Save:    func(v model.ViewState) { s.saver.Save(key, v) },
		Advisor: newAdvisor(prices),
	}, nil
}

// pricing builds the price provider for a cluster. EKS API server URLs name
// their region; other clusters use the AWS SDK's configured region.
func (s *sessions) pricing(ctx context.Context, server string) (pricing.Provider, string, error) {
	region := client.RegionFromServer(server)
	if s.flags.DisablePricing {
		return aws.NewStaticPricingProvider(), region, nil
	}
	cfg, err := config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile(""))
	if err != nil {
		return nil, "", fmt.Errorf("loading AWS SDK config: %w", err)
	}
	if region != "" {
		cfg.Region = region
	}
	return aws.NewPricingProvider(ctx, cfg), cfg.Region, nil
}

// Close stops the current connection.
func (s *sessions) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
}

// awsAdvisor answers the recommendations from the instance catalog and the
// session's prices (recommendations D2).
type awsAdvisor struct {
	prices interface {
		OnDemandPrice(ec2types.InstanceType) (float64, bool)
		SpotPriceInZone(ec2types.InstanceType, string) (float64, bool)
	}
}

// newAdvisor is nil when the provider can't answer list prices.
func newAdvisor(p pricing.Provider) model.Advisor {
	ap, ok := p.(interface {
		OnDemandPrice(ec2types.InstanceType) (float64, bool)
		SpotPriceInZone(ec2types.InstanceType, string) (float64, bool)
	})
	if !ok {
		return nil
	}
	return awsAdvisor{prices: ap}
}

func (a awsAdvisor) Spec(t string) (recommend.Spec, bool) {
	s, ok := aws.LookupInstance(t)
	if !ok {
		return recommend.Spec{}, false
	}
	return recommend.Spec{VCPU: s.VCPU, MemGiB: s.MemoryGiB, Arch: s.Arch, Class: s.Class, Current: s.CurrentGeneration}, true
}

func (a awsAdvisor) OnDemand(t string) (float64, bool) {
	return a.prices.OnDemandPrice(ec2types.InstanceType(t))
}

func (a awsAdvisor) SpotIn(t, zone string) (float64, bool) {
	return a.prices.SpotPriceInZone(ec2types.InstanceType(t), zone)
}

func (a awsAdvisor) Types() []string { return aws.InstanceTypes() }
