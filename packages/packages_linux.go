//  Copyright 2017 Google Inc. All Rights Reserved.
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
	"errors"
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/osconfig/agentconfig"
	"github.com/GoogleCloudPlatform/osconfig/clog"
	"github.com/GoogleCloudPlatform/osconfig/osinfo"
	"github.com/package-url/packageurl-go"
)

// GetPackageUpdates gets all available package updates from any known
// installed package manager.
func GetPackageUpdates(ctx context.Context, oi osinfo.OSInfo) (Packages, error) {
	pkgs, errs := getPackageUpdates(ctx, oi)

	var err error
	if len(errs) != 0 {
		err = errors.New(strings.Join(errs, "\n"))
	}

	return pkgs, err
}

func getPackageUpdates(ctx context.Context, oi osinfo.OSInfo) (Packages, []string) {
	pkgs := Packages{}
	shortname := oi.ShortName

	var errs []string
	if AptExists {
		apt, err := AptUpdates(ctx, AptGetUpgradeType(AptGetFullUpgrade), AptGetUpgradeShowNew(false))
		if err != nil {
			msg := fmt.Sprintf("error getting apt updates: %v", err)
			clog.Debugf(ctx, "Error: %s", msg)
			errs = append(errs, msg)
		} else {
			apt = enrichPkgInfoWithPurl(apt, shortname)
			pkgs.Apt = apt
		}
	}
	if YumExists {
		yum, err := YumUpdates(ctx)
		if err != nil {
			msg := fmt.Sprintf("error getting yum updates: %v", err)
			clog.Debugf(ctx, "Error: %s", msg)
			errs = append(errs, msg)
		} else {
			yum = enrichPkgInfoWithPurl(yum, shortname)
			pkgs.Yum = yum
		}
	}
	if ZypperExists {
		zypper, err := ZypperUpdates(ctx)
		if err != nil {
			msg := fmt.Sprintf("error getting zypper updates: %v", err)
			clog.Debugf(ctx, "Error: %s", msg)
			errs = append(errs, msg)
		} else {
			zypper = enrichPkgInfoWithPurl(zypper, shortname)
			pkgs.Zypper = zypper
		}
		zypperPatches, err := ZypperPatches(ctx)
		if err != nil {
			msg := fmt.Sprintf("error getting zypper available patches: %v", err)
			clog.Debugf(ctx, "Error: %s", msg)
			errs = append(errs, msg)
		} else {
			// ZypperPatch PURL is set before API request
			pkgs.ZypperPatches = zypperPatches
		}
	}
	if GemExists {
		gem, err := GemUpdates(ctx)
		if err != nil {
			msg := fmt.Sprintf("error getting gem updates: %v", err)
			clog.Debugf(ctx, "Error: %s", msg)
		} else {
			gem = enrichGemPkgInfoWithPurl(gem)
			pkgs.Gem = gem
		}
	}
	if PipExists {
		pip, err := PipUpdates(ctx)
		if err != nil {
			msg := fmt.Sprintf("error getting pip updates: %v", err)
			clog.Debugf(ctx, "Error: %s", msg)
		} else {
			pip = enrichPipPkgInfoWithPurl(pip)
			pkgs.Pip = pip
		}
	}

	return pkgs, errs
}

// GetInstalledPackages gets all installed packages from any known installed
// package manager.
func GetInstalledPackages(ctx context.Context, oi osinfo.OSInfo) (Packages, error) {
	pkgs, errs := getInstalledPackages(ctx, oi)
	var err error
	if len(errs) != 0 {
		err = errors.New(strings.Join(errs, "\n"))
	}

	return pkgs, err
}

