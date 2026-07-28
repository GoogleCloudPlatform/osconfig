package packages

import (
	"context"
	"errors"
	"os"
	"path"
	"testing"

	"github.com/GoogleCloudPlatform/osconfig/osinfo"
	"github.com/GoogleCloudPlatform/osconfig/util"
	utilmocks "github.com/GoogleCloudPlatform/osconfig/util/mocks"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
	"github.com/golang/mock/gomock"
	scalibr "github.com/google/osv-scalibr"
	"github.com/google/osv-scalibr/extractor"
	scalibrcos "github.com/google/osv-scalibr/extractor/filesystem/os/cos/metadata"
	dpkgmetadata "github.com/google/osv-scalibr/extractor/filesystem/os/dpkg/metadata"
	scalibrrpm "github.com/google/osv-scalibr/extractor/filesystem/os/rpm/metadata"
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
					Name: "7zip", Version: "24.09+dfsg-4", PURLType: "deb",
					Metadata: &dpkgmetadata.Metadata{PackageName: "7zip", Status: "install ok installed", SourceName: "", SourceVersion: "", PackageVersion: "24.09+dfsg-4", OSID: "debian", OSVersionCodename: "rodete", OSVersionID: "", Maintainer: "YOKOTA Hiroshi <yokota.hgml@gmail.com>", Architecture: "amd64"},
				},
				{
					Name: "llvm-16", Version: "1:16.0.6-27+build3", PURLType: "deb",
					Metadata: &dpkgmetadata.Metadata{PackageName: "llvm-16", Status: "install ok installed", SourceName: "llvm-toolchain-16", SourceVersion: "", PackageVersion: "1:16.0.6-27+build3", OSID: "debian", OSVersionCodename: "rodete", OSVersionID: "", Maintainer: "LLVM Packaging Team <pkg-llvm-team@lists.alioth.debian.org>", Architecture: "amd64"},
				},
			},
			want: Packages{Deb: []*PkgInfo{
				{Name: "7zip", Version: "24.09+dfsg-4", Arch: "x86_64", Source: Source{Name: "7zip", Version: "24.09+dfsg-4"}, Type: "deb", Purl: "pkg:deb/debian/7zip@24.09%2Bdfsg-4?arch=amd64&distro=rodete"},
				{Name: "llvm-16", Version: "1:16.0.6-27+build3", Arch: "x86_64", Source: Source{Name: "llvm-toolchain-16", Version: "1:16.0.6-27+build3"}, Type: "deb", Purl: "pkg:deb/debian/llvm-16@1%3A16.0.6-27%2Bbuild3?arch=amd64&distro=rodete&source=llvm-toolchain-16"},
			}},
		},
		{
			name: "os/rpm extractor maps correctly",
			pkgs: []*extractor.Package{
				{
					Name:     "acl",
					Version:  "2.2.51-15.el7",
					PURLType: "rpm",
					Metadata: &scalibrrpm.Metadata{PackageName: "acl", SourceRPM: "acl-2.2.51-15.el7.src.rpm", Epoch: 0, OSName: "CentOS Linux", OSID: "centos", OSVersionID: "7", OSBuildID: "", Vendor: "CentOS", Architecture: "x86_64", OSCPEName: ""},
				},
				{
					Name:     "gpg-pubkey",
					Version:  "352c64e5-52ae6884",
					PURLType: "rpm",
					Metadata: &scalibrrpm.Metadata{PackageName: "gpg-pubkey", SourceRPM: "", Epoch: 0, OSName: "CentOS Linux", OSID: "centos", OSVersionID: "7", OSBuildID: "", Vendor: "", Architecture: "", OSCPEName: ""},
				},
			},
			want: Packages{Rpm: []*PkgInfo{
				{Name: "acl", Version: "2.2.51-15.el7", Arch: "x86_64", Source: Source{Name: "acl-2.2.51-15.el7.src.rpm", Version: ""}, Type: "rpm", Purl: "pkg:rpm/centos/acl@2.2.51-15.el7?arch=x86_64&distro=centos-7&sourcerpm=acl-2.2.51-15.el7.src.rpm"},
				{Name: "gpg-pubkey", Version: "352c64e5-52ae6884", Arch: "all", Source: Source{Name: "gpg-pubkey", Version: ""}, Type: "rpm", Purl: "pkg:rpm/centos/gpg-pubkey@352c64e5-52ae6884?distro=centos-7"},
			}},
		},
		{
			name: "os/cos extractor maps correctly",
			arch: "x86_64",
			pkgs: []*extractor.Package{
				{
					Name:     "PySocks",
					Version:  "17412.448.8",
					PURLType: "cos",
					Metadata: &scalibrcos.Metadata{Name: "PySocks", Version: "17412.448.8", Category: "dev-python", OSVersion: "105", OSVersionID: "105", EbuildVersion: "1.6.7-r1"},
				},
				{
					Name:     "chromeos-bsp",
					Version:  "17412.448.8",
					PURLType: "cos",
					Metadata: &scalibrcos.Metadata{Name: "chromeos-bsp", Version: "17412.448.8", Category: "virtual", OSVersion: "105", OSVersionID: "105", EbuildVersion: "3-r1"},
				},
			},
			want: Packages{COS: []*PkgInfo{
				{Name: "dev-python/PySocks", Version: "17412.448.8", Arch: "x86_64", Type: "cos", Purl: "pkg:cos/PySocks@17412.448.8?distro=cos-105"},
				{Name: "virtual/chromeos-bsp", Version: "17412.448.8", Arch: "x86_64", Type: "cos", Purl: "pkg:cos/chromeos-bsp@17412.448.8?distro=cos-105"},
			}},
		},
		{
			name: "unknown package metadata type is ignored",
			pkgs: []*extractor.Package{
				{
					Name:     "unknown",
					Version:  "1.0",
					PURLType: "unknown",
					Metadata: "invalid-metadata-type",
				},
			},
			want: Packages{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pkgInfosFromExtractorPackages(context.TODO(), &scalibr.ScanResult{Inventory: inventory.Inventory{Packages: tt.pkgs}}, &osinfo.OSInfo{Architecture: tt.arch})
			utiltest.AssertEquals(t, got, tt.want)
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

type errorProvider struct{}

func (errorProvider) GetOSInfo(ctx context.Context) (osinfo.OSInfo, error) {
	return osinfo.OSInfo{}, errors.New("osinfo error")
}

func TestScalibrIntegration(t *testing.T) {
	utiltest.OverrideVariable(t, &ZypperExists, false)
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)

	tests := []struct {
		name     string
		setup    func(t *testing.T)
		provider InstalledPackagesProvider
		wantErr  error
		wantPkgs Packages
	}{
		{
			name:  "successful scan, expect packages",
			setup: func(t *testing.T) {},
			provider: scalibrInstalledPackagesProvider{
				osinfoProvider: stubProvider{},
				extractors:     []string{"os/dpkg"},
				scanRootPaths:  []string{arrangeVirtualRoot(t, "./testdata/debian.dpkg-status", "/var/lib/dpkg/status")},
				dirsToSkip:     []string{},
			},
			wantErr: nil,
			wantPkgs: Packages{Deb: []*PkgInfo{
				{Name: "7zip", Version: "24.09+dfsg-4", Arch: "x86_64", Source: Source{Name: "7zip", Version: "24.09+dfsg-4"}, Type: "deb", Purl: "pkg:deb/linux/7zip@24.09%2Bdfsg-4?arch=amd64"},
				{Name: "llvm-16", Version: "1:16.0.6-27+build3", Arch: "x86_64", Source: Source{Name: "llvm-toolchain-16", Version: "1:16.0.6-27+build3"}, Type: "deb", Purl: "pkg:deb/linux/llvm-16@1%3A16.0.6-27%2Bbuild3?arch=amd64&source=llvm-toolchain-16"},
			}},
		},
		{
			name:  "osinfo provider error, expect osinfo error",
			setup: func(t *testing.T) {},
			provider: scalibrInstalledPackagesProvider{
				osinfoProvider: errorProvider{},
				extractors:     []string{"os/dpkg"},
				scanRootPaths:  []string{arrangeVirtualRoot(t, "./testdata/debian.dpkg-status", "/var/lib/dpkg/status")},
				dirsToSkip:     []string{},
			},
			wantErr: errors.New("osinfo error"),
		},
		{
			name:  "invalid extractor, expect unknown plugin error",
			setup: func(t *testing.T) {},
			provider: scalibrInstalledPackagesProvider{
				osinfoProvider: stubProvider{},
				extractors:     []string{"invalid/extractor"},
				scanRootPaths:  []string{arrangeVirtualRoot(t, "./testdata/debian.dpkg-status", "/var/lib/dpkg/status")},
				dirsToSkip:     []string{},
			},
			wantErr: errors.New("unknown plugin \"invalid/extractor\""),
		},
		{
			name:  "skipped directory, expect no packages",
			setup: func(t *testing.T) {},
			provider: scalibrInstalledPackagesProvider{
				osinfoProvider: stubProvider{},
				extractors:     []string{"os/dpkg"},
				scanRootPaths:  []string{arrangeVirtualRoot(t, "./testdata/debian.dpkg-status", "/var/lib/dpkg/status")},
				dirsToSkip:     []string{"testdata/virtualTestRoot/var"},
			},
			wantErr:  nil,
			wantPkgs: Packages{},
		},
		{
			name: "zypper patches succeeds, expect patches in output",
			setup: func(t *testing.T) {
				utiltest.OverrideVariable[util.CommandRunner](t, &runner, mockCommandRunner)
				utiltest.OverrideVariable(t, &ZypperExists, true)
				mockCommandRunner.EXPECT().Run(gomock.Any(), gomock.Any()).Return([]byte("Repo | PatchName | security | critical | --- | applied | Patch summary\n"), []byte(""), nil).Times(1)
			},
			provider: scalibrInstalledPackagesProvider{
				osinfoProvider: stubProvider{},
				extractors:     []string{"os/dpkg"},
				scanRootPaths:  []string{arrangeVirtualRoot(t, "./testdata/debian.dpkg-status", "/var/lib/dpkg/status")},
				dirsToSkip:     []string{},
			},
			wantErr: nil,
			wantPkgs: Packages{
				Deb: []*PkgInfo{
					{Name: "7zip", Version: "24.09+dfsg-4", Arch: "x86_64", Source: Source{Name: "7zip", Version: "24.09+dfsg-4"}, Type: "deb", Purl: "pkg:deb/linux/7zip@24.09%2Bdfsg-4?arch=amd64"},
					{Name: "llvm-16", Version: "1:16.0.6-27+build3", Arch: "x86_64", Source: Source{Name: "llvm-toolchain-16", Version: "1:16.0.6-27+build3"}, Type: "deb", Purl: "pkg:deb/linux/llvm-16@1%3A16.0.6-27%2Bbuild3?arch=amd64&source=llvm-toolchain-16"},
				},
				ZypperPatches: []*ZypperPatch{
					{Name: "PatchName", Category: "security", Severity: "critical", Summary: "Patch summary"},
				},
			},
		},
		{
			name: "zypper patches fails, expect error",
			setup: func(t *testing.T) {
				utiltest.OverrideVariable[util.CommandRunner](t, &runner, mockCommandRunner)
				utiltest.OverrideVariable(t, &ZypperExists, true)
				mockCommandRunner.EXPECT().Run(gomock.Any(), gomock.Any()).Return(nil, nil, errors.New("zypper error")).Times(1)
			},
			provider: scalibrInstalledPackagesProvider{
				osinfoProvider: stubProvider{},
				extractors:     []string{"os/dpkg"},
				scanRootPaths:  []string{arrangeVirtualRoot(t, "./testdata/debian.dpkg-status", "/var/lib/dpkg/status")},
				dirsToSkip:     []string{},
			},
			wantErr: errors.New("error getting zypper installed patches: error running /usr/bin/zypper with args [\"--gpg-auto-import-keys\" \"-q\" \"list-patches\" \"--all\"]: zypper error, stdout: \"\", stderr: \"\""),
			wantPkgs: Packages{
				Deb: []*PkgInfo{
					{Name: "7zip", Version: "24.09+dfsg-4", Arch: "x86_64", Source: Source{Name: "7zip", Version: "24.09+dfsg-4"}, Type: "deb", Purl: "pkg:deb/linux/7zip@24.09%2Bdfsg-4?arch=amd64"},
					{Name: "llvm-16", Version: "1:16.0.6-27+build3", Arch: "x86_64", Source: Source{Name: "llvm-toolchain-16", Version: "1:16.0.6-27+build3"}, Type: "deb", Purl: "pkg:deb/linux/llvm-16@1%3A16.0.6-27%2Bbuild3?arch=amd64&source=llvm-toolchain-16"},
				},
			},
		},
		{
			name:  "scalibr scan fails, expect unhealthy status error",
			setup: func(t *testing.T) {},
			provider: scalibrInstalledPackagesProvider{
				osinfoProvider: stubProvider{},
				extractors:     []string{"os/dpkg"},
				scanRootPaths:  []string{arrangeVirtualRoot(t, "./testdata/debian.dpkg-status", "/var/lib/dpkg/status")},
				dirsToSkip:     []string{"/proc"},
			},
			wantErr: errors.New("scalibr scan.Status is unhealthy, status: FAILED: path not relative to any of the scan roots, plugins: []"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)

			gotPkgs, gotErr := tt.provider.GetInstalledPackages(context.Background())
			utiltest.AssertErrorMatch(t, gotErr, tt.wantErr)
			utiltest.AssertEquals(t, gotPkgs, tt.wantPkgs)
		})
	}
}
