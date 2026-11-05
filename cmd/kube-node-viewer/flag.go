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

// Modified by the kube-node-viewer authors, 2026.

package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"k8s.io/client-go/util/homedir"
)

const configFileName = ".kube-node-viewer"

var (
	homeDir string
	version = "dev"
	commit  = ""
	date    = ""
	builtBy = ""
)

func init() {
	homeDir = homedir.HomeDir()
}

type Flags struct {
	Context         string
	NodeSelector    string
	ExtraLabels     string
	NodeSort        string
	GroupBy         string
	Style           string
	Kubeconfig      string
	Resources       string
	LabelsAllow     string
	LabelsDeny      string
	GroupsOnly      bool
	DisablePricing  bool
	ShowAttribution bool
	Version         bool

	// Explicit holds the flags given on the command line, by name, as typed.
	// They win over saved view state for this run only (design D8).
	Explicit map[string]string
	// Config is the config file, a standing default below saved state.
	Config configFile
}

func ParseFlags() (Flags, error) {
	flagSet := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	var flags Flags

	cfg, err := loadConfigFile(configPath(homeDir))
	if err != nil {
		return Flags{}, fmt.Errorf("load config file: %w", err)
	}

	flagSet.BoolVar(&flags.Version, "v", false, "Display kube-node-viewer version")
	flagSet.BoolVar(&flags.Version, "version", false, "Display kube-node-viewer version")

	contextDefault := cfg.getValue("context", "")
	flagSet.StringVar(&flags.Context, "context", contextDefault, "Name of the kubernetes context to use")

	nodeSelectorDefault := cfg.getValue("node-selector", "")
	flagSet.StringVar(&flags.NodeSelector, "node-selector", nodeSelectorDefault, "Node label selector used to filter nodes, if empty all nodes are selected ")

	extraLabelsDefault := cfg.getValue("extra-labels", "")
	flagSet.StringVar(&flags.ExtraLabels, "extra-labels", extraLabelsDefault, "A comma separated set of extra node labels to display")

	groupByDefault := cfg.getValue("group-by", "")
	flagSet.StringVar(&flags.GroupBy, "group-by", groupByDefault, "Node label to summarize usage and cost by, optionally with a filter on a comma separated list of its values, e.g. 'karpenter.sh/nodepool' or 'karpenter.sh/nodepool=default,gpu'. Applies to this run; the last view chosen in the TUI is restored per cluster, unless config.yaml sets default-view with start-with: default-view")

	groupsOnlyDefault := cfg.getBoolValue("groups-only", false)
	flagSet.BoolVar(&flags.GroupsOnly, "groups-only", groupsOnlyDefault, "Start folded to the --group-by groups, with the node list hidden. At runtime 'n' folds and unfolds one level per press")

	nodeSort := cfg.getValue("node-sort", "creation=dsc")
	flagSet.StringVar(&flags.NodeSort, "node-sort", nodeSort, "Sort order for the nodes, either 'creation' or a label name. The sort order defaults to ascending and can be controlled by appending =asc or =dsc to the value.")

	style := cfg.getValue("style", "#04B575,#FFFF00,#FF0000")
	flagSet.StringVar(&flags.Style, "style", style, "Three color to use for styling 'good','ok' and 'bad' values. These are also used in the gradients displayed from bad -> good.")

	// flag overrides env. var. and env. var. overrides config file
	kubeconfigDefault := getStringEnv("KUBECONFIG", cfg.getValue("kubeconfig", filepath.Join(homeDir, ".kube", "config")))
	flagSet.StringVar(&flags.Kubeconfig, "kubeconfig", kubeconfigDefault, "Absolute path to the kubeconfig file")

	resourcesDefault := cfg.getValue("resources", "cpu")
	flagSet.StringVar(&flags.Resources, "resources", resourcesDefault, "List of comma separated resources to monitor")

	disablePricingDefault := cfg.getBoolValue("disable-pricing", false)
	flagSet.BoolVar(&flags.DisablePricing, "disable-pricing", disablePricingDefault, "Disable pricing lookups")

	// no defaults from the config file: the lists live in config.yaml, and a
	// flag replaces its list only when given (label-visibility R7, D4). The
	// help names the file this run reads, under XDG_CONFIG_HOME when set (R9).
	settingsFile := settingsPath(homeDir)
	flagSet.StringVar(&flags.LabelsAllow, "labels-allow", "", "Comma separated node label key patterns the panels offer ('*' matches anything), in place of labels.allow in "+settingsFile+" for this run; '' offers every label")
	flagSet.StringVar(&flags.LabelsDeny, "labels-deny", "", "Comma separated node label key patterns the panels leave out, in place of labels.deny in "+settingsFile+" for this run; '' denies none")

	flagSet.BoolVar(&flags.ShowAttribution, "attribution", false, "Show the Open Source Attribution")

	if err := flagSet.Parse(os.Args[1:]); err != nil {
		return Flags{}, err
	}
	flags.Explicit = map[string]string{}
	flagSet.Visit(func(f *flag.Flag) {
		flags.Explicit[f.Name] = f.Value.String()
	})
	flags.Config = cfg
	return flags, nil
}

// --- env vars ---

func getStringEnv(envName string, defaultValue string) string {
	env, ok := os.LookupEnv(envName)
	if !ok {
		return defaultValue
	}
	return env
}

// --- config file ---

type configFile map[string]string

func (c configFile) getValue(key string, defaultValue string) string {
	if val, ok := c[key]; ok {
		return val
	}
	return defaultValue
}

func (c configFile) getBoolValue(key string, defaultValue bool) bool {
	if val, ok := c[key]; ok {
		if boolVal, err := strconv.ParseBool(val); err == nil {
			return boolVal
		}
	}
	return defaultValue
}

// configPath is the config file in the home directory.
func configPath(home string) string {
	return filepath.Join(home, configFileName)
}

func loadConfigFile(configPath string) (configFile, error) {
	fileContent := make(map[string]string)
	if _, err := os.Stat(configPath); errors.Is(err, os.ErrNotExist) {
		return fileContent, nil
	}

	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") {
			continue
		}
		lineKV := strings.SplitN(line, "=", 2)
		if len(lineKV) == 2 {
			key := strings.TrimSpace(lineKV[0])
			value := strings.TrimSpace(lineKV[1])
			fileContent[key] = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return fileContent, nil
}