func getInstalledPackages(ctx context.Context, oi osinfo.OSInfo) (Packages, []string) {
	pkgs := Packages{}
	shortname := oi.ShortName

	var errs []string
	if RPMQueryExists {
		rpm, err := InstalledRPMPackages(ctx)
		if err != nil {
			msg := fmt.Sprintf("error listing installed rpm packages: %v", err)
			clog.Debugf(ctx, "Error: %s", msg)
			errs = append(errs, msg)
		} else {
			rpm = enrichPkgInfoWithPurl(rpm, shortname)
			pkgs.Rpm = rpm
		}
	}
	if ZypperExists {
		zypperPatches, err := ZypperInstalledPatches(ctx)
		if err != nil {
			msg := fmt.Sprintf("error getting zypper installed patches: %v", err)
			clog.Debugf(ctx, "Error: %s", msg)
			errs = append(errs, msg)
		} else {
			// ZypperPatch PURL is set before API request
			pkgs.ZypperPatches = zypperPatches
		}
	}
	if DpkgQueryExists {
		deb, err := InstalledDebPackages(ctx)
		if err != nil {
			msg := fmt.Sprintf("error listing installed deb packages: %v", err)
			clog.Debugf(ctx, "Error: %s", msg)
			errs = append(errs, msg)
		} else {
			deb = enrichPkgInfoWithPurl(deb, shortname)
			pkgs.Deb = deb
		}
	}
	if COSPkgInfoExists {
		cos, err := InstalledCOSPackages()
		if err != nil {
			msg := fmt.Sprintf("error listing installed COS packages: %v", err)
			clog.Debugf(ctx, "Error: %s", msg)
			errs = append(errs, msg)
		} else {
			cos = enrichPkgInfoWithPurl(cos, shortname)
			pkgs.COS = cos
		}
	}
	if GemExists {
		gem, err := InstalledGemPackages(ctx)
		if err != nil {
			msg := fmt.Sprintf("error listing installed gem packages: %v", err)
			clog.Debugf(ctx, "Error: %s", msg)
		} else {
			gem = enrichGemPkgInfoWithPurl(gem)
			pkgs.Gem = gem
		}
	}
	if PipExists {
		pip, err := InstalledPipPackages(ctx)
		if err != nil {
			msg := fmt.Sprintf("error listing installed pip packages: %v", err)
			clog.Debugf(ctx, "Error: %s", msg)
		} else {
			pip = enrichPipPkgInfoWithPurl(pip)
			pkgs.Pip = pip
		}
	}

	return pkgs, errs
}

func enrichPkgInfoWithPurl(pkgs []*PkgInfo, shortname string) []*PkgInfo {
	for i, pkg := range pkgs {
		qualifiers := packageurl.Qualifiers{packageurl.Qualifier{Key: "arch", Value: pkg.Arch}}
		pkgs[i].Purl = packageurl.NewPackageURL(pkg.Type, shortname, pkg.Name, pkg.Version, qualifiers, "").ToString()
	}
	return pkgs
}

func enrichGemPkgInfoWithPurl(pkgs []*PkgInfo) []*PkgInfo {
	for i, pkg := range pkgs {
		pkgs[i].Purl = packageurl.NewPackageURL(pkg.Type, "", pkg.Name, pkg.Version, packageurl.Qualifiers{}, "").ToString()
	}
	return pkgs
}

func enrichPipPkgInfoWithPurl(pkgs []*PkgInfo) []*PkgInfo {
	for i, pkg := range pkgs {
		pkgs[i].Purl = packageurl.NewPackageURL(pkg.Type, "", pkg.Name, pkg.Version, packageurl.Qualifiers{}, "").ToString()
	}
	return pkgs
}

// NewInstalledPackagesProvider makes provider that uses osv-scalibr as its implementation if enabled by config, otherwise falls back to default legacy implementation.
func NewInstalledPackagesProvider(osinfoProvider osinfo.Provider) InstalledPackagesProvider {
	if agentconfig.ScalibrLinuxEnabled() {
		return scalibrInstalledPackagesProvider{
			extractors: []string{
				"os/cos",
				"os/dpkg",
				"os/rpm",
			},
			osinfoProvider: osinfoProvider,
		}
	}

	return defaultInstalledPackagesProvider{}
}
