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

package model_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// sdkAllowed says whether a Go file under cmd/ or pkg/ may use the AWS SDK:
// AGENTS.md keeps it in pkg/aws and its wiring in cmd/<dir>/session.go.
// hack/ tools (hack/catalog) are outside cmd/ and pkg/, so exempt.
func sdkAllowed(rel string) bool {
	rel = filepath.ToSlash(rel)
	if strings.HasPrefix(rel, "pkg/aws/") {
		return true
	}
	parts := strings.Split(rel, "/")
	return len(parts) == 3 && parts[0] == "cmd" && parts[2] == "session.go"
}

// pkgAWSAllowed says whether a file may import pkg/aws, which pulls the SDK
// in: the files sdkAllowed accepts, and the wiring's test.
func pkgAWSAllowed(rel string) bool {
	if sdkAllowed(rel) {
		return true
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	return len(parts) == 3 && parts[0] == "cmd" && parts[2] == "session_test.go"
}

const pkgAWS = "github.com/staffops/kube-node-viewer/pkg/aws"

// TestAWSSDKStaysInPkgAWS fails when a file under cmd/ or pkg/, tests
// included, uses the AWS SDK outside the allowed places: directly (an import
// of github.com/aws/aws-sdk-go-v2), and through pkg/aws (an import of it,
// which pulls the SDK in). hack/ is not checked.
func TestAWSSDKStaysInPkgAWS(t *testing.T) {
	root := filepath.Join("..", "..")
	fset := token.NewFileSet()
	checked := 0
	for _, dir := range []string{"cmd", "pkg"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			checked++
			for _, imp := range f.Imports {
				p, _ := strconv.Unquote(imp.Path.Value)
				if strings.HasPrefix(p, "github.com/aws/aws-sdk-go-v2") && !sdkAllowed(rel) {
					t.Errorf("%s imports %s: the AWS SDK belongs in pkg/aws (and its wiring in cmd/*/session.go)", rel, p)
				}
				if p == pkgAWS && !pkgAWSAllowed(rel) {
					t.Errorf("%s imports %s, which pulls the AWS SDK in: only cmd/*/session.go wires it", rel, p)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if checked < 50 {
		t.Fatalf("checked only %d Go files under cmd/ and pkg/; the walk is not finding the tree", checked)
	}
}

func TestSDKAllowed(t *testing.T) {
	for rel, want := range map[string][2]bool{ // sdkAllowed, pkgAWSAllowed
		"pkg/aws/pricing.go":                   {true, true},
		"pkg/aws/region_test.go":               {true, true},
		"cmd/kube-node-viewer/session.go":      {true, true},
		"cmd/kube-node-viewer/session_test.go": {false, true},
		"cmd/kube-node-viewer/main.go":         {false, false},
		"cmd/a/b/session.go":                   {false, false},
		"cmd/session.go":                       {false, false},
		"pkg/model/node.go":                    {false, false},
		"pkg/awsx/x.go":                        {false, false},
		"pkg/client/controller.go":             {false, false},
		"cmd/kube-node-viewer/session_x.go":    {false, false},
	} {
		if got := sdkAllowed(rel); got != want[0] {
			t.Errorf("sdkAllowed(%q) = %v, want %v", rel, got, want[0])
		}
		if got := pkgAWSAllowed(rel); got != want[1] {
			t.Errorf("pkgAWSAllowed(%q) = %v, want %v", rel, got, want[1])
		}
	}
}
