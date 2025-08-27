package packages

import (
	"context"
	"os"
	"path"
	"reflect"
	"testing"

	"github.com/GoogleCloudPlatform/osconfig/osinfo"
	"github.com/google/go-cmp/cmp"
	scalibr "github.com/google/osv-scalibr"
	"github.com/google/osv-scalibr/extractor"
	scalibrcos "github.com/google/osv-scalibr/extractor/filesystem/os/cos"
	dpkgmetadata "github.com/google/osv-scalibr/extractor/filesystem/os/dpkg/metadata"
	scalibrrpm "github.com/google/osv-scalibr/extractor/filesystem/os/rpm"
	"github.com/google/osv-scalibr/inventory"
)

func TestExtractedPackageMappings(t *testing.T) {
	tests := []struct {
		name string
		arch string
		pkgs []*extractor.Package
		want Packages
	}{
		{
			name: "os/dpkg extractor maps correctly",
			pkgs: []*extractor.Package{
				{
					Name: "7zip", Version: "24.09+dfsg-4",
					Metadata: &dpkgmetadata.Metadata{PackageName: "7zip", Status: "install ok installed", SourceName: "", SourceVersion: "", PackageVersion: "24.09+dfsg-4", OSID: "debian", OSVersionCodename: "rodete", OSVersionID: "", Maintainer: "YOKOTA Hiroshi <yokota.hgml@gmail.com>", Architecture: "amd64"},
				},
				{
					Name: "llvm-16", Version: "1:16.0.6-27+build3",
					Metadata: &dpkgmetadata.Metadata{PackageName: "llvm-16", Status: "install ok installed", SourceName: "llvm-toolchain-16", SourceVersion: "", PackageVersion: "1:16.0.6-27+build3", OSID: "debian", OSVersionCodename: "rodete", OSVersionID: "", Maintainer: "LLVM Packaging Team <pkg-llvm-team@lists.alioth.debian.org>", Architecture: "amd64"},
				},
			},
			want: Packages{Deb: []*PkgInfo{
				{Name: "7zip", Version: "24.09+dfsg-4", Arch: "x86_64", Source: Source{Name: "7zip", Version: "24.09+dfsg-4"}},
				{Name: "llvm-16", Version: "1:16.0.6-27+build3", Arch: "x86_64", Source: Source{Name: "llvm-toolchain-16", Version: "1:16.0.6-27+build3"}},
			}},
		},
		{
			name: "os/rpm extractor maps correctly",
			pkgs: []*extractor.Package{
				{
					Name:     "acl",
					Version:  "2.2.51-15.el7",
					Metadata: &scalibrrpm.Metadata{PackageName: "acl", SourceRPM: "acl-2.2.51-15.el7.src.rpm", Epoch: 0, OSName: "CentOS Linux", OSID: "centos", OSVersionID: "7", OSBuildID: "", Vendor: "CentOS", Architecture: "x86_64", License: "GPLv2+"},
				},
				{
					Name:     "gpg-pubkey",
					Version:  "352c64e5-52ae6884",
					Metadata: &scalibrrpm.Metadata{PackageName: "gpg-pubkey", SourceRPM: "", Epoch: 0, OSName: "CentOS Linux", OSID: "centos", OSVersionID: "7", OSBuildID: "", Vendor: "", Architecture: "", License: "pubkey"},
				},
			},
			want: Packages{Rpm: []*PkgInfo{
				{Name: "acl", Version: "2.2.51-15.el7", Arch: "x86_64", Source: Source{Name: "acl-2.2.51-15.el7.src.rpm", Version: ""}},
				{Name: "gpg-pubkey", Version: "352c64e5-52ae6884", Arch: "all", Source: Source{Name: "gpg-pubkey", Version: ""}},
			}},
		},
		{
			name: "os/cos extractor maps correctly",
			arch: "x86_64",
			pkgs: []*extractor.Package{
				{
					Name:     "PySocks",
					Version:  "17412.448.8",
					Metadata: &scalibrcos.Metadata{Name: "PySocks", Version: "17412.448.8", Category: "dev-python", OSVersion: "105", OSVersionID: "105", EbuildVersion: "1.6.7-r1"},
				},
				{
					Name:     "chromeos-bsp",
					Version:  "17412.448.8",
					Metadata: &scalibrcos.Metadata{Name: "chromeos-bsp", Version: "17412.448.8", Category: "virtual", OSVersion: "105", OSVersionID: "105", EbuildVersion: "3-r1"},
				},
			},
			want: Packages{COS: []*PkgInfo{
				{Name: "dev-python/PySocks", Version: "17412.448.8", Arch: "x86_64"},
				{Name: "virtual/chromeos-bsp", Version: "17412.448.8", Arch: "x86_64"},
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(*testing.T) {
			got := pkgInfosFromExtractorPackages(context.TODO(), &scalibr.ScanResult{Inventory: inventory.Inventory{Packages: tt.pkgs}}, &osinfo.OSInfo{Architecture: tt.arch})
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func arrangeVirtualRoot(t *testing.T, dbFilepath string, targetFilepath string) string {
	virtualRootPath := "./testdata/virtualTestRoot"
	if err := os.RemoveAll(virtualRootPath); err != nil {
		t.Error(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(virtualRootPath); err != nil {
			t.Error(err)
		}
	})

	targetFilepathInsideVirtualRoot := path.Join(virtualRootPath, targetFilepath)
	if err := os.MkdirAll(path.Dir(targetFilepathInsideVirtualRoot), 0700); err != nil {
		t.Error(err)
	}

	if err := os.Link(dbFilepath, targetFilepathInsideVirtualRoot); err != nil {
		t.Error(err)
	}
	return virtualRootPath
}

type stubProvider struct {
}

func (stubProvider) GetOSInfo(ctx context.Context) (osinfo.OSInfo, error) {
	return osinfo.OSInfo{}, nil
}

func withZypperDisabled(t *testing.T) {
	prev := ZypperExists
	ZypperExists = false
	t.Cleanup(func() { ZypperExists = prev })
}

func TestScalibrIntegration(t *testing.T) {
	withZypperDisabled(t)
	tests := []struct {
		provider InstalledPackagesProvider
		wantErr  error
		wantPkgs Packages
	}{
		{
			provider: scalibrInstalledPackagesProvider{
				osinfoProvider: stubProvider{},
				extractors:     []string{"os/dpkg"},
				scanRootPaths:  []string{arrangeVirtualRoot(t, "./testdata/debian.dpkg-status", "/var/lib/dpkg/status")},
				dirsToSkip:     []string{},
			},
			wantPkgs: Packages{Deb: []*PkgInfo{
				{Name: "7zip", Version: "24.09+dfsg-4", Arch: "x86_64", Source: Source{Name: "7zip", Version: "24.09+dfsg-4"}},
				{Name: "llvm-16", Version: "1:16.0.6-27+build3", Arch: "x86_64", Source: Source{Name: "llvm-toolchain-16", Version: "1:16.0.6-27+build3"}},
			}},
		},
	}
	for _, tt := range tests {
		pkgs, err := tt.provider.GetInstalledPackages(context.Background())

		if !reflect.DeepEqual(err, tt.wantErr) {
			t.Errorf("err: want %v, got %v", tt.wantErr, err)
		}

		if !reflect.DeepEqual(pkgs, tt.wantPkgs) {
			t.Errorf("pkgs: want %v, got %v", tt.wantPkgs, pkgs)
		}
	}
}
