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
	"context"
	"encoding/json"
	"runtime"

	"github.com/GoogleCloudPlatform/osconfig/util"
)

var (
	pip string

	pipListArgs     = []string{"list", "--format=json"}
	pipOutdatedArgs = append(pipListArgs, "--outdated")
)

func init() {
	if runtime.GOOS != "windows" {
		pip = "/usr/bin/pip"
	}
	PipExists = util.Exists(pip)
}

type pipUpdatesPkg struct {
	Name          string `json:"name"`
	LatestVersion string `json:"latest_version"`
}

type pipInstalledPkg struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// PipUpdates queries for all available pip updates.
func PipUpdates(ctx context.Context) ([]PkgInfo, error) {
	out, err := run(ctx, pip, pipOutdatedArgs)
	if err != nil {
		return nil, err
	}

	var pipUpdates []pipUpdatesPkg
	if err := json.Unmarshal(out, &pipUpdates); err != nil {
		return nil, err
	}

	var pkgs []PkgInfo
	for _, pkg := range pipUpdates {
		pkgs = append(pkgs, PkgInfo{Name: pkg.Name, Arch: noarch, Version: pkg.LatestVersion})
	}

	return pkgs, nil
}

// InstalledPipPackages queries for all installed pip packages.
func InstalledPipPackages(ctx context.Context) ([]PkgInfo, error) {
	out, err := run(ctx, pip, pipListArgs)
	if err != nil {
		return nil, err
	}

	var pipUpdates []pipInstalledPkg
	if err := json.Unmarshal(out, &pipUpdates); err != nil {
		return nil, err
	}

	var pkgs []PkgInfo
	for _, pkg := range pipUpdates {
		pkgs = append(pkgs, PkgInfo{Name: pkg.Name, Arch: noarch, Version: pkg.Version})
	}

	return pkgs, nil
}
