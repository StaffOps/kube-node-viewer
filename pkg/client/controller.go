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

package client

import (
	"context"
	"math"
	"strconv"
	"sync"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/watchlist"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	"github.com/staffops/kube-node-viewer/pkg/model"
	"github.com/staffops/kube-node-viewer/pkg/pricing"
)

type Controller struct {
	kubeClient      kubernetes.Interface
	cluster         *model.Cluster
	pricing         pricing.Provider
	nodeSelector    labels.Selector
	nodeClaimClient *rest.RESTClient
	// notifier, when set, hears about every change to the cluster or prices
	notifier *Notifier

	// member marks a Clusters screen member (clusters-screen D3, D10): its
	// watches stop on a refused or failing credential, and Fargate nodes
	// are priced from a Fargate-only pod watch until StartPods
	member      bool
	fargatePods bool

	mu          sync.Mutex
	nodeSynced  cache.InformerSynced
	podSynced   cache.InformerSynced
	podsStarted bool
	fargate     bool
	ctx         context.Context
	status      watchStatus
}

// NewController watches one cluster into its own store. A context switch
// builds a new controller and store, so late events from the old watches can
// only reach the old store. nodeClaimClient may be nil: no nodeclaim watch.
func NewController(kubeClient kubernetes.Interface, nodeClaimClient *rest.RESTClient, cluster *model.Cluster, nodeSelector labels.Selector, pricing pricing.Provider) *Controller {
	c := &Controller{
		kubeClient:      kubeClient,
		cluster:         cluster,
		pricing:         pricing,
		nodeSelector:    nodeSelector,
		nodeClaimClient: nodeClaimClient,
	}
	pricing.OnUpdate(c.RefreshNodePrices)
	return c
}

// SetNotifier makes the controller report changes to n. Set it before Start.
func (m *Controller) SetNotifier(n *Notifier) {
	m.notifier = n
}

// SetMember makes the controller a Clusters screen member (clusters-screen
// D3, D10): a watch refused with 403, a 401 that repeats after the
// credential refresh, or a failing exec plugin stops that watch; with
// fargatePods, Fargate nodes seen before StartPods are priced from a watch
// of Fargate pods only. Set it before StartNodes.
func (m *Controller) SetMember(fargatePods bool) {
	m.member, m.fargatePods = true, fargatePods
}

// Status is what the Clusters screen shows of a connection
// (clusters-screen D7).
type Status struct {
	// Synced is the node watch's first list being in the store
	Synced bool
	// PodsSynced is the same for the full pod watch; false until StartPods
	PodsSynced bool
	// Err is the last error of the watches, without the API server URL;
	// empty once a list or an event succeeds after it
	Err string
}

// Status reads the connection's state; safe from any goroutine.
func (m *Controller) Status() Status {
	m.mu.Lock()
	nodes, pods := m.nodeSynced, m.podSynced
	m.mu.Unlock()
	s := Status{Err: m.status.get()}
	s.Synced = nodes != nil && nodes()
	s.PodsSynced = pods != nil && pods()
	return s
}

// Start runs every watch, as Overview needs them.
func (m *Controller) Start(ctx context.Context) {
	m.StartPods(ctx)
	m.StartNodes(ctx)
}

// StartNodes runs the node watch, and the nodeclaim watch when Karpenter's
// CRD is there. It makes a blocking API call.
func (m *Controller) StartNodes(ctx context.Context) {
	m.mu.Lock()
	m.ctx = ctx
	m.mu.Unlock()
	synced := m.startNodeWatch(ctx, m.cluster)
	m.mu.Lock()
	m.nodeSynced = synced
	m.mu.Unlock()
	m.notifyOnSync(ctx, synced)

	// If a NodeClaims Get returns an error, then don't startup the nodeclaims controller since the CRD is not registered
	if m.nodeClaimClient == nil {
		return
	}
	if err := m.nodeClaimClient.Get().Do(ctx).Error(); err == nil {
		m.startNodeClaimWatch(ctx, m.cluster)
	}
}

// StartPods runs the full pod watch, once; later calls do nothing.
func (m *Controller) StartPods(ctx context.Context) {
	m.mu.Lock()
	if m.podsStarted {
		m.mu.Unlock()
		return
	}
	m.podsStarted = true
	m.mu.Unlock()
	synced := m.startPodWatch(ctx, m.cluster, fields.Everything())
	m.mu.Lock()
	m.podSynced = synced
	m.mu.Unlock()
	m.notifyOnSync(ctx, synced)
}

// notifyOnSync reports the end of a first list, so an empty cluster leaves
// "connecting…" (clusters-screen D7).
func (m *Controller) notifyOnSync(ctx context.Context, synced cache.InformerSynced) {
	go func() {
		if cache.WaitForCacheSync(ctx.Done(), synced) {
			m.notifier.Changed()
		}
	}()
}

