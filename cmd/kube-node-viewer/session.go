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
	"net/http"
	"sync"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
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
	// azure replaces az and Resource Manager in tests only; no flag,
	// environment variable or config key sets it (azure-live-skus D6)
	azure azureOption
	// sizes reads the current connection's Azure VM sizes; closed with it
	sizes *azure.Sizes
	// fleet is the Clusters screen's members (clusters-screen D3)
	fleet *fleet
	// contexts is the context lists in effect for the run; zero lists every
	// context (context-visibility D2, D3)
	contexts model.LabelPolicy
}

// azureOption is the Go option of azure-live-skus D6: zero is production
// (az on PATH, https://management.azure.com).
type azureOption struct {
	run       azure.AzRunner
	base      string
	transport http.RoundTripper
	// lister replaces the Azure candidate prices
	lister azure.PriceLister
}

func (o azureOption) sizes(ctx context.Context) *azure.Sizes {
	run, base := o.run, o.base
	if run == nil {
		run = azure.RunAz
	}
	if base == "" {
		base = azure.DefaultARM
	}
	return azure.NewSizes(ctx, run, base, o.transport)
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

// Contexts lists the kubeconfig's contexts the lists show, and the current
// one (context-visibility R1, R2).
func (s *sessions) Contexts() ([]string, string, error) {
	names, current, err := client.Contexts(s.flags.Kubeconfig)
	if err != nil {
		return nil, "", err
	}
	shown := names[:0]
	for _, n := range names {
		if s.contexts.Shows(n) {
			shown = append(shown, n)
		}
	}
	return shown, current, nil
}

// effectivePolicy is the context lists for the run: off (zero, every context
// listed) when they hide the context the run opens on; a run with no context
// name is never hidden (context-visibility R5, R6, D3).
func effectivePolicy(start string, p model.LabelPolicy) (model.LabelPolicy, bool) {
	if start == "" || p.Shows(start) {
		return p, false
	}
	return model.LabelPolicy{}, true
}

// conn is one connection to a context, built but not started
// (clusters-screen T2): Switch and the Clusters screen's members share it.
type conn struct {
	cluster    client.Cluster
	store      *model.Cluster
	controller *client.Controller
	prices     pricing.Provider
	info       model.ClusterInfo
	ctx        context.Context
	cancel     context.CancelFunc
}

// priceFunc builds the price provider of a connection and the region to show.
type priceFunc func(ctx context.Context, server string) (pricing.Provider, string, error)

// connect builds the clients, store, controller and prices of a context;
// notify hears the store's changes. The caller starts the watches.
func (s *sessions) connect(name string, prices priceFunc, notify func(gen uint64)) (*conn, error) {
	cluster, err := client.ResolveCluster(s.flags.Kubeconfig, name)
	if err != nil {
		return nil, err
	}
	cs, err := client.NewKubernetes(s.flags.Kubeconfig, cluster.Context)
	if err != nil {
		return nil, fmt.Errorf("creating client: %w", err)
	}
	nodeClaims, err := client.NewNodeClaims(s.flags.Kubeconfig, cluster.Context)
	if err != nil {
		return nil, fmt.Errorf("creating node claim client: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	p, region, err := prices(ctx, cluster.Server)
	if err != nil {
		cancel()
		return nil, err
	}
	store := model.NewCluster()
	gen := store.Generation()
	controller := client.NewController(cs, nodeClaims, store, s.nodeSelector, p)
	controller.SetNotifier(client.NewNotifier(ctx, changeInterval, func() { notify(gen) }))
	return &conn{
		cluster:    cluster,
		store:      store,
		controller: controller,
		prices:     p,
		info:       model.ClusterInfo{Context: cluster.Context, Server: cluster.Server, Region: region},
		ctx:        ctx,
		cancel:     cancel,
	}, nil
}

// connStatus reads a controller's state for the model.
func connStatus(c *client.Controller) func() model.ConnStatus {
	return func() model.ConnStatus {
		st := c.Status()
		return model.ConnStatus{Synced: st.Synced, PodsSynced: st.PodsSynced, Err: st.Err}
	}
}

// Switch connects to the context, then stops the previous connection.
func (s *sessions) Switch(name string) (model.Session, error) {
	cluster, err := client.ResolveCluster(s.flags.Kubeconfig, name)
	if err != nil {
		return model.Session{}, err
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
	// the flags still apply to the next connection when this one fails (R2)
	keepExplicit := func() {
		s.mu.Lock()
		if s.explicit == nil {
			s.explicit = explicit
		}
		s.mu.Unlock()
	}
	view, err := resolveView(s.flags.Config, saved, start, explicit)
	if err != nil {
		keepExplicit()
		return model.Session{}, err
	}

	c, err := s.connect(name, s.pricing, func(gen uint64) {
		s.program.Send(model.ChangedMsg{Gen: gen})
	})
	if err != nil {
		keepExplicit()
		return model.Session{}, err
	}

	var sizes *azure.Sizes
	if !s.flags.DisablePricing {
		sizes = s.azure.sizes(c.ctx)
	}
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	old := s.sizes
	s.cancel, s.sizes = c.cancel, sizes
	s.mu.Unlock()
	if old != nil {
		old.Close() // a running az of the previous connection is killed
	}
	// Start makes a blocking API call
	go c.controller.Start(c.ctx)

	s.starts.opened(key, view, start != nil)
	return model.Session{
		Cluster: c.store,
		Info:    c.info,
		View:    view,
		Save: func(v model.ViewState) {
			s.starts.changed(key, v)
			s.saver.Save(key, v)
		},
		Advisor: newAdvisor(c.prices, s.flags.DisablePricing, sizes, s.azure.lister),
		Status:  connStatus(c.controller),
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
		Clouds: map[string]pricing.Provider{
			"aws":   awsPrices,
			"azure": azure.NewProvider(ctx, azure.Endpoint),
			// no GCE price source yet: never AWS prices (clusters-screen D4)
			"gce": pricing.None{},
		},
		Fallback: awsPrices,
	}, region, nil
}

// visitPricing is a visit's price set (clusters-screen D8): one AWS provider
// per region, one Azure provider, nothing for GCE, all dropped with the
// visit. Without an AWS config the AWS members are unpriced, not failed.
func (s *sessions) visitPricing(ctx context.Context) priceFunc {
	if s.flags.DisablePricing {
		static := aws.NewStaticPricingProvider()
		return func(context.Context, string) (pricing.Provider, string, error) {
			return static, "", nil
		}
	}
	var mu sync.Mutex
	byRegion := map[string]pricing.Provider{}
	var azureP pricing.Provider
	// loaded by the first AWS member, off the UI goroutine and outside mu
	var cfg awssdk.Config
	var cfgErr error
	var load sync.Once
	return func(_ context.Context, server string) (pricing.Provider, string, error) {
		region := client.RegionFromServer(server)
		if !client.IsAKSServer(server) {
			load.Do(func() { cfg, cfgErr = config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile("")) })
		}
		mu.Lock()
		defer mu.Unlock()
		if azureP == nil {
			azureP = azure.NewProvider(ctx, azure.Endpoint)
		}
		awsP, ok := byRegion[region]
		if !ok {
			switch {
			case cfgErr != nil, client.IsAKSServer(server):
				awsP = pricing.None{}
			case region != "":
				c := cfg.Copy()
				c.Region = region
				awsP = aws.NewPricingProvider(ctx, c)
			default:
				awsP = aws.NewRegionalPricingProvider(ctx, cfg)
			}
			if !client.IsAKSServer(server) {
				byRegion[region] = awsP
			}
		}
		return pricing.ByCloud{
			Clouds:   map[string]pricing.Provider{"aws": awsP, "azure": azureP, "gce": pricing.None{}},
			Fallback: awsP,
		}, region, nil
	}
}

// Close stops the current connection and the Clusters screen's members, and
// kills a running az.
func (s *sessions) Close() {
	if s.fleet != nil {
		s.fleet.Close()
	}
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	sizes := s.sizes
	s.sizes = nil
	s.mu.Unlock()
	if sizes != nil {
		sizes.Close()
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
// Azure from the live VM sizes (azure-live-skus) with the Azure provider's
// candidate prices. With --disable-pricing the Azure advisor asks for no
// price and reads no size (aks-recommendations R3, azure-live-skus R7).
func newAdvisor(p pricing.Provider, disablePricing bool, sizes *azure.Sizes, lister azure.PriceLister) model.Advisor {
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
	// the Azure provider lists candidate prices (aks-recommendations R3);
	// TestSessionPricingPerServer checks the session's provider is one and
	// that a candidate is queued with it
	if lister == nil {
		lister, _ = azureP.(azure.PriceLister)
	}
	advisors[recommend.CloudAzure] = azure.NewAdvisor(lister, disablePricing, sizes)
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
