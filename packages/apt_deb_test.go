//  Copyright 2019 Google Inc. All Rights Reserved.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package packages

import (
	"errors"
	"os"
	"os/exec"
	"reflect"
	"testing"

	utilmocks "github.com/GoogleCloudPlatform/osconfig/util/mocks"
	"github.com/golang/mock/gomock"
)

func TestInstallAptPackages(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner

	expectedCmd := exec.Command(aptGet, append(aptGetInstallArgs, pkgs...)...)
	expectedCmd.Env = append(os.Environ(),
		"DEBIAN_FRONTEND=noninteractive",
	)

	mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).Return([]byte("stdout"), []byte("stderr"), nil).Times(1)
	if err := InstallAptPackages(testCtx, pkgs); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	first := mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).Return([]byte("stdout"), dpkgErr, errors.New("error")).Times(1)
	repair := mockCommandRunner.EXPECT().Run(testCtx, exec.Command(dpkg, dpkgRepairArgs...)).After(first).Return([]byte("stdout"), []byte("stderr"), nil).Times(1)
	mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).After(repair).Return([]byte("stdout"), []byte("stderr"), errors.New("error")).Times(1)
	if err := InstallAptPackages(testCtx, pkgs); err == nil {
		t.Errorf("did not get expected error")
	}
}

func TestRemoveAptPackages(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	expectedCmd := exec.Command(aptGet, append(aptGetRemoveArgs, pkgs...)...)
	expectedCmd.Env = append(os.Environ(),
		"DEBIAN_FRONTEND=noninteractive",
	)

	mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).Return([]byte("stdout"), []byte("stderr"), nil).Times(1)
	if err := RemoveAptPackages(testCtx, pkgs); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	first := mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).Return([]byte("stdout"), dpkgErr, errors.New("error")).Times(1)
	repair := mockCommandRunner.EXPECT().Run(testCtx, exec.Command(dpkg, dpkgRepairArgs...)).After(first).Return([]byte("stdout"), []byte("stderr"), nil).Times(1)
	mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).After(repair).Return([]byte("stdout"), []byte("stderr"), errors.New("error")).Times(1)
	if err := RemoveAptPackages(testCtx, pkgs); err == nil {
		t.Errorf("did not get expected error")
	}
}

func TestInstalledDebPackages(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	expectedCmd := exec.Command(dpkgQuery, dpkgQueryArgs...)
	data := []byte("foo amd64 1.2.3-4 installed")

	mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).Return(data, []byte("stderr"), nil).Times(1)
	ret, err := InstalledDebPackages(testCtx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	want := []*PkgInfo{{"foo", "x86_64", "1.2.3-4"}}
	if !reflect.DeepEqual(ret, want) {
		t.Errorf("InstalledDebPackages() = %v, want %v", ret, want)
	}

	mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).Return(data, []byte("stderr"), errors.New("error")).Times(1)
	if _, err := InstalledDebPackages(testCtx); err == nil {
		t.Errorf("did not get expected error")
	}
}

func TestParseInstalledDebpackages(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want []*PkgInfo
	}{
		{"NormalCase", []byte("foo amd64 1.2.3-4 installed\nbar noarch 1.2.3-4 installed\nbaz noarch 1.2.3-4 config-files"), []*PkgInfo{{"foo", "x86_64", "1.2.3-4"}, {"bar", "all", "1.2.3-4"}}},
		{"NoPackages", []byte("nothing here"), nil},
		{"nil", nil, nil},
		{"UnrecognizedPackage", []byte("something we dont understand\n bar noarch 1.2.3-4 installed"), []*PkgInfo{{"bar", "all", "1.2.3-4"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseInstalledDebpackages(tt.data); !reflect.DeepEqual(got, tt.want) {
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
		data    []byte
		showNew bool
		want    []*PkgInfo
	}{
		{"NormalCase", []byte(normalCase), false, []*PkgInfo{{"libldap-common", "all", "2.4.45+dfsg-1ubuntu1.3"}, {"google-cloud-sdk", "x86_64", "246.0.0-0"}}},
		{"NormalCaseShowNew", []byte(normalCase), true, []*PkgInfo{{"libldap-common", "all", "2.4.45+dfsg-1ubuntu1.3"}, {"google-cloud-sdk", "x86_64", "246.0.0-0"}, {"firmware-linux-free", "all", "3.4"}}},
		{"NoPackages", []byte("nothing here"), false, nil},
		{"nil", nil, false, nil},
		{"UnrecognizedPackage", []byte("Inst something [we dont understand\n Inst google-cloud-sdk [245.0.0-0] (246.0.0-0 cloud-sdk-stretch:cloud-sdk-stretch [amd64])"), false, []*PkgInfo{{"google-cloud-sdk", "x86_64", "246.0.0-0"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseAptUpdates(testCtx, tt.data, tt.showNew); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseAptUpdates() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAptUpdates(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	updateCmd := exec.Command(aptGet, aptGetUpdateArgs...)
	expectedCmd := exec.Command(aptGet, append(aptGetUpgradableArgs, aptGetUpgradeCmd)...)
	data := []byte("Inst google-cloud-sdk [245.0.0-0] (246.0.0-0 cloud-sdk-stretch:cloud-sdk-stretch [amd64])")

	first := mockCommandRunner.EXPECT().Run(testCtx, updateCmd).Return(data, []byte("stderr"), nil).Times(1)
	mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).After(first).Return(data, []byte("stderr"), nil).Times(1)
	ret, err := AptUpdates(testCtx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	want := []*PkgInfo{{"google-cloud-sdk", "x86_64", "246.0.0-0"}}
	if !reflect.DeepEqual(ret, want) {
		t.Errorf("AptUpdates() = %v, want %v", ret, want)
	}

	first = mockCommandRunner.EXPECT().Run(testCtx, updateCmd).Return(data, []byte("stderr"), nil).Times(1)
	mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).After(first).Return(data, []byte("stderr"), errors.New("error")).Times(1)
	if _, err := AptUpdates(testCtx); err == nil {
		t.Errorf("did not get expected error")
	}
}

func TestDebPkgInfo(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	testPkg := "test.deb"
	expectedCmd := exec.Command(dpkgDeb, "-I", testPkg)
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

	want := &PkgInfo{"google-guest-agent", "x86_64", "1:1dummy-g1"}
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
