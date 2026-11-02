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
	"net/url"
	"regexp"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth" // pull auth
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	karpv1apis "sigs.k8s.io/karpenter/pkg/apis"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
)

func NewKubernetes(kubeconfig, context string) (*kubernetes.Clientset, error) {
	config, err := getConfig(kubeconfig, context)
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return clientset, err
}

func NewNodeClaims(kubeconfig, context string) (*rest.RESTClient, error) {
	c, err := getConfig(kubeconfig, context)
	if err != nil {
		return nil, err
	}

	gv := schema.GroupVersion{Group: karpv1apis.Group, Version: "v1"}
	scheme.Scheme.AddKnownTypes(gv,
		&karpv1.NodeClaim{},
		&karpv1.NodeClaimList{})

	config := *c
	config.ContentConfig.GroupVersion = &gv
	config.APIPath = "/apis"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	config.UserAgent = rest.DefaultKubernetesUserAgent()

	return rest.RESTClientFor(&config)
}

func getConfig(kubeconfig, context string) (*rest.Config, error) {
	// use the current context in kubeconfig
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{Precedence: strings.Split(kubeconfig, ":")},
		&clientcmd.ConfigOverrides{CurrentContext: context}).ClientConfig()
}

// Cluster names the cluster a kubeconfig context points at. An empty context
// means the current one.
type Cluster struct {
	Context string
	Server  string
}

// Key identifies the cluster for saved state: the context name and its API
// server URL, since merged kubeconfigs can repeat context names.
func (c Cluster) Key() string {
	return c.Context + "@" + c.Server
}

// ResolveCluster reads the context and its API server URL from the kubeconfig.
func ResolveCluster(kubeconfig, context string) (Cluster, error) {
	raw, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{Precedence: strings.Split(kubeconfig, ":")},
		&clientcmd.ConfigOverrides{CurrentContext: context}).RawConfig()
	if err != nil {
		return Cluster{}, err
	}
	c := Cluster{Context: context}
	if c.Context == "" {
		c.Context = raw.CurrentContext
	}
	if ctx, ok := raw.Contexts[c.Context]; ok {
		if cluster, ok := raw.Clusters[ctx.Cluster]; ok {
			c.Server = cluster.Server
		}
	}
	return c, nil
}

// Contexts lists the kubeconfig's context names, sorted, and the current one.
func Contexts(kubeconfig string) (names []string, current string, err error) {
	raw, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{Precedence: strings.Split(kubeconfig, ":")},
		&clientcmd.ConfigOverrides{}).RawConfig()
	if err != nil {
		return nil, "", err
	}
	for name := range raw.Contexts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, raw.CurrentContext, nil
}

var eksServerRegion = regexp.MustCompile(`\.([a-z]{2}(?:-gov)?-[a-z]+-\d+)\.eks\.amazonaws\.com`)

// RegionFromServer is the AWS region in an EKS API server URL
// (https://<id>.gr7.us-east-1.eks.amazonaws.com), empty for any other server.
func RegionFromServer(server string) string {
	if m := eksServerRegion.FindStringSubmatch(server); m != nil {
		return m[1]
	}
	return ""
}

// IsAKSServer tells whether the API server URL is an AKS cluster's
// (https://<dns-prefix>-<hash>.hcp.<region>.azmk8s.io, private ones under
// privatelink.<region>.azmk8s.io).
func IsAKSServer(server string) bool {
	u, err := url.Parse(server)
	if err != nil {
		return false
	}
	return strings.HasSuffix(u.Hostname(), ".azmk8s.io")
}
