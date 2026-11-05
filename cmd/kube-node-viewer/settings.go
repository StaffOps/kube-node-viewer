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
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/staffops/kube-node-viewer/pkg/model"
)

// settingsFileName is the YAML config file, under the config directory
// (label-visibility R9).
const settingsFileName = "config.yaml"

// settingsPath is $XDG_CONFIG_HOME/kube-node-viewer/config.yaml, or
// ~/.config/kube-node-viewer/config.yaml when XDG_CONFIG_HOME is unset,
// empty or not an absolute path, as the XDG Base Directory spec says.
func settingsPath(home string) string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "kube-node-viewer", settingsFileName)
}

// Values of start-with (config-views R4, R5).
const (
	startWithDefaultView = "default-view"
	startWithLastView    = "last-view"
)

// settings is what config.yaml holds once checked.
type settings struct {
	policy model.LabelPolicy
	// views are the config views, in file order
	views []configView
	// defaultView is the view each cluster opens with, nil without one
	defaultView *model.ViewState
	// lastView is start-with: last-view; a saved view wins over defaultView
	lastView bool
}

// unmarshalPrefix is what sigs.k8s.io/yaml puts before every decoding error.
const unmarshalPrefix = "error unmarshaling JSON: while decoding JSON: json: "

// yamlPrefix is what sigs.k8s.io/yaml puts before a YAML error, and
// yamlListPrefix what go-yaml puts before its list of decoding errors, one
// per line (a key written twice).
const (
	yamlPrefix     = "error converting YAML to JSON: yaml: "
	yamlListPrefix = "unmarshal errors:\n"
)

// yamlError is the decoding error on one line, without the library's
// prefixes: line 3: key "name" already set in map (fix-config-case R2).
func yamlError(err error) string {
	msg := strings.TrimPrefix(err.Error(), unmarshalPrefix)
	rest, ok := strings.CutPrefix(msg, yamlPrefix)
	if !ok {
		return msg
	}
	if list, ok := strings.CutPrefix(rest, yamlListPrefix); ok {
		var lines []string
		for _, l := range strings.Split(list, "\n") {
			if l = strings.TrimSpace(l); l != "" {
				lines = append(lines, l)
			}
		}
		rest = strings.Join(lines, "; ")
	}
	// an error with no line, unknown anchor 'a' referenced, still says what
	// kind of problem it is
	if !strings.HasPrefix(rest, "line ") {
		rest = "YAML: " + rest
	}
	return rest
}

// loadSettings reads config.yaml at path: the label lists, replaced by
// --labels-allow or --labels-deny when given on the command line
// (label-visibility R7), and the config views (config-views). A missing file
// or directory means no settings; any other problem is an error naming the
// file and the place in it (label-visibility R8, config-views R6).
func loadSettings(path string, explicit map[string]string) (settings, error) {
	file, err := readSettingsFile(path)
	if err != nil {
		return settings{}, fmt.Errorf("config file %s: %w", path, err)
	}
	allow, deny := file.allow, file.deny
	if v, ok := explicit["labels-allow"]; ok {
		allow = splitPatterns(v)
	}
	if v, ok := explicit["labels-deny"]; ok {
		deny = splitPatterns(v)
	}
	policy, err := model.NewLabelPolicy(allow, deny)
	if err != nil {
		return settings{}, fmt.Errorf("label lists: %w", err)
	}
	return settings{policy: policy, views: file.views, defaultView: file.defaultView, lastView: file.lastView}, nil
}

// settingsFile is config.yaml as written, checked but before the label flags.
type settingsFile struct {
	allow, deny []string
	views       []configView
	defaultView *model.ViewState
	lastView    bool
}

// readSettingsFile decodes the file into plain values and walks them by hand
// (config-views D5): keys compare exactly, unknown keys and wrong types are
// errors with their path, and scalars keep their YAML type, so 123, yes and
// ~ are not strings.
func readSettingsFile(path string) (settingsFile, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return settingsFile{}, nil
	}
	if err != nil {
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) {
			// the message names the file already
			return settingsFile{}, pathErr.Err
		}
		return settingsFile{}, err
	}
	var doc any
	// strict: a key written twice is an error
	if err := yaml.UnmarshalStrict(data, &doc); err != nil {
		return settingsFile{}, errors.New(yamlError(err))
	}
	top, err := mapping("", doc, "labels", "views", "default-view", "start-with")
	if err != nil {
		return settingsFile{}, err
	}
	var file settingsFile
	if file.allow, file.deny, err = readLabelLists(top["labels"]); err != nil {
		return settingsFile{}, err
	}
	if file.views, err = readViews(top["views"]); err != nil {
		return settingsFile{}, err
	}
	if err := file.readStartView(top); err != nil {
		return settingsFile{}, err
	}
	return file, nil
}

// readLabelLists checks labels.allow and labels.deny: lists of non-empty
// strings, either may be missing or null.
func readLabelLists(value any) (allow, deny []string, err error) {
	labels, err := mapping("labels", value, "allow", "deny")
	if err != nil {
		return nil, nil, err
	}
	for _, l := range []struct {
		key string
		out *[]string
	}{{"allow", &allow}, {"deny", &deny}} {
		items, err := list("labels."+l.key, labels[l.key])
		if err != nil {
			return nil, nil, err
		}
		if *l.out, err = patternList("labels."+l.key, items); err != nil {
			return nil, nil, err
		}
	}
	return allow, deny, nil
}

