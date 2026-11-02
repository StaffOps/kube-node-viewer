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
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestNotifierCoalesces(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var fired atomic.Int32
	const interval = 100 * time.Millisecond
	n := NewNotifier(ctx, interval, func() { fired.Add(1) })

	waitFor := func(want int32, within time.Duration) {
		t.Helper()
		deadline := time.Now().Add(within)
		for fired.Load() < want && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		if got := fired.Load(); got != want {
			t.Fatalf("fired %d times, want %d", got, want)
		}
	}

	// a first change fires right away
	n.Changed()
	waitFor(1, interval/2)

	// a burst during the interval folds into a single call once it ends
	for i := 0; i < 1000; i++ {
		n.Changed()
	}
	time.Sleep(interval / 4)
	if got := fired.Load(); got != 1 {
		t.Fatalf("fired %d times during the interval, want 1", got)
	}
	waitFor(2, 2*interval)
	time.Sleep(2 * interval)
	if got := fired.Load(); got != 2 {
		t.Errorf("fired %d times in total, want 2", got)
	}
}

func TestNotifierNilIsNoop(t *testing.T) {
	var n *Notifier
	n.Changed()
}
