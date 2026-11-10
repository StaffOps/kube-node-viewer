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

const testKubeconfig = `apiVersion: v1
kind: Config
current-context: b
clusters:
- name: ca
  cluster: {server: "https://AAA.gr7.us-east-1.eks.amazonaws.com"}
- name: cb
  cluster: {server: "https://kind.local:6443"}
contexts:
- name: b
  context: {cluster: cb, user: u}
- name: a
  context: {cluster: ca, user: u}
users:
- name: u
  user: {token: t}
`

func writeKubeconfig(t *testing.T) string {
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte(testKubeconfig), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestContexts(t *testing.T) {
	names, current, err := Contexts(writeKubeconfig(t))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(names, []string{"a", "b"}) || current != "b" {
		t.Errorf("names %v current %q", names, current)
	}
}

func TestResolveCluster(t *testing.T) {
	path := writeKubeconfig(t)
	c, err := ResolveCluster(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if c.Context != "b" || c.Server != "https://kind.local:6443" || c.Key() != "b@https://kind.local:6443" {
		t.Errorf("current: %+v", c)
	}
	c, _ = ResolveCluster(path, "a")
	if c.Server != "https://AAA.gr7.us-east-1.eks.amazonaws.com" {
		t.Errorf("named: %+v", c)
	}
}

func TestRegionFromServer(t *testing.T) {
	cases := map[string]string{
		"https://ABCDEF01.gr7.us-east-1.eks.amazonaws.com":  "us-east-1",
		"https://ABC.yl4.ap-southeast-2.eks.amazonaws.com":  "ap-southeast-2",
		"https://ABC.sk1.us-gov-west-1.eks.amazonaws.com":   "us-gov-west-1",
		"https://kind.local:6443":                           "",
		"https://my-cluster.example.com/eks.amazonaws.com/": "",
	}
	for server, want := range cases {
		if got := RegionFromServer(server); got != want {
			t.Errorf("%s: got %q, want %q", server, got, want)
		}
	}
}

func TestIsAKSServer(t *testing.T) {
	cases := map[string]bool{
		"https://example1-demo1-a1b2c3d4.hcp.eastus2.azmk8s.io:443": true,
		"https://cluster-abc.privatelink.eastus2.azmk8s.io:443":     true,
		"https://ABCDEF01.gr7.us-east-1.eks.amazonaws.com":          false,
		"https://evil.example.com/.azmk8s.io":                       false,
		"https://kind.local:6443":                                   false,
	}
	for server, want := range cases {
		if got := IsAKSServer(server); got != want {
			t.Errorf("%s: got %v, want %v", server, got, want)
		}
	}
}

// short-arn-context R3: the saved-state key keeps the full context name
// short-gke-context R4
// hide-cloud-ids R4
// short-edge-context R4
func TestClusterKeyKeepsTheFullContext(t *testing.T) {
	for _, context := range []string{
		"arn:aws:eks:us-east-1:111122223333:cluster/demo-prod",
		"gke_example-project_us-central1-a_demo-prod",
		"gke_" + "aws_example-project_us-west1_demo-prod",
		"connectgateway_example-project_global_demo-prod_ns-team-a",
		"edgecontainer_example-project_us-central1_demo-prod_ops_user@example.org_offline",
	} {
		c := Cluster{Context: context, Server: "https://example.com"}
		if got, want := c.Key(), context+"@https://example.com"; got != want {
			t.Errorf("Key() = %q, want %q", got, want)
		}
	}
}