// startFargateWatch watches only the pods of the EKS Fargate scheduler, which
// price their Fargate node (clusters-screen R16). It runs until the member's
// context ends, also after StartPods: both write the same store by
// namespace/name, so they converge.
func (m *Controller) startFargateWatch() {
	m.mu.Lock()
	if !m.member || !m.fargatePods || m.fargate || m.podsStarted || m.ctx == nil {
		m.mu.Unlock()
		return
	}
	m.fargate = true
	ctx := m.ctx
	m.mu.Unlock()
	m.startPodWatch(ctx, m.cluster, fields.OneTermEqualSelector("spec.schedulerName", FargateScheduler))
}

// FargateScheduler is the scheduler name of EKS Fargate pods
// ([ASSUMPTION] in clusters-screen R16, checked in T12).
const FargateScheduler = "fargate-scheduler"

// listWatch builds the list/watch of a typed client call, wrapped to record
// its errors (clusters-screen D7).
func (m *Controller) listWatch(cancel context.CancelFunc,
	list func(context.Context, metav1.ListOptions) (runtime.Object, error),
	watchFn func(context.Context, metav1.ListOptions) (watch.Interface, error)) cache.ListerWatcher {
	return &statusLW{
		list:        list,
		watch:       watchFn,
		status:      &m.status,
		stop:        m.stopper(cancel),
		unsupported: watchlist.DoesClientNotSupportWatchListSemantics(m.kubeClient),
	}
}

// stopper is nil for Overview's controller, which keeps retrying as before.
func (m *Controller) stopper(cancel context.CancelFunc) context.CancelFunc {
	if !m.member {
		return nil
	}
	return cancel
}

func (m *Controller) startNodeClaimWatch(ctx context.Context, cluster *model.Cluster) {
	ctx, cancel := context.WithCancel(ctx)
	inner := cache.NewFilteredListWatchFromClient(m.nodeClaimClient, "nodeclaims",
		v1.NamespaceAll, func(options *metav1.ListOptions) {
			options.LabelSelector = m.nodeSelector.String()
		})
	nodeClaimWatchList := &statusLW{
		list: func(ctx context.Context, o metav1.ListOptions) (runtime.Object, error) {
			return inner.ListWithContext(ctx, o)
		},
		watch: func(ctx context.Context, o metav1.ListOptions) (watch.Interface, error) {
			return inner.WatchWithContext(ctx, o)
		},
		status: &m.status,
		stop:   m.stopper(cancel),
	}
	_, nodeClaimController := cache.NewInformerWithOptions(cache.InformerOptions{
		ListerWatcher: nodeClaimWatchList,
		ObjectType:    &karpv1.NodeClaim{},
		Transform:     slimNodeClaim,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				defer m.notifier.Changed()
				m.status.event()
				nc := obj.(*karpv1.NodeClaim)
				if nc.Status.ProviderID == "" {
					return
				}
				if _, ok := cluster.GetNode(nc.Status.ProviderID); ok {
					return
				}
				node := model.NewNodeFromNodeClaim(nc)
				m.updatePrice(node)
				n := cluster.AddNode(node)
				n.Show()
			},
			DeleteFunc: func(obj interface{}) {
				defer m.notifier.Changed()
				m.status.event()
				cluster.DeleteNode(ignoreDeletedFinalStateUnknown(obj).(*karpv1.NodeClaim).Status.ProviderID)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				defer m.notifier.Changed()
				m.status.event()
				nc := newObj.(*karpv1.NodeClaim)
				if nc.Status.ProviderID == "" {
					return
				}
				if _, ok := cluster.GetNode(nc.Status.ProviderID); ok {
					return
				}
				node := model.NewNodeFromNodeClaim(nc)
				m.updatePrice(node)
				n := cluster.AddNode(node)
				n.Show()
			},
		},
	})
	go nodeClaimController.Run(ctx.Done())
}

