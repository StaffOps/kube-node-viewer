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

package recommend

// xenFamilies run on the Xen hypervisor, not Nitro (R8). The Price List does
// not say; this is AWS's published list of previous-hypervisor families.
var xenFamilies = map[string]bool{
	"a1": true, "c1": true, "c3": true, "c4": true, "d2": true, "g2": true, "g3": true,
	"i2": true, "i3": true, "m1": true, "m2": true, "m3": true, "m4": true, "p2": true,
	"p3": true, "r3": true, "r4": true, "t1": true, "t2": true, "x1": true, "x1e": true,
}

// generationTarget is the current-generation family of the same class and
// architecture for an old or Xen family (R8).
var generationTarget = map[string]string{
	"t1": "t3", "t2": "t3",
	"m1": "m7i", "m3": "m7i", "m4": "m7i",
	"c1": "c7i", "c3": "c7i", "c4": "c7i",
	"m2": "r7i", "r3": "r7i", "r4": "r7i",
	"i2": "i4i", "i3": "i4i",
	"d2": "d3",
	"x1": "x2iedn", "x1e": "x2iedn",
	"a1": "m7g",
	"g2": "g4dn", "g3": "g4dn",
}

// gravitonTarget is the arm64 counterpart of the same class (R9).
var gravitonTarget = map[string]string{
	"m5": "m7g", "m5a": "m7g", "m6i": "m7g", "m6a": "m7g", "m7i": "m7g", "m7a": "m7g", "m7i-flex": "m7g",
	"c5": "c7g", "c5a": "c7g", "c6i": "c7g", "c6a": "c7g", "c7i": "c7g", "c7a": "c7g",
	"r5": "r7g", "r5a": "r7g", "r6i": "r7g", "r6a": "r7g", "r7i": "r7g", "r7a": "r7g",
	"t3": "t4g", "t3a": "t4g",
}
