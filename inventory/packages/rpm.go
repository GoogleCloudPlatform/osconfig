//  Copyright 2019 Google Inc. All Rights Reserved.
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
	"os/exec"
	"runtime"

	"github.com/GoogleCloudPlatform/osconfig/inventory/osinfo"
	"github.com/GoogleCloudPlatform/osconfig/util"
)

var (
	rpmquery string

	rpmqueryArgs = []string{"-a", "--queryformat", "%{NAME} %{ARCH} %{VERSION}-%{RELEASE}\n"}
)

func init() {
	if runtime.GOOS != "windows" {
		rpmquery = "/usr/bin/rpmquery"
	}
}

func parseInstalledRPMPackages(data []byte) []PkgInfo {
	/*
	   foo x86_64 1.2.3-4
	   bar noarch 1.2.3-4
	   ...
	*/
	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))

	var pkgs []PkgInfo
	for _, ln := range lines {
		pkg := bytes.Fields(ln)
		if len(pkg) != 3 {
			continue
		}

		pkgs = append(pkgs, PkgInfo{Name: string(pkg[0]), Arch: osinfo.Architecture(string(pkg[1])), Version: string(pkg[2])})
	}
	return pkgs
}

// InstalledRPMPackages queries for all installed rpm packages.
func InstalledRPMPackages() ([]PkgInfo, error) {
	out, err := util.Run(exec.Command(rpmquery, rpmqueryArgs...), DebugLogger)
	if err != nil {
		return nil, err
	}

	return parseInstalledRPMPackages(out), nil
}
