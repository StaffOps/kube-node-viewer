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

// sdkAllowed says whether a Go file under cmd/ or pkg/ may import the AWS
// SDK: AGENTS.md keeps the SDK in pkg/aws and its wiring in cmd/*/session.go.
func sdkAllowed(rel string) bool {
	rel = filepath.ToSlash(rel)
	if strings.HasPrefix(rel, "pkg/aws/") {
		return true
	}
	return strings.HasPrefix(rel, "cmd/") && filepath.Base(rel) == "session.go"
}

// TestAWSSDKStaysInPkgAWS fails when a file under cmd/ or pkg/, tests
// included, imports github.com/aws/aws-sdk-go-v2 outside the allowed places.
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
	for rel, want := range map[string]bool{
		"pkg/aws/pricing.go":                true,
		"pkg/aws/region_test.go":            true,
		"cmd/kube-node-viewer/session.go":   true,
		"cmd/kube-node-viewer/main.go":      false,
		"pkg/model/node.go":                 false,
		"pkg/awsx/x.go":                     false,
		"pkg/client/controller.go":          false,
		"cmd/kube-node-viewer/session_x.go": false,
	} {
		if got := sdkAllowed(rel); got != want {
			t.Errorf("sdkAllowed(%q) = %v, want %v", rel, got, want)
		}
	}
}
