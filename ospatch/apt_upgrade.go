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
	"context"
	"fmt"

	"github.com/GoogleCloudPlatform/osconfig/clog"
	"github.com/GoogleCloudPlatform/osconfig/packages"
)

type aptGetUpgradeOpts struct {
	exclusivePackages []string
	excludes          []*Exclude
	upgradeType       packages.AptUpgradeType
	dryrun            bool
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
func AptGetExcludes(excludes []*Exclude) AptGetUpgradeOption {
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
func RunAptGetUpgrade(ctx context.Context, opts ...AptGetUpgradeOption) error {
	aptOpts := &aptGetUpgradeOpts{
		upgradeType:       packages.AptGetUpgrade,
		excludes:          nil,
		exclusivePackages: nil,
		dryrun:            false,
	}

	for _, opt := range opts {
		opt(aptOpts)
	}

	pkgs, err := packages.AptUpdates(ctx, packages.AptGetUpgradeType(aptOpts.upgradeType), packages.AptGetUpgradeShowNew(true))
	if err != nil {
		return err
	}

	fPkgs, err := filterPackages(pkgs, aptOpts.exclusivePackages, aptOpts.excludes)
	if err != nil {
		return err
	}
	if len(fPkgs) == 0 {
		clog.Infof(ctx, "No packages to update.")
		return nil
	}

	var pkgNames []string
	for _, pkg := range fPkgs {
		pkgNames = append(pkgNames, pkg.Name)
	}

	msg := fmt.Sprintf("%d packages: %q", len(pkgNames), fPkgs)
	if aptOpts.dryrun {
		clog.Infof(ctx, "Running in dryrun mode, not updating %s", msg)
		return nil
	}

	ops := opsToReport{
		packages: fPkgs,
	}
	logOps(ctx, ops)

	err = packages.InstallAptPackages(ctx, pkgNames)
	if err == nil {
		logSuccess(ctx, ops)
	} else {
		logFailure(ctx, ops, err)
	}

	return err
}
