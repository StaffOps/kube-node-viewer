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

// watchStatus holds the last error of a controller's watches
// (clusters-screen D7). A list that succeeds, or any event delivered, clears
// it; a Watch call that returns is not enough, since a stream can open and
// fail before its first event.
type watchStatus struct {
	mu  sync.Mutex
	err string
}

func (s *watchStatus) fail(err error) {
	s.mu.Lock()
	s.err = Sanitize(err)
	s.mu.Unlock()
}

func (s *watchStatus) event() {
	s.mu.Lock()
	s.err = ""
	s.mu.Unlock()
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

	mu           sync.Mutex
	unauthorized int
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
	l.mu.Lock()
	l.unauthorized = 0
	l.mu.Unlock()
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
	return w, nil
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
		// first 401; a second one means the refresh did not help
		l.mu.Lock()
		l.unauthorized++
		n := l.unauthorized
		l.mu.Unlock()
		if n >= 2 {
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

var urlPattern = regexp.MustCompile(`"?https?://[^\s"]+"?`)

// Sanitize gives an error's text without any URL, so a heading never shows
// the API server (clusters-screen R3).
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
	return strings.TrimSpace(msg)
}
