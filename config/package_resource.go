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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/agentconfig"
	"github.com/GoogleCloudPlatform/osconfig/clog"
	"github.com/GoogleCloudPlatform/osconfig/packages"
	"github.com/GoogleCloudPlatform/osconfig/util"

	agentendpointpb "google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1"
	"google.golang.org/protobuf/proto"
)

var (
	packageInfoCacheFile = filepath.Join(agentconfig.CacheDir(), "config_package_info.cache")
	// Clear out the entry if the last lookup is > 7 days ago.
	packageInfoCacheTimeout = -168 * time.Hour
	packageInfoCacheStore   packageInfoCache
)

type packageResouce struct {
	*agentendpointpb.OSPolicy_Resource_PackageResource

	managedPackage ManagedPackage
}

// AptPackage describes an apt package resource.
type AptPackage struct {
	PackageResource *agentendpointpb.OSPolicy_Resource_PackageResource_APT
	DesiredState    agentendpointpb.OSPolicy_Resource_PackageResource_DesiredState
}

// DebPackage describes a deb package resource.
type DebPackage struct {
	PackageResource *agentendpointpb.OSPolicy_Resource_PackageResource_Deb
	name, localPath string
}

// GooGetPackage describes a googet package resource.
type GooGetPackage struct {
	PackageResource *agentendpointpb.OSPolicy_Resource_PackageResource_GooGet
	DesiredState    agentendpointpb.OSPolicy_Resource_PackageResource_DesiredState
}

// MSIPackage describes an msi package resource.
type MSIPackage struct {
	PackageResource                     *agentendpointpb.OSPolicy_Resource_PackageResource_MSI
	productName, productCode, localPath string
}

// YumPackage describes a yum package resource.
type YumPackage struct {
	PackageResource *agentendpointpb.OSPolicy_Resource_PackageResource_YUM
	DesiredState    agentendpointpb.OSPolicy_Resource_PackageResource_DesiredState
}

// ZypperPackage describes a zypper package resource.
type ZypperPackage struct {
	PackageResource *agentendpointpb.OSPolicy_Resource_PackageResource_Zypper
	DesiredState    agentendpointpb.OSPolicy_Resource_PackageResource_DesiredState
}

// RPMPackage describes an rpm package resource.
type RPMPackage struct {
	PackageResource *agentendpointpb.OSPolicy_Resource_PackageResource_RPM
	name, localPath string
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

	tempDir string
}

func (p *packageResouce) validateFile(file *agentendpointpb.OSPolicy_Resource_File) error {
	if file.GetLocalPath() != "" {
		if !util.Exists(file.GetLocalPath()) {
			return fmt.Errorf("%q does not exist", file.GetLocalPath())
		}
	}
	return nil
}

type packageInfoCache map[string]packageInfo

type packageInfo struct {
	PkgInfo    *packages.PkgInfo
	LastLookup time.Time
}

func getPackageInfoCacheKey(pkgFile *agentendpointpb.OSPolicy_Resource_File) (string, error) {
	// We use the base64 encoded binary proto as the key.
	raw, err := proto.Marshal(pkgFile)
	if err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(raw), nil
}

func loadPackageInfoCache(ctx context.Context) {
	if packageInfoCacheStore != nil {
		return
	}
	data, err := ioutil.ReadFile(packageInfoCacheFile)
	if err != nil {
		// Just ignore the error and return early
		// The error mode here is to just always redownload the file.
		clog.Debugf(ctx, "Error reading the package info cache: %v", err)
		packageInfoCacheStore = packageInfoCache{}
		return
	}
	var cache packageInfoCache
	if err := json.Unmarshal(data, &cache); err != nil {
		clog.Debugf(ctx, "Error unmarshaling the package info cache: %v", err)
		packageInfoCacheStore = packageInfoCache{}
		return
	}
	packageInfoCacheStore = cache
}

func getPackageInfoFromCache(ctx context.Context, pkgFile *agentendpointpb.OSPolicy_Resource_File) *packages.PkgInfo {
	loadPackageInfoCache(ctx)
	key, err := getPackageInfoCacheKey(pkgFile)
	if err != nil {
		// Just ignore the error and return early
		// The error mode here is just always redownload the file.
		clog.Debugf(ctx, "Error creating the package info cache key: %v", err)
		return nil
	}
	packageInfo, ok := packageInfoCacheStore[key]
	if !ok {
		return nil
	}
	return packageInfo.PkgInfo
}

