//go:build windows

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

import "os/exec"

// killTree kills only az.cmd on Windows; its Python child may outlive a
// timeout and exits on its own (azure-live-skus D1, accepted).
func killTree(*exec.Cmd) {}
