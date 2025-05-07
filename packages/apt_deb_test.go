//  Copyright 2019 Google Inc. All Rights Reserved.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0 //
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package packages

import (
	"errors"
	"fmt"
	"os/exec"
	"reflect"
	"slices"
	"testing"

	utilmocks "github.com/GoogleCloudPlatform/osconfig/util/mocks"
	utiltest "github.com/GoogleCloudPlatform/osconfig/util/utiltest"
	"github.com/golang/mock/gomock"
)

func TestInstallAptPackages(t *testing.T) {
	tests := []struct {
		name string
		pkgs []string

		expectedCommandsChain []expectedCommand
		expectedError         error
	}{
		{
			name: "basic installation",
			pkgs: []string{"pkg1", "pkg2"},

			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command(aptGet, append(aptGetInstallArgs, pkgs...)...),
					envs:   []string{"DEBIAN_FRONTEND=noninteractive"},
					stdout: []byte("stdout"),
					stderr: []byte("stderr"),
					err:    nil,
				},
			},
			expectedError: nil,
		},
		{
			name: "allow downgrade added if specific error",
			pkgs: []string{"pkg1", "pkg2"},

			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command(aptGet, append(aptGetInstallArgs, pkgs...)...),
					envs:   []string{"DEBIAN_FRONTEND=noninteractive"},
					stdout: []byte("stdout"),
					stderr: []byte("E: Packages were downgraded and -y was used without --allow-downgrades."),
					err:    errors.New("unexpected error"),
				},
				{
					cmd:    exec.Command(aptGet, append(append(aptGetInstallArgs, pkgs...), allowDowngradesArg)...),
					envs:   []string{"DEBIAN_FRONTEND=noninteractive"},
					stdout: []byte("stdout"),
					stderr: []byte("stderr"),
					err:    nil,
				},
			},
			expectedError: nil,
		},
		{
			name: "run dpkg repair on dpkg error",
			pkgs: []string{"pkg1", "pkg2"},

			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command(aptGet, append(aptGetInstallArgs, pkgs...)...),
					envs:   []string{"DEBIAN_FRONTEND=noninteractive"},
					stdout: []byte("stdout"),
					stderr: dpkgErr,
					err:    errors.New("unexpected error"),
				},
				{
					cmd:    exec.CommandContext(testCtx, dpkg, dpkgRepairArgs...),
					envs:   nil,
					stdout: []byte("stdout"),
					stderr: []byte("stderr"),
					err:    nil,
				},
				{
					cmd:    exec.Command(aptGet, append(aptGetInstallArgs, pkgs...)...),
					envs:   []string{"DEBIAN_FRONTEND=noninteractive"},
					stdout: []byte("stdout"),
					stderr: []byte("stderr"),
					err:    nil,
				},
			},
			expectedError: nil,
		},
		{
			name: "throw an error if non dpkgErr",
			pkgs: []string{"pkg1", "pkg2"},
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command(aptGet, append(slices.Clone(aptGetInstallArgs), pkgs...)...),
					envs:   []string{"DEBIAN_FRONTEND=noninteractive"},
					stdout: []byte("stdout"),
					stderr: []byte("stderr"),
					err:    errors.New("unexpected error"),
				},
			},
			expectedError: errors.New("error running /usr/bin/apt-get with args" +
				" [\"install\" \"-y\" \"pkg1\" \"pkg2\"]:" +
				" unexpected error, stdout: \"stdout\", stderr: \"stderr\""),
		},
		{
			name: "throw an error if any at the end",
			pkgs: []string{"pkg1", "pkg2"},

			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command(aptGet, append(aptGetInstallArgs, pkgs...)...),
					envs:   []string{"DEBIAN_FRONTEND=noninteractive"},
					stdout: []byte("stdout"),
					stderr: dpkgErr,
					err:    errors.New("unexpected error"),
				},
				{
					cmd:    exec.CommandContext(testCtx, dpkg, dpkgRepairArgs...),
					envs:   nil,
					stdout: []byte("stdout"),
					stderr: []byte("stderr"),
					err:    nil,
				},
				{
					cmd:    exec.Command(aptGet, append(aptGetInstallArgs, pkgs...)...),
					envs:   []string{"DEBIAN_FRONTEND=noninteractive"},
					stdout: []byte("stdout"),
					stderr: []byte("stderr"),
					err:    errors.New("unexpected error"),
				},
			},
			expectedError: errors.New("error running /usr/bin/apt-get with args" +
				" [\"install\" \"-y\" \"pkg1\" \"pkg2\"]:" +
				" unexpected error, stdout: \"stdout\", stderr: \"stderr\""),
		},
	}

	for _, tt := range tests {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
		runner = mockCommandRunner

		t.Run(tt.name, func(t *testing.T) {
			setExpectations(mockCommandRunner, tt.expectedCommandsChain)

			err := InstallAptPackages(testCtx, tt.pkgs)
			if !reflect.DeepEqual(err, tt.expectedError) {
				t.Errorf("InstallAptPackages: unexpected error, expect %q, got %q", formatError(tt.expectedError), formatError(err))
			}
		})
	}
}