func updatePackageInfoCache(ctx context.Context, info *packages.PkgInfo, pkgFile *agentendpointpb.OSPolicy_Resource_File) {
	loadPackageInfoCache(ctx)
	for k, v := range packageInfoCacheStore {
		if time.Now().Add(packageInfoCacheTimeout).After(v.LastLookup) {
			delete(packageInfoCacheStore, k)
		}
	}
	key, err := getPackageInfoCacheKey(pkgFile)
	if err != nil {
		// Just ignore the error and return early
		// The error mode here is to just always redownload the file.
		clog.Warningf(ctx, "Error creating the package info cache: %v", err)
		return
	}
	packageInfoCacheStore[key] = packageInfo{PkgInfo: info, LastLookup: time.Now()}
}

func savePackageInfoCache(ctx context.Context) error {
	if packageInfoCacheStore == nil {
		return nil
	}
	data, err := json.Marshal(packageInfoCacheStore)
	if err != nil {
		return err
	}
	packageInfoCacheStore = nil
	return ioutil.WriteFile(packageInfoCacheFile, data, 0644)
}

func (p *packageResouce) validate(ctx context.Context) (*ManagedResources, error) {
	switch p.GetSystemPackage().(type) {
	case *agentendpointpb.OSPolicy_Resource_PackageResource_Apt:
		pr := p.GetApt()
		if !packages.AptExists {
			return nil, fmt.Errorf("cannot manage Apt package %q because apt-get does not exist on the system", pr.GetName())
		}

		p.managedPackage.Apt = &AptPackage{DesiredState: p.GetDesiredState(), PackageResource: pr}

	case *agentendpointpb.OSPolicy_Resource_PackageResource_Deb_:
		pr := p.GetDeb()
		if !packages.DpkgExists {
			return nil, fmt.Errorf("cannot manage Deb package because dpkg does not exist on the system")
		}
		if p.GetDesiredState() != agentendpointpb.OSPolicy_Resource_PackageResource_INSTALLED {
			return nil, fmt.Errorf("desired state of %q not applicable for deb package", p.GetDesiredState())
		}
		if err := p.validateFile(pr.GetSource()); err != nil {
			return nil, err
		}
		var localPath string
		var err error
		source := p.GetDeb().GetSource()
		info := getPackageInfoFromCache(ctx, source)
		if info == nil {
			localPath, err = p.download(ctx, "pkg.deb", source)
			if err != nil {
				return nil, err
			}
			info, err = packages.DebPkgInfo(ctx, localPath)
			if err != nil {
				return nil, err
			}
		}
		// Always update the cache to update the timestamps.
		updatePackageInfoCache(ctx, info, source)

		p.managedPackage.Deb = &DebPackage{PackageResource: pr, localPath: localPath, name: info.Name}

	case *agentendpointpb.OSPolicy_Resource_PackageResource_Googet:
		pr := p.GetGooget()
		if !packages.GooGetExists {
			return nil, fmt.Errorf("cannot manage GooGet package %q because googet does not exist on the system", pr.GetName())
		}

		p.managedPackage.GooGet = &GooGetPackage{DesiredState: p.GetDesiredState(), PackageResource: pr}

	case *agentendpointpb.OSPolicy_Resource_PackageResource_Msi:
		pr := p.GetMsi()
		if !packages.MSIExists {
			return nil, fmt.Errorf("cannot manage MSI package because msiexec does not exist on the system")
		}
		if p.GetDesiredState() != agentendpointpb.OSPolicy_Resource_PackageResource_INSTALLED {
			return nil, fmt.Errorf("desired state of %q not applicable for MSI package", p.GetDesiredState())
		}
		if err := p.validateFile(pr.GetSource()); err != nil {
			return nil, err
		}
		var localPath string
		var err error
		source := p.GetMsi().GetSource()
		info := getPackageInfoFromCache(ctx, source)
		if info == nil {
			localPath, err = p.download(ctx, "pkg.msi", source)
			if err != nil {
				return nil, err
			}
			productName, productCode, err := packages.MSIInfo(localPath)
			if err != nil {
				return nil, err
			}
			// We store productCode as version in the packageinfo struct.
			info = &packages.PkgInfo{Name: productName, Version: productCode}
		}
		// Always update the cache to update the timestamps.
		updatePackageInfoCache(ctx, info, source)

		p.managedPackage.MSI = &MSIPackage{PackageResource: pr, localPath: localPath, productName: info.Name, productCode: info.Version}

	case *agentendpointpb.OSPolicy_Resource_PackageResource_Yum:
		pr := p.GetYum()
		if !packages.YumExists {
			return nil, fmt.Errorf("cannot manage Yum package %q because yum does not exist on the system", pr.GetName())
		}

		p.managedPackage.Yum = &YumPackage{DesiredState: p.GetDesiredState(), PackageResource: pr}

	case *agentendpointpb.OSPolicy_Resource_PackageResource_Zypper_:
		pr := p.GetZypper()
		if !packages.ZypperExists {
			return nil, fmt.Errorf("cannot manage Zypper package %q because zypper does not exist on the system", pr.GetName())
		}

		p.managedPackage.Zypper = &ZypperPackage{DesiredState: p.GetDesiredState(), PackageResource: pr}

	case *agentendpointpb.OSPolicy_Resource_PackageResource_Rpm:
		pr := p.GetRpm()
		if !packages.RPMExists {
			return nil, fmt.Errorf("cannot manage RPM package because rpm does not exist on the system")
		}
		if p.GetDesiredState() != agentendpointpb.OSPolicy_Resource_PackageResource_INSTALLED {
			return nil, fmt.Errorf("desired state of %q not applicable for rpm package", p.GetDesiredState())
		}
		if err := p.validateFile(pr.GetSource()); err != nil {
			return nil, err
		}
		var localPath string
		var err error
		source := p.GetRpm().GetSource()
		info := getPackageInfoFromCache(ctx, source)
		if info == nil {
			localPath, err = p.download(ctx, "pkg.rpm", source)
			if err != nil {
				return nil, err
			}
			info, err = packages.RPMPkgInfo(ctx, localPath)
			if err != nil {
				return nil, err
			}
		}
		// Always update the cache to update the timestamps.
		updatePackageInfoCache(ctx, info, source)

		p.managedPackage.RPM = &RPMPackage{PackageResource: pr, localPath: localPath, name: info.Name}

	default:
		return nil, fmt.Errorf("SystemPackage field not set or references unknown package manager: %v", p.GetSystemPackage())
	}

	return &ManagedResources{Packages: []ManagedPackage{p.managedPackage}}, nil
}

