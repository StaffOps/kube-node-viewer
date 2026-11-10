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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// AzRunner runs the Azure CLI with args and returns its stdout and stderr;
// ErrAzNotFound when there is no az to run. The session's Go option replaces
// it in tests (azure-live-skus D6).
type AzRunner func(ctx context.Context, args ...string) (stdout, stderr []byte, err error)

// ErrAzNotFound is an AzRunner's answer when az is not on PATH (R5 row 1).
var ErrAzNotFound = errors.New("az not found")

// azTimeout is a choice, unmeasured (azure-live-skus D1).
var azTimeout = 30 * time.Second

// beforeRun lets a test see the command RunAz is about to start.
var beforeRun = func(*exec.Cmd) {}

// tokenMargin: a token this close to expiry is fetched again (R4, a choice).
const tokenMargin = 2 * time.Minute

// RunAz runs az from PATH: stdin not attached, stderr captured, killed with
// its children at the timeout (azure-live-skus D1).
func RunAz(ctx context.Context, args ...string) ([]byte, []byte, error) {
	path, err := exec.LookPath("az")
	if err != nil {
		return nil, nil, ErrAzNotFound
	}
	ctx, cancel := context.WithTimeout(ctx, azTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, args...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	cmd.WaitDelay = 2 * time.Second
	killTree(cmd)
	beforeRun(cmd)
	err = cmd.Run()
	if ctx.Err() != nil {
		return out.Bytes(), errb.Bytes(), context.DeadlineExceeded
	}
	return out.Bytes(), errb.Bytes(), err
}

// row is a line of the azure-live-skus R5 table.
type row int

const (
	rowNoAz row = iota + 1
	rowSignIn
	rowNoSubscription
	rowDisabled
	rowAzFailed
	row401
	row403
	row403Other
	row404
	row4xx
	rowUnavailable
	rowBadAnswer
	rowBadSubscription
	rowBadRegion
)

// sizeError is a failed size read, by R5 row; status fills rows 10 and 11.
type sizeError struct {
	row    row
	status string
	// retry is the delay of rows 11 and 12 (fix-azure-retry rules)
	retry time.Duration
}

func (e *sizeError) Error() string { return "Azure VM sizes: " + e.line() }

func (e *sizeError) line() string {
	switch e.row {
	case rowNoAz:
		return "the Azure CLI (az) was not found; install it and run az login"
	case rowSignIn:
		return "not signed in to Azure; run az login"
	case rowNoSubscription:
		return "the signed-in account can't see the nodes' subscription; az login with an account that can"
	case rowDisabled, row404:
		return "the nodes' subscription was not found or is disabled"
	case rowAzFailed:
		return "the Azure CLI failed to give a token; run az account get-access-token to see why"
	case row401:
		return "Azure refused the token; run az login"
	case row403:
		return "no permission to read VM sizes (Microsoft.Compute/skus/read on the subscription, e.g. the Reader role)"
	case row403Other:
		return "Azure refused the request (403); check the subscription's policies"
	case row4xx:
		return "Azure refused the request (" + e.status + ")"
	case rowUnavailable:
		return "Azure Resource Manager unavailable (" + e.status + "); retrying in " + shortDuration(e.retry)
	case rowBadAnswer:
		return "Azure Resource Manager gave an answer that can't be read; retrying in " + shortDuration(e.retry)
	case rowBadSubscription:
		return "the nodes' provider ID names no valid subscription"
	case rowBadRegion:
		return "a node has no valid region label"
	}
	return "unknown failure"
}

func shortDuration(d time.Duration) string {
	d = d.Round(time.Second)
	if d >= time.Minute && d%time.Minute == 0 {
		return strconv.Itoa(int(d/time.Minute)) + "m"
	}
	return d.String()
}

var (
	guid       = regexp.MustCompile(`^[0-9a-fA-F]{8}-([0-9a-fA-F]{4}-){3}[0-9a-fA-F]{12}$`)
	regionName = regexp.MustCompile(`^[a-z0-9]+$`)
)

// validate checks the values a Node object gives before they reach az or a
// URL (azure-live-skus R10).
func validate(subscription, region string) *sizeError {
	if !guid.MatchString(subscription) {
		return &sizeError{row: rowBadSubscription}
	}
	if !regionName.MatchString(region) {
		return &sizeError{row: rowBadRegion}
	}
	return nil
}

type account struct {
	CloudName string `json:"cloudName"`
	ID        string `json:"id"`
	State     string `json:"state"`
	TenantID  string `json:"tenantId"`
	User      struct {
		Name string `json:"name"`
	} `json:"user"`
}

type cachedToken struct {
	token   string
	expires time.Time
	// who is the account entry the token was taken for, in memory only
	who string
}

type inflight struct {
	done  chan struct{}
	token string
	err   error
}

// tokens gets and caches Resource Manager tokens from az, per subscription,
// one request in flight per subscription (azure-live-skus R4, D1).
type tokens struct {
	run AzRunner
	now func() time.Time

	mu       sync.Mutex
	cache    map[string]cachedToken
	inflight map[string]*inflight
}

func newTokens(run AzRunner, now func() time.Time) *tokens {
	return &tokens{run: run, now: now, cache: map[string]cachedToken{}, inflight: map[string]*inflight{}}
}

// drop forgets a subscription's token: after a 401 or 403 (R4).
func (t *tokens) drop(subscription string) {
	t.mu.Lock()
	delete(t.cache, strings.ToLower(subscription))
	t.mu.Unlock()
}

// get returns a token for subscription. recheck runs az account list even
// with a cached token, dropping it if the account entry changed (a retry
// after a failure, R4).
func (t *tokens) get(ctx context.Context, subscription string, recheck bool) (string, error) {
	key := strings.ToLower(subscription)
	t.mu.Lock()
	if f, ok := t.inflight[key]; ok {
		t.mu.Unlock()
		select {
		case <-f.done:
			return f.token, f.err
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	c, cached := t.cache[key]
	if cached && !recheck && t.now().Add(tokenMargin).Before(c.expires) {
		t.mu.Unlock()
		return c.token, nil
	}
	f := &inflight{done: make(chan struct{})}
	t.inflight[key] = f
	t.mu.Unlock()

	f.token, f.err = t.fetch(ctx, key, c, cached)

	t.mu.Lock()
	delete(t.inflight, key)
	t.mu.Unlock()
	close(f.done)
	return f.token, f.err
}

func (t *tokens) fetch(ctx context.Context, key string, c cachedToken, cached bool) (string, error) {
	who, err := t.account(ctx, key)
	if err != nil {
		t.drop(key)
		return "", err
	}
	if cached && c.who == who && t.now().Add(tokenMargin).Before(c.expires) {
		return c.token, nil
	}
	out, stderr, err := t.run(ctx, "account", "get-access-token", "--resource", DefaultARM+"/", "--subscription", key, "--output", "json")
	if err != nil {
		return "", azFailure(err, stderr)
	}
	var tok struct {
		AccessToken string          `json:"accessToken"`
		ExpiresOn   json.RawMessage `json:"expires_on"`
		ExpiresOnL  string          `json:"expiresOn"`
	}
	if json.Unmarshal(out, &tok) != nil || tok.AccessToken == "" {
		return "", &sizeError{row: rowAzFailed}
	}
	exp, ok := expiry(tok.ExpiresOn, tok.ExpiresOnL)
	if !ok {
		return "", &sizeError{row: rowAzFailed}
	}
	t.mu.Lock()
	t.cache[key] = cachedToken{token: tok.AccessToken, expires: exp, who: who}
	t.mu.Unlock()
	return tok.AccessToken, nil
}

// account finds the subscription in az account list (public cloud only) and
// returns who it is signed in as, in memory only (R5 rows 2–4).
func (t *tokens) account(ctx context.Context, key string) (string, error) {
	out, stderr, err := t.run(ctx, "account", "list", "--all", "--output", "json")
	if err != nil {
		return "", azFailure(err, stderr)
	}
	var list []account
	if json.Unmarshal(out, &list) != nil {
		return "", &sizeError{row: rowAzFailed}
	}
	if len(list) == 0 {
		return "", &sizeError{row: rowSignIn}
	}
	for _, a := range list {
		if a.CloudName != "" && a.CloudName != "AzureCloud" {
			continue // --all lists every cloud the CLI knows
		}
		if !strings.EqualFold(a.ID, key) {
			continue
		}
		if a.State == "Disabled" || a.State == "Deleted" {
			return "", &sizeError{row: rowDisabled}
		}
		return a.TenantID + "\x00" + a.User.Name, nil
	}
	return "", &sizeError{row: rowNoSubscription}
}

// azFailure classifies a failed az run; its output is never kept (R5).
func azFailure(err error, stderr []byte) error {
	switch {
	case errors.Is(err, ErrAzNotFound):
		return &sizeError{row: rowNoAz}
	case bytes.Contains(stderr, []byte("AADSTS")), bytes.Contains(stderr, []byte("authenticate interactively")), bytes.Contains(stderr, []byte("az login")):
		return &sizeError{row: rowSignIn}
	}
	return &sizeError{row: rowAzFailed}
}

// expiry reads expires_on (POSIX seconds, az 2.54.0 on) or expiresOn (local
// time) (R4).
func expiry(posix json.RawMessage, local string) (time.Time, bool) {
	if len(posix) > 0 && string(posix) != "null" {
		s := strings.Trim(string(posix), `"`)
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			return time.Unix(n, 0), true
		}
	}
	for _, layout := range []string{"2006-01-02 15:04:05.999999", "2006-01-02 15:04:05"} {
		if t, err := time.ParseInLocation(layout, local, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
