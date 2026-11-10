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

package aws

import (
	"math"
	"sync"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/staffops/kube-node-viewer/pkg/model"
)

// fix-node-name-lock R3
func TestFargatePriceReadsThePodsOnce(t *testing.T) {
	p := &pricingProvider{fargateVCPUPricePerHour: 0.04, fargateGBPricePerHour: 0.004}
	n := model.NewNode(&v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "f", Labels: map[string]string{"eks.amazonaws.com/compute-type": "fargate"}}})
	pod := func() *model.Pod {
		return model.NewPod(&v1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "p",
			Annotations: map[string]string{"CapacityProvisioned": "0.25vCPU 0.5GB"}}})
	}
	n.BindPod(pod())
	if price, ok := p.NodePrice(n); !ok || price != 0.25*0.04+0.5*0.004 {
		t.Fatalf("Fargate price %v %v", price, ok)
	}

	// a pod deleted between two reads of the pods made the second one index
	// an empty list
	deadline := time.Now().Add(200 * time.Millisecond)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for time.Now().Before(deadline) {
			n.DeletePod("demo", "p")
			n.BindPod(pod())
		}
	}()
	go func() {
		defer wg.Done()
		for time.Now().Before(deadline) {
			p.NodePrice(n)
		}
	}()
	wg.Wait()
}

// fix-ready-lock-fargate-panic R3
func TestFargatePriceMalformed(t *testing.T) {
	p := &pricingProvider{fargateVCPUPricePerHour: 0.04, fargateGBPricePerHour: 0.004}
	n := model.NewNode(&v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "f", Labels: map[string]string{"eks.amazonaws.com/compute-type": "fargate"}}})
	n.BindPod(model.NewPod(&v1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "p",
		Annotations: map[string]string{"CapacityProvisioned": "2vCPU"}}}))
	if price, ok := p.NodePrice(n); ok || !math.IsNaN(price) {
		t.Errorf("malformed annotation: price %v %v, want NaN false", price, ok)
	}
}