func TestAptUpdates(t *testing.T) {
	tests := []struct {
		name                  string
		args                  []AptGetUpgradeOption
		expectedCommandsChain []expectedCommand
		expectedResults       []*PkgInfo
		expectedResultsFile   string
		expectedError         error
	}{
		{
			name:                  "UnexpectedUpgradeType",
			args:                  []AptGetUpgradeOption{AptGetUpgradeType(10)},
			expectedCommandsChain: nil,
			expectedResults:       nil,
			expectedError:         fmt.Errorf("unknown upgrade type: %q", 10),
		},
		{
			name: "apt-get updates",
			args: nil,
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command(aptGet, aptGetUpdateArgs...),
					envs:   []string{"DEBIAN_FRONTEND=noninteractive"},
					stdout: []byte("stdout"),
					stderr: []byte("stderr"),
					err:    errors.New("unexpected error"),
				},
			},
			expectedResults: nil,
			expectedError:   errors.New("unexpected error"),
		},
		{
			name: "apt-get upgrade fail",
			args: nil,
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command(aptGet, aptGetUpdateArgs...),
					envs:   []string{"DEBIAN_FRONTEND=noninteractive"},
					stdout: []byte("stdout"),
					stderr: []byte(""),
					err:    nil,
				},
				{
					cmd:    exec.Command(aptGet, append(slices.Clone(aptGetUpgradableArgs), aptGetUpgradeCmd)...),
					envs:   []string{"DEBIAN_FRONTEND=noninteractive"},
					stdout: []byte("stdout"),
					stderr: []byte("stderr"),
					err:    errors.New("unexpected error"),
				},
			},
			expectedResults: nil,
			expectedError:   errors.New("unexpected error"),
		},
		{
			name: "Default upgrade type",
			args: nil,
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command(aptGet, aptGetUpdateArgs...),
					envs:   []string{"DEBIAN_FRONTEND=noninteractive"},
					stdout: []byte("stdout"),
					stderr: []byte(""),
					err:    nil,
				},
				{
					cmd:    exec.Command(aptGet, append(slices.Clone(aptGetUpgradableArgs), aptGetUpgradeCmd)...),
					envs:   []string{"DEBIAN_FRONTEND=noninteractive"},
					stdout: []byte("Inst google-cloud-sdk [245.0.0-0] (246.0.0-0 cloud-sdk-stretch:cloud-sdk-stretch [amd64])"),
					stderr: []byte(""),
					err:    nil,
				},
			},
			expectedResults: []*PkgInfo{{Name: "google-cloud-sdk", Arch: "x86_64", Version: "246.0.0-0"}},
			expectedError:   nil,
		},
		{
			name: "Dist upgrade type",
			args: []AptGetUpgradeOption{AptGetUpgradeType(AptGetDistUpgrade)},
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command(aptGet, aptGetUpdateArgs...),
					envs:   []string{"DEBIAN_FRONTEND=noninteractive"},
					stdout: []byte("stdout"),
					stderr: []byte(""),
					err:    nil,
				},
				{
					cmd:    exec.Command(aptGet, append(slices.Clone(aptGetUpgradableArgs), aptGetDistUpgradeCmd)...),
					envs:   []string{"DEBIAN_FRONTEND=noninteractive"},
					stdout: []byte("Inst google-cloud-sdk [245.0.0-0] (246.0.0-0 cloud-sdk-stretch:cloud-sdk-stretch [amd64])"),
					stderr: []byte(""),
					err:    nil,
				},
			},
			expectedResults: []*PkgInfo{{Name: "google-cloud-sdk", Arch: "x86_64", Version: "246.0.0-0"}},
			expectedError:   nil,
		},
		{
			name: "Full upgrade type",
			args: []AptGetUpgradeOption{AptGetUpgradeType(AptGetFullUpgrade)},
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command(aptGet, aptGetUpdateArgs...),
					envs:   []string{"DEBIAN_FRONTEND=noninteractive"},
					stdout: []byte("stdout"),
					stderr: []byte(""),
					err:    nil,
				},
				{
					cmd:    exec.Command(aptGet, append(slices.Clone(aptGetUpgradableArgs), aptGetFullUpgradeCmd)...),
					envs:   []string{"DEBIAN_FRONTEND=noninteractive"},
					stdout: []byte("Inst google-cloud-sdk [245.0.0-0] (246.0.0-0 cloud-sdk-stretch:cloud-sdk-stretch [amd64])"),
					stderr: []byte(""),
					err:    nil,
				},
			},
			expectedResults: []*PkgInfo{{Name: "google-cloud-sdk", Arch: "x86_64", Version: "246.0.0-0"}},
			expectedError:   nil,
		},
		{
			name: "debian-10-1 mapped full-upgrade stdout matches snapshot",
			args: []AptGetUpgradeOption{AptGetUpgradeType(AptGetFullUpgrade)},
			expectedCommandsChain: []expectedCommand{
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
					stdout: utiltest.BytesFromFile(t, "./testdata/debian-10-1.apt-get-full-upgrade.stdout"),
					stderr: []byte(""),
					err:    nil,
				},
			},
			expectedResultsFile: "./testdata/debian-10-1.apt-get-full-upgrade.expected",
			expectedError:       nil,
		},
		{
			name: "debian-11-1 mapped full-upgrade stdout matches snapshot",
			args: []AptGetUpgradeOption{AptGetUpgradeType(AptGetFullUpgrade)},
			expectedCommandsChain: []expectedCommand{
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
					stdout: utiltest.BytesFromFile(t, "./testdata/debian-11-1.apt-get-full-upgrade.stdout"),
					stderr: []byte(""),
					err:    nil,
				},
			},
			expectedResultsFile: "./testdata/debian-11-1.apt-get-full-upgrade.expected",
			expectedError:       nil,
		},
		{
			name: "debian-12-1 mapped full-upgrade stdout matches snapshot",
			args: []AptGetUpgradeOption{AptGetUpgradeType(AptGetFullUpgrade)},
			expectedCommandsChain: []expectedCommand{
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
					stdout: utiltest.BytesFromFile(t, "./testdata/debian-12-1.apt-get-full-upgrade.stdout"),
					stderr: []byte(""),
					err:    nil,
				},
			},
			expectedResultsFile: "./testdata/debian-12-1.apt-get-full-upgrade.expected",
			expectedError:       nil,
		},
		{
			name: "ubuntu-20 mapped full-upgrade stdout matches snapshot",
			args: []AptGetUpgradeOption{AptGetUpgradeType(AptGetFullUpgrade)},
			expectedCommandsChain: []expectedCommand{
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
					stdout: utiltest.BytesFromFile(t, "./testdata/ubuntu-20.apt-get-full-upgrade.stdout"),
					stderr: []byte(""),
					err:    nil,
				},
			},
			expectedResultsFile: "./testdata/ubuntu-20.apt-get-full-upgrade.expected",
			expectedError:       nil,
		},
		{
			name: "ubuntu-22 mapped full-upgrade stdout matches snapshot",
			args: []AptGetUpgradeOption{AptGetUpgradeType(AptGetFullUpgrade)},
			expectedCommandsChain: []expectedCommand{
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
					stdout: utiltest.BytesFromFile(t, "./testdata/ubuntu-22.apt-get-full-upgrade.stdout"),
					stderr: []byte(""),
					err:    nil,
				},
			},
			expectedResultsFile: "./testdata/ubuntu-22.apt-get-full-upgrade.expected",
			expectedError:       nil,
		},
		{
			name: "ubuntu-24 mapped full-upgrade stdout matches snapshot",
			args: []AptGetUpgradeOption{AptGetUpgradeType(AptGetFullUpgrade)},
			expectedCommandsChain: []expectedCommand{
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
					stdout: utiltest.BytesFromFile(t, "./testdata/ubuntu-24.apt-get-full-upgrade.stdout"),
					stderr: []byte(""),
					err:    nil,
				},
			},
			expectedResultsFile: "./testdata/ubuntu-24.apt-get-full-upgrade.expected",
			expectedError:       nil,
		},
		{
			name: "Default upgrade type with showNew equals true",
			args: []AptGetUpgradeOption{AptGetUpgradeShowNew(true)},
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command(aptGet, aptGetUpdateArgs...),
					envs:   []string{"DEBIAN_FRONTEND=noninteractive"},
					stdout: []byte("stdout"),
					stderr: []byte(""),
					err:    nil,
				},
				{
					cmd:  exec.Command(aptGet, append(slices.Clone(aptGetUpgradableArgs), aptGetUpgradeCmd)...),
					envs: []string{"DEBIAN_FRONTEND=noninteractive"},
					stdout: []byte(
						"Inst google-cloud-sdk [245.0.0-0] (246.0.0-0 cloud-sdk-stretch:cloud-sdk-stretch [amd64])\n" +
							"Inst firmware-linux-free (3.4 Debian:9.9/stable [all]) []"),
					stderr: []byte(""),
					err:    nil,
				},
			},
			expectedResults: []*PkgInfo{
				{Name: "google-cloud-sdk", Arch: "x86_64", Version: "246.0.0-0"},
				{Name: "firmware-linux-free", Arch: "all", Version: "3.4"},
			},
			expectedError: nil,
		},
		{
			name: "Default upgrade type with showNew equals false",
			args: []AptGetUpgradeOption{AptGetUpgradeShowNew(false)},
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command(aptGet, aptGetUpdateArgs...),
					envs:   []string{"DEBIAN_FRONTEND=noninteractive"},
					stdout: []byte("stdout"),
					stderr: []byte(""),
					err:    nil,
				},
				{
					cmd:  exec.Command(aptGet, append(slices.Clone(aptGetUpgradableArgs), aptGetUpgradeCmd)...),
					envs: []string{"DEBIAN_FRONTEND=noninteractive"},
					stdout: []byte(
						"Inst google-cloud-sdk [245.0.0-0] (246.0.0-0 cloud-sdk-stretch:cloud-sdk-stretch [amd64])\n" +
							"Inst firmware-linux-free (3.4 Debian:9.9/stable [all]) []"),
					stderr: []byte(""),
					err:    nil,
				},
			},
			expectedResults: []*PkgInfo{
				{Name: "google-cloud-sdk", Arch: "x86_64", Version: "246.0.0-0"},
			},
			expectedError: nil,
		},
		{
			name: "Add --allow-downgrades when specific error provided.",
			args: nil,
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command(aptGet, aptGetUpdateArgs...),
					envs:   []string{"DEBIAN_FRONTEND=noninteractive"},
					stdout: []byte("stdout"),
					stderr: []byte(""),
					err:    nil,
				},
				{
					cmd:    exec.Command(aptGet, append(slices.Clone(aptGetUpgradableArgs), aptGetUpgradeCmd)...),
					envs:   []string{"DEBIAN_FRONTEND=noninteractive"},
					stdout: []byte("stdout"),
					stderr: []byte("E: Packages were downgraded and -y was used without --allow-downgrades."),
					err:    errors.New("failure"),
				},
				{
					cmd:    exec.Command(aptGet, append(slices.Clone(aptGetUpgradableArgs), aptGetUpgradeCmd, allowDowngradesArg)...),
					envs:   []string{"DEBIAN_FRONTEND=noninteractive"},
					stdout: []byte("Inst google-cloud-sdk [245.0.0-0] (246.0.0-0 cloud-sdk-stretch:cloud-sdk-stretch [amd64])"),
					stderr: []byte("stderr"),
					err:    nil,
				},
			},
			expectedResults: []*PkgInfo{
				{Name: "google-cloud-sdk", Arch: "x86_64", Version: "246.0.0-0"},
			},
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
		runner = mockCommandRunner

		t.Run(tt.name, func(t *testing.T) {
			setExpectations(mockCommandRunner, tt.expectedCommandsChain)

			pkgs, err := AptUpdates(testCtx, tt.args...)
			if !reflect.DeepEqual(err, tt.expectedError) {
				t.Errorf("AptUpdates: unexpected error, expect %q, got %q", formatError(tt.expectedError), formatError(err))
			}

			if tt.expectedResultsFile != "" {
				utiltest.MatchSnapshot(t, pkgs, tt.expectedResultsFile)
			} else if !reflect.DeepEqual(pkgs, tt.expectedResults) {
				t.Errorf("AptUpdates: unexpected result, expect %v, got %v", tt.expectedResults, pkgs)
			}
		})
	}
}

