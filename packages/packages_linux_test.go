package packages

import (
	"context"
	"fmt"
	"os/exec"
	"slices"
	"testing"

	"github.com/GoogleCloudPlatform/osconfig/osinfo"
	utilmocks "github.com/GoogleCloudPlatform/osconfig/util/mocks"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
)

func TestGetPackageUpdate(t *testing.T) {
	enableAllPackageUpdates()

	wantCommandChain := []expectedCommand{
		{
			cmd:    exec.Command(aptGet, aptGetUpdateArgs...),
			envs:   []string{"DEBIAN_FRONTEND=noninteractive"},
			stdout: []byte(""),
			stderr: []byte(""),
			err:    nil,
		},
		{
			cmd:    exec.Command(aptGet, append(slices.Clone(aptGetUpgradableArgs), aptGetFullUpgradeCmd)...),
			envs:   []string{"DEBIAN_FRONTEND=noninteractive"},
			stdout: []byte(""),
			stderr: []byte(""),
			err:    nil,
		},
		{
			cmd:    exec.Command(yum, yumCheckUpdateArgs...),
			stdout: []byte(""),
			stderr: []byte(""),
			err:    exec.Command("/bin/bash", "-c", "exit 100").Run(),
		},
		{
			cmd:    exec.Command(yum, yumListUpdatesArgs...),
			stdout: utiltest.BytesFromFile(t, "./testdata/centos-7-1.yum-update.stdout"),
			stderr: utiltest.BytesFromFile(t, "./testdata/centos-7-1.yum-update.stderr"),
			err:    nil,
		},
		{
			cmd:    exec.Command(zypper, zypperListUpdatesArgs...),
			stdout: utiltest.BytesFromFile(t, "./testdata/sles-12-1.zypper-list-updates.stdout"),
			stderr: []byte(""),
			err:    nil,
		},
		{
			cmd:    exec.Command(zypper, append(zypperListPatchesArgs, "--all")...),
			stdout: utiltest.BytesFromFile(t, "./testdata/sles-12-1.zypper-list-patches.stdout"),
			stderr: []byte(""),
			err:    nil,
		},
		{
			cmd:    exec.Command("/usr/bin/gem", "outdated", "--local"),
			stdout: []byte(""),
			stderr: []byte(""),
			err:    nil,
		},
		{
			cmd:    exec.Command("/usr/bin/pip", "list", "--format=json", "--outdated"),
			stdout: utiltest.BytesFromFile(t, "./testdata/linux-pip-list-outdated-format-json.stdout"),
			stderr: []byte(""),
			err:    nil,
		},
	}

	oi := osinfo.OSInfo{Hostname: "Hostname",
		LongName:      "Longname",
		ShortName:     "Shortname",
		Version:       "Version",
		KernelVersion: "KernelVersion",
		KernelRelease: "KernelRelease",
		Architecture:  "Architecture"}

	oiProvider := &stubOsInfoProvider{
		osinfo: func(_ context.Context) (osinfo.OSInfo, error) { return oi, nil },
	}

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	ptyrunner = mockCommandRunner

	setExpectations(mockCommandRunner, wantCommandChain)

	packageUpdatesProvider := NewPackageUpdatesProvider(oiProvider)
	_, err := packageUpdatesProvider.GetPackageUpdates(context.Background())
	if err != nil {
		t.Errorf("unexpected error, got: %v, want: <nil>", err)
	}
}

