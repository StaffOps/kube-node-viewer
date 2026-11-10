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

package azure

import (
	"context"
	"errors"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/staffops/kube-node-viewer/pkg/model"
	"github.com/staffops/kube-node-viewer/pkg/recommend"
)

// sizeRetry is how soon a failed size read is tried again while
// Recommendations is shown (azure-live-skus R6).
const sizeRetry = time.Minute

// fetchTimeout bounds one size read: the token and every page.
const fetchTimeout = 5 * time.Minute

// sizesLog is the start of every logged size line, and what a resolution
// line repeats after model.ResolvedPrefix.
const sizesLog = "Azure VM sizes"

// Sizes reads the VM sizes of each (subscription, region) once per session
// from the Resource SKUs API with the az sign-in (azure-live-skus R1, R4).
type Sizes struct {
	client *skuClient
	tokens *tokens
	now    func() time.Time
	// ctx ends with the session: Close cancels it, which kills a running
	// az's process group (azure-live-skus D1)
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu     sync.Mutex
	pairs  map[string]*pairState
	gen    uint64
	logged map[string]bool
	failed bool
}

type pairState struct {
	subscription, region string
	fetching             bool
	sizes                map[string]Size // nil until loaded
	err                  *sizeError
	retryAt              time.Time
	due                  bool // the retry's generation bump was given
	failedBefore         bool
}

// pairView is a pair's state in a snapshot.
type pairView struct {
	loading bool
	err     *sizeError
}

// snapshot is what one analysis sees (azure-live-skus D2).
type snapshot struct {
	loaded map[string]map[string]Size
	union  map[string]Size
	names  []string
	notes  []string
}

// NewSizes reads sizes with run (RunAz in production) from base
// (DefaultARM) until ctx ends or Close; transport nil is
// http.DefaultTransport. Only tests change run, base and transport
// (azure-live-skus D6, R10).
func NewSizes(ctx context.Context, run AzRunner, base string, transport http.RoundTripper) *Sizes {
	return newSizes(ctx, run, base, transport, time.Now)
}

func newSizes(ctx context.Context, run AzRunner, base string, transport http.RoundTripper, now func() time.Time) *Sizes {
	ctx, cancel := context.WithCancel(ctx)
	return &Sizes{client: newSKUClient(base, transport), tokens: newTokens(run, now), now: now, ctx: ctx, cancel: cancel,
		pairs: map[string]*pairState{}, logged: map[string]bool{}}
}

// closeWait bounds how long Close waits for a read to stop.
const closeWait = 3 * time.Second

// Close stops every read: a running az is killed with its process group,
// and Close waits for the reads to return, at most closeWait. A SIGKILL of
// the program itself can still leave a hung az behind (azure-live-skus D1).
func (s *Sizes) Close() {
	s.cancel()
	done := make(chan struct{})
	go func() { s.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(closeWait):
	}
}

func pairKeyOf(subscription, region string) string {
	return strings.ToLower(subscription) + "/" + strings.ToLower(region)
}

// begin starts the reads the pairs need and returns the snapshot.
func (s *Sizes) begin(pairs []recommend.Pair) *snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	views := map[string]pairView{}
	snap := &snapshot{loaded: map[string]map[string]Size{}, union: map[string]Size{}}
	for _, p := range pairs {
		k := pairKeyOf(p.Subscription, p.Region)
		if err := validate(p.Subscription, p.Region); err != nil {
			views[k] = pairView{err: err} // never reaches az or a URL (R10)
			s.logOnce(err)
			continue
		}
		st, ok := s.pairs[k]
		if !ok {
			st = &pairState{subscription: p.Subscription, region: p.Region}
			s.pairs[k] = st
		}
		if st.sizes == nil && !st.fetching && (st.err == nil || !now.Before(st.retryAt)) && s.ctx.Err() == nil {
			st.fetching, st.due = true, false
			s.wg.Add(1)
			go func(sub, region string, recheck bool) {
				defer s.wg.Done()
				s.fetch(k, sub, region, recheck)
			}(st.subscription, st.region, st.failedBefore)
		}
		switch {
		case st.sizes != nil:
			snap.loaded[k] = st.sizes
		case st.err != nil:
			views[k] = pairView{err: st.err}
		default:
			views[k] = pairView{loading: true}
		}
	}
	for _, m := range snap.loaded {
		for name, sz := range m {
			if _, ok := snap.union[name]; !ok {
				snap.union[name] = sz
				snap.names = append(snap.names, name)
			}
		}
	}
	sort.Strings(snap.names)
	snap.notes = pairNotes(pairs, views)
	return snap
}

