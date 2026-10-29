package packages

import (
	"context"
	"errors"
	"fmt"

	"github.com/GoogleCloudPlatform/osconfig/clog"
	"github.com/GoogleCloudPlatform/osconfig/osinfo"
	scalibr "github.com/google/osv-scalibr"
	"github.com/google/osv-scalibr/binary/platform"
	"github.com/google/osv-scalibr/binary/proto/config_go_proto"
	"github.com/google/osv-scalibr/extractor"
	scalibrchoco "github.com/google/osv-scalibr/extractor/filesystem/os/chocolatey/metadata"
	scalibrcos "github.com/google/osv-scalibr/extractor/filesystem/os/cos/metadata"
	dpkgmetadata "github.com/google/osv-scalibr/extractor/filesystem/os/dpkg/metadata"
	scalibrrpm "github.com/google/osv-scalibr/extractor/filesystem/os/rpm/metadata"
	scalibrsnap "github.com/google/osv-scalibr/extractor/filesystem/os/snap/metadata"
	scalibrwinget "github.com/google/osv-scalibr/extractor/filesystem/os/winget/metadata"
	scalibrfs "github.com/google/osv-scalibr/fs"
	"github.com/google/osv-scalibr/plugin"
	pl "github.com/google/osv-scalibr/plugin/list"
	"github.com/google/osv-scalibr/purl"
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
		Name:     pkg.Name,
		Version:  pkg.Version,
		Arch:     osinfo.NormalizeArchitecture(metadata.Architecture),
		Source:   source,
		Type:     purl.TypeDebian,
		Purl:     pkg.PURL().String(),
		Location: pkg.Locations,
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
		Name:     pkg.Name,
		Version:  version,
		Arch:     osinfo.NormalizeArchitecture(architecture),
		Source:   source,
		Type:     purl.TypeRPM,
		Purl:     pkg.PURL().String(),
		Location: pkg.Locations,
	}
}

func pkgInfoFromCosExtractorPackage(pkg *extractor.Package, metadata *scalibrcos.Metadata, osinfo *osinfo.OSInfo) *PkgInfo {
	return &PkgInfo{
		Name:     fmt.Sprintf("%s/%s", metadata.Category, pkg.Name),
		Version:  pkg.Version,
		Arch:     osinfo.Architecture,
		Type:     purl.TypeCOS,
		Purl:     pkg.PURL().String(),
		Location: pkg.Locations,
	}
}

// pkgInfoFromSnapExtractorPackage creates a PkgInfo from a SCALIBR snap extractor package.
func pkgInfoFromSnapExtractorPackage(pkg *extractor.Package, metadata *scalibrsnap.Metadata, defaultArch string) *PkgInfo {
	arch := defaultArch
	if metadata.Architectures[0] != "" {
		arch = metadata.Architectures[0]
	}
	return &PkgInfo{
		Name:     pkg.Name,
		Version:  pkg.Version,
		Arch:     osinfo.NormalizeArchitecture(arch),
		Type:     purl.TypeSnap,
		Purl:     pkg.PURL().String(),
		Location: pkg.Locations,
	}
}

func pkgInfoFromGenericExtractorPackage(pkg *extractor.Package, pkgType string, arch string) *PkgInfo {
	return &PkgInfo{
		Name:     pkg.Name,
		Version:  pkg.Version,
		Arch:     arch,
		Type:     pkgType,
		Purl:     pkg.PURL().String(),
		Location: pkg.Locations,
	}
}

var languagePackageMappers = map[string]func(*Packages, *extractor.Package){
	purl.TypePyPi: func(pkgs *Packages, pkg *extractor.Package) {
		pkgs.Pip = append(pkgs.Pip, pkgInfoFromGenericExtractorPackage(pkg, purl.TypePyPi, ""))
	},
	purl.TypeGem: func(pkgs *Packages, pkg *extractor.Package) {
		pkgs.Gem = append(pkgs.Gem, pkgInfoFromGenericExtractorPackage(pkg, purl.TypeGem, ""))
	},
	purl.TypeNPM: func(pkgs *Packages, pkg *extractor.Package) {
		pkgs.Npm = append(pkgs.Npm, pkgInfoFromGenericExtractorPackage(pkg, purl.TypeNPM, ""))
	},
	purl.TypeMaven: func(pkgs *Packages, pkg *extractor.Package) {
		pkgs.Maven = append(pkgs.Maven, pkgInfoFromGenericExtractorPackage(pkg, purl.TypeMaven, ""))
	},
	purl.TypeGolang: func(pkgs *Packages, pkg *extractor.Package) {
		pkgs.Go = append(pkgs.Go, pkgInfoFromGenericExtractorPackage(pkg, purl.TypeGolang, ""))
	},
	purl.TypeCargo: func(pkgs *Packages, pkg *extractor.Package) {
		pkgs.Cargo = append(pkgs.Cargo, pkgInfoFromGenericExtractorPackage(pkg, purl.TypeCargo, ""))
	},
	purl.TypeComposer: func(pkgs *Packages, pkg *extractor.Package) {
		pkgs.Composer = append(pkgs.Composer, pkgInfoFromGenericExtractorPackage(pkg, purl.TypeComposer, ""))
	},
	purl.TypeSwift: func(pkgs *Packages, pkg *extractor.Package) {
		pkgs.Swift = append(pkgs.Swift, pkgInfoFromGenericExtractorPackage(pkg, purl.TypeSwift, ""))
	},
	purl.TypeCocoapods: func(pkgs *Packages, pkg *extractor.Package) {
		pkgs.Swift = append(pkgs.Swift, pkgInfoFromGenericExtractorPackage(pkg, purl.TypeSwift, ""))
	},
	purl.TypePub: func(pkgs *Packages, pkg *extractor.Package) {
		pkgs.Pub = append(pkgs.Pub, pkgInfoFromGenericExtractorPackage(pkg, purl.TypePub, ""))
	},
	purl.TypeConda: func(pkgs *Packages, pkg *extractor.Package) {
		pkgs.Pip = append(pkgs.Pip, pkgInfoFromGenericExtractorPackage(pkg, purl.TypeConda, ""))
	},
	purl.TypeNuget: func(pkgs *Packages, pkg *extractor.Package) {
		pkgs.Nuget = append(pkgs.Nuget, pkgInfoFromGenericExtractorPackage(pkg, purl.TypeNuget, ""))
	},
}

