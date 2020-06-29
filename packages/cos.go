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
	"bytes"
	"fmt"
	"io/ioutil"
	"runtime"
	"strings"

	"github.com/GoogleCloudPlatform/osconfig/osinfo"
	"github.com/GoogleCloudPlatform/osconfig/util"
)

var (
	cosPkgList string
)

func init() {
	if runtime.GOOS != "windows" {
		cosPkgList = "/etc/package-list"
	}
	CosPkgListExists = util.Exists(cosPkgList)
}

var readMachineArch = func() (string, error) {
	oi, err := osinfo.Get()
	if err != nil {
		return "", fmt.Errorf("error getting osinfo: %v", err)
	}
	return oi.Architecture, nil
}

func parseInstalledCosPackages(data []byte) ([]PkgInfo, error) {
	arch, err := readMachineArch()
	if err != nil {
		return nil, fmt.Errorf("error from readMachineArch: %v", err)
	}

	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	var pkgs []PkgInfo
	for _, ln := range lines {
		pkg := bytes.Fields(ln)
		if len(pkg) != 2 {
			continue
		}
		pkgs = append(pkgs, PkgInfo{Name: string(pkg[0]), Arch: arch, Version: string(pkg[1])})
	}
	return pkgs, nil
}

var readCosPackageList = func() ([]byte, error) {
	return ioutil.ReadFile(cosPkgList)
}

// InstalledCosPackages queries for all installed COS packages.
func InstalledCosPackages() ([]PkgInfo, error) {
	b, err := readCosPackageList()
	DebugLogger.Printf("cosPkgList contents:\n%s", strings.ReplaceAll(string(b), "\n", "\n "))
	if err != nil {
		return nil, fmt.Errorf("error reading COS package list with args :%v, contents: %s", err, b)
	}
	return parseInstalledCosPackages(b)
}
