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
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"
)

// maxRetryAfter caps what the API may ask for, so a bad or huge header does
// not stall prices for hours (fix-azure-retry D2; unmeasured).
const maxRetryAfter = 10 * time.Minute

// retryHeaders are read on a 429 or 503 answer; the larger usable delay
// wins. The second is documented for the China Retail Prices API; whether
// the global API sends either is unmeasured (fix-azure-retry D1).
var retryHeaders = []string{"Retry-After", "x-ms-ratelimit-microsoft.consumption-retry-after"}

var seconds = regexp.MustCompile(`^[0-9]+$`)

// statusError is a non-200 answer, with what retryDelay needs.
type statusError struct {
	code   int
	status string
	header http.Header
}

func (e *statusError) Error() string { return "retail prices API: " + e.status }

// retryDelay is how long to wait before asking a region again after a failed
// answer: the larger of floor and the answer's retry header (positive
// seconds or a future HTTP date, capped at maxRetryAfter) on a 429 or 503, else floor. note
// describes the header for the log, empty when the status has none to read
// (fix-azure-retry R1).
func retryDelay(code int, h http.Header, now time.Time, floor time.Duration) (d time.Duration, note string) {
	if code != http.StatusTooManyRequests && code != http.StatusServiceUnavailable {
		return floor, ""
	}
	var ra time.Duration
	found := false
	for _, name := range retryHeaders {
		v := h.Get(name)
		if v == "" {
			continue
		}
		var got time.Duration
		if seconds.MatchString(v) {
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil || n > int64(maxRetryAfter/time.Second)*1000 {
				n = int64(maxRetryAfter / time.Second) // absurd: cap below
			}
			got = time.Duration(n) * time.Second
		} else if t, err := http.ParseTime(v); err == nil {
			got = t.Sub(now)
		} else {
			continue
		}
		if got <= 0 {
			continue
		}
		found = true
		if got > ra {
			ra = got
		}
	}
	if !found {
		return floor, "no retry header"
	}
	d = ra
	if d > maxRetryAfter {
		d = maxRetryAfter
	}
	if d < floor {
		d = floor
	}
	note = fmt.Sprintf("retry header %s", ra.Round(time.Second))
	if d != ra {
		note += fmt.Sprintf(", waiting %s", d)
	}
	return d, note
}
