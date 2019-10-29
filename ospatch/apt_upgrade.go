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
	"os/exec"

	"github.com/GoogleCloudPlatform/guest-logging-go/logger"
	"github.com/GoogleCloudPlatform/osconfig/inventory/packages"
)

type aptGetUpgradeOpts struct {
	upgradeType       packages.AptUpgradeType
	exclusivePackages []string
	excludes          []string
	dryrun            bool

	runner func(cmd *exec.Cmd) ([]byte, error)
}

// AptGetUpgradeOption is an option for apt-get update.
type AptGetUpgradeOption func(*aptGetUpgradeOpts)

// AptGetUpgradeType returns a AptGetUpgradeOption that specifies upgrade type.
func AptGetUpgradeType(upgradeType packages.AptUpgradeType) AptGetUpgradeOption {
	return func(args *aptGetUpgradeOpts) {
		args.upgradeType = upgradeType
	}
}

// AptGetExcludes excludes these packages from upgrade.
func AptGetExcludes(excludes []string) AptGetUpgradeOption {
	return func(args *aptGetUpgradeOpts) {
		args.excludes = excludes
	}
}

// AptGetExclusivePackages includes only these packages in the upgrade.
func AptGetExclusivePackages(exclusivePackages []string) AptGetUpgradeOption {
	return func(args *aptGetUpgradeOpts) {
		args.exclusivePackages = exclusivePackages
	}
}

// AptGetDryRun performs a dry run.
func AptGetDryRun(dryrun bool) AptGetUpgradeOption {
	return func(args *aptGetUpgradeOpts) {
		args.dryrun = dryrun
	}
}

// RunAptGetUpgrade runs apt-get upgrade.
func RunAptGetUpgrade(opts ...AptGetUpgradeOption) error {
	aptOpts := &aptGetUpgradeOpts{
		upgradeType: packages.AptGetUpgrade,
		dryrun:      false,
	}

	for _, opt := range opts {
		opt(aptOpts)
	}

	pkgs, err := packages.AptUpdates(packages.AptGetUpgradeType(aptOpts.upgradeType), packages.AptGetUpgradeShowNew(true))
	if err != nil {
		return err
	}

	fPkgs, err := filterPackages(pkgs, aptOpts.exclusivePackages, aptOpts.excludes)
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

	if aptOpts.dryrun {
		logger.Infof("Running in dryrun mode, not updating packages.")
		return nil
	}

	return packages.InstallAptPackages(pkgNames)
}
