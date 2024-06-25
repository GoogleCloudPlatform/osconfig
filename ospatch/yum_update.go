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

const yum = "/usr/bin/yum"

var (
	yumUpdateArgs        = []string{"update", "-y"}
	yumUpdateMinimalArgs = []string{"update-minimal", "-y"}
)

type yumUpdateOpts struct {
	exclusivePackages []string
	excludes          []*Exclude
	security          bool
	minimal           bool
	dryrun            bool
}

// YumUpdateOption is an option for yum update.
type YumUpdateOption func(*yumUpdateOpts)

// YumUpdateSecurity returns a YumUpdateOption that specifies the --security flag should
// be used.
func YumUpdateSecurity(security bool) YumUpdateOption {
	return func(args *yumUpdateOpts) {
		args.security = security
	}
}

// YumUpdateMinimal returns a YumUpdateOption that specifies the update-minimal
// command should be used.
func YumUpdateMinimal(minimal bool) YumUpdateOption {
	return func(args *yumUpdateOpts) {
		args.minimal = minimal
	}
}

// YumUpdateExcludes returns a YumUpdateOption that specifies what packages to add to
// the --exclude flag.
func YumUpdateExcludes(excludes []*Exclude) YumUpdateOption {
	return func(args *yumUpdateOpts) {
		args.excludes = excludes
	}
}

// YumExclusivePackages includes only these packages in the upgrade.
func YumExclusivePackages(exclusivePackages []string) YumUpdateOption {
	return func(args *yumUpdateOpts) {
		args.exclusivePackages = exclusivePackages
	}
}

// YumDryRun performs a dry run.
func YumDryRun(dryrun bool) YumUpdateOption {
	return func(args *yumUpdateOpts) {
		args.dryrun = dryrun
	}
}

// RunYumUpdate runs yum update.
func RunYumUpdate(ctx context.Context, opts ...YumUpdateOption) error {
	yumOpts := &yumUpdateOpts{
		security: false,
		minimal:  false,
		dryrun:   false,
	}

	for _, opt := range opts {
		opt(yumOpts)
	}

	pkgs, err := packages.YumUpdates(ctx, packages.YumUpdateMinimal(yumOpts.minimal), packages.YumUpdateSecurity(yumOpts.security))
	if err != nil {
		return err
	}

	// Yum excludes are already excluded while listing yumUpdates, so we send
	// and empty list.
	fPkgs, err := filterPackages(pkgs, yumOpts.exclusivePackages, yumOpts.excludes)
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
	if yumOpts.dryrun {
		clog.Infof(ctx, "Running in dryrun mode, not updating %s", msg)
		return nil
	}
	ops := opsToReport{
		packages: fPkgs,
	}

	logOps(ctx, ops)

	err = packages.InstallYumPackages(ctx, pkgNames)
	if err == nil {
		logSuccess(ctx, ops)
	} else {
		logFailure(ctx, ops, err)
	}
	return err
}