type packageCache struct {
	cache     map[string]struct{}
	refreshed time.Time
}

var (
	aptInstalled    = &packageCache{}
	debInstalled    = &packageCache{}
	gooInstalled    = &packageCache{}
	yumInstalled    = &packageCache{}
	zypperInstalled = &packageCache{}
	rpmInstalled    = &packageCache{}
)

func populateInstalledCache(ctx context.Context, mp ManagedPackage) error {
	var cache *packageCache
	var refreshFunc func(context.Context) ([]*packages.PkgInfo, error)
	var err error
	switch {
	case mp.Apt != nil:
		cache = aptInstalled
		refreshFunc = packages.InstalledDebPackages

	case mp.Deb != nil:
		cache = debInstalled
		refreshFunc = packages.InstalledDebPackages

	case mp.GooGet != nil:
		cache = gooInstalled
		refreshFunc = packages.InstalledGooGetPackages

	case mp.MSI != nil:
		// We just query per each MSI.
		return nil

	// TODO: implement yum functions
	case mp.Yum != nil:
		cache = yumInstalled
		refreshFunc = packages.InstalledRPMPackages

	// TODO: implement zypper functions
	case mp.Zypper != nil:
		cache = zypperInstalled
		refreshFunc = packages.InstalledRPMPackages

	case mp.RPM != nil:
		cache = rpmInstalled
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

// TODO: use a persistent cache for downloaded files so we dont need to redownload them each time
func (p *packageResouce) download(ctx context.Context, name string, file *agentendpointpb.OSPolicy_Resource_File) (string, error) {
	var path string
	perms := os.FileMode(0644)
	switch {
	case file.GetLocalPath() != "":
		path = file.GetLocalPath()
	default:
		tmpDir, err := ioutil.TempDir("", "osconfig_package_resource_")
		if err != nil {
			return "", fmt.Errorf("failed to create temp dir: %s", err)
		}
		p.managedPackage.tempDir = tmpDir
		path = filepath.Join(p.managedPackage.tempDir, name)
		if _, err := downloadFile(ctx, path, perms, file); err != nil {
			return "", err
		}
	}

	return path, nil
}

func (p *packageResouce) checkState(ctx context.Context) (inDesiredState bool, err error) {
	if err := populateInstalledCache(ctx, p.managedPackage); err != nil {
		return false, err
	}

	var desiredState agentendpointpb.OSPolicy_Resource_PackageResource_DesiredState
	var pkgIns bool

	switch {
	case p.managedPackage.Apt != nil:
		desiredState = p.managedPackage.Apt.DesiredState
		_, pkgIns = aptInstalled.cache[p.managedPackage.Apt.PackageResource.GetName()]

	case p.managedPackage.Deb != nil:
		desiredState = agentendpointpb.OSPolicy_Resource_PackageResource_INSTALLED
		_, pkgIns = debInstalled.cache[p.managedPackage.Deb.name]

	case p.managedPackage.GooGet != nil:
		desiredState = p.managedPackage.GooGet.DesiredState
		_, pkgIns = gooInstalled.cache[p.managedPackage.GooGet.PackageResource.GetName()]

	case p.managedPackage.MSI != nil:
		desiredState = agentendpointpb.OSPolicy_Resource_PackageResource_INSTALLED
		pkgIns, err = packages.MSIInstalled(p.managedPackage.MSI.productCode)
		if err != nil {
			return false, err
		}

	case p.managedPackage.Yum != nil:
		desiredState = p.managedPackage.Yum.DesiredState
		_, pkgIns = yumInstalled.cache[p.managedPackage.Yum.PackageResource.GetName()]

	case p.managedPackage.Zypper != nil:
		desiredState = p.managedPackage.Zypper.DesiredState
		_, pkgIns = zypperInstalled.cache[p.managedPackage.Zypper.PackageResource.GetName()]

	case p.managedPackage.RPM != nil:
		desiredState = agentendpointpb.OSPolicy_Resource_PackageResource_INSTALLED
		_, pkgIns = rpmInstalled.cache[p.managedPackage.RPM.name]
	}

	switch desiredState {
	case agentendpointpb.OSPolicy_Resource_PackageResource_INSTALLED:
		if pkgIns {
			return true, nil
		}
	case agentendpointpb.OSPolicy_Resource_PackageResource_REMOVED:
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
			actionFunc     func() error
			installedCache *packageCache
			name           string
			action         string
			packageType    string
		}
	)

	switch {
	case p.managedPackage.Apt != nil:
		enforcePackage.name = p.managedPackage.Apt.PackageResource.GetName()
		enforcePackage.packageType = "apt"
		enforcePackage.installedCache = aptInstalled
		switch p.managedPackage.Apt.DesiredState {
		case agentendpointpb.OSPolicy_Resource_PackageResource_INSTALLED:
			enforcePackage.action, enforcePackage.actionFunc = installing, func() error {
				if _, err := packages.AptUpdate(ctx); err != nil {
					return err
				}
				return packages.InstallAptPackages(ctx, []string{enforcePackage.name})
			}
		case agentendpointpb.OSPolicy_Resource_PackageResource_REMOVED:
			enforcePackage.action, enforcePackage.actionFunc = removing, func() error { return packages.RemoveAptPackages(ctx, []string{enforcePackage.name}) }
		}

	case p.managedPackage.Deb != nil:
		enforcePackage.name = p.managedPackage.Deb.name
		enforcePackage.packageType = "deb"
		enforcePackage.installedCache = debInstalled
		enforcePackage.action = installing
		// Check if we have not pulled the package yet.
		if p.managedPackage.Deb.localPath == "" {
			localPath, err := p.download(ctx, "pkg.deb", p.GetDeb().GetSource())
			if err != nil {
				return false, err
			}
			p.managedPackage.Deb.localPath = localPath
		}
		if p.GetDeb().GetPullDeps() {
			enforcePackage.actionFunc = func() error { return packages.InstallAptPackages(ctx, []string{p.managedPackage.Deb.localPath}) }
		} else {
			enforcePackage.actionFunc = func() error { return packages.DpkgInstall(ctx, p.managedPackage.Deb.localPath) }
		}

	case p.managedPackage.GooGet != nil:
		enforcePackage.name = p.managedPackage.GooGet.PackageResource.GetName()
		enforcePackage.packageType = "googet"
		enforcePackage.installedCache = gooInstalled
		switch p.managedPackage.GooGet.DesiredState {
		case agentendpointpb.OSPolicy_Resource_PackageResource_INSTALLED:
			enforcePackage.action, enforcePackage.actionFunc = installing, func() error { return packages.InstallGooGetPackages(ctx, []string{enforcePackage.name}) }
		case agentendpointpb.OSPolicy_Resource_PackageResource_REMOVED:
			enforcePackage.action, enforcePackage.actionFunc = removing, func() error { return packages.RemoveGooGetPackages(ctx, []string{enforcePackage.name}) }
		}

	case p.managedPackage.MSI != nil:
		enforcePackage.name = p.managedPackage.MSI.productName
		enforcePackage.packageType = "msi"
		enforcePackage.action = installing
		enforcePackage.installedCache = &packageCache{} // No package cache for msi.
		// Check if we have not pulled the package yet.
		if p.managedPackage.MSI.localPath == "" {
			localPath, err := p.download(ctx, "pkg.msi", p.GetMsi().GetSource())
			if err != nil {
				return false, err
			}
			p.managedPackage.MSI.localPath = localPath
		}
		enforcePackage.actionFunc = func() error {
			return packages.InstallMSIPackage(ctx, p.managedPackage.MSI.localPath, p.managedPackage.MSI.PackageResource.GetProperties())
		}

	case p.managedPackage.Yum != nil:
		enforcePackage.name = p.managedPackage.Yum.PackageResource.GetName()
		enforcePackage.packageType = "yum"
		enforcePackage.installedCache = yumInstalled
		switch p.managedPackage.Yum.DesiredState {
		case agentendpointpb.OSPolicy_Resource_PackageResource_INSTALLED:
			enforcePackage.action, enforcePackage.actionFunc = installing, func() error { return packages.InstallYumPackages(ctx, []string{enforcePackage.name}) }
		case agentendpointpb.OSPolicy_Resource_PackageResource_REMOVED:
			enforcePackage.action, enforcePackage.actionFunc = removing, func() error { return packages.RemoveYumPackages(ctx, []string{enforcePackage.name}) }
		}

	case p.managedPackage.Zypper != nil:
		enforcePackage.name = p.managedPackage.Zypper.PackageResource.GetName()
		enforcePackage.packageType = "zypper"
		enforcePackage.installedCache = zypperInstalled
		switch p.managedPackage.Zypper.DesiredState {
		case agentendpointpb.OSPolicy_Resource_PackageResource_INSTALLED:
			enforcePackage.action, enforcePackage.actionFunc = installing, func() error { return packages.InstallZypperPackages(ctx, []string{enforcePackage.name}) }
		case agentendpointpb.OSPolicy_Resource_PackageResource_REMOVED:
			enforcePackage.action, enforcePackage.actionFunc = removing, func() error { return packages.RemoveZypperPackages(ctx, []string{enforcePackage.name}) }
		}

	case p.managedPackage.RPM != nil:
		enforcePackage.name = p.managedPackage.RPM.name
		enforcePackage.packageType = "rpm"
		enforcePackage.installedCache = rpmInstalled
		enforcePackage.action = installing
		// Check if we have not pulled the package yet.
		if p.managedPackage.RPM.localPath == "" {
			localPath, err := p.download(ctx, "pkg.rpm", p.GetRpm().GetSource())
			if err != nil {
				return false, err
			}
			p.managedPackage.RPM.localPath = localPath
		}
		if p.GetRpm().GetPullDeps() {
			switch {
			case packages.YumExists:
				enforcePackage.actionFunc = func() error { return packages.InstallYumPackages(ctx, []string{p.managedPackage.RPM.localPath}) }
			case packages.ZypperExists:
				enforcePackage.actionFunc = func() error { return packages.InstallZypperPackages(ctx, []string{p.managedPackage.RPM.localPath}) }
			default:
				return false, fmt.Errorf("cannot install rpm %q with 'PullDeps' option as neither yum or zypper exist on system", enforcePackage.name)
			}
		} else {
			enforcePackage.actionFunc = func() error { return packages.RPMInstall(ctx, p.managedPackage.RPM.localPath) }
		}
	}

	clog.Infof(ctx, "%s %s package %q", strings.Title(enforcePackage.action), enforcePackage.packageType, enforcePackage.name)
	// Reset the cache as we are taking action on.
	enforcePackage.installedCache.cache = nil
	if err := enforcePackage.actionFunc(); err != nil {
		return false, fmt.Errorf("error %s %s package %q", enforcePackage.action, enforcePackage.packageType, enforcePackage.name)
	}

	return true, nil
}

func (p *packageResouce) populateOutput(rCompliance *agentendpointpb.OSPolicyResourceCompliance) {}

func (p *packageResouce) cleanup(ctx context.Context) error {
	// Save cache and clear the variable.
	if err := savePackageInfoCache(ctx); err != nil {
		clog.Warningf(ctx, "Error saving the package info cache: %v", err)
	}
	if p.managedPackage.tempDir != "" {
		return os.RemoveAll(p.managedPackage.tempDir)
	}
	return nil
}
