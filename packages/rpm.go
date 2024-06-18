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
	"context"
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/GoogleCloudPlatform/osconfig/clog"
	"github.com/GoogleCloudPlatform/osconfig/util"
)

var (
	rpmquery string
	rpm      string

	rpmqueryFields = map[string]string{
		"package":      "%{NAME}",
		"architecture": "%{ARCH}",
		// %|EPOCH?{%{EPOCH}:}:{}| == if EPOCH then prepend "%{EPOCH}:" to version.
		"version":     "%|EPOCH?{%{EPOCH}:}:{}|%{VERSION}-%{RELEASE}",
		"source_name": "%{SOURCERPM}",
	}

	rpmInstallArgs = []string{"--upgrade", "--replacepkgs", "-v"}
	rpmqueryArgs   = []string{"--queryformat", formatFieldsMappingToFormattingString(rpmqueryFields)}

	rpmqueryInstalledArgs = append(rpmqueryArgs, "-a")
	rpmqueryRPMArgs       = append(rpmqueryArgs, "-p")
)

func init() {
	if runtime.GOOS != "windows" {
		rpmquery = "/usr/bin/rpmquery"
		rpm = "/bin/rpm"
	}
	RPMQueryExists = util.Exists(rpmquery)
	RPMExists = util.Exists(rpm)
}

func parseInstalledRPMPackages(ctx context.Context, data []byte) []*PkgInfo {
	/*
		Each line contains an entry in a json format, keep in mind that whole output is not valid json.

		{"architecture":"x86_64","package":"gcc","source_name":"gcc-11.4.1-3.el9.src.rpm","version":"11.4.1-3.el9"}
		{"architecture":"noarch","package":"golang-src","source_name":"golang-1.22.3-1.el9.src.rpm","version":"1.22.3-1.el9"}
	*/

	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))

	var result []*PkgInfo
	for _, entry := range lines {
		var rpm packageMetadata
		if err := json.Unmarshal(entry, &rpm); err != nil {
			clog.Debugf(ctx, "unable to parse rpm package info, err %s, raw - %s", err, string(entry))
			continue
		}

		pkg := pkgInfoFromPackageMetadata(rpm)
		result = append(result, pkg)
	}

	return result
}

// InstalledRPMPackages queries for all installed rpm packages.
func InstalledRPMPackages(ctx context.Context) ([]*PkgInfo, error) {
	out, err := run(ctx, rpmquery, rpmqueryInstalledArgs)
	if err != nil {
		return nil, err
	}

	return parseInstalledRPMPackages(ctx, out), nil
}

// RPMInstall installs an rpm packages.
func RPMInstall(ctx context.Context, path string) error {
	_, err := run(ctx, rpm, append(rpmInstallArgs, path))
	return err
}

// RPMPkgInfo gets PkgInfo from a rpm package.
func RPMPkgInfo(ctx context.Context, path string) (*PkgInfo, error) {
	out, err := run(ctx, rpmquery, append(rpmqueryRPMArgs, path))
	if err != nil {
		return nil, err
	}

	pkgs := parseInstalledRPMPackages(ctx, out)
	if len(pkgs) != 1 {
		return nil, fmt.Errorf("unexpected number of parsed rpm packages %d: %q", len(pkgs), out)
	}
	return pkgs[0], nil
}