func Test_getPackageUpdateErrorPropagation(t *testing.T) {
	enableAllPackageUpdates()

	wantCommandChain := []expectedCommand{
		{
			cmd:    exec.Command(aptGet, aptGetUpdateArgs...),
			envs:   []string{"DEBIAN_FRONTEND=noninteractive"},
			stdout: []byte(""),
			stderr: []byte(""),
			err:    fmt.Errorf("apt-get update fail"),
		},
		{
			cmd:    exec.Command(yum, yumCheckUpdateArgs...),
			stdout: []byte(""),
			stderr: []byte(""),
			err:    exec.Command("/bin/bash", "-c", "exit 100").Run(),
		},
		{
			cmd:    exec.Command(yum, yumListUpdatesArgs...),
			stdout: []byte(""),
			stderr: []byte(""),
			err:    fmt.Errorf("yum list updates fail"),
		},
		{
			cmd:    exec.Command(zypper, zypperListUpdatesArgs...),
			stdout: []byte(""),
			stderr: []byte(""),
			err:    fmt.Errorf("zypper list updates fail"),
		},
		{
			cmd:    exec.Command(zypper, append(zypperListPatchesArgs, "--all")...),
			stdout: []byte(""),
			stderr: []byte(""),
			err:    fmt.Errorf("zypper list patches fail"),
		},
		{
			cmd:    exec.Command("/usr/bin/gem", "outdated", "--local"),
			stdout: []byte(""),
			stderr: []byte(""),
			err:    fmt.Errorf("gem outdated fail"),
		},
		{
			cmd:    exec.Command("/usr/bin/pip", "list", "--format=json", "--outdated"),
			stdout: []byte(""),
			stderr: []byte(""),
			err:    fmt.Errorf("pip list --outdated fail"),
		},
	}

	oi := osinfo.OSInfo{Hostname: "Hostname",
		LongName:      "Longname",
		ShortName:     "Shortname",
		Version:       "Version",
		KernelVersion: "KernelVersion",
		KernelRelease: "KernelRelease",
		Architecture:  "Architecture"}

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	ptyrunner = mockCommandRunner

	setExpectations(mockCommandRunner, wantCommandChain)

	wantErrors := []string{
		"error getting apt updates: apt-get update fail",
		`error getting yum updates: error running /usr/bin/yum with args ["update" "--assumeno" "--color=never"]: yum list updates fail, stdout: "", stderr: ""`,
		`error getting zypper updates: error running /usr/bin/zypper with args ["--gpg-auto-import-keys" "-q" "list-updates"]: zypper list updates fail, stdout: "", stderr: ""`,
		`error getting zypper available patches: error running /usr/bin/zypper with args ["--gpg-auto-import-keys" "-q" "list-patches" "--all"]: zypper list patches fail, stdout: "", stderr: ""`,
	}

	_, errs := getPackageUpdates(context.Background(), oi)
	if diff := cmp.Diff(wantErrors, errs); diff != "" {
		t.Errorf("expected set of errors, Diff:\n%s", diff)
	}
}