// generation moves on every state change, and once when a failed pair's
// retry falls due, so recKey changes while Recommendations is shown
// (azure-live-skus D2, R6).
func (s *Sizes) generation() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for _, st := range s.pairs {
		if st.err != nil && !st.fetching && !st.due && !now.Before(st.retryAt) {
			st.due = true
			s.gen++
		}
	}
	return s.gen
}

func (s *Sizes) fetch(k, subscription, region string, recheck bool) {
	ctx, cancel := context.WithTimeout(s.ctx, fetchTimeout)
	defer cancel()
	sizes, err := s.read(ctx, subscription, region, recheck)

	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.pairs[k]
	st.fetching = false
	s.gen++
	if err == nil {
		st.sizes, st.err = sizes, nil
		if s.failed && !s.anyFailed() {
			s.failed = false
			s.logged = map[string]bool{}
			log.Printf("%s%s", model.ResolvedPrefix, sizesLog)
		}
		return
	}
	st.err, st.failedBefore = err, true
	st.retryAt = s.now().Add(err.retry)
	s.failed = true
	s.logOnce(err)
}

// logOnce logs a failure the first time it is seen (R5); s.mu is held.
func (s *Sizes) logOnce(err *sizeError) {
	id := strconv.Itoa(int(err.row)) + "|" + err.status
	if !s.logged[id] {
		s.logged[id] = true
		log.Printf("%s: %s", sizesLog, err.line())
	}
}

func (s *Sizes) anyFailed() bool {
	for _, st := range s.pairs {
		if st.err != nil {
			return true
		}
	}
	return false
}

// read gets a token and the answer, and classifies a failure (R5, R6).
func (s *Sizes) read(ctx context.Context, subscription, region string, recheck bool) (map[string]Size, *sizeError) {
	token, err := s.tokens.get(ctx, subscription, recheck)
	if err != nil {
		var se *sizeError
		if errors.As(err, &se) {
			// a copy: waiters of one token request share its error
			c := *se
			c.retry = sizeRetry
			return nil, &c
		}
		return nil, &sizeError{row: rowAzFailed, retry: sizeRetry}
	}
	sizes, err := s.client.list(ctx, token, subscription, region)
	if err == nil {
		return sizes, nil
	}
	var ae *armError
	switch {
	case errors.As(err, &ae):
		se := &sizeError{retry: sizeRetry, status: strconv.Itoa(ae.status)}
		switch {
		case ae.status == http.StatusUnauthorized:
			se.row = row401
		case ae.status == http.StatusForbidden && ae.code == "AuthorizationFailed":
			se.row = row403
		case ae.status == http.StatusForbidden:
			se.row = row403Other
		case ae.status == http.StatusNotFound && ae.code == "SubscriptionNotFound":
			se.row = row404
		case ae.status == http.StatusTooManyRequests || ae.status >= 500:
			se.row = rowUnavailable
			se.retry, _ = retryDelay(ae.status, ae.header, s.now(), sizeRetry)
		default:
			se.row = row4xx
		}
		if ae.status == http.StatusUnauthorized || ae.status == http.StatusForbidden {
			s.tokens.drop(subscription) // a new sign-in is used next time (R4)
		}
		return nil, se
	case errors.Is(err, errBadAnswer):
		return nil, &sizeError{row: rowBadAnswer, retry: sizeRetry}
	case isTimeout(err):
		return nil, &sizeError{row: rowUnavailable, status: "timeout", retry: sizeRetry}
	}
	return nil, &sizeError{row: rowUnavailable, status: "network error", retry: sizeRetry}
}
