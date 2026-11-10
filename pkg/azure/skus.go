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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultARM is the only Resource Manager the token goes to (azure-live-skus
// R10); tests change it through the session's Go option (design D6).
const DefaultARM = "https://management.azure.com"

const (
	skusAPIVersion = "2021-07-01"
	// skuTimeout is a choice, unmeasured (azure-live-skus D2)
	skuTimeout = 60 * time.Second
	skuMaxPage = 50
)

// skuPageCap is a choice, unmeasured (azure-live-skus D2); tests lower it.
var skuPageCap = 64 << 20

// Size is an Azure VM size read from the Resource SKUs API (azure-live-skus
// R2), with what the rules need to know about its availability (R3).
type Size struct {
	Name string
	// Family is the SKU's family, lower-cased, without the "standard"
	// prefix and the "Family" suffix: standardDSv5Family → dsv5
	Family string
	// VCPU is the usable count: vCPUsAvailable when given
	VCPU   float64
	MemGiB float64
	// Arch is x86_64 or arm64, the words the rules use
	Arch string
	// Constrained marks a constrained-vCPU size (Standard_E8-4ds_v5)
	Constrained bool
	// TempDiskMB is MaxResourceVolumeMB; 0 read as no temp disk (R9)
	TempDiskMB float64

	// restricted marks a Location restriction; zones are the zones the
	// size is offered in, zoneBlocked the zones a Zone restriction names
	restricted  bool
	zones       map[string]bool
	zoneBlocked map[string]bool
}

// AvailableIn tells whether the size can run in zone (empty: a regional,
// non-zonal node) of the answer's region (azure-live-skus R3).
func (s Size) AvailableIn(zone string) bool {
	if s.restricted {
		return false
	}
	if zone == "" {
		return true
	}
	return s.zones[zone] && !s.zoneBlocked[zone]
}

// NormalizeFamily turns a SKU family into the key the rules use.
func NormalizeFamily(skuFamily string) string {
	f := strings.ToLower(skuFamily)
	f = strings.TrimPrefix(f, "standard")
	return strings.TrimSuffix(f, "family")
}

type skuAnswer struct {
	Value []struct {
		ResourceType string `json:"resourceType"`
		Name         string `json:"name"`
		Family       string `json:"family"`
		LocationInfo []struct {
			Location string   `json:"location"`
			Zones    []string `json:"zones"`
		} `json:"locationInfo"`
		Capabilities []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"capabilities"`
		Restrictions []struct {
			Type            string `json:"type"`
			RestrictionInfo struct {
				Zones []string `json:"zones"`
			} `json:"restrictionInfo"`
		} `json:"restrictions"`
	} `json:"value"`
	NextLink string `json:"nextLink"`
}

// parseSKUs reads one page of an answer for region: virtualMachines only,
// entries missing a field the rules need left out (azure-live-skus R2).
func parseSKUs(body []byte, region string) (map[string]Size, string, error) {
	var a skuAnswer
	if err := json.Unmarshal(body, &a); err != nil {
		return nil, "", err
	}
	out := map[string]Size{}
	for _, v := range a.Value {
		if !strings.EqualFold(v.ResourceType, "virtualMachines") || v.Name == "" || v.Family == "" {
			continue
		}
		caps := map[string]string{}
		for _, c := range v.Capabilities {
			caps[c.Name] = c.Value
		}
		num := func(name string) (float64, bool) {
			x, err := strconv.ParseFloat(caps[name], 64)
			return x, err == nil
		}
		total, okTotal := num("vCPUs")
		usable, okUsable := num("vCPUsAvailable")
		if !okUsable {
			usable, okUsable = total, okTotal
		}
		mem, okMem := num("MemoryGB")
		var arch string
		switch caps["CpuArchitectureType"] {
		case "x64":
			arch = "x86_64"
		case "Arm64":
			arch = "arm64"
		}
		if !okUsable || !okMem || usable <= 0 || mem <= 0 || arch == "" {
			continue
		}
		temp, _ := num("MaxResourceVolumeMB")
		s := Size{
			Name: v.Name, Family: NormalizeFamily(v.Family), VCPU: usable, MemGiB: mem, Arch: arch, TempDiskMB: temp,
			Constrained: strings.Contains(v.Name, "-") || (okTotal && usable < total),
			zones:       map[string]bool{}, zoneBlocked: map[string]bool{},
		}
		for _, li := range v.LocationInfo {
			if strings.EqualFold(li.Location, region) {
				for _, z := range li.Zones {
					s.zones[z] = true
				}
			}
		}
		for _, r := range v.Restrictions {
			switch r.Type {
			case "Location":
				s.restricted = true
			case "Zone":
				for _, z := range r.RestrictionInfo.Zones {
					s.zoneBlocked[z] = true
				}
			}
		}
		out[v.Name] = s
	}
	return out, a.NextLink, nil
}

// armError is a failed Resource Manager answer: its status and error code
// only, never its message, which can name the caller and the scope
// (azure-live-skus R5, D3).
type armError struct {
	status int
	code   string
	header http.Header
}

func (e *armError) Error() string {
	if e.code != "" {
		return fmt.Sprintf("resource manager: %d %s", e.status, e.code)
	}
	return fmt.Sprintf("resource manager: %d", e.status)
}

// errBadAnswer is an answer that can't be read: malformed, over the cap, a
// redirect or a nextLink to another host (R5 row 12).
var errBadAnswer = errors.New("resource manager: an answer that can't be read")

// skuClient lists the VM sizes of a subscription's region.
type skuClient struct {
	base string
	http *http.Client
}

func newSKUClient(base string, transport http.RoundTripper) *skuClient {
	return &skuClient{base: strings.TrimSuffix(base, "/"), http: &http.Client{
		Timeout:   skuTimeout,
		Transport: transport,
		// a redirect could carry the token elsewhere (R10)
		CheckRedirect: func(*http.Request, []*http.Request) error { return errBadAnswer },
	}}
}

// firstURL is the request for a subscription and region, the query built
// with url.Values (R10).
func (c *skuClient) firstURL(subscription, region string) string {
	q := url.Values{}
	q.Set("api-version", skusAPIVersion)
	q.Set("$filter", "location eq '"+region+"'")
	return c.base + "/subscriptions/" + subscription + "/providers/Microsoft.Compute/skus?" + strings.ReplaceAll(q.Encode(), "+", "%20")
}

// sameHost tells whether link goes to the client's Resource Manager over
// https (R10); in tests the base is the fake's https address.
func (c *skuClient) sameHost(link string) bool {
	b, err1 := url.Parse(c.base)
	l, err2 := url.Parse(link)
	return err1 == nil && err2 == nil && l.Scheme == "https" && b.Scheme == "https" && strings.EqualFold(l.Host, b.Host)
}

// list reads every page of the region's sizes.
func (c *skuClient) list(ctx context.Context, token, subscription, region string) (map[string]Size, error) {
	out := map[string]Size{}
	link := c.firstURL(subscription, region)
	for page := 0; link != ""; page++ {
		if page >= skuMaxPage || !c.sameHost(link) {
			return nil, errBadAnswer
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
		if err != nil {
			return nil, errBadAnswer
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")
		resp, err := c.http.Do(req)
		if err != nil {
			if errors.Is(err, errBadAnswer) {
				return nil, errBadAnswer
			}
			return nil, err
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, int64(skuPageCap)+1))
		_ = resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			e := &armError{status: resp.StatusCode, header: resp.Header}
			var eb struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if json.Unmarshal(body, &eb) == nil {
				e.code = eb.Error.Code
			}
			return nil, e
		}
		if len(body) > skuPageCap {
			return nil, errBadAnswer
		}
		sizes, next, err := parseSKUs(body, region)
		if err != nil {
			return nil, errBadAnswer
		}
		for k, v := range sizes {
			out[k] = v
		}
		link = next
	}
	return out, nil
}

// isTimeout tells a network timeout from other network errors.
func isTimeout(err error) bool {
	var ne net.Error
	return errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &ne) && ne.Timeout())
}
