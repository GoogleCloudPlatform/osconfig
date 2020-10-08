//  Copyright 2020 Google Inc. All Rights Reserved.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package packages

import (
	"fmt"
	"strconv"

	"cos.googlesource.com/cos/tools.git/src/pkg/cos"
	"github.com/GoogleCloudPlatform/osconfig/osinfo"
)

func init() {
	COSPkgInfoExists = cos.PackageInfoExists()
}

var readMachineArch = func() (string, error) {
	oi, err := osinfo.Get()
	if err != nil {
		return "", fmt.Errorf("error getting osinfo: %v", err)
	}
	return oi.Architecture, nil
}

func parseInstalledCOSPackages(cosPkgInfo cos.PackageInfo) ([]PkgInfo, error) {
	arch, err := readMachineArch()
	if err != nil {
		return nil, fmt.Errorf("error from readMachineArch: %v", err)
	}

	var pkgs []PkgInfo
	for _, pkg := range cosPkgInfo.InstalledPackages {
		name := pkg.Category + "/" + pkg.Name
		version := pkg.Version
		if pkg.Revision != 0 {
			version += "-r" + strconv.Itoa(pkg.Revision)
		}
		pkgs = append(pkgs, PkgInfo{Name: name, Arch: arch, Version: version})
	}
	return pkgs, nil
}

var readCOSPackageInfo = func() (cos.PackageInfo, error) {
	return cos.GetPackageInfo()
}

// InstalledCOSPackages queries for all installed COS packages.
func InstalledCOSPackages() ([]PkgInfo, error) {
	packageInfo, err := readCOSPackageInfo()
	if err != nil {
		return nil, fmt.Errorf("error reading COS package list with args: %v, contents: %v", err, packageInfo)
	}
	return parseInstalledCOSPackages(packageInfo)
}