func TestGetInstalledPackages(t *testing.T) {
	enableAllInstalledPackages()
	COSPkgInfoExists = false //Skip explicitly as there is different model.

	wantCommandChain := []expectedCommand{
		{
			cmd: exec.Command(rpmquery, rpmqueryInstalledArgs...),
			stdout: []byte("" +
				`{"architecture":"x86_64","package":"gcc","source_name":"gcc-11.4.1-3.el9.src.rpm","version":"11.4.1-3.el9"}` + "\n"),
			stderr: []byte(""),
			err:    nil,
		},
		{
			cmd:    exec.Command(zypper, append(zypperListPatchesArgs, "--all")...),
			stdout: []byte("SLE-Module-Basesystem15-SP1-Updates | SUSE-SLE-Module-Basesystem-15-SP1-2019-1258 | recommended | moderate  | ---         | applied     | Recommended update for postfix"),
			stderr: []byte(""),
			err:    nil,
		},
		{
			cmd:    exec.Command(dpkgQuery, dpkgQueryArgs...),
			stdout: []byte(`{"package":"git","architecture":"amd64","version":"1:2.25.1-1ubuntu3.12","status":"installed","source_name":"git","source_version":"1:2.25.1-1ubuntu3.12"}`),
			stderr: []byte(""),
			err:    nil,
		},
		{
			cmd:    exec.Command("/usr/bin/gem", "list", "--local"),
			stdout: []byte(""),
			stderr: []byte(""),
			err:    nil,
		},
		{
			cmd:    exec.Command("/usr/bin/pip", "list", "--format=json"),
			stdout: utiltest.BytesFromFile(t, "./testdata/linux-pip-list-format-json.stdout"),
			stderr: []byte(""),
			err:    nil,
		},
	}

	oi := osinfo.OSInfo{Hostname: "Hostname",
		LongName:      "Longname",
		ShortName:     "Shortname",
		Version:       "Version",
		KernelVersion: "KernelVersion",
		KernelRelease: "KernelRelease",
		Architecture:  "Architecture"}

	oiProvider := &stubOsInfoProvider{
		osinfo: func(_ context.Context) (osinfo.OSInfo, error) { return oi, nil },
	}

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	ptyrunner = mockCommandRunner
	setExpectations(mockCommandRunner, wantCommandChain)

	installedPackagesProvider := NewInstalledPackagesProvider(oiProvider)
	if _, err := installedPackagesProvider.GetInstalledPackages(context.Background()); err != nil {
		t.Errorf("unexpected error, got: %v, want: <nil>", err)
	}
}
func Test_getInstalledPackages(t *testing.T) {
	enableAllInstalledPackages()
	COSPkgInfoExists = false //explicitly skip for now

	wantCommandChain := []expectedCommand{
		{
			cmd:    exec.Command(rpmquery, rpmqueryInstalledArgs...),
			stdout: []byte(""),
			stderr: []byte(""),
			err:    fmt.Errorf("rpm query failed"),
		},
		{
			cmd:    exec.Command(zypper, append(zypperListPatchesArgs, "--all")...),
			stdout: []byte(""),
			stderr: []byte(""),
			err:    fmt.Errorf("zypper list patches failed"),
		},
		{
			cmd:    exec.Command(dpkgQuery, dpkgQueryArgs...),
			stdout: []byte(``),
			stderr: []byte(""),
			err:    fmt.Errorf("dpkg query failed"),
		},
		{
			cmd:    exec.Command("/usr/bin/gem", "list", "--local"),
			stdout: []byte(""),
			stderr: []byte(""),
			err:    fmt.Errorf("gem failed"),
		},
		{
			cmd:    exec.Command("/usr/bin/pip", "list", "--format=json"),
			stdout: utiltest.BytesFromFile(t, "./testdata/linux-pip-list-format-json.stdout"),
			stderr: []byte(""),
			err:    fmt.Errorf("pip list failed"),
		},
	}

	oi := osinfo.OSInfo{Hostname: "Hostname",
		LongName:      "Longname",
		ShortName:     "Shortname",
		Version:       "Version",
		KernelVersion: "KernelVersion",
		KernelRelease: "KernelRelease",
		Architecture:  "Architecture"}

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	ptyrunner = mockCommandRunner

	setExpectations(mockCommandRunner, wantCommandChain)

	wantErrors := []string{
		`error listing installed rpm packages: error running /usr/bin/rpmquery with args ["--queryformat" "\\{\"architecture\":\"%{ARCH}\",\"package\":\"%{NAME}\",\"source_name\":\"%{SOURCERPM}\",\"version\":\"%|EPOCH?{%{EPOCH}:}:{}|%{VERSION}-%{RELEASE}\"\\}\n" "-a"]: rpm query failed, stdout: "", stderr: ""`,
		`error getting zypper installed patches: error running /usr/bin/zypper with args ["--gpg-auto-import-keys" "-q" "list-patches" "--all"]: zypper list patches failed, stdout: "", stderr: ""`,
		`error listing installed deb packages: error running /usr/bin/dpkg-query with args ["-W" "-f" "\\{\"architecture\":\"${Architecture}\",\"package\":\"${Package}\",\"source_name\":\"${source:Package}\",\"source_version\":\"${source:Version}\",\"status\":\"${db:Status-Status}\",\"version\":\"${Version}\"\\}\n"]: dpkg query failed, stdout: "", stderr: ""`,
	}

	_, errs := getInstalledPackages(context.Background(), oi)
	if diff := cmp.Diff(wantErrors, errs); diff != "" {
		t.Errorf("expected set of errors, Diff:\n%s", diff)
	}
}

