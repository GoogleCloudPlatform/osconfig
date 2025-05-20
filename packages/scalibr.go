package packages

import (
	"context"
	"fmt"

	"github.com/GoogleCloudPlatform/osconfig/clog"
	"github.com/GoogleCloudPlatform/osconfig/osinfo"
	scalibr "github.com/google/osv-scalibr"
	"github.com/google/osv-scalibr/binary/platform"
	extractor "github.com/google/osv-scalibr/extractor"
	fslist "github.com/google/osv-scalibr/extractor/filesystem/list"
	scalibrcos "github.com/google/osv-scalibr/extractor/filesystem/os/cos"
	scalibrdpkg "github.com/google/osv-scalibr/extractor/filesystem/os/dpkg"
	scalibrrpm "github.com/google/osv-scalibr/extractor/filesystem/os/rpm"
	scalibrfs "github.com/google/osv-scalibr/fs"
	"github.com/google/osv-scalibr/plugin"
)

func pkgInfoFromDpkgExtractorPackage(pkg *extractor.Package, metadata *scalibrdpkg.Metadata) *PkgInfo {
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
		if metadata, ok := pkg.Metadata.(*scalibrdpkg.Metadata); ok {
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

func gatherScanRoots() ([]*scalibrfs.ScanRoot, error) {
	scanRootPaths, err := platform.DefaultScanRoots(true)
	if err != nil {
		return nil, err
	}
	var scanRoots []*scalibrfs.ScanRoot
	for _, path := range scanRootPaths {
		scanRoots = append(scanRoots, scalibrfs.RealFSScanRoot(path))
	}
	return scanRoots, nil
}

func gatherConfig() (*scalibr.ScanConfig, []error) {
	var errs []error
	extractors := []string{
		"os/dpkg",
		"os/rpm",
		"os/cos",
		// TODO: implement "os/zypper" for `zypper patches` — excluded from scan till then
	}

	filesystemExtractors, err := fslist.ExtractorsFromNames(extractors)
	if err != nil {
		errs = append(errs, err)
	}

	scanRoots, err := gatherScanRoots()
	if err != nil {
		errs = append(errs, err)
	}

	dirsToSkip, err := platform.DefaultIgnoredDirectories()
	if err != nil {
		errs = append(errs, err)
	}

	return &scalibr.ScanConfig{
		FilesystemExtractors: filesystemExtractors,
		ScanRoots:            scanRoots,
		DirsToSkip:           dirsToSkip,
	}, errs
}

type scalibrInstalledPackagesProvider struct {
	osinfoProvider osinfo.Provider
}

// NewScalibrInstalledPackagesProvider makes provider that uses osv-scalibr as its implementation.
func NewScalibrInstalledPackagesProvider(osinfoProvider osinfo.Provider) InstalledPackagesProvider {
	return scalibrInstalledPackagesProvider{osinfoProvider: osinfoProvider}
}

func combineErrors(errors []error) error {
	if len(errors) > 0 {
		return fmt.Errorf("erroneous scan: %v", errors)
	}
	return nil
}

func (p scalibrInstalledPackagesProvider) GetInstalledPackages(ctx context.Context) (Packages, error) {
	config, errs := gatherConfig()

	scan := scalibr.New().Scan(ctx, config)

	if scan.Status.Status != plugin.ScanStatusSucceeded {
		errs = append(errs, fmt.Errorf("scan.Status is unhealthy: %v", scan.Status))
	}

	osinfo, err := p.osinfoProvider.GetOSInfo(ctx)
	if err != nil {
		errs = append(errs, err)
	}

	pkgs := pkgInfosFromExtractorPackages(ctx, scan, &osinfo)
	return pkgs, combineErrors(errs)
}
