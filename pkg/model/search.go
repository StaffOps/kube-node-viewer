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
//
// A leading `!` negates the rest (search-negation R1, D2), and text with `=`
// also matches each label as `key=value` (R2, D1); Node.contains applies
// both.
type searchMatcher struct {
	re       *regexp.Regexp // nil: words
	words    []string       // lowercased; any of them, as substrings
	invalid  bool           // had regex characters but did not compile
	hasRegex bool           // the text has a regex character (footer)
	negate   bool           // a leading `!`: keep the nodes it does not match
	pairs    bool           // the text has `=`: also match labels as key=value
	// buf holds key=value for the regex while matching pairs, reused for
	// every label of every node of one Select (design, Performance); a
	// matcher is built per call, so it is never shared between goroutines
	buf *[]byte
}

// newSearchMatcher builds the matcher for the search text; surrounding
// spaces are ignored, and so are spaces after a leading `!`. `!` alone is
// an empty search (search-negation R4).
func newSearchMatcher(text string) searchMatcher {
	text = strings.TrimSpace(text)
	negate := false
	if rest, ok := strings.CutPrefix(text, "!"); ok {
		text, negate = strings.TrimSpace(rest), true
	}
	if text == "" {
		return searchMatcher{}
	}
	m := newTextMatcher(text)
	m.negate = negate
	m.pairs = strings.Contains(text, "=")
	if m.pairs && m.re != nil {
		m.buf = new([]byte)
	}
	return m
}

// newTextMatcher builds the matcher for trimmed, non-empty text, without
// negation (search-regex D1-D3).
func newTextMatcher(text string) searchMatcher {
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

// matchPair reports whether the matcher matches the label written as
// key=value (search-negation R2). Literal words are checked on the key and
// value without building the pair (design, Performance): label keys and
// values cannot hold `=`, so a word's only `=` must fall on the separator,
// the part before it ending the key and the part after it starting the
// value; a word without `=` matches when the key or the value holds it.
func (m searchMatcher) matchPair(key, value string) bool {
	if m.re != nil {
		if m.buf == nil {
			return m.re.MatchString(key + "=" + value)
		}
		b := append(append(append((*m.buf)[:0], key...), '='), value...)
		*m.buf = b
		return m.re.Match(b)
	}
	lkey, lvalue := strings.ToLower(key), strings.ToLower(value)
	for _, w := range m.words {
		i := strings.IndexByte(w, '=')
		switch {
		case i < 0:
			if strings.Contains(lkey, w) || strings.Contains(lvalue, w) {
				return true
			}
		case strings.IndexByte(w[i+1:], '=') < 0 && !strings.Contains(key, "=") && !strings.Contains(value, "="):
			if strings.HasSuffix(lkey, w[:i]) && strings.HasPrefix(lvalue, w[i+1:]) {
				return true
			}
		default: // a second `=`, or one in the label: the pair as text
			if strings.Contains(lkey+"="+lvalue, w) {
				return true
			}
		}
	}
	return false
}

// searchInvalid reports whether the search text has regex characters but is
// not a valid expression, so it is matched as text.
func searchInvalid(text string) bool { return newSearchMatcher(text).invalid }
