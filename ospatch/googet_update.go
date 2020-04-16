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

package ospatch

import (
	"github.com/GoogleCloudPlatform/guest-logging-go/logger"
	"github.com/GoogleCloudPlatform/osconfig/packages"
)

type googetUpdateOpts struct {
	exclusivePackages []string
	excludes          []string
	dryrun            bool
}

// GooGetUpdateOption is an option for apt-get update.
type GooGetUpdateOption func(*googetUpdateOpts)

// GooGetExcludes excludes these packages from upgrade.
func GooGetExcludes(excludes []string) GooGetUpdateOption {
	return func(args *googetUpdateOpts) {
		args.excludes = excludes
	}
}

// GooGetExclusivePackages includes only these packages in the upgrade.
func GooGetExclusivePackages(exclusivePackages []string) GooGetUpdateOption {
	return func(args *googetUpdateOpts) {
		args.exclusivePackages = exclusivePackages
	}
}

// GooGetDryRun performs a dry run.
func GooGetDryRun(dryrun bool) GooGetUpdateOption {
	return func(args *googetUpdateOpts) {
		args.dryrun = dryrun
	}
}

// RunGooGetUpdate runs googet update.
func RunGooGetUpdate(opts ...GooGetUpdateOption) error {
	googetOpts := &googetUpdateOpts{}

	for _, opt := range opts {
		opt(googetOpts)
	}

	pkgs, err := packages.GooGetUpdates()
	if err != nil {
		return err
	}

	fPkgs, err := filterPackages(pkgs, googetOpts.exclusivePackages, googetOpts.excludes)
	if err != nil {
		return err
	}
	if len(fPkgs) == 0 {
		logger.Infof("No packages to update.")
		return nil
	}

	var pkgNames []string
	for _, pkg := range fPkgs {
		pkgNames = append(pkgNames, pkg.Name)
	}
	logger.Infof("Updating %d packages.", len(pkgNames))
	logger.Debugf("Packages to be installed: %s", fPkgs)

	if googetOpts.dryrun {
		logger.Infof("Running in dryrun mode, not updating packages.")
		return nil
	}

	return packages.InstallGooGetPackages(pkgNames)
}
