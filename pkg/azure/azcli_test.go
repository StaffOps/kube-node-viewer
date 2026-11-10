//go:build !windows

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
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// fakeAzScript is a fake az for PATH: it logs its arguments and answers from
// files next to it (accounts, token, token.err, token.rc, token.sleep).
const fakeAzScript = `#!/bin/sh
dir=$(dirname "$0")
echo "$*" >> "$dir/calls"
case "$1 $2" in
"account list")
  cat "$dir/accounts" 2>/dev/null
  exit 0;;
"account get-access-token")
  read -r line
  if [ -f "$dir/token.sleep" ]; then
    sleep "$(cat "$dir/token.sleep")" &
    echo $! > "$dir/child.pid"
    wait
  fi
  cat "$dir/token.err" >&2 2>/dev/null
  cat "$dir/token" 2>/dev/null
  exit "$(cat "$dir/token.rc" 2>/dev/null || echo 0)";;
esac
exit 3
`

const enabledAccount = `[{"cloudName":"AzureCloud","id":"` + zeroSub + `","state":"Enabled","tenantId":"11111111-1111-1111-1111-111111111111","user":{"name":"admin@example.com","type":"user"}}]`

type fakeAz struct {
	t   *testing.T
	dir string
}

func newFakeAz(t *testing.T) *fakeAz {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "az"), []byte(fakeAzScript), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+"/usr/bin:/bin")
	f := &fakeAz{t: t, dir: dir}
	f.set("accounts", enabledAccount)
	f.set("token", `{"accessToken":"tok-1","expires_on":`+strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10)+`,"tokenType":"Bearer"}`)
	return f
}

func (f *fakeAz) set(name, content string) {
	f.t.Helper()
	if err := os.WriteFile(filepath.Join(f.dir, name), []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fakeAz) unset(name string) { _ = os.Remove(filepath.Join(f.dir, name)) }

// calls lists the az runs whose arguments start with prefix.
func (f *fakeAz) calls(prefix string) []string {
	raw, _ := os.ReadFile(filepath.Join(f.dir, "calls"))
	var out []string
	for _, l := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if l != "" && strings.HasPrefix(l, prefix) {
			out = append(out, l)
		}
	}
	return out
}

func rowOf(err error) row {
	var se *sizeError
	if errors.As(err, &se) {
		return se.row
	}
	return 0
}

// azure-live-skus R4: the token comes from az for the node's subscription,
// for Resource Manager, and is reused while it is valid.
func TestTokenFromAz(t *testing.T) {
	f := newFakeAz(t)
	tk := newTokens(RunAz, time.Now)
	got, err := tk.get(context.Background(), zeroSub, false)
	if err != nil || got != "tok-1" {
		t.Fatalf("token %q, %v", got, err)
	}
	if _, err := tk.get(context.Background(), zeroSub, false); err != nil {
		t.Fatal(err)
	}
	runs := f.calls("account get-access-token")
	if len(runs) != 1 {
		t.Fatalf("get-access-token runs: %v", runs)
	}
	if want := "account get-access-token --resource https://management.azure.com/ --subscription " + zeroSub + " --output json"; runs[0] != want {
		t.Errorf("args %q, want %q", runs[0], want)
	}
}

// azure-live-skus R4: a token two minutes from expiry is fetched again; an
// answer with only expiresOn (az before 2.54.0) is read; warnings on stderr
// with exit 0 still give a token.
func TestTokenExpiryAndOldAz(t *testing.T) {
	f := newFakeAz(t)
	f.set("token", `{"accessToken":"tok-old","expiresOn":"`+time.Now().Add(90*time.Second).Format("2006-01-02 15:04:05.000000")+`"}`)
	f.set("token.err", "WARNING: this is a warning\n")
	tk := newTokens(RunAz, time.Now)
	if got, err := tk.get(context.Background(), zeroSub, false); err != nil || got != "tok-old" {
		t.Fatalf("token %q, %v", got, err)
	}
	if _, err := tk.get(context.Background(), zeroSub, false); err != nil {
		t.Fatal(err)
	}
	if n := len(f.calls("account get-access-token")); n != 2 {
		t.Errorf("a token 90 s from expiry was reused: %d runs", n)
	}
}

// azure-live-skus R4: after a 401 the token is dropped and the next one is
// fetched; a changed account entry drops it too.
func TestTokenDroppedAndAccountChange(t *testing.T) {
	f := newFakeAz(t)
	tk := newTokens(RunAz, time.Now)
	if _, err := tk.get(context.Background(), zeroSub, false); err != nil {
		t.Fatal(err)
	}
	f.set("token", `{"accessToken":"tok-2","expires_on":`+strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10)+`}`)
	tk.drop(zeroSub)
	if got, _ := tk.get(context.Background(), zeroSub, false); got != "tok-2" {
		t.Errorf("after the drop: %q", got)
	}
	f.set("token", `{"accessToken":"tok-3","expires_on":`+strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10)+`}`)
	if got, _ := tk.get(context.Background(), zeroSub, true); got != "tok-2" {
		t.Errorf("same account, recheck: %q, want the cached tok-2", got)
	}
	f.set("accounts", strings.Replace(enabledAccount, "admin@example.com", "other@example.com", 1))
	if got, _ := tk.get(context.Background(), zeroSub, true); got != "tok-3" {
		t.Errorf("another account: %q, want a new token", got)
	}
}

