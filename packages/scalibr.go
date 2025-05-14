package packages

import (
	"context"
	"fmt"

	"github.com/GoogleCloudPlatform/osconfig/clog"
	"github.com/GoogleCloudPlatform/osconfig/osinfo"
	scalibr "github.com/google/osv-scalibr"
	extractor "github.com/google/osv-scalibr/extractor"
	scalibrcos "github.com/google/osv-scalibr/extractor/filesystem/os/cos"
	scalibrdpkg "github.com/google/osv-scalibr/extractor/filesystem/os/dpkg"
	scalibrrpm "github.com/google/osv-scalibr/extractor/filesystem/os/rpm"
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

func pkgInfosFromExtractorPackages(ctx context.Context, scan *scalibr.ScanResult, osinfo *osinfo.OSInfo) []*PkgInfo {
	pkgs := make([]*PkgInfo, 0, len(scan.Inventory.Packages))
	for _, pkg := range scan.Inventory.Packages {
		if metadata, ok := pkg.Metadata.(*scalibrdpkg.Metadata); ok {
			pkgs = append(pkgs, pkgInfoFromDpkgExtractorPackage(pkg, metadata))
		} else if metadata, ok := pkg.Metadata.(*scalibrrpm.Metadata); ok {
			pkgs = append(pkgs, pkgInfoFromRpmExtractorPackage(pkg, metadata))
		} else if metadata, ok := pkg.Metadata.(*scalibrcos.Metadata); ok {
			pkgs = append(pkgs, pkgInfoFromCosExtractorPackage(pkg, metadata, osinfo))
		} else {
			clog.Warningf(ctx, "Ignored package kind: %v", pkg)
		}
	}
	return pkgs
}