func TestRemoveAptPackages(t *testing.T) {
	tests := []struct {
		name string
		pkgs []string

		expectedCommandsChain []expectedCommand
		expectedError         error
	}{
		{
			name: "Successful path",
			pkgs: []string{"pkg1", "pkg2"},
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command(aptGet, append(slices.Clone(aptGetRemoveArgs), pkgs...)...),
					envs:   []string{"DEBIAN_FRONTEND=noninteractive"},
					stdout: []byte("stdout"),
					stderr: []byte("stderr"),
					err:    nil,
				},
			},
			expectedError: nil,
		},
		{
			name: "Run dpkg repair on dpkg error",
			pkgs: []string{"pkg1", "pkg2"},
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command(aptGet, append(slices.Clone(aptGetRemoveArgs), pkgs...)...),
					envs:   []string{"DEBIAN_FRONTEND=noninteractive"},
					stdout: []byte("stdout"),
					stderr: dpkgErr,
					err:    errors.New("error"),
				},
				{
					cmd:    exec.CommandContext(testCtx, dpkg, dpkgRepairArgs...),
					envs:   nil,
					stdout: []byte("stdout"),
					stderr: []byte("stderr"),
					err:    nil,
				},
				{
					cmd:    exec.Command(aptGet, append(slices.Clone(aptGetRemoveArgs), pkgs...)...),
					envs:   []string{"DEBIAN_FRONTEND=noninteractive"},
					stdout: []byte("stdout"),
					stderr: []byte("stderr"),
					err:    nil,
				},
			},
			expectedError: nil,
		},
		{
			name: "throw an error if non dpkgErr",
			pkgs: []string{"pkg1", "pkg2"},
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command(aptGet, append(slices.Clone(aptGetRemoveArgs), pkgs...)...),
					envs:   []string{"DEBIAN_FRONTEND=noninteractive"},
					stdout: []byte("stdout"),
					stderr: []byte("stderr"),
					err:    errors.New("unexpected error"),
				},
			},
			expectedError: errors.New("error running /usr/bin/apt-get with args" +
				" [\"remove\" \"-y\" \"pkg1\" \"pkg2\"]:" +
				" unexpected error, stdout: \"stdout\", stderr: \"stderr\""),
		},
		{
			name: "throw an error if any at the end",
			pkgs: []string{"pkg1", "pkg2"},

			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command(aptGet, append(aptGetRemoveArgs, pkgs...)...),
					envs:   []string{"DEBIAN_FRONTEND=noninteractive"},
					stdout: []byte("stdout"),
					stderr: dpkgErr,
					err:    errors.New("unexpected error"),
				},
				{
					cmd:    exec.CommandContext(testCtx, dpkg, dpkgRepairArgs...),
					envs:   nil,
					stdout: []byte("stdout"),
					stderr: []byte("stderr"),
					err:    nil,
				},
				{
					cmd:    exec.Command(aptGet, append(aptGetRemoveArgs, pkgs...)...),
					envs:   []string{"DEBIAN_FRONTEND=noninteractive"},
					stdout: []byte("stdout"),
					stderr: []byte("stderr"),
					err:    errors.New("unexpected error"),
				},
			},
			expectedError: errors.New("error running /usr/bin/apt-get with args" +
				" [\"remove\" \"-y\" \"pkg1\" \"pkg2\"]:" +
				" unexpected error, stdout: \"stdout\", stderr: \"stderr\""),
		},
	}
	for _, tt := range tests {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
		runner = mockCommandRunner

		t.Run(tt.name, func(t *testing.T) {
			setExpectations(mockCommandRunner, tt.expectedCommandsChain)

			err := RemoveAptPackages(testCtx, tt.pkgs)
			if !reflect.DeepEqual(err, tt.expectedError) {
				t.Errorf("RemoveAptPackages: unexpected error, expect %q, got %q", formatError(tt.expectedError), formatError(err))
			}
		})
	}

}

