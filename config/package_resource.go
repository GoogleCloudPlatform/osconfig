//  Copyright 2020 Google Inc. All Rights Reserved.
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

package config

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/clog"
	agentendpointpb "github.com/GoogleCloudPlatform/osconfig/internal/google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1alpha1"
	"github.com/GoogleCloudPlatform/osconfig/packages"
)

type packageResouce struct {
	*agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource

	managedPackage ManagedPackage
}

// AptPackage describes an apt package resource.
type AptPackage struct {
	PackageResource *agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_APT
	DesiredState    agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_DesiredState
}

// DebPackage describes a deb package resource.
type DebPackage struct {
	PackageResource *agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Deb
	DesiredState    agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_DesiredState
}

// GooGetPackage describes a googet package resource.
type GooGetPackage struct {
	PackageResource *agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_GooGet
	DesiredState    agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_DesiredState
}

// MSIPackage describes an msi package resource.
type MSIPackage struct {
	PackageResource *agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_MSI
	DesiredState    agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_DesiredState
}

// YumPackage describes a yum package resource.
type YumPackage struct {
	PackageResource *agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_YUM
	DesiredState    agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_DesiredState
}

// ZypperPackage describes a zypper package resource.
type ZypperPackage struct {
	PackageResource *agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Zypper
	DesiredState    agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_DesiredState
}

// RPMPackage describes an rpm package resource.
type RPMPackage struct {
	PackageResource *agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_RPM
	DesiredState    agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_DesiredState
}

// ManagedPackage is the package that this PackageResource manages.
type ManagedPackage struct {
	Apt    *AptPackage
	Deb    *DebPackage
	GooGet *GooGetPackage
	MSI    *MSIPackage
	Yum    *YumPackage
	Zypper *ZypperPackage
	RPM    *RPMPackage
}

func (p *packageResouce) validate() (*ManagedResources, error) {
	switch p.GetSystemPackage().(type) {
	case *agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Apt:
		pr := p.GetApt()
		if !packages.AptExists {
			return nil, fmt.Errorf("cannot manage Apt package %q because apt-get does not exist on the system", pr.GetName())
		}

		p.managedPackage.Apt = &AptPackage{DesiredState: p.GetDesiredState(), PackageResource: pr}

	case *agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Deb_:
		pr := p.GetDeb()
		if !packages.DpkgExists {
			return nil, fmt.Errorf("cannot manage Deb package because dpkg does not exist on the system")
		}

		p.managedPackage.Deb = &DebPackage{DesiredState: p.GetDesiredState(), PackageResource: pr}

	case *agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Googet:
		pr := p.GetGooget()
		if !packages.GooGetExists {
			return nil, fmt.Errorf("cannot manage GooGet package %q because googet does not exist on the system", pr.GetName())
		}

		p.managedPackage.GooGet = &GooGetPackage{DesiredState: p.GetDesiredState(), PackageResource: pr}

	case *agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Msi:
		pr := p.GetMsi()
		if !packages.MSIExecExists {
			return nil, fmt.Errorf("cannot manage MSI package because msiexec does not exist on the system")
		}

		p.managedPackage.MSI = &MSIPackage{DesiredState: p.GetDesiredState(), PackageResource: pr}

	case *agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Yum:
		pr := p.GetYum()
		if !packages.YumExists {
			return nil, fmt.Errorf("cannot manage Yum package %q because yum does not exist on the system", pr.GetName())
		}

		p.managedPackage.Yum = &YumPackage{DesiredState: p.GetDesiredState(), PackageResource: pr}

	case *agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Zypper_:
		pr := p.GetZypper()
		if !packages.ZypperExists {
			return nil, fmt.Errorf("cannot manage Zypper package %q because zypper does not exist on the system", pr.GetName())
		}

		p.managedPackage.Zypper = &ZypperPackage{DesiredState: p.GetDesiredState(), PackageResource: pr}

	case *agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Rpm:
		pr := p.GetRpm()
		if !packages.RPMExists {
			return nil, fmt.Errorf("cannot manage RPM package because rpm does not exist on the system")
		}

		p.managedPackage.RPM = &RPMPackage{DesiredState: p.GetDesiredState(), PackageResource: pr}

	default:
		return nil, errors.New("SystemPackage field not set or references unknown package manager")
	}

	return &ManagedResources{Packages: []ManagedPackage{p.managedPackage}}, nil
}

