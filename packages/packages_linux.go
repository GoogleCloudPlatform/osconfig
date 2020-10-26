/*
Copyright 2017 Google Inc. All Rights Reserved.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package packages

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/osconfig/clog"
	"github.com/GoogleCloudPlatform/osconfig/util"
)

// GetPackageUpdates gets all available package updates from any known
// installed package manager.
func GetPackageUpdates(ctx context.Context) (*Packages, error) {
	pkgs := Packages{}
	var errs []string
	if AptExists {
		apt, err := AptUpdates(ctx, AptGetUpgradeType(AptGetFullUpgrade), AptGetUpgradeShowNew(false))
		if err != nil {
			msg := fmt.Sprintf("error getting apt updates: %v", err)
			clog.Debugf(ctx, "Error: %s", msg)
			errs = append(errs, msg)
		} else {
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
			pkgs.Zypper = zypper
		}
		zypperPatches, err := ZypperPatches(ctx)
		if err != nil {
			msg := fmt.Sprintf("error getting zypper available patches: %v", err)
			clog.Debugf(ctx, "Error: %s", msg)
			errs = append(errs, msg)
		} else {
			pkgs.ZypperPatches = zypperPatches
		}
	}
	if GemExists {
		gem, err := GemUpdates(ctx)
		if err != nil {
			msg := fmt.Sprintf("error getting gem updates: %v", err)
			clog.Debugf(ctx, "Error: %s", msg)
		} else {
			pkgs.Gem = gem
		}
	}
	if PipExists {
		pip, err := PipUpdates(ctx)
		if err != nil {
			msg := fmt.Sprintf("error getting pip updates: %v", err)
			clog.Debugf(ctx, "Error: %s", msg)
		} else {
			pkgs.Pip = pip
		}
	}

	var err error
	if len(errs) != 0 {
		err = errors.New(strings.Join(errs, "\n"))
	}
	return &pkgs, err
}

// GetInstalledPackages gets all installed packages from any known installed
// package manager.
func GetInstalledPackages(ctx context.Context) (*Packages, error) {
	pkgs := Packages{}
	var errs []string
	if util.Exists(rpmquery) {
		rpm, err := InstalledRPMPackages(ctx)
		if err != nil {
			msg := fmt.Sprintf("error listing installed rpm packages: %v", err)
			clog.Debugf(ctx, "Error: %s", msg)
			errs = append(errs, msg)
		} else {
			pkgs.Rpm = rpm
		}
	}
	if util.Exists(zypper) {
		zypperPatches, err := ZypperInstalledPatches(ctx)
		if err != nil {
			msg := fmt.Sprintf("error getting zypper installed patches: %v", err)
			clog.Debugf(ctx, "Error: %s", msg)
			errs = append(errs, msg)
		} else {
			pkgs.ZypperPatches = zypperPatches
		}
	}
	if util.Exists(dpkgQuery) {
		deb, err := InstalledDebPackages(ctx)
		if err != nil {
			msg := fmt.Sprintf("error listing installed deb packages: %v", err)
			clog.Debugf(ctx, "Error: %s", msg)
			errs = append(errs, msg)
		} else {
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
			pkgs.COS = cos
		}
	}
	if util.Exists(gem) {
		gem, err := InstalledGemPackages(ctx)
		if err != nil {
			msg := fmt.Sprintf("error listing installed gem packages: %v", err)
			clog.Debugf(ctx, "Error: %s", msg)
		} else {
			pkgs.Gem = gem
		}
	}
	if util.Exists(pip) {
		pip, err := InstalledPipPackages(ctx)
		if err != nil {
			msg := fmt.Sprintf("error listing installed pip packages: %v", err)
			clog.Debugf(ctx, "Error: %s", msg)
		} else {
			pkgs.Pip = pip
		}
	}

	var err error
	if len(errs) != 0 {
		err = errors.New(strings.Join(errs, "\n"))
	}
	return &pkgs, err
}