func TestInstalledDebPackages(t *testing.T) {
	tests := []struct {
		name             string
		cmds             []expectedCommand
		wantErr          error
		wantPkgs         []*PkgInfo
		wantPkgsSnapshot string
	}{
		{
			name: "single entry maps to package",
			cmds: []expectedCommand{
				{
					cmd:    exec.Command(dpkgQuery, dpkgQueryArgs...),
					stdout: []byte(`{"package":"git","architecture":"amd64","version":"1:2.25.1-1ubuntu3.12","status":"installed","source_name":"git","source_version":"1:2.25.1-1ubuntu3.12"}`),
					stderr: []byte(""),
				},
			},
			wantPkgs: []*PkgInfo{{Name: "git", Arch: "x86_64", Version: "1:2.25.1-1ubuntu3.12", Source: Source{Name: "git", Version: "1:2.25.1-1ubuntu3.12"}}},
		},
		{
			name: "empty package list maps to nil",
			cmds: []expectedCommand{
				{
					cmd:    exec.Command(dpkgQuery, dpkgQueryArgs...),
					stdout: []byte(""),
					stderr: []byte(""),
				},
			},
			wantPkgs: nil,
		},
		{
			name: "non-zero exit code propagates as error",
			cmds: []expectedCommand{
				{
					cmd:    exec.Command(dpkgQuery, dpkgQueryArgs...),
					stdout: []byte(""),
					stderr: []byte("stderr"),
					err:    errors.New("error"),
				},
			},
			wantErr: fmt.Errorf("error running %s with args %q: %v, stdout: %q, stderr: %q", dpkgQuery, dpkgQueryArgs, errors.New("error"), []byte(""), []byte("stderr")),
		},
		{
			name: "debian-10-1 mapped stdout matches snapshot",
			cmds: []expectedCommand{
				{
					cmd:    exec.Command(dpkgQuery, dpkgQueryArgs...),
					stdout: utiltest.BytesFromFile(t, "./testdata/debian-10-1.dpkg-query-show.stdout"),
					stderr: []byte(""),
				},
			},
			wantPkgsSnapshot: "./testdata/debian-10-1.dpkg-query-show.want",
		},
		{
			name: "debian-11-1 mapped stdout matches snapshot",
			cmds: []expectedCommand{
				{
					cmd:    exec.Command(dpkgQuery, dpkgQueryArgs...),
					stdout: utiltest.BytesFromFile(t, "./testdata/debian-11-1.dpkg-query-show.stdout"),
					stderr: []byte(""),
				},
			},
			wantPkgsSnapshot: "./testdata/debian-11-1.dpkg-query-show.want",
		},
		{
			name: "debian-12-1 mapped stdout matches snapshot",
			cmds: []expectedCommand{
				{
					cmd:    exec.Command(dpkgQuery, dpkgQueryArgs...),
					stdout: utiltest.BytesFromFile(t, "./testdata/debian-12-1.dpkg-query-show.stdout"),
					stderr: []byte(""),
				},
			},
			wantPkgsSnapshot: "./testdata/debian-12-1.dpkg-query-show.want",
		},
		{
			name: "ubuntu-20-1 mapped stdout matches snapshot",
			cmds: []expectedCommand{
				{
					cmd:    exec.Command(dpkgQuery, dpkgQueryArgs...),
					stdout: utiltest.BytesFromFile(t, "./testdata/ubuntu-20-1.dpkg-query-show.stdout"),
					stderr: []byte(""),
				},
			},
			wantPkgsSnapshot: "./testdata/ubuntu-20-1.dpkg-query-show.want",
		},
		{
			name: "ubuntu-22-1 mapped stdout matches snapshot",
			cmds: []expectedCommand{
				{
					cmd:    exec.Command(dpkgQuery, dpkgQueryArgs...),
					stdout: utiltest.BytesFromFile(t, "./testdata/ubuntu-22-1.dpkg-query-show.stdout"),
					stderr: []byte(""),
				},
			},
			wantPkgsSnapshot: "./testdata/ubuntu-22-1.dpkg-query-show.want",
		},
		{
			name: "ubuntu-24-1 mapped stdout matches snapshot",
			cmds: []expectedCommand{
				{
					cmd:    exec.Command(dpkgQuery, dpkgQueryArgs...),
					stdout: utiltest.BytesFromFile(t, "./testdata/ubuntu-24-1.dpkg-query-show.stdout"),
					stderr: []byte(""),
				},
			},
			wantPkgsSnapshot: "./testdata/ubuntu-24-1.dpkg-query-show.want",
		},
	}

	for _, tt := range tests {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
		runner = mockCommandRunner

		t.Run(tt.name, func(t *testing.T) {
			setExpectations(mockCommandRunner, tt.cmds)

			pkgs, err := InstalledDebPackages(testCtx)
			if !reflect.DeepEqual(err, tt.wantErr) {
				t.Errorf("InstalledDebPackages err: want %v, got %v", tt.wantErr, err)
			}

			if tt.wantPkgsSnapshot != "" {
				utiltest.MatchSnapshot(t, pkgs, tt.wantPkgsSnapshot)
			} else if !reflect.DeepEqual(pkgs, tt.wantPkgs) {
				t.Errorf("InstalledDebPackages pkgs: want %v, got %v", tt.wantPkgs, pkgs)
			}
		})
	}
}

