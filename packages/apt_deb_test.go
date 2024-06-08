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
	"os"
	"os/exec"
	"reflect"
	"slices"
	"testing"

	utilmocks "github.com/GoogleCloudPlatform/osconfig/util/mocks"
	"github.com/golang/mock/gomock"
)

type expectedCommand struct {
	cmd    *exec.Cmd
	envs   []string
	stdout []byte
	stderr []byte
	err    error
}

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
		expectedResult        []*PkgInfo
		expectedError         error
	}{
		{
			name:                  "UnexpectedUpgradeType",
			args:                  []AptGetUpgradeOption{AptGetUpgradeType(10)},
			expectedCommandsChain: nil,
			expectedResult:        nil,
			expectedError:         fmt.Errorf("unknown upgrade type: %q", 10),
		},
		{
			name: "apt-get update",
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
			expectedResult: nil,
			expectedError:  errors.New("unexpected error"),
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
			expectedResult: nil,
			expectedError:  errors.New("unexpected error"),
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
			expectedResult: []*PkgInfo{{Name: "google-cloud-sdk", Arch: "x86_64", Version: "246.0.0-0"}},
			expectedError:  nil,
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
			expectedResult: []*PkgInfo{{Name: "google-cloud-sdk", Arch: "x86_64", Version: "246.0.0-0"}},
			expectedError:  nil,
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
			expectedResult: []*PkgInfo{{Name: "google-cloud-sdk", Arch: "x86_64", Version: "246.0.0-0"}},
			expectedError:  nil,
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
			expectedResult: []*PkgInfo{
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
			expectedResult: []*PkgInfo{
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
			expectedResult: []*PkgInfo{
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

			if !reflect.DeepEqual(pkgs, tt.expectedResult) {
				t.Errorf("AptUpdates: unexpected result, expect %v, got %v", pkgs, tt.expectedResult)
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
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner

	//Successfully returns result
	dpkgQueryCmd := utilmocks.EqCmd(exec.Command(dpkgQuery, dpkgQueryArgs...))
	stdout, stderr := []byte("foo amd64 1.2.3-4 installed"), []byte("stderr")
	mockCommandRunner.EXPECT().Run(testCtx, dpkgQueryCmd).Return(stdout, stderr, nil).Times(1)

	result, err := InstalledDebPackages(testCtx)
	if err != nil {
		t.Errorf("InstalledDebPackages(): got unexpected error: %v", err)
	}

	want := []*PkgInfo{{Name: "foo", Arch: "x86_64", Version: "1.2.3-4"}}
	if !reflect.DeepEqual(result, want) {
		t.Errorf("InstalledDebPackages() = %v, want %v", result, want)
	}

	//Returns error if any
	mockCommandRunner.EXPECT().Run(testCtx, dpkgQueryCmd).Return(stdout, stderr, errors.New("error")).Times(1)
	if _, err := InstalledDebPackages(testCtx); err == nil {
		t.Errorf("did not get expected error")
	}
}

func TestParseInstalledDebpackages(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  []*PkgInfo
	}{
		{
			name:  "Two valid packages in input",
			input: []byte("foo amd64 1.2.3-4 installed\nbar noarch 1.2.3-4 installed\nbaz noarch 1.2.3-4 config-files"),
			want:  []*PkgInfo{{Name: "foo", Arch: "x86_64", Version: "1.2.3-4"}, {Name: "bar", Arch: "all", Version: "1.2.3-4"}},
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
			name:  "Skip wrongly formatted lines",
			input: []byte("something we dont understand\n bar noarch 1.2.3-4 installed"),
			want:  []*PkgInfo{{Name: "bar", Arch: "all", Version: "1.2.3-4"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseInstalledDebpackages(tt.input); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseInstalledDebpackages() = %v, want %v", got, tt.want)
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

func setExpectations(mockCommandRunner *utilmocks.MockCommandRunner, expectedCommandsChain []expectedCommand) {
	if len(expectedCommandsChain) == 0 {
		return
	}

	var prev *gomock.Call
	for _, expectedCmd := range expectedCommandsChain {
		cmd := expectedCmd.cmd
		if len(expectedCmd.envs) > 0 {
			cmd.Env = append(os.Environ(), expectedCmd.envs...)
		}

		if prev == nil {
			prev = mockCommandRunner.EXPECT().
				Run(testCtx, utilmocks.EqCmd(cmd)).
				Return(expectedCmd.stdout, expectedCmd.stderr, expectedCmd.err).Times(1)
		} else {
			prev = mockCommandRunner.EXPECT().
				Run(testCtx, utilmocks.EqCmd(cmd)).
				After(prev).
				Return(expectedCmd.stdout, expectedCmd.stderr, expectedCmd.err).Times(1)
		}
	}
}

func formatError(err error) string {
	if err == nil {
		return "<nil>"
	}

	return err.Error()
}
