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
	"regexp"
	"strings"
)

// regexChars are the characters that make the search text a regular
// expression (search-regex D1). `.` is not one of them: it is in IPs,
// instance types and host names, so text with only `.` stays literal.
const regexChars = `|()[]{}^$*+?\`

// searchMatcher matches the `/` search text against a node's values:
// literally (ignoring case) when the text has no regex character, as a
// case-insensitive, unanchored RE2 expression otherwise. Text that has a
// regex character but does not compile is matched literally and marked
// invalid (D2). A pattern that is only words joined by `|` (`a|b|c`) is
// matched as those words, the regex's meaning at substring speed (D3).
type searchMatcher struct {
	re       *regexp.Regexp // nil: words
	words    []string       // lowercased; any of them, as substrings
	invalid  bool           // had regex characters but did not compile
	hasRegex bool           // the text has a regex character (footer)
}

// newSearchMatcher builds the matcher for the search text; surrounding
// spaces are ignored.
func newSearchMatcher(text string) searchMatcher {
	text = strings.TrimSpace(text)
	if text == "" {
		return searchMatcher{}
	}
	if !strings.ContainsAny(text, regexChars) {
		return searchMatcher{words: []string{strings.ToLower(text)}}
	}
	m := searchMatcher{hasRegex: true}
	re, err := regexp.Compile("(?i)" + text)
	if err != nil {
		m.invalid = true
		m.words = []string{strings.ToLower(text)}
		return m
	}
	if words, ok := alternationOfWords(text); ok {
		m.words = words
		return m
	}
	m.re = re
	return m
}

// alternationOfWords returns the words of a pattern that is one word or
// words joined by `|`, with no other regex syntax, lowercased. ASCII only:
// (?i) folds some non-ASCII letters differently from strings.ToLower (ſ and
// s, ς and σ), and node names, IPs, types and label values are ASCII.
func alternationOfWords(text string) ([]string, bool) {
	for i := 0; i < len(text); i++ {
		if text[i] >= 0x80 {
			return nil, false
		}
	}
	words := strings.Split(text, "|")
	for _, w := range words {
		if w == "" || strings.ContainsAny(w, regexChars) {
			return nil, false
		}
	}
	// `.` is "any character" inside a regex: only words without it are
	// literal
	for i, w := range words {
		if strings.Contains(w, ".") {
			return nil, false
		}
		words[i] = strings.ToLower(w)
	}
	return words, true
}

// empty reports whether the matcher keeps every node (no search text).
func (m searchMatcher) empty() bool { return m.re == nil && len(m.words) == 0 }

func (m searchMatcher) match(s string) bool {
	if m.re != nil {
		return m.re.MatchString(s)
	}
	s = strings.ToLower(s)
	for _, w := range m.words {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

// searchInvalid reports whether the search text has regex characters but is
// not a valid expression, so it is matched as text.
func searchInvalid(text string) bool { return newSearchMatcher(text).invalid }
