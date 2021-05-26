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

const zypper = "/usr/bin/zypper"

var (
	zypperPatchArgs = []string{"patch", "-y"}
)

type zypperPatchOpts struct {
	categories       []string
	severities       []string
	excludes         []string
	exclusivePatches []string
	withOptional     bool
	withUpdate       bool
	dryrun           bool
}

// ZypperPatchOption is an option for zypper patch.
type ZypperPatchOption func(*zypperPatchOpts)

// ZypperPatchCategories returns a ZypperUpdateOption that specifies what
// categories to add to the --categories flag.
func ZypperPatchCategories(categories []string) ZypperPatchOption {
	return func(args *zypperPatchOpts) {
		args.categories = categories
	}
}

// ZypperPatchSeverities returns a ZypperUpdateOption that specifies what
// categories to add to the --categories flag.
func ZypperPatchSeverities(severities []string) ZypperPatchOption {
	return func(args *zypperPatchOpts) {
		args.severities = severities
	}
}

// ZypperUpdateWithOptional returns a ZypperUpdateOption that specifies the
// --with-optional flag should be used.
func ZypperUpdateWithOptional(withOptional bool) ZypperPatchOption {
	return func(args *zypperPatchOpts) {
		args.withOptional = withOptional
	}
}

// ZypperUpdateWithUpdate returns a ZypperUpdateOption that specifies the
// --with-update flag should be used.
func ZypperUpdateWithUpdate(withUpdate bool) ZypperPatchOption {
	return func(args *zypperPatchOpts) {
		args.withUpdate = withUpdate
	}
}

// ZypperUpdateWithExcludes returns a ZypperUpdateOption that specifies
// list of packages to be excluded from update
func ZypperUpdateWithExcludes(excludes []string) ZypperPatchOption {
	return func(args *zypperPatchOpts) {
		args.excludes = excludes
	}
}

// ZypperUpdateWithExclusivePatches returns a ZypperUpdateOption that specifies
// list of exclusive packages to be updated
func ZypperUpdateWithExclusivePatches(exclusivePatches []string) ZypperPatchOption {
	return func(args *zypperPatchOpts) {
		args.exclusivePatches = exclusivePatches
	}
}

// ZypperUpdateDryrun returns a ZypperUpdateOption that specifies the runner.
func ZypperUpdateDryrun(dryrun bool) ZypperPatchOption {
	return func(args *zypperPatchOpts) {
		args.dryrun = dryrun
	}
}

// RunZypperPatch runs zypper patch.
func RunZypperPatch(ctx context.Context, opts ...ZypperPatchOption) error {
	zOpts := &zypperPatchOpts{
		excludes:         nil,
		exclusivePatches: nil,
		categories:       nil,
		severities:       nil,
		withOptional:     false,
		withUpdate:       false,
	}

	for _, opt := range opts {
		opt(zOpts)
	}

	zListOpts := []packages.ZypperListOption{
		packages.ZypperListPatchCategories(zOpts.categories),
		packages.ZypperListPatchSeverities(zOpts.severities),
		packages.ZypperListPatchWithOptional(zOpts.withOptional),
		// if there is no filter on category and severity,
		// zypper fetches all available patch updates
	}
	patches, err := packages.ZypperPatches(ctx, zListOpts...)
	if err != nil {
		return err
	}

	// if user specifies, --with-update get the necessary patch/package
	// information and then runfilter on them
	var pkgToPatchesMap map[string][]string
	var pkgUpdates []*packages.PkgInfo
	if zOpts.withUpdate {
		pkgUpdates, err = packages.ZypperUpdates(ctx)
		if err != nil {
			return nil
		}
		pkgToPatchesMap, err = packages.ZypperPackagesInPatch(ctx, patches)
		if err != nil {
			return nil
		}
	}

	fPatches, fpkgs, err := runFilter(patches, zOpts.exclusivePatches, zOpts.excludes, pkgUpdates, pkgToPatchesMap, zOpts.withUpdate)

	if len(fPatches) == 0 && len(fpkgs) == 0 {
		clog.Infof(ctx, "No updates required.")
		return nil
	}

	if len(fPatches) == 0 {
		clog.Infof(ctx, "No patches to install.")
	} else {
		msg := fmt.Sprintf("%d patches: %v", len(fPatches), fPatches)
		if zOpts.dryrun {
			clog.Infof(ctx, "Running in dryrun mode, not installing %s", msg)
		} else {
			clog.Infof(ctx, "Installing %s", msg)
		}
	}

	if len(fpkgs) == 0 {
		clog.Infof(ctx, "No non-patch packages to update.")
	} else {
		msg := fmt.Sprintf("%d patches: %v", len(fpkgs), fpkgs)
		if zOpts.dryrun {
			clog.Infof(ctx, "Running in dryrun mode, not Updating %s", msg)
		} else {
			clog.Infof(ctx, "Updating %s", msg)
		}
	}

	if zOpts.dryrun {
		return nil
	}

	return packages.ZypperInstall(ctx, fPatches, fpkgs)
}

func runFilter(patches []*packages.ZypperPatch, exclusivePatches, excludes []string, pkgUpdates []*packages.PkgInfo, pkgToPatchesMap map[string][]string, withUpdate bool) ([]*packages.ZypperPatch, []*packages.PkgInfo, error) {
	// exclusive patches
	var fPatches []*packages.ZypperPatch
	var fPkgs []*packages.PkgInfo
	if len(exclusivePatches) > 0 {
		for _, patch := range patches {
			if containsString(exclusivePatches, patch.Name) {
				fPatches = append(fPatches, patch)
			}
		}
		return fPatches, fPkgs, nil
	}

	// if --with-update is specified, filter out the packages
	// that will be updated as a part of a patch update
	if withUpdate {
		for _, pkg := range pkgUpdates {
			if _, ok := pkgToPatchesMap[pkg.Name]; !ok {
				fPkgs = append(fPkgs, pkg)
			}
		}
	}

	// we have the list of patches which is already filtered
	// as per the configurations provided by user;
	// we remove the excluded patches from the list
	for _, patch := range patches {
		if !containsString(excludes, patch.Name) {
			fPatches = append(fPatches, patch)
		}
	}
	return fPatches, fPkgs, nil
}
