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

// Azure tables (aks-recommendations D5), keyed by the normalised SkuFamily.

// azureTempDisk are the families with a temp disk, from each family's docs
// "Local Storage" line (read 2026-11-10). A family with a temp disk maps only
// to a family with one.
var azureTempDisk = map[string]bool{
	"dv2": true, "dsv2": true, "dv3": true, "dsv3": true, "dav4": true, "dasv4": true,
	"ddv4": true, "ddsv4": true, "ddv5": true, "ddsv5": true, "dadsv5": true, "ddsv6": true, "dadv6": true,
	"dldsv5": true, "dldsv6": true, "daldv6": true,
	"ev3": true, "esv3": true, "eav4": true, "easv4": true, "edv4": true, "edsv4": true,
	"edv5": true, "edsv5": true, "eadsv5": true, "edsv6": true, "eadv6": true,
	"bs": true, "dpdsv5": true, "dpldsv5": true, "epdsv5": true,
}

// azureGeneration is the newer x86 family of the same class and features,
// temp disk kept (R8). Targets are v5: v6 Intel costs more.
var azureGeneration = map[string]string{
	"dv2": "ddv5", "dv3": "ddv5", "dsv2": "ddsv5", "dsv3": "ddsv5",
	"dv4": "dv5", "dsv4": "dsv5",
	"ddv4": "ddv5", "ddsv4": "ddsv5",
	"dav4": "dadsv5", "dasv4": "dadsv5",
	"ev3": "edv5", "esv3": "edsv5", "ev4": "ev5", "esv4": "esv5",
	"edv4": "edv5", "edsv4": "edsv5", "eav4": "eadsv5", "easv4": "eadsv5",
}

// azureArm is the Arm64 counterpart among the v5 families AKS documents for
// Arm64 node pools (Dpsv5, Dplsv5, Epsv5 and their d variants; decided by the
// owner, 2026-11-10), temp disk kept (R9).
var azureArm = map[string]string{
	"dv4": "dpsv5", "dv5": "dpsv5", "dsv4": "dpsv5", "dsv5": "dpsv5", "dsv6": "dpsv5", "dasv5": "dpsv5", "dav6": "dpsv5",
	"dv2": "dpdsv5", "dsv2": "dpdsv5", "dv3": "dpdsv5", "dsv3": "dpdsv5", "dav4": "dpdsv5", "dasv4": "dpdsv5",
	"ddv4": "dpdsv5", "ddv5": "dpdsv5", "ddsv4": "dpdsv5", "ddsv5": "dpdsv5", "ddsv6": "dpdsv5", "dadsv5": "dpdsv5", "dadv6": "dpdsv5",
	"dlsv5": "dplsv5", "dlsv6": "dplsv5", "dalv6": "dplsv5",
	"dldsv5": "dpldsv5", "dldsv6": "dpldsv5", "daldv6": "dpldsv5",
	"ev4": "epsv5", "ev5": "epsv5", "esv4": "epsv5", "esv5": "epsv5", "esv6": "epsv5", "easv5": "epsv5", "eav6": "epsv5",
	"ev3": "epdsv5", "esv3": "epdsv5", "eav4": "epdsv5", "easv4": "epdsv5", "edv4": "epdsv5", "edv5": "epdsv5",
	"edsv4": "epdsv5", "edsv5": "epdsv5", "edsv6": "epdsv5", "eadsv5": "epdsv5", "eadv6": "epdsv5",
}

// azureBSeries are the B-series families, never a system pool's size (R15).
var azureBSeries = map[string]bool{"bs": true, "bsv2": true, "basv2": true, "bpsv2": true}

// azureNoArm lists the x86 d*/e* families of v2–v6 that have no Arm
// counterpart, with the reason; the coverage test checks every other one is
// a key of azureArm (D5).
var azureNoArm = map[string]string{
	"dcsv2": "confidential", "dcsv3": "confidential", "ddcsv3": "confidential",
	"dcasv5": "confidential", "dcadsv5": "confidential", "dcasv6": "confidential", "dcadsv6": "confidential",
	"dcaccv5": "confidential", "dcadccv5": "confidential", "dcev5": "confidential", "dcedv5": "confidential",
	"dcev6": "confidential", "dcedv6": "confidential",
	"ecasv5": "confidential", "ecadsv5": "confidential", "ecasv6": "confidential", "ecadsv6": "confidential",
	"ecaccv5": "confidential", "ecadccv5": "confidential", "ecev5": "confidential", "ecedv5": "confidential",
	"ecev6": "confidential", "ecedv6": "confidential",
	"eciasv5": "confidential", "eciadsv5": "confidential", "eciev5": "confidential", "eciedv5": "confidential",
	"eiv3": "isolated", "eisv3": "isolated", "eiv5": "isolated", "eisv5": "isolated", "eidv5": "isolated", "eidsv5": "isolated",
	"eiasv4": "isolated", "eiasv5": "isolated", "eiadsv5": "isolated", "eibsv5": "isolated", "eibdsv5": "isolated",
	"ebsv5": "remote storage", "ebdsv5": "remote storage",
}
