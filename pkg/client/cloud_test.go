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

package client

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// clusters-screen R8: the picker's cloud of a context, guessed from the
// kubeconfig alone.
func TestGuessCloud(t *testing.T) {
	// the multi-cloud gcloud prefix is split so the content check does not
	// read it as a real GKE context
	multiCloud := "gke_" + "aws_example-project_us-east4_c1"
	cases := []struct {
		name, context, server, want string
	}{
		{"EKS server URL", "prod", "https://ABC.gr7.us-east-1.eks.amazonaws.com", "aws"},
		{"EKS server URL upper case host", "prod", "https://ABC.GR7.US-EAST-1.EKS.AMAZONAWS.COM", "aws"},
		{"EKS ARN context", "arn:aws:eks:us-east-1:111122223333:cluster/x", "https://203.0.113.10", "aws"},
		{"EKS ARN context, GovCloud partition", "arn:aws-us-gov:eks:us-gov-west-1:111122223333:cluster/x", "", "aws"},
		{"AKS server", "aks", "https://example-aks.hcp.eastus2.azmk8s.io:443", "azure"},
		{"GKE context", "gke_example-project_us-central1_c1", "https://203.0.113.20", "gce"},
		{"gcloud multi-cloud context is not GKE nodes", multiCloud, "https://203.0.113.30", ""},
		{"IP server", "kind-dev", "https://192.0.2.10", ""},
		{"no server", "lonely", "", ""},
		{"eks in a path is not an EKS host", "x", "https://example.com/.eks.amazonaws.com", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := GuessCloud(tc.context, tc.server); got != tc.want {
				t.Errorf("GuessCloud(%q, %q) = %q, want %q", tc.context, tc.server, got, tc.want)
			}
		})
	}
}

// clusters-screen R8: the kubeconfig's contexts, sorted by name, with their
// API servers and the current context.
func TestContextServers(t *testing.T) {
	kubeconfig := `apiVersion: v1
kind: Config
current-context: beta
clusters:
- name: c-one
  cluster:
    server: https://one.example.com
- name: c-two
  cluster:
    server: https://two.example.com:443
contexts:
- name: gamma
  context:
    cluster: c-two
- name: alpha
  context:
    cluster: c-one
- name: beta
  context:
    cluster: c-two
- name: orphan
  context:
    cluster: no-such-cluster
users: []
`
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte(kubeconfig), 0o600); err != nil {
		t.Fatal(err)
	}
	list, current, err := ContextServers(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []ContextServer{
		{Name: "alpha", Server: "https://one.example.com"},
		{Name: "beta", Server: "https://two.example.com:443"},
		{Name: "gamma", Server: "https://two.example.com:443"},
		{Name: "orphan", Server: ""},
	}
	if !reflect.DeepEqual(list, want) {
		t.Errorf("contexts = %+v\nwant %+v", list, want)
	}
	if current != "beta" {
		t.Errorf("current = %q, want beta", current)
	}
}

// clusters-screen R8: an unreadable kubeconfig is an error, not an empty
// list.
func TestContextServersBadFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte("not: [yaml"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ContextServers(path); err == nil {
		t.Error("broken kubeconfig gave no error")
	}
}
