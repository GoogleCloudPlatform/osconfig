package packages

import (
	"context"
	"fmt"

	"github.com/GoogleCloudPlatform/osconfig/clog"
	"github.com/GoogleCloudPlatform/osconfig/osinfo"
	scalibr "github.com/google/osv-scalibr"
	"github.com/google/osv-scalibr/binary/platform"
	"github.com/google/osv-scalibr/extractor"
	fslist "github.com/google/osv-scalibr/extractor/filesystem/list"
	scalibrcos "github.com/google/osv-scalibr/extractor/filesystem/os/cos"
	dpkgmetadata "github.com/google/osv-scalibr/extractor/filesystem/os/dpkg/metadata"
	scalibrrpm "github.com/google/osv-scalibr/extractor/filesystem/os/rpm"
	scalibrfs "github.com/google/osv-scalibr/fs"
	"github.com/google/osv-scalibr/plugin"
)

func pkgInfoFromDpkgExtractorPackage(pkg *extractor.Package, metadata *dpkgmetadata.Metadata) *PkgInfo {
	source := Source{Name: metadata.SourceName, Version: metadata.SourceVersion}
	if source.Name == "" {
		source.Name = pkg.Name
	}
	if source.Version == "" {
		source.Version = pkg.Version
	}
	return &PkgInfo{
		Name:    pkg.Name,
		Version: pkg.Version,
		Arch:    osinfo.NormalizeArchitecture(metadata.Architecture),
		Source:  source,
	}
}

func pkgInfoFromRpmExtractorPackage(pkg *extractor.Package, metadata *scalibrrpm.Metadata) *PkgInfo {
	source := Source{Name: metadata.SourceRPM}
	if source.Name == "" {
		source.Name = pkg.Name
	}

	version := pkg.Version
	// `metadata.Epoch != nil` would be better match with
	// legacy extractors' stdout parsing logic
	// Scalibr underlying dependency exposes it: https://github.com/knqyf263/go-rpmdb/pull/21
	// See also https://docs.redhat.com/fr/documentation/red_hat_enterprise_linux/9/html/packaging_and_distributing_software/epoch-scriplets-and-triggers_advanced-topics#packaging-epoch_epoch-scriplets-and-triggers
	if metadata.Epoch != 0 {
		version = fmt.Sprintf("%d:%s", metadata.Epoch, version)
	}

	architecture := metadata.Architecture
	if architecture == "" {
		architecture = "noarch"
	}

	return &PkgInfo{
		Name:    pkg.Name,
		Version: version,
		Arch:    osinfo.NormalizeArchitecture(architecture),
		Source:  source,
	}
}

func pkgInfoFromCosExtractorPackage(pkg *extractor.Package, metadata *scalibrcos.Metadata, osinfo *osinfo.OSInfo) *PkgInfo {
	return &PkgInfo{
		Name:    fmt.Sprintf("%s/%s", metadata.Category, pkg.Name),
		Version: pkg.Version,
		Arch:    osinfo.Architecture,
	}
}

func pkgInfosFromExtractorPackages(ctx context.Context, scan *scalibr.ScanResult, osinfo *osinfo.OSInfo) Packages {
	var packages Packages
	for _, pkg := range scan.Inventory.Packages {
		if metadata, ok := pkg.Metadata.(*dpkgmetadata.Metadata); ok {
			packages.Deb = append(packages.Deb, pkgInfoFromDpkgExtractorPackage(pkg, metadata))
		} else if metadata, ok := pkg.Metadata.(*scalibrrpm.Metadata); ok {
			packages.Rpm = append(packages.Rpm, pkgInfoFromRpmExtractorPackage(pkg, metadata))
		} else if metadata, ok := pkg.Metadata.(*scalibrcos.Metadata); ok {
			packages.COS = append(packages.COS, pkgInfoFromCosExtractorPackage(pkg, metadata, osinfo))
		} else {
			clog.Errorf(ctx, "Package type not implemented: %v", pkg)
		}
	}
	return packages
}

func (p scalibrInstalledPackagesProvider) getScanConfig() (*scalibr.ScanConfig, error) {
	var err error

	scanRootPaths := p.scanRootPaths
	if scanRootPaths == nil {
		scanRootPaths, err = platform.DefaultScanRoots(true)
		if err != nil {
			return nil, err
		}
	}

	var scanRoots []*scalibrfs.ScanRoot
	for _, path := range scanRootPaths {
		scanRoots = append(scanRoots, scalibrfs.RealFSScanRoot(path))
	}

	filesystemExtractors, err := fslist.ExtractorsFromNames(p.extractors)
	if err != nil {
		return nil, err
	}

	dirsToSkip := p.dirsToSkip
	if dirsToSkip == nil {
		dirsToSkip, err = platform.DefaultIgnoredDirectories()
		if err != nil {
			return nil, err
		}
	}

	return &scalibr.ScanConfig{
		FilesystemExtractors: filesystemExtractors,
		ScanRoots:            scanRoots,
		DirsToSkip:           dirsToSkip,
	}, nil
}

type scalibrInstalledPackagesProvider struct {
	extractors     []string
	osinfoProvider osinfo.Provider
	scanRootPaths  []string
	dirsToSkip     []string
}

func (p scalibrInstalledPackagesProvider) GetInstalledPackages(ctx context.Context) (Packages, error) {
	config, err := p.getScanConfig()
	if err != nil {
		return Packages{}, err
	}

	scan := scalibr.New().Scan(ctx, config)
	if scan.Status.Status != plugin.ScanStatusSucceeded {
		return Packages{}, fmt.Errorf("scalibr scan.Status is unhealthy, status: %v, plugins: %v", scan.Status, scan.PluginStatus)
	}

	osinfo, err := p.osinfoProvider.GetOSInfo(ctx)
	if err != nil {
		return Packages{}, err
	}

	pkgs := pkgInfosFromExtractorPackages(ctx, scan, &osinfo)

	// TODO: replace zypper patches legacy extractor with implemented "os/zypper" extractor
	if ZypperExists {
		zypperPatches, err := ZypperInstalledPatches(ctx)
		if err != nil {
			return pkgs, fmt.Errorf("error getting zypper installed patches: %v", err)
		}
		pkgs.ZypperPatches = zypperPatches
	}
	return pkgs, err
}
