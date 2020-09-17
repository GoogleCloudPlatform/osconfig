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
	"errors"
	"fmt"

	agentendpointpb "github.com/GoogleCloudPlatform/osconfig/internal/google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1alpha1"
	"github.com/GoogleCloudPlatform/osconfig/packages"
)

type packageResouce struct {
	*agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource

	managedPackage ManagedPackage
}

// AptPackage describes an apt package resource.
type AptPackage struct {
	Install, Remove *agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_APT
}

// DebPackage describes a deb package resource.
type DebPackage struct {
	Install, Remove *agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Deb
}

// GooGetPackage describes a googet package resource.
type GooGetPackage struct {
	Install, Remove *agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_GooGet
}

// MSIPackage describes an msi package resource.
type MSIPackage struct {
	Install, Remove *agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_MSI
}

// YumPackage describes a yum package resource.
type YumPackage struct {
	Install, Remove *agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_YUM
}

// ZypperPackage describes a zypper package resource.
type ZypperPackage struct {
	Install, Remove *agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Zypper
}

// RPMPackage describes an rpm package resource.
type RPMPackage struct {
	Install, Remove *agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_RPM
}

// ManagedPackage is the package that this PackageResource manages.
type ManagedPackage struct {
	Apt    AptPackage
	Deb    DebPackage
	GooGet GooGetPackage
	MSI    MSIPackage
	Yum    YumPackage
	Zypper ZypperPackage
	RPM    RPMPackage
}

func (p *packageResouce) validate() (*ManagedResources, error) {
	switch p.GetSystemPackage().(type) {
	case *agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Apt:
		pr := p.GetApt()
		if !packages.AptExists {
			return nil, fmt.Errorf("cannot manage Apt package %q because apt-get does not exist on the system", pr.GetName())
		}

		switch p.GetDesiredState() {
		case agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_INSTALLED:
			p.managedPackage.Apt.Install = pr
		case agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_REMOVED:
			p.managedPackage.Apt.Remove = pr
		}
	case *agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Deb_:
		pr := p.GetDeb()
		if !packages.DpkgExists {
			return nil, fmt.Errorf("cannot manage Deb package because dpkg does not exist on the system")
		}

		switch p.GetDesiredState() {
		case agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_INSTALLED:
			p.managedPackage.Deb.Install = pr
		case agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_REMOVED:
			p.managedPackage.Deb.Remove = pr
		}
	case *agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Msi:
		pr := p.GetMsi()
		if !packages.MSIExecExists {
			return nil, fmt.Errorf("cannot manage MSI package because msiexec does not exist on the system")
		}

		switch p.GetDesiredState() {
		case agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_INSTALLED:
			p.managedPackage.MSI.Install = pr
		case agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_REMOVED:
			p.managedPackage.MSI.Remove = pr
		}
	case *agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Googet:
		pr := p.GetGooget()
		if !packages.GooGetExists {
			return nil, fmt.Errorf("cannot manage GooGet package %q because googet does not exist on the system", pr.GetName())
		}

		switch p.GetDesiredState() {
		case agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_INSTALLED:
			p.managedPackage.GooGet.Install = pr
		case agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_REMOVED:
			p.managedPackage.GooGet.Remove = pr
		}
	case *agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Yum:
		pr := p.GetYum()
		if !packages.YumExists {
			return nil, fmt.Errorf("cannot manage Yum package %q because yum does not exist on the system", pr.GetName())
		}

		switch p.GetDesiredState() {
		case agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_INSTALLED:
			p.managedPackage.Yum.Install = pr
		case agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_REMOVED:
			p.managedPackage.Yum.Remove = pr
		}
	case *agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Zypper_:
		pr := p.GetZypper()
		if !packages.ZypperExists {
			return nil, fmt.Errorf("cannot manage Zypper package %q because zypper does not exist on the system", pr.GetName())
		}

		switch p.GetDesiredState() {
		case agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_INSTALLED:
			p.managedPackage.Zypper.Install = pr
		case agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_REMOVED:
			p.managedPackage.Zypper.Remove = pr
		}
	case *agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Rpm:
		pr := p.GetRpm()
		if !packages.RPMExists {
			return nil, fmt.Errorf("cannot manage RPM package because rpm does not exist on the system")
		}

		switch p.GetDesiredState() {
		case agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_INSTALLED:
			p.managedPackage.RPM.Install = pr
		case agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_REMOVED:
			p.managedPackage.RPM.Remove = pr
		}
	default:
		return nil, errors.New("SystemPackage field not set or references unknown package manager")
	}

	return &ManagedResources{Packages: []ManagedPackage{p.managedPackage}}, nil
}

// TODO: implement a caching system for installed packages.
var aptInstalled []packages.PkgInfo
var debInstalled []packages.PkgInfo
var gooInstalled []packages.PkgInfo
var yumInstalled []packages.PkgInfo
var zypperInstalled []packages.PkgInfo
var rpmInstalled []packages.PkgInfo

func pkgInstalled(installedPkgs []packages.PkgInfo, name string) bool {
	for _, pkg := range installedPkgs {
		if pkg.Name == name {
			return true
		}
	}
	return false
}

func (p *packageResouce) checkState() (inDesiredState bool, err error) {
	switch {
	case p.managedPackage.Apt.Install != nil:
		if pkgInstalled(aptInstalled, p.managedPackage.Apt.Install.GetName()) {
			return true, nil
		}
	case p.managedPackage.Apt.Remove != nil:
		if !pkgInstalled(aptInstalled, p.managedPackage.Apt.Remove.GetName()) {
			return true, nil
		}

	// TODO: implement check for deb
	case p.managedPackage.Deb.Install != nil:
	case p.managedPackage.Deb.Remove != nil:

	case p.managedPackage.GooGet.Install != nil:
		if pkgInstalled(gooInstalled, p.managedPackage.GooGet.Install.GetName()) {
			return true, nil
		}
	case p.managedPackage.GooGet.Remove != nil:
		if !pkgInstalled(gooInstalled, p.managedPackage.GooGet.Remove.GetName()) {
			return true, nil
		}

	// TODO: implement check for msi
	case p.managedPackage.MSI.Install != nil:
	case p.managedPackage.MSI.Remove != nil:

	case p.managedPackage.Yum.Install != nil:
		if pkgInstalled(yumInstalled, p.managedPackage.Yum.Install.GetName()) {
			return true, nil
		}
	case p.managedPackage.Yum.Remove != nil:
		if !pkgInstalled(yumInstalled, p.managedPackage.Yum.Remove.GetName()) {
			return true, nil
		}

	case p.managedPackage.Zypper.Install != nil:
		if pkgInstalled(zypperInstalled, p.managedPackage.Zypper.Install.GetName()) {
			return true, nil
		}
	case p.managedPackage.Zypper.Remove != nil:
		if !pkgInstalled(zypperInstalled, p.managedPackage.Zypper.Remove.GetName()) {
			return true, nil
		}

	// TODO: implement check for rpm
	case p.managedPackage.RPM.Install != nil:
	case p.managedPackage.RPM.Remove != nil:
	}

	// If we got here we are not in the desired state.
	return false, nil
}

func (p *packageResouce) enforceState() (inDesiredState bool, err error) {
	// TODO: implement
	return true, nil
}