func Test_enrichPkgInfoWithPurl(t *testing.T) {
	tests := []struct {
		name                  string
		pkgInfo               []*PkgInfo
		shortname             string
		version               string
		enrichPkgInfoFunction func([]*PkgInfo, string, string) []*PkgInfo
		wantEnrichedPkgInfo   []*PkgInfo
	}{
		{
			name: "Correctly create PURL for RPM packages",
			pkgInfo: []*PkgInfo{
				{
					Name:    "RpmPkg",
					Arch:    "x86_64",
					RawArch: "noarch",
					Version: "Version",
					Type:    "rpm",
				},
			},
			shortname:             "Namespace",
			version:               "Version",
			enrichPkgInfoFunction: enrichRpmPkgInfoWithPurl,
			wantEnrichedPkgInfo: []*PkgInfo{
				{
					Name:    "RpmPkg",
					Arch:    "x86_64",
					RawArch: "noarch",
					Version: "Version",
					Type:    "rpm",
					Purl:    "pkg:rpm/Namespace/RpmPkg@Version?arch=x86_64&distro=Version",
				},
			},
		},
		{
			name: "Correctly create PURL for Deb packages",
			pkgInfo: []*PkgInfo{
				{
					Name:    "DebPkg",
					Arch:    "x86_64",
					RawArch: "",
					Version: "Version",
					Type:    "deb",
					Source:  Source{Name: "SourceName", Version: "SourceVersion"},
				},
			},
			shortname:             "Namespace",
			version:               "Version",
			enrichPkgInfoFunction: enrichDebPkgInfoWithPurl,
			wantEnrichedPkgInfo: []*PkgInfo{
				{
					Name:    "DebPkg",
					Arch:    "x86_64",
					RawArch: "",
					Version: "Version",
					Type:    "deb",
					Source:  Source{Name: "SourceName", Version: "SourceVersion"},
					Purl:    "pkg:deb/Namespace/DebPkg@Version?arch=x86_64&distro=Version&source=SourceName",
				},
			},
		},
		{
			name: "Correctly create PURL for Cos packages",
			pkgInfo: []*PkgInfo{
				{
					Name:    "CosPkg",
					Arch:    "x86_64",
					RawArch: "noarch",
					Version: "Version",
					Type:    "cos",
				},
			},
			shortname:             "Namespace",
			version:               "Version",
			enrichPkgInfoFunction: enrichCosPkgInfoWithPurl,
			wantEnrichedPkgInfo: []*PkgInfo{
				{
					Name:    "CosPkg",
					Arch:    "x86_64",
					RawArch: "noarch",
					Version: "Version",
					Type:    "cos",
					Purl:    "pkg:cos/Namespace/CosPkg@Version?arch=x86_64&distro=Namespace-Version",
				},
			},
		}}

	for _, tt := range tests {
		got := tt.enrichPkgInfoFunction(tt.pkgInfo, tt.shortname, tt.version)

		if diff := cmp.Diff(tt.wantEnrichedPkgInfo, got); diff != "" {
			t.Errorf("unexpected diff, diff:\n%s", diff)
		}
	}
}

func Test_enrichGemPkgInfoWithPurl(t *testing.T) {
	tests := []struct {
		name                string
		pkgInfo             []*PkgInfo
		wantEnrichedPkgInfo []*PkgInfo
	}{
		{
			name: "Correctly create PURL for GEM packages",
			pkgInfo: []*PkgInfo{
				{
					Name:    "GemPkg",
					Arch:    "x86_64",
					RawArch: "noarch",
					Version: "Version",
					Type:    "gem",
				},
			},
			wantEnrichedPkgInfo: []*PkgInfo{
				{
					Name:    "GemPkg",
					Arch:    "x86_64",
					RawArch: "noarch",
					Version: "Version",
					Type:    "gem",
					Purl:    "pkg:gem/GemPkg@Version",
				},
			},
		}}

	for _, tt := range tests {
		got := enrichGemPkgInfoWithPurl(tt.pkgInfo)

		if diff := cmp.Diff(tt.wantEnrichedPkgInfo, got); diff != "" {
			t.Errorf("unexpected diff, diff:\n%s", diff)
		}
	}
}

func Test_enrichPipPkgInfoWithPurl(t *testing.T) {
	tests := []struct {
		name                string
		pkgInfo             []*PkgInfo
		wantEnrichedPkgInfo []*PkgInfo
	}{
		{
			name: "Correctly create PURL for PyPI packages",
			pkgInfo: []*PkgInfo{
				{
					Name:    "PipPkg",
					Arch:    "x86_64",
					RawArch: "noarch",
					Version: "Version",
					Type:    "pypi",
				},
			},
			wantEnrichedPkgInfo: []*PkgInfo{
				{
					Name:    "PipPkg",
					Arch:    "x86_64",
					RawArch: "noarch",
					Version: "Version",
					Type:    "pypi",
					Purl:    "pkg:pypi/PipPkg@Version",
				},
			},
		}}

	for _, tt := range tests {
		got := enrichPipPkgInfoWithPurl(tt.pkgInfo)

		if diff := cmp.Diff(tt.wantEnrichedPkgInfo, got); diff != "" {
			t.Errorf("unexpected diff, diff:\n%s", diff)
		}
	}
}

func enableAllPackageUpdates() {
	AptExists = true
	YumExists = true
	ZypperExists = true
	GemExists = true
	PipExists = true
}

func enableAllInstalledPackages() {
	RPMQueryExists = true
	ZypperExists = true
	DpkgQueryExists = true
	COSPkgInfoExists = true
	GemExists = true
	PipExists = true
}

type stubOsInfoProvider struct {
	osinfo func(context.Context) (osinfo.OSInfo, error)
}

func (p stubOsInfoProvider) GetOSInfo(ctx context.Context) (osinfo.OSInfo, error) {
	return p.osinfo(ctx)
}