type packageCache struct {
	cache     map[string]struct{}
	refreshed time.Time
}

var aptInstalled packageCache
var debInstalled packageCache
var gooInstalled packageCache
var yumInstalled packageCache
var zypperInstalled packageCache
var rpmInstalled packageCache

func populateInstalledCache(ctx context.Context, mp ManagedPackage) error {
	var cache *packageCache
	var refreshFunc func(context.Context) ([]packages.PkgInfo, error)
	var err error
	switch {
	// TODO: implement apt functions
	case mp.Apt != nil:
		cache = &aptInstalled
		refreshFunc = packages.InstalledDebPackages

	case mp.Deb != nil:
		cache = &debInstalled
		refreshFunc = packages.InstalledDebPackages

	case mp.GooGet != nil:
		cache = &gooInstalled
		refreshFunc = packages.InstalledGooGetPackages

	// TODO: implement msi functions
	case mp.MSI != nil:
		return errors.New("msi not implemented")

	// TODO: implement yum functions
	case mp.Yum != nil:
		cache = &yumInstalled
		refreshFunc = packages.InstalledRPMPackages

	// TODO: implement zypper functions
	case mp.Zypper != nil:
		cache = &zypperInstalled
		refreshFunc = packages.InstalledRPMPackages

	case mp.RPM != nil:
		cache = &rpmInstalled
		refreshFunc = packages.InstalledRPMPackages
	default:
		return fmt.Errorf("unknown or unpopulated ManagedPackage package type: %+v", mp)
	}

	// Cache already populated within the last 3 min.
	if cache.cache != nil && cache.refreshed.After(time.Now().Add(-3*time.Minute)) {
		return nil
	}

	pis, err := refreshFunc(ctx)
	if err != nil {
		return err
	}

	cache.cache = map[string]struct{}{}
	for _, pkg := range pis {
		cache.cache[pkg.Name] = struct{}{}
	}
	cache.refreshed = time.Now()

	return nil
}

func (p *packageResouce) checkState(ctx context.Context) (inDesiredState bool, err error) {
	if err := populateInstalledCache(ctx, p.managedPackage); err != nil {
		return false, err
	}

	var desiredState agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_DesiredState
	var pkgIns bool

	switch {
	case p.managedPackage.Apt != nil:
		desiredState = p.managedPackage.Apt.DesiredState
		_, pkgIns = aptInstalled.cache[p.managedPackage.Apt.PackageResource.GetName()]

	// TODO: implement check for deb
	case p.managedPackage.Deb != nil:
		desiredState = p.managedPackage.Deb.DesiredState
		return false, errors.New("deb not implemented")

	case p.managedPackage.GooGet != nil:
		desiredState = p.managedPackage.GooGet.DesiredState
		_, pkgIns = gooInstalled.cache[p.managedPackage.GooGet.PackageResource.GetName()]

	// TODO: implement check for msi
	case p.managedPackage.MSI != nil:
		desiredState = p.managedPackage.MSI.DesiredState
		return false, errors.New("msi not implemented")

	case p.managedPackage.Yum != nil:
		desiredState = p.managedPackage.Yum.DesiredState
		_, pkgIns = yumInstalled.cache[p.managedPackage.Yum.PackageResource.GetName()]

	case p.managedPackage.Zypper != nil:
		desiredState = p.managedPackage.Zypper.DesiredState
		_, pkgIns = zypperInstalled.cache[p.managedPackage.Zypper.PackageResource.GetName()]

	// TODO: implement check for rpm
	case p.managedPackage.RPM != nil:
		desiredState = p.managedPackage.RPM.DesiredState
		return false, errors.New("rpm not implemented")
	}

	switch desiredState {
	case agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_INSTALLED:
		if pkgIns {
			return true, nil
		}
	case agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_REMOVED:
		if !pkgIns {
			return true, nil
		}
	default:
		return false, fmt.Errorf("DesiredState field not set or references state: %q", desiredState)
	}

	return false, nil
}