// azure-live-skus R4: two fetches of one subscription share one token request.
func TestTokenOneInFlight(t *testing.T) {
	f := newFakeAz(t)
	f.set("token.sleep", "1")
	tk := newTokens(RunAz, time.Now)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := tk.get(context.Background(), zeroSub, false); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if n := len(f.calls("account get-access-token")); n != 1 {
		t.Errorf("%d token requests, want 1", n)
	}
}

// azure-live-skus R5 rows 1–5: each az failure gets its row; a Warned
// subscription is readable; another cloud's entry is ignored.
func TestAzFailureRows(t *testing.T) {
	for _, tt := range []struct {
		name  string
		setup func(f *fakeAz)
		want  row
	}{
		{"not signed in", func(f *fakeAz) { f.set("accounts", "[]") }, rowSignIn},
		{"no such subscription", func(f *fakeAz) {
			f.set("accounts", strings.Replace(enabledAccount, zeroSub, "22222222-2222-2222-2222-222222222222", 1))
		}, rowNoSubscription},
		{"another cloud", func(f *fakeAz) {
			f.set("accounts", strings.Replace(enabledAccount, "AzureCloud", "AzureChinaCloud", 1))
		}, rowNoSubscription},
		{"disabled", func(f *fakeAz) { f.set("accounts", strings.Replace(enabledAccount, "Enabled", "Disabled", 1)) }, rowDisabled},
		{"warned", func(f *fakeAz) { f.set("accounts", strings.Replace(enabledAccount, "Enabled", "Warned", 1)) }, 0},
		{"interaction required", func(f *fakeAz) {
			f.set("token.err", "ERROR: AADSTS50076: Due to a configuration change made by your administrator, you must use multi-factor authentication.\nRun the command below to authenticate interactively; additional arguments may be added as needed:\naz login --scope https://management.core.windows.net//.default\n")
			f.set("token.rc", "1")
			f.unset("token")
		}, rowSignIn},
		{"please run az login", func(f *fakeAz) {
			f.set("token.err", "ERROR: Please run 'az login' to setup account.\n")
			f.set("token.rc", "1")
			f.unset("token")
		}, rowSignIn},
		{"other failure", func(f *fakeAz) {
			f.set("token.err", "ERROR: something else\n")
			f.set("token.rc", "1")
			f.unset("token")
		}, rowAzFailed},
		{"malformed token", func(f *fakeAz) { f.set("token", "not json") }, rowAzFailed},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeAz(t)
			tt.setup(f)
			_, err := newTokens(RunAz, time.Now).get(context.Background(), zeroSub, false)
			if got := rowOf(err); got != tt.want {
				t.Errorf("row %d (%v), want %d", got, err, tt.want)
			}
		})
	}
	t.Run("no az", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		_, err := newTokens(RunAz, time.Now).get(context.Background(), zeroSub, false)
		if rowOf(err) != rowNoAz {
			t.Errorf("err %v, want row 1", err)
		}
	})
}

// azure-live-skus R4, D1: stdin is not attached (the command has none, and a
// fake az reading it returns at once), and a hung az is killed with its
// children at the timeout.
func TestAzTimeoutAndStdin(t *testing.T) {
	f := newFakeAz(t)
	var stdinSet atomic.Bool
	beforeRun = func(c *exec.Cmd) { stdinSet.Store(c.Stdin != nil) }
	defer func() { beforeRun = func(*exec.Cmd) {} }()
	start := time.Now()
	if _, err := newTokens(RunAz, time.Now).get(context.Background(), zeroSub, false); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 5*time.Second || stdinSet.Load() {
		t.Errorf("az has a stdin (%v) or waited on it: %v", stdinSet.Load(), time.Since(start))
	}
	old := azTimeout
	azTimeout = 300 * time.Millisecond
	defer func() { azTimeout = old }()
	f.set("token.sleep", "47")
	start = time.Now()
	_, err := newTokens(RunAz, time.Now).get(context.Background(), zeroSub, false)
	if took := time.Since(start); rowOf(err) != rowAzFailed || took > 1500*time.Millisecond {
		t.Errorf("hung az: %v after %v", err, took)
	}
	raw, err := os.ReadFile(filepath.Join(f.dir, "child.pid"))
	if err != nil {
		t.Fatal(err)
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(raw)))
	deadline := time.Now().Add(time.Second)
	for syscall.Kill(pid, 0) == nil && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if syscall.Kill(pid, 0) == nil {
		_ = syscall.Kill(pid, syscall.SIGKILL)
		t.Error("az's child outlived the timeout")
	}
}

// azure-live-skus R10: a subscription that is not a GUID, or a region outside
// [a-z0-9], never reaches az or a URL.
func TestValidateNodeValues(t *testing.T) {
	for _, tt := range []struct {
		sub, region string
		want        row
	}{
		{zeroSub, "eastus", 0},
		{"00000000-0000-0000-0000-00000000000&", "eastus", rowBadSubscription},
		{`x" & calc`, "eastus", rowBadSubscription},
		{"not a guid", "eastus", rowBadSubscription},
		{"&" + zeroSub, "eastus", rowBadSubscription},
		{zeroSub + "&calc", "eastus", rowBadSubscription},
		{zeroSub + "\n", "eastus", rowBadSubscription},
		{zeroSub, "east us", rowBadRegion},
		{zeroSub, "", rowBadRegion},
		{zeroSub, "eastus'", rowBadRegion},
	} {
		err := validate(tt.sub, tt.region)
		got := row(0)
		if err != nil {
			got = err.row
		}
		if got != tt.want {
			t.Errorf("validate(%q, %q) = %d, want %d", tt.sub, tt.region, got, tt.want)
		}
	}
}
