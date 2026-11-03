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

package model

import (
	"errors"
	"fmt"
	"strings"
)

// LabelPolicy decides which node label keys the Filters, Group by, Sort and
// Columns panels offer (label-visibility R1-R4). Its zero value shows every
// key.
type LabelPolicy struct {
	allow []labelPattern
	deny  []labelPattern
}

// labelPattern is a pattern split at its '*'s; parts has one element when the
// pattern has no '*'.
type labelPattern struct {
	parts []string
}

// NewLabelPolicy compiles the allow and deny patterns. An empty allow list
// means no allow list: every key not denied is shown. A pattern is literal
// text where '*' matches any run of characters, '/' and '.' included; it must
// match the whole key, case-sensitively. An empty pattern is an error.
func NewLabelPolicy(allow, deny []string) (LabelPolicy, error) {
	var p LabelPolicy
	var err error
	if p.allow, err = compileLabelPatterns(allow); err != nil {
		return LabelPolicy{}, fmt.Errorf("allow: %w", err)
	}
	if p.deny, err = compileLabelPatterns(deny); err != nil {
		return LabelPolicy{}, fmt.Errorf("deny: %w", err)
	}
	return p, nil
}

func compileLabelPatterns(patterns []string) ([]labelPattern, error) {
	var out []labelPattern
	for i, s := range patterns {
		if s == "" {
			return nil, fmt.Errorf("pattern %d: %w", i, errEmptyPattern)
		}
		out = append(out, labelPattern{parts: strings.Split(s, "*")})
	}
	return out, nil
}

var errEmptyPattern = errors.New("empty pattern")

// Shows reports whether the panels list the label key: it matches an allow
// pattern, or there is no allow list, and it matches no deny pattern.
func (p LabelPolicy) Shows(key string) bool {
	for _, d := range p.deny {
		if d.matches(key) {
			return false
		}
	}
	if len(p.allow) == 0 {
		return true
	}
	for _, a := range p.allow {
		if a.matches(key) {
			return true
		}
	}
	return false
}

// matches reports whether the whole key matches the pattern.
func (p labelPattern) matches(key string) bool {
	if len(p.parts) == 1 {
		return key == p.parts[0]
	}
	first, last := p.parts[0], p.parts[len(p.parts)-1]
	if len(key) < len(first)+len(last) || !strings.HasPrefix(key, first) || !strings.HasSuffix(key, last) {
		return false
	}
	// the middle parts appear in order between the prefix and the suffix;
	// taking each at its first place leaves the most room for the next
	rest := key[len(first) : len(key)-len(last)]
	for _, part := range p.parts[1 : len(p.parts)-1] {
		at := strings.Index(rest, part)
		if at < 0 {
			return false
		}
		rest = rest[at+len(part):]
	}
	return true
}
