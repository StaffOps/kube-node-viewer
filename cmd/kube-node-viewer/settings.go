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
	"os"
	"path/filepath"
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

// settingsFile is the YAML file's shape. The list items are decoded as any so
// that 123, yes and ~ are rejected instead of becoming strings (design D1).
type settingsFile struct {
	Labels labelLists `json:"labels"`
}

type labelLists struct {
	Allow []any `json:"allow"`
	Deny  []any `json:"deny"`
}

// unmarshalPrefix is what sigs.k8s.io/yaml puts before every decoding error.
const unmarshalPrefix = "error unmarshaling JSON: while decoding JSON: json: "

// loadSettings reads the label lists from the config file at path, replaces
// either with --labels-allow or --labels-deny when given on the command line
// (R7), and compiles them. A missing file or directory means no lists; any
// other problem is an error naming the file (R8).
func loadSettings(path string, explicit map[string]string) (model.LabelPolicy, error) {
	allow, deny, err := readLabelLists(path)
	if err != nil {
		return model.LabelPolicy{}, fmt.Errorf("config file %s: %w", path, err)
	}
	if v, ok := explicit["labels-allow"]; ok {
		allow = splitPatterns(v)
	}
	if v, ok := explicit["labels-deny"]; ok {
		deny = splitPatterns(v)
	}
	policy, err := model.NewLabelPolicy(allow, deny)
	if err != nil {
		return model.LabelPolicy{}, fmt.Errorf("label lists: %w", err)
	}
	return policy, nil
}

// readLabelLists decodes labels.allow and labels.deny strictly: unknown keys,
// wrong types and items that are not non-empty strings are errors.
func readLabelLists(path string) (allow, deny []string, err error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) {
			// the message names the file already
			return nil, nil, pathErr.Err
		}
		return nil, nil, err
	}
	var s settingsFile
	if err := yaml.UnmarshalStrict(data, &s); err != nil {
		return nil, nil, errors.New(strings.TrimPrefix(err.Error(), unmarshalPrefix))
	}
	if allow, err = patternList("labels.allow", s.Labels.Allow); err != nil {
		return nil, nil, err
	}
	if deny, err = patternList("labels.deny", s.Labels.Deny); err != nil {
		return nil, nil, err
	}
	return allow, deny, nil
}

// patternList checks that every item is a non-empty YAML string, naming the
// first that is not by its position, e.g. labels.allow[2].
func patternList(name string, items []any) ([]string, error) {
	var out []string
	for i, item := range items {
		s, ok := item.(string)
		switch {
		case !ok:
			return nil, fmt.Errorf("%s[%d]: %s is not a string; quote it", name, i, describeItem(item))
		case s == "":
			return nil, fmt.Errorf("%s[%d]: empty pattern", name, i)
		}
		out = append(out, s)
	}
	return out, nil
}

// describeItem shows a decoded YAML item the way the user wrote it, as far as
// JSON keeps it.
func describeItem(item any) string {
	if item == nil {
		return "null"
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
