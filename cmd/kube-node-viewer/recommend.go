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

package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/staffops/kube-node-viewer/pkg/model"
)

// recommendOptions reads the config-file settings of the recommendations
// (recommendations R12): recommend-headroom (percent, default 80) and
// recommend-node-age (30d, 12h or a Go duration; default 30d).
func recommendOptions(cfg configFile) (model.RecommendOptions, error) {
	opts := model.RecommendOptions{Headroom: 0.8, OldAge: 30 * 24 * time.Hour}
	if v, ok := cfg["recommend-headroom"]; ok {
		pct, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(v), "%"), 64)
		if err != nil || pct <= 0 || pct > 100 {
			return opts, fmt.Errorf("recommend-headroom: %q is not a percent between 1 and 100", v)
		}
		opts.Headroom = pct / 100
	}
	if v, ok := cfg["recommend-node-age"]; ok {
		age, err := parseAge(v)
		if err != nil || age <= 0 {
			return opts, fmt.Errorf("recommend-node-age: %q is not a duration like 30d or 12h", v)
		}
		opts.OldAge = age
	}
	return opts, nil
}

// parseAge takes days ("30d") or any Go duration ("12h", "90m").
func parseAge(v string) (time.Duration, error) {
	v = strings.TrimSpace(v)
	if days, ok := strings.CutSuffix(v, "d"); ok {
		n, err := strconv.ParseFloat(days, 64)
		if err != nil {
			return 0, err
		}
		return time.Duration(n * float64(24*time.Hour)), nil
	}
	return time.ParseDuration(v)
}