// appendOSPackage converts and appends an OS package to pkgs if recognized.
func appendOSPackage(pkgs *Packages, pkg *extractor.Package, osinfo *osinfo.OSInfo) bool {
	switch metadata := pkg.Metadata.(type) {
	case *dpkgmetadata.Metadata:
		pkgs.Deb = append(pkgs.Deb, pkgInfoFromDpkgExtractorPackage(pkg, metadata))
		return true
	case *scalibrrpm.Metadata:
		pkgs.Rpm = append(pkgs.Rpm, pkgInfoFromRpmExtractorPackage(pkg, metadata))
		return true
	case *scalibrcos.Metadata:
		pkgs.COS = append(pkgs.COS, pkgInfoFromCosExtractorPackage(pkg, metadata, osinfo))
		return true
	case *scalibrsnap.Metadata:
		pkgs.Snap = append(pkgs.Snap, pkgInfoFromSnapExtractorPackage(pkg, metadata, osinfo.Architecture))
		return true
	case *scalibrchoco.Metadata:
		pkgs.Chocolatey = append(pkgs.Chocolatey, pkgInfoFromGenericExtractorPackage(pkg, purl.TypeChocolatey, osinfo.Architecture))
		return true
	case *scalibrwinget.Metadata:
		pkgs.WinGet = append(pkgs.WinGet, pkgInfoFromGenericExtractorPackage(pkg, purl.TypeWinget, osinfo.Architecture))
		return true
	default:
		return false
	}
}

// appendLanguagePackage converts and appends a language package to pkgs if recognized.
func appendLanguagePackage(pkgs *Packages, pkg *extractor.Package) bool {
	p := pkg.PURL()
	if p == nil {
		return false
	}
	mapper, ok := languagePackageMappers[p.Type]
	if !ok {
		return false
	}
	mapper(pkgs, pkg)
	return true
}

// pkgInfosFromExtractorPackages converts SCALIBR inventory packages into Packages.
func pkgInfosFromExtractorPackages(ctx context.Context, scan *scalibr.ScanResult, osinfo *osinfo.OSInfo) Packages {
	var packages Packages
	for _, pkg := range scan.Inventory.Packages {
		if appendOSPackage(&packages, pkg, osinfo) {
			continue
		}
		if appendLanguagePackage(&packages, pkg) {
			continue
		}
		clog.Errorf(ctx, "Package type not implemented: %v", pkg)
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

	plugins, err := pl.FromNames(p.extractors, &config_go_proto.PluginConfig{})
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
		Plugins:    plugins,
		ScanRoots:  scanRoots,
		DirsToSkip: dirsToSkip,
	}, nil
}

type scalibrInstalledPackagesProvider struct {
	extractors     []string
	osinfoProvider osinfo.Provider
	scanRootPaths  []string
	dirsToSkip     []string
}

func scanFailed(ctx context.Context, scan *scalibr.ScanResult) bool {
	if scan == nil {
		clog.Errorf(ctx, "scalibr scan failed: nil scan result")
		return true
	}
	if scan.Status == nil || scan.Status.Status == plugin.ScanStatusFailed {
		clog.Errorf(ctx, "scalibr scan failed")
		return true
	}
	return false
}

func failedExtractors(pluginStatuses []*plugin.Status) []string {
	var failed []string
	for _, ps := range pluginStatuses {
		if ps.Status == nil || ps.Status.Status != plugin.ScanStatusSucceeded {
			failed = append(failed, ps.Name)
		}
	}
	return failed
}

func handleScanStatus(ctx context.Context, scan *scalibr.ScanResult) error {
	if scanFailed(ctx, scan) {
		return errors.New("failed to extract inventory via scalibr")
	}

	if scan.Status.Status == plugin.ScanStatusPartiallySucceeded {
		clog.Warningf(ctx, "scalibr scan partially succeeded, failed extractors: %v", failedExtractors(scan.PluginStatus))
	}

	return nil
}

func (p scalibrInstalledPackagesProvider) GetInstalledPackages(ctx context.Context) (Packages, error) {
	config, err := p.getScanConfig()
	if err != nil {
		return Packages{}, err
	}

	scan := scalibr.New().Scan(ctx, config)
	if err := handleScanStatus(ctx, scan); err != nil {
		return Packages{}, err
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
