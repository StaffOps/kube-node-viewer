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
	"github.com/staffops/kube-node-viewer/pkg/azure"
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
	starts   *startViews
}

// startViews picks, per connection, the start view of config.yaml
// (config-views R4, R5, D2). In default-view mode a cluster opens with the
// default view the first time the run connects to it; in last-view mode only
// when it has no saved view. A cluster opened with the default view comes
// back, with c, as it was left in this run.
type startViews struct {
	// view is the default view, nil without one
	view     *model.ViewState
	lastView bool

	mu        sync.Mutex
	connected map[string]bool
	// run is the view of each cluster opened with the default view, as it
	// is now, by cluster key
	run map[string]model.ViewState
}

func newStartViews(view *model.ViewState, lastView bool) *startViews {
	return &startViews{view: view, lastView: lastView, connected: map[string]bool{}, run: map[string]model.ViewState{}}
}

// pick returns the saved view and the start view to resolve a connection to
// the cluster from; either may be nil.
func (p *startViews) pick(key string, saved *model.ViewState) (base, start *model.ViewState) {
	if p == nil || p.view == nil {
		return saved, nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if v, ok := p.run[key]; ok {
		return &v, nil
	}
	switch {
	case p.lastView && saved == nil:
		return nil, p.view
	case p.lastView:
		return saved, nil
	case !p.connected[key]:
		return saved, p.view
	}
	return saved, nil
}

// opened records a connection made with the view it shows; fromStart is
// whether pick gave a start view.
func (p *startViews) opened(key string, view model.ViewState, fromStart bool) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.connected[key] = true
	if fromStart {
		p.run[key] = view
	}
}

// changed records a view change of a cluster opened with the default view.
func (p *startViews) changed(key string, view model.ViewState) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.run[key]; ok {
		p.run[key] = view
	}
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
	key := cluster.Key()
	var saved *model.ViewState
	if v, ok := s.states.Get(key); ok {
		saved = &v
	}
	saved, start := s.starts.pick(key, saved)
	view, err := resolveView(s.flags.Config, saved, start, explicit)
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

	s.starts.opened(key, view, start != nil)
	return model.Session{
		Cluster: store,
		Info:    info,
		View:    view,
		Save: func(v model.ViewState) {
			s.starts.changed(key, v)
			s.saver.Save(key, v)
		},
		Advisor: newAdvisor(prices, s.flags.DisablePricing),
	}, nil
}

// pricing builds the price provider for a cluster and the region to show.
// EKS API server URLs name their region; for other clusters each AWS node is
// priced in its own region (its region label, or its zone's), and the rest in
// the AWS SDK's configured region (fix-aws-pricing-region); the header shows
// the nodes' region label. AKS clusters make no AWS lookup. Azure nodes are
// priced from the public Retail Prices API, fetched only once one is seen
// (aks-pricing).
func (s *sessions) pricing(ctx context.Context, server string) (pricing.Provider, string, error) {
	region := client.RegionFromServer(server)
	aks := client.IsAKSServer(server)
	if s.flags.DisablePricing {
		return aws.NewStaticPricingProvider(), region, nil
	}
	var awsPrices pricing.Provider
	if aks {
		awsPrices = aws.NewStaticPricingProvider()
	} else {
		cfg, err := config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile(""))
		if err != nil {
			return nil, "", fmt.Errorf("loading AWS SDK config: %w", err)
		}
		if region != "" {
			cfg.Region = region
			awsPrices = aws.NewPricingProvider(ctx, cfg)
		} else {
			awsPrices = aws.NewRegionalPricingProvider(ctx, cfg)
		}
	}
	return pricing.ByCloud{
		Clouds:   map[string]pricing.Provider{"aws": awsPrices, "azure": azure.NewProvider(ctx, azure.Endpoint)},
		Fallback: awsPrices,
	}, region, nil
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

// newAdvisor gives Recommendations an advisor per cloud
// (aks-recommendations D2): AWS when the provider answers list prices, and
// Azure from the size catalog with the Azure provider's candidate prices.
// With --disable-pricing the Azure advisor asks for no price (R3).
func newAdvisor(p pricing.Provider, disablePricing bool) model.Advisor {
	advisors := model.AdvisorsByCloud{}
	awsP, azureP := p, pricing.Provider(nil)
	if b, ok := p.(pricing.ByCloud); ok {
		awsP, azureP = b.Clouds["aws"], b.Clouds["azure"]
	}
	if ap, ok := awsP.(interface {
		OnDemandPrice(ec2types.InstanceType) (float64, bool)
		SpotPriceInZone(ec2types.InstanceType, string) (float64, bool)
	}); ok {
		advisors[recommend.CloudAWS] = awsAdvisor{prices: ap}
	}
	// the Azure provider lists candidate prices once T2 lands; until then
	// the rows that need them are absent
	lister, _ := azureP.(azure.PriceLister)
	advisors[recommend.CloudAzure] = azure.NewAdvisor(lister, disablePricing)
	return advisors
}

func (a awsAdvisor) Spec(t string) (recommend.Spec, bool) {
	s, ok := aws.LookupInstance(t)
	if !ok {
		return recommend.Spec{}, false
	}
	return recommend.Spec{VCPU: s.VCPU, MemGiB: s.MemoryGiB, Arch: s.Arch, Class: s.Class, Current: s.CurrentGeneration}, true
}

func (a awsAdvisor) OnDemand(_ recommend.Node, t string) (float64, bool) {
	return a.prices.OnDemandPrice(ec2types.InstanceType(t))
}

func (a awsAdvisor) SpotIn(n recommend.Node, t string) (float64, bool) {
	return a.prices.SpotPriceInZone(ec2types.InstanceType(t), n.Zone)
}

func (a awsAdvisor) Types() []string { return aws.InstanceTypes() }