func (m *Controller) startNodeWatch(ctx context.Context, cluster *model.Cluster) cache.InformerSynced {
	ctx, cancel := context.WithCancel(ctx)
	selector := func(o metav1.ListOptions) metav1.ListOptions {
		o.LabelSelector = m.nodeSelector.String()
		return o
	}
	nodes := m.kubeClient.CoreV1().Nodes()
	nodeWatchList := m.listWatch(cancel,
		func(ctx context.Context, o metav1.ListOptions) (runtime.Object, error) {
			return nodes.List(ctx, selector(o))
		},
		func(ctx context.Context, o metav1.ListOptions) (watch.Interface, error) {
			return nodes.Watch(ctx, selector(o))
		})
	_, nodeController := cache.NewInformerWithOptions(cache.InformerOptions{
		ListerWatcher: nodeWatchList,
		ObjectType:    &v1.Node{},
		Transform:     slimNode,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				defer m.notifier.Changed()
				m.status.event()
				node := model.NewNode(obj.(*v1.Node))
				if node.ProviderID() == "" {
					return
				}
				m.updatePrice(node)
				n := cluster.AddNode(node)
				n.Show()
				if n.IsFargate() {
					m.startFargateWatch()
				}
			},
			DeleteFunc: func(obj interface{}) {
				defer m.notifier.Changed()
				m.status.event()
				cluster.DeleteNode(ignoreDeletedFinalStateUnknown(obj).(*v1.Node).Spec.ProviderID)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				defer m.notifier.Changed()
				m.status.event()
				n := newObj.(*v1.Node)
				if !n.DeletionTimestamp.IsZero() && len(n.Finalizers) == 0 {
					cluster.DeleteNode(n.Spec.ProviderID)
				} else {
					node, ok := cluster.UpdateNode(n)
					if ok {
						m.updatePrice(node)
						node.Show()
					}
				}
			},
		},
	})
	go nodeController.Run(ctx.Done())
	return nodeController.HasSynced
}

func (m *Controller) startPodWatch(ctx context.Context, cluster *model.Cluster, selector fields.Selector) cache.InformerSynced {
	ctx, cancel := context.WithCancel(ctx)
	pods := m.kubeClient.CoreV1().Pods(v1.NamespaceAll)
	withSelector := func(o metav1.ListOptions) metav1.ListOptions {
		o.FieldSelector = selector.String()
		return o
	}
	podWatchList := m.listWatch(cancel,
		func(ctx context.Context, o metav1.ListOptions) (runtime.Object, error) {
			return pods.List(ctx, withSelector(o))
		},
		func(ctx context.Context, o metav1.ListOptions) (watch.Interface, error) {
			return pods.Watch(ctx, withSelector(o))
		})

	_, podController := cache.NewInformerWithOptions(cache.InformerOptions{
		ListerWatcher: podWatchList,
		ObjectType:    &v1.Pod{},
		Transform:     SlimPod,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				defer m.notifier.Changed()
				m.status.event()
				p := obj.(*v1.Pod)
				if !isTerminalPod(p) {
					cluster.AddPod(model.NewPod(p))
					node, ok := cluster.GetNodeByName(p.Spec.NodeName)
					// need to potentially update node price as we need the fargate pod in order to figure out the cost
					if ok && node.IsFargate() && !node.HasPrice() {
						m.updatePrice(node)
					}
				}
			},
			DeleteFunc: func(obj interface{}) {
				defer m.notifier.Changed()
				m.status.event()
				p := ignoreDeletedFinalStateUnknown(obj).(*v1.Pod)
				cluster.DeletePod(p.Namespace, p.Name)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				defer m.notifier.Changed()
				m.status.event()
				p := newObj.(*v1.Pod)
				if isTerminalPod(p) {
					cluster.DeletePod(p.Namespace, p.Name)
				} else {
					cluster.AddPod(model.NewPod(p))
				}
			},
		},
	})
	go podController.Run(ctx.Done())
	return podController.HasSynced
}

func (m *Controller) updatePrice(node *model.Node) {
	// If the node has the instance-price override label, don't look up pricing
	// and use the value here; a value that doesn't parse falls through to the
	// lookup.
	labels := node.Labels()
	for _, key := range []string{model.LabelPrefix + "instance-price"} {
		if val, ok := labels[key]; ok {
			if price, err := strconv.ParseFloat(val, 64); err == nil {
				node.SetPrice(price)
				return
			}
		}
	}
	// lookup our n price
	node.SetPrice(math.NaN())
	if price, ok := m.pricing.NodePrice(node); ok {
		if node.CapacityType() == model.CapacityReserved {
			node.SetEquivalentPrice(price)
		} else {
			node.SetPrice(price)
		}
	}

}

func (m *Controller) RefreshNodePrices() {
	m.cluster.ForEachNode(func(n *model.Node) {
		m.updatePrice(n)
	})
	m.notifier.Changed()
}

// isTerminalPod returns true if the pod is deleting or in a terminal state
func isTerminalPod(p *v1.Pod) bool {
	if !p.DeletionTimestamp.IsZero() {
		return true
	}
	switch p.Status.Phase {
	case v1.PodSucceeded, v1.PodFailed:
		return true
	}
	return false
}

// ignoreDeletedFinalStateUnknown returns the object wrapped in
// DeletedFinalStateUnknown. Useful in OnDelete resource event handlers that do
// not need the additional context.
func ignoreDeletedFinalStateUnknown(obj interface{}) interface{} {
	if obj, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		return obj.Obj
	}
	return obj
}