func (p *packageResouce) enforceState(ctx context.Context) (inDesiredState bool, err error) {
	var (
		installing = "installing"
		removing   = "removing"

		enforcePackage struct {
			name           string
			action         string
			packageType    string
			actionFunc     func() error
			installedCache map[string]struct{}
		}
	)

	switch {
	case p.managedPackage.Apt != nil:
		enforcePackage.name = p.managedPackage.Apt.PackageResource.GetName()
		enforcePackage.packageType = "apt"
		enforcePackage.installedCache = aptInstalled.cache
		switch p.managedPackage.Apt.DesiredState {
		case agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_INSTALLED:
			enforcePackage.action, enforcePackage.actionFunc = installing, func() error { return packages.InstallAptPackages(ctx, []string{enforcePackage.name}) }
		case agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_REMOVED:
			enforcePackage.action, enforcePackage.actionFunc = removing, func() error { return packages.RemoveAptPackages(ctx, []string{enforcePackage.name}) }
		}

	// TODO: implement check for deb
	case p.managedPackage.Deb != nil:
		enforcePackage.packageType = "deb"
		enforcePackage.installedCache = debInstalled.cache

	case p.managedPackage.GooGet != nil:
		enforcePackage.name = p.managedPackage.GooGet.PackageResource.GetName()
		enforcePackage.packageType = "googet"
		enforcePackage.installedCache = gooInstalled.cache
		switch p.managedPackage.GooGet.DesiredState {
		case agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_INSTALLED:
			enforcePackage.action, enforcePackage.actionFunc = installing, func() error { return packages.InstallGooGetPackages(ctx, []string{enforcePackage.name}) }
		case agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_REMOVED:
			enforcePackage.action, enforcePackage.actionFunc = removing, func() error { return packages.RemoveGooGetPackages(ctx, []string{enforcePackage.name}) }
		}

	// TODO: implement check for msi
	case p.managedPackage.MSI != nil:
		enforcePackage.packageType = "msi"

	case p.managedPackage.Yum != nil:
		enforcePackage.name = p.managedPackage.Yum.PackageResource.GetName()
		enforcePackage.packageType = "yum"
		enforcePackage.installedCache = yumInstalled.cache
		switch p.managedPackage.Yum.DesiredState {
		case agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_INSTALLED:
			enforcePackage.action, enforcePackage.actionFunc = installing, func() error { return packages.InstallYumPackages(ctx, []string{enforcePackage.name}) }
		case agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_REMOVED:
			enforcePackage.action, enforcePackage.actionFunc = removing, func() error { return packages.RemoveYumPackages(ctx, []string{enforcePackage.name}) }
		}

	case p.managedPackage.Zypper != nil:
		enforcePackage.name = p.managedPackage.Zypper.PackageResource.GetName()
		enforcePackage.packageType = "zypper"
		enforcePackage.installedCache = zypperInstalled.cache
		switch p.managedPackage.Zypper.DesiredState {
		case agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_INSTALLED:
			enforcePackage.action, enforcePackage.actionFunc = installing, func() error { return packages.InstallZypperPackages(ctx, []string{enforcePackage.name}) }
		case agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_REMOVED:
			enforcePackage.action, enforcePackage.actionFunc = removing, func() error { return packages.RemoveZypperPackages(ctx, []string{enforcePackage.name}) }
		}

	// TODO: implement check for rpm
	case p.managedPackage.RPM != nil:
		enforcePackage.packageType = "rpm"
		enforcePackage.installedCache = rpmInstalled.cache
	}

	clog.Infof(ctx, "%s %s package %q", strings.Title(enforcePackage.action), enforcePackage.packageType, enforcePackage.name)
	// Reset the cache as we are taking action.
	enforcePackage.installedCache = nil
	if err := enforcePackage.actionFunc(); err != nil {
		return false, fmt.Errorf("error %s %s package %q", enforcePackage.action, enforcePackage.packageType, enforcePackage.name)
	}

	return true, nil
}