// readStartView checks default-view and start-with.
func (f *settingsFile) readStartView(top map[string]any) error {
	name, hasDefault, err := optionalString("default-view", top)
	if err != nil {
		return err
	}
	startWith, hasStartWith, err := optionalString("start-with", top)
	if err != nil {
		return err
	}
	switch {
	case !hasStartWith, startWith == startWithDefaultView:
		if hasStartWith && !hasDefault {
			return fmt.Errorf("start-with: %s needs default-view", startWithDefaultView)
		}
	case startWith == startWithLastView:
		f.lastView = true
	default:
		return fmt.Errorf("start-with: %q is not %s or %s", startWith, startWithDefaultView, startWithLastView)
	}
	if !hasDefault {
		return nil
	}
	for i := range f.views {
		if f.views[i].name == name {
			view := f.views[i].view
			f.defaultView = &view
			return nil
		}
	}
	return fmt.Errorf("default-view: no view is named %q", name)
}

// optionalString is a string setting of the top level; ok is false when the
// key is not in the file.
func optionalString(key string, top map[string]any) (value string, ok bool, err error) {
	raw, ok := top[key]
	if !ok {
		return "", false, nil
	}
	value, err = str(key, raw)
	return value, true, err
}

// mapping checks that value is a map, or null for none, with only the given
// keys, compared exactly. path names the value in errors; the top level has
// none.
func mapping(path string, value any, keys ...string) (map[string]any, error) {
	if value == nil {
		return map[string]any{}, nil
	}
	m, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s is not a map of keys", orFile(path))
	}
	// sorted, so the error is the same on every run
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		if !slices.Contains(keys, k) {
			if path == "" {
				return nil, fmt.Errorf("unknown field %q", k)
			}
			return nil, fmt.Errorf("%s: unknown field %q", path, k)
		}
	}
	return m, nil
}

// orFile is path, or "the file" for the top level.
func orFile(path string) string {
	if path == "" {
		return "the file"
	}
	return path
}

// list checks that value is a list, or null for none.
func list(path string, value any) ([]any, error) {
	if value == nil {
		return nil, nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("%s: %s is not a list", path, describeItem(value))
	}
	return items, nil
}

// str checks that value is a non-empty string.
func str(path string, value any) (string, error) {
	s, ok := value.(string)
	switch {
	case !ok:
		return "", notString(path, value)
	case strings.TrimSpace(s) == "":
		return "", fmt.Errorf("%s: empty", path)
	}
	return s, nil
}

// notString is the error for a value that should be a string: quoting is
// the fix for a scalar YAML read as another type, not for a map or a list
// (fix-config-errors R4).
func notString(path string, value any) error {
	switch value.(type) {
	case map[string]any:
		return fmt.Errorf("%s: %s is a map, not a string", path, describeItem(value))
	case []any:
		return fmt.Errorf("%s: %s is a list, not a string", path, describeItem(value))
	}
	return fmt.Errorf("%s: %s is not a string; quote it", path, describeItem(value))
}

// unpadded refuses a string with leading or trailing whitespace
// (fix-config-errors R1, R2).
func unpadded(path, s string) error {
	if strings.TrimSpace(s) != s {
		return fmt.Errorf("%s: %q has leading or trailing whitespace", path, s)
	}
	return nil
}

// boolean checks that value is true or false; YAML also reads yes/no and
// on/off as those.
func boolean(path string, value any) (bool, error) {
	b, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("%s: %s is not true or false", path, describeItem(value))
	}
	return b, nil
}

// patternList checks that every item is a non-empty YAML string, naming the
// first that is not by its position, e.g. labels.allow[2].
func patternList(name string, items []any) ([]string, error) {
	var out []string
	for i, item := range items {
		path := fmt.Sprintf("%s[%d]", name, i)
		s, ok := item.(string)
		switch {
		case !ok:
			return nil, notString(path, item)
		case s == "":
			return nil, fmt.Errorf("%s: empty pattern", path)
		}
		if err := unpadded(path, s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

// describeItem shows a decoded YAML item close to the way the user wrote
// it, as far as JSON keeps it: maps as {key: value} with sorted keys, lists
// as [a, b], whole numbers without an exponent (fix-config-errors R4).
func describeItem(item any) string {
	switch v := item.(type) {
	case nil:
		return "null"
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = k + ": " + describeItem(v[k])
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case []any:
		parts := make([]string, len(v))
		for i, e := range v {
			parts[i] = describeItem(e)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case float64:
		if v == math.Trunc(v) && math.Abs(v) < 1e21 {
			return strconv.FormatFloat(v, 'f', -1, 64)
		}
		return strconv.FormatFloat(v, 'g', -1, 64)
	}
	return fmt.Sprint(item)
}

// splitPatterns is a flag value: comma-separated patterns, trimmed; an empty
// value is no list.
func splitPatterns(v string) []string {
	var out []string
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