func TestParseInstalledDebpackages(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  []*PkgInfo
	}{
		{
			name: "two valid packages in input",
			input: []byte("" +
				`{"package":"python3-gi","architecture":"amd64","version":"3.36.0-1","status":"installed","source_name":"pygobject","source_version":"3.36.0-1"}` +
				"\n" +
				`{"package":"man-db","architecture":"amd64","version":"2.9.1-1","status":"installed","source_name":"man-db","source_version":"2.9.1-1"}`),
			want: []*PkgInfo{
				{Name: "python3-gi", Arch: "x86_64", Version: "3.36.0-1", Source: Source{Name: "pygobject", Version: "3.36.0-1"}},
				{Name: "man-db", Arch: "x86_64", Version: "2.9.1-1", Source: Source{Name: "man-db", Version: "2.9.1-1"}}},
		},
		{
			name:  "No lines formatted as a package info",
			input: []byte("nothing here"),
			want:  nil,
		},
		{
			name:  "Nil as input does not panic",
			input: nil,
			want:  nil,
		},
		{
			name: "Skip wrongly formatted lines",
			input: []byte("something we dont understand\n" +
				`{"package":"python3-gi","architecture":"amd64","version":"3.36.0-1","status":"installed","source_name":"pygobject","source_version":"3.36.0-1"}`),
			want: []*PkgInfo{{Name: "python3-gi", Arch: "x86_64", Version: "3.36.0-1", Source: Source{Name: "pygobject", Version: "3.36.0-1"}}},
		},
		{
			name: "Skip entries that have status other than 'installed'",
			input: []byte("" +
				`{"package":"python3-gi","architecture":"amd64","version":"3.36.0-1","status":"installed","source_name":"pygobject","source_version":"3.36.0-1"}` + "\n" +
				`{"package":"man-db","architecture":"amd64","version":"2.9.1-1","status":"config-files","source_name":"man-db","source_version":"2.9.1-1"}`),
			want: []*PkgInfo{{Name: "python3-gi", Arch: "x86_64", Version: "3.36.0-1", Source: Source{Name: "pygobject", Version: "3.36.0-1"}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseInstalledDebPackages(testCtx, tt.input); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseInstalledDebPackages() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseAptUpdates(t *testing.T) {
	normalCase := `
Inst libldap-common [2.4.45+dfsg-1ubuntu1.2] (2.4.45+dfsg-1ubuntu1.3 Ubuntu:18.04/bionic-updates, Ubuntu:18.04/bionic-security [all])
Inst google-cloud-sdk [245.0.0-0] (246.0.0-0 cloud-sdk-stretch:cloud-sdk-stretch [amd64]) []
Inst firmware-linux-free (3.4 Debian:9.9/stable [all])
Conf firmware-linux-free (3.4 Debian:9.9/stable [all])
`

	tests := []struct {
		name    string
		input   []byte
		showNew bool
		want    []*PkgInfo
	}{
		{
			name:    "Set of packages with new, show new - false",
			input:   []byte(normalCase),
			showNew: false,
			want: []*PkgInfo{
				{Name: "libldap-common", Arch: "all", Version: "2.4.45+dfsg-1ubuntu1.3"},
				{Name: "google-cloud-sdk", Arch: "x86_64", Version: "246.0.0-0"},
			},
		},
		{
			name:    "Set of packages with new, show new - true",
			input:   []byte(normalCase),
			showNew: true,
			want: []*PkgInfo{
				{Name: "libldap-common", Arch: "all", Version: "2.4.45+dfsg-1ubuntu1.3"},
				{Name: "google-cloud-sdk", Arch: "x86_64", Version: "246.0.0-0"},
				{Name: "firmware-linux-free", Arch: "all", Version: "3.4"},
			},
		},
		{
			name:    "No lines formatted as a package info",
			input:   []byte("nothing here"),
			showNew: false,
			want:    nil,
		},
		{
			name:    "Nil as input does not panic",
			input:   nil,
			showNew: false,
			want:    nil,
		},
		{
			name:    "Skip wrongly formatted lines",
			input:   []byte("Inst something [we dont understand\n Inst google-cloud-sdk [245.0.0-0] (246.0.0-0 cloud-sdk-stretch:cloud-sdk-stretch [amd64])"),
			showNew: false,
			want: []*PkgInfo{
				{Name: "google-cloud-sdk", Arch: "x86_64", Version: "246.0.0-0"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseAptUpdates(testCtx, tt.input, tt.showNew); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseAptUpdates() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDebPkgInfo(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner

	testPkg := "test.deb"
	expectedCmd := utilmocks.EqCmd(exec.Command(dpkgDeb, "-I", testPkg))
	out := []byte(`new Debian package, version 2.0.
	size 6731954 bytes: control archive=2138 bytes.
		498 bytes,    12 lines      control
	   3465 bytes,    31 lines      md5sums
	   2793 bytes,    65 lines   *  postinst             #!/bin/sh
		938 bytes,    28 lines   *  postrm               #!/bin/sh
		216 bytes,     7 lines   *  prerm                #!/bin/sh
	Package: google-guest-agent
	Version: 1:1dummy-g1
	Architecture: amd64
	Maintainer: Google Cloud Team <gc-team@google.com>
	Installed-Size: 23279
	Depends: init-system-helpers (>= 1.18~)
	Conflicts: python-google-compute-engine, python3-google-compute-engine
	Section: misc
	Priority: optional
	Description: Google Compute Engine Guest Agent
	 Contains the guest agent and metadata script runner binaries.
	Git: https://github.com/GoogleCloudPlatform/guest-agent/tree/c3d526e650c4e45ae3258c07836fd72f85fd9fc8`)
	mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).Return(out, []byte("stderr"), nil).Times(1)
	ret, err := DebPkgInfo(testCtx, testPkg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	want := &PkgInfo{Name: "google-guest-agent", Arch: "x86_64", Version: "1:1dummy-g1"}
	if !reflect.DeepEqual(ret, want) {
		t.Errorf("DebPkgInfo() = %+v, want %+v", ret, want)
	}

	// Error output.
	mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).Return([]byte("stdout"), []byte("stderr"), errors.New("bad error")).Times(1)
	if _, err := DebPkgInfo(testCtx, testPkg); err == nil {
		t.Errorf("did not get expected error")
	}
	// No package
	mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).Return([]byte(""), []byte("stderr"), nil).Times(1)
	if _, err := DebPkgInfo(testCtx, testPkg); err == nil {
		t.Errorf("did not get expected error")
	}
}

func Test_dpkgRepair(t *testing.T) {

	tests := []struct {
		name        string
		input       []byte
		expected    bool
		expectedCmd *exec.Cmd
	}{
		{
			name:        "NonDpkgError",
			input:       []byte("some random error"),
			expected:    false,
			expectedCmd: nil,
		},
		{
			name:        "DpkgError",
			input:       dpkgErr,
			expected:    true,
			expectedCmd: exec.CommandContext(testCtx, dpkg, dpkgRepairArgs...),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
			runner = mockCommandRunner

			if tt.expectedCmd != nil {
				mockCommandRunner.EXPECT().Run(testCtx, utilmocks.EqCmd(tt.expectedCmd)).Return([]byte("output"), []byte(""), nil).Times(1)

			}

			if result := dpkgRepair(testCtx, tt.input); result != tt.expected {
				t.Errorf("unexpected result of dpkgRepair, expected %t, got %t", tt.expected, result)
			}
		})
	}
}

func TestDpkgInstall(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner

	path := "/tmp/test.dpkg"
	dpkgInstallCmd := exec.CommandContext(testCtx, dpkg, append(dpkgInstallArgs, path)...)

	//Dpkg install fail
	wantErr := errors.New("unexpected error")
	mockCommandRunner.EXPECT().Run(testCtx, utilmocks.EqCmd(dpkgInstallCmd)).Return([]byte("stdout"), []byte("stderr"), wantErr).Times(1)
	if err := DpkgInstall(testCtx, path); err == nil {
		t.Errorf("DpkgInstall: expected error %q, but got <nil>", formatError(wantErr))
	}

	//Dpkg install succeeded
	mockCommandRunner.EXPECT().Run(testCtx, utilmocks.EqCmd(dpkgInstallCmd)).Return([]byte("stdout"), []byte("stderr"), nil).Times(1)
	if err := DpkgInstall(testCtx, path); err != nil {
		t.Errorf("DpkgInstall: got unexpected error %q", err)
	}
}
