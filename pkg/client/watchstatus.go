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
	"errors"
	"net/url"
	"regexp"
	"strings"
	"sync"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
)

// watchStatus holds the last error of one informer (clusters-screen D7). A
// list that succeeds, or an event the informer delivers, clears it; a Watch
// call that returns is not enough, since a stream can open and fail before
// its first event. Each informer has its own, so one watch's events never
// hide another's error.
type watchStatus struct {
	mu  sync.Mutex
	err string
	// unauthorized counts 401s since the last success (D3)
	unauthorized int
}

func (s *watchStatus) fail(err error) {
	s.mu.Lock()
	s.err = Sanitize(err)
	s.mu.Unlock()
}

func (s *watchStatus) event() {
	s.mu.Lock()
	s.err, s.unauthorized = "", 0
	s.mu.Unlock()
}

// unauthorizedAgain counts a 401 and tells whether it is the second in a row.
func (s *watchStatus) unauthorizedAgain() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.unauthorized++
	return s.unauthorized >= 2
}

func (s *watchStatus) get() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// statusLW is a list/watch that records its errors in status and, for a
// Clusters screen member, stops its informer on a credential failure
// (clusters-screen D3). It keeps the client's watch-list support, so the
// reflector takes the same path with or without it.
type statusLW struct {
	list   func(context.Context, metav1.ListOptions) (runtime.Object, error)
	watch  func(context.Context, metav1.ListOptions) (watch.Interface, error)
	status *watchStatus
	// stop ends the informer; nil keeps the reflector's retries
	stop        context.CancelFunc
	unsupported bool
}

func (l *statusLW) List(o metav1.ListOptions) (runtime.Object, error) {
	return l.ListWithContext(context.Background(), o)
}

func (l *statusLW) ListWithContext(ctx context.Context, o metav1.ListOptions) (runtime.Object, error) {
	obj, err := l.list(ctx, o)
	if err != nil {
		l.failed(err)
		return nil, err
	}
	l.status.event()
	return obj, nil
}

func (l *statusLW) Watch(o metav1.ListOptions) (watch.Interface, error) {
	return l.WatchWithContext(context.Background(), o)
}

func (l *statusLW) WatchWithContext(ctx context.Context, o metav1.ListOptions) (watch.Interface, error) {
	w, err := l.watch(ctx, o)
	if err != nil {
		l.failed(err)
		return nil, err
	}
	// any event of the stream, a bookmark included, shows it works: a quiet
	// informer whose watch was refused once must not keep that error
	return newEventWatch(w, l.status), nil
}

// eventWatch passes a watch's events on unchanged and clears status on each
// that is not an error. Unlike watch.Filter, Stop also releases an event
// waiting to be read, so a reflector that stops reading leaks nothing.
type eventWatch struct {
	inner  watch.Interface
	out    chan watch.Event
	done   chan struct{}
	stop   sync.Once
	status *watchStatus
}

func newEventWatch(inner watch.Interface, status *watchStatus) *eventWatch {
	w := &eventWatch{inner: inner, out: make(chan watch.Event), done: make(chan struct{}), status: status}
	go w.loop()
	return w
}

func (w *eventWatch) loop() {
	defer close(w.out)
	in := w.inner.ResultChan()
	for {
		var e watch.Event
		var ok bool
		select {
		case e, ok = <-in:
			if !ok {
				return
			}
		case <-w.done:
			return
		}
		if e.Type != watch.Error {
			w.status.event()
		}
		select {
		case w.out <- e:
		case <-w.done:
			return
		}
	}
}

func (w *eventWatch) ResultChan() <-chan watch.Event { return w.out }

func (w *eventWatch) Stop() {
	w.stop.Do(func() {
		close(w.done)
		w.inner.Stop()
	})
}

// IsWatchListSemanticsUnSupported passes on what the client says
// (k8s.io/client-go/util/watchlist).
func (l *statusLW) IsWatchListSemanticsUnSupported() bool { return l.unsupported }

func (l *statusLW) failed(err error) {
	l.status.fail(err)
	if l.stop == nil {
		return
	}
	switch {
	case apierrors.IsForbidden(err), IsExecFailure(err):
		l.stop()
	case apierrors.IsUnauthorized(err):
		// client-go's exec authenticator refreshes the credential after a
		// first 401; a second one with no success between means the
		// refresh did not help
		if l.status.unauthorizedAgain() {
			l.stop()
		}
	}
}

// execFailurePrefix starts the error of a failing exec credential plugin;
// client-go wraps it with %v, so only the text tells it
// (plugin/pkg/client/auth/exec/exec.go). TestExecFailurePrefix pins it.
const execFailurePrefix = "getting credentials:"

// IsExecFailure reports an exec credential plugin failure.
func IsExecFailure(err error) bool {
	return err != nil && strings.Contains(err.Error(), execFailurePrefix)
}

var (
	urlPattern = regexp.MustCompile(`"?https?://[^\s"]+"?`)
	// a DNS lookup's host, an IP with or without port, a host:port
	lookupPattern   = regexp.MustCompile(`lookup [^\s:]+`)
	ipPattern       = regexp.MustCompile(`\[[0-9A-Fa-f:]+\](:[0-9]+)?|\b[0-9]{1,3}(\.[0-9]{1,3}){3}(:[0-9]+)?\b`)
	hostPortPattern = regexp.MustCompile(`\b[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)+:[0-9]+\b`)
	// x509 errors list the names a certificate is valid for, and quote
	// names of authorities
	x509ValidFor = regexp.MustCompile(`(?i)(x509: certificate is (?:not )?valid for).*$`)
	quoted       = regexp.MustCompile(`"[^"]*"`)
	dnsName      = regexp.MustCompile(`(?i)\b[a-z0-9-]+(\.[a-z0-9-]+)+\b`)
)

// Sanitize gives an error's text without any URL, host or address, so a
// heading never shows the API server (clusters-screen R3).
func Sanitize(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	var ue *url.Error
	if errors.As(err, &ue) && ue.Err != nil {
		msg = ue.Op + ": " + ue.Err.Error()
	}
	msg = urlPattern.ReplaceAllString(msg, "the API server")
	msg = lookupPattern.ReplaceAllString(msg, "lookup of the API server")
	msg = ipPattern.ReplaceAllString(msg, "an address")
	msg = hostPortPattern.ReplaceAllString(msg, "the API server")
	if strings.Contains(msg, "x509:") {
		msg = x509ValidFor.ReplaceAllString(msg, "${1} other names")
		msg = quoted.ReplaceAllString(msg, "a name")
		msg = dnsName.ReplaceAllString(msg, "a name")
	}
	return strings.TrimSpace(msg)
}
