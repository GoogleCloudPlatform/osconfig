package packages

import (
	"context"
	"fmt"
	"os/exec"
	"slices"
	"testing"

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
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	ptyrunner = mockCommandRunner

	setExpectations(mockCommandRunner, wantCommandChain)

	_, err := GetPackageUpdates(context.Background())
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

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	ptyrunner = mockCommandRunner

	setExpectations(mockCommandRunner, wantCommandChain)

	wantErrors := []string{
		"error getting apt updates: apt-get update fail",
		`error getting yum updates: error running /usr/bin/yum with args ["update" "--assumeno" "--cacheonly" "--color=never"]: yum list updates fail, stdout: "", stderr: ""`,
		`error getting zypper updates: error running /usr/bin/zypper with args ["--gpg-auto-import-keys" "-q" "list-updates"]: zypper list updates fail, stdout: "", stderr: ""`,
		`error getting zypper available patches: error running /usr/bin/zypper with args ["--gpg-auto-import-keys" "-q" "list-patches" "--all"]: zypper list patches fail, stdout: "", stderr: ""`,
	}

	_, errs := getPackageUpdates(context.Background())
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

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	ptyrunner = mockCommandRunner
	setExpectations(mockCommandRunner, wantCommandChain)

	if _, err := GetInstalledPackages(context.Background()); err != nil {
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

	_, errs := getInstalledPackages(context.Background())
	if diff := cmp.Diff(wantErrors, errs); diff != "" {
		t.Errorf("expected set of errors, Diff:\n%s", diff)
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
