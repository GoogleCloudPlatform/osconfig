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
	"os/exec"
	"reflect"
	"strings"
	"testing"

	utilmocks "github.com/GoogleCloudPlatform/osconfig/util/mocks"
	"github.com/golang/mock/gomock"
)

func TestZypperInstalls(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	expectedCmd := exec.Command(zypper, append(zypperInstallArgs, pkgs...)...)

	mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).Return([]byte("stdout"), []byte("stderr"), nil).Times(1)
	if err := InstallZypperPackages(testCtx, pkgs); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).Return([]byte("stdout"), []byte("stderr"), errors.New("error")).Times(1)
	if err := InstallZypperPackages(testCtx, pkgs); err == nil {
		t.Errorf("did not get expected error")
	}
}

func TestRemoveZypper(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	expectedCmd := exec.Command(zypper, append(zypperRemoveArgs, pkgs...)...)

	mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).Return([]byte("stdout"), []byte("stderr"), nil).Times(1)
	if err := RemoveZypperPackages(testCtx, pkgs); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).Return([]byte("stdout"), []byte("stderr"), errors.New("error")).Times(1)
	if err := RemoveZypperPackages(testCtx, pkgs); err == nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestParseZypperUpdates(t *testing.T) {
	normalCase := `S | Repository          | Name                   | Current Version | Available Version | Arch
--+---------------------+------------------------+-----------------+-------------------+-------
v | SLES12-SP3-Updates  | at                     | 3.1.14-7.3      | 3.1.14-8.3.1      | x86_64
v | SLES12-SP3-Updates  | autoyast2-installation | 3.2.17-1.3      | 3.2.22-2.9.2      | noarch
this is junk data`

	tests := []struct {
		name string
		data []byte
		want []*PkgInfo
	}{
		{"NormalCase", []byte(normalCase), []*PkgInfo{{"at", "x86_64", "3.1.14-8.3.1"}, {"autoyast2-installation", "all", "3.2.22-2.9.2"}}},
		{"NoPackages", []byte("nothing here"), nil},
		{"nil", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseZypperUpdates(tt.data); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseZypperUpdates() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestZypperUpdates(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	expectedCmd := exec.Command(zypper, zypperListUpdatesArgs...)

	data := []byte("v | SLES12-SP3-Updates  | at                     | 3.1.14-7.3      | 3.1.14-8.3.1      | x86_64")
	mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).Return(data, []byte("stderr"), nil).Times(1)
	ret, err := ZypperUpdates(testCtx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	want := []*PkgInfo{{"at", "x86_64", "3.1.14-8.3.1"}}
	if !reflect.DeepEqual(ret, want) {
		t.Errorf("ZypperUpdates() = %v, want %v", ret, want)
	}

	mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).Return([]byte("stdout"), []byte("stderr"), errors.New("error")).Times(1)
	if _, err := ZypperUpdates(testCtx); err == nil {
		t.Errorf("did not get expected error")
	}
}

func TestParseZypperPatches(t *testing.T) {
	normalCase := `Repository                          | Name                                        | Category    | Severity  | Interactive | Status     | Summary
------------------------------------+---------------------------------------------+-------------+-----------+-------------+------------+------------------------------------------------------------
SLE-Module-Basesystem15-SP1-Updates | SUSE-SLE-Module-Basesystem-15-SP1-2019-1206 | security    | low       | ---         | applied    | Security update for bzip2
SLE-Module-Basesystem15-SP1-Updates | SUSE-SLE-Module-Basesystem-15-SP1-2019-1221 | security    | moderate  | ---         | needed     | Security update for libxslt
SLE-Module-Basesystem15-SP1-Updates | SUSE-SLE-Module-Basesystem-15-SP1-2019-1229 | recommended | moderate  | ---         | not needed | Recommended update for sensors
SLE-Module-Basesystem15-SP1-Updates | SUSE-SLE-Module-Basesystem-15-SP1-2019-1258 | recommended | moderate  | ---         | needed     | Recommended update for postfix
some junk data`

	tests := []struct {
		name      string
		data      []byte
		wantIns   []*ZypperPatch
		wantAvail []*ZypperPatch
	}{
		{
			"NormalCase",
			[]byte(normalCase),
			[]*ZypperPatch{{"SUSE-SLE-Module-Basesystem-15-SP1-2019-1206", "security", "low", "Security update for bzip2"}},
			[]*ZypperPatch{{"SUSE-SLE-Module-Basesystem-15-SP1-2019-1221", "security", "moderate", "Security update for libxslt"}, {"SUSE-SLE-Module-Basesystem-15-SP1-2019-1258", "recommended", "moderate", "Recommended update for postfix"}},
		},
		{"NoPackages", []byte("nothing here"), nil, nil},
		{"nil", nil, nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIns, gotAvail := parseZypperPatches(tt.data)
			if !reflect.DeepEqual(gotIns, tt.wantIns) {
				t.Errorf("parseZypperPatches() = %v, want %v", gotIns, tt.wantIns)
			}
			if !reflect.DeepEqual(gotAvail, tt.wantAvail) {
				t.Errorf("parseZypperPatches() = %v, want %v", gotAvail, tt.wantAvail)
			}
		})
	}
}

func TestZypperPatches(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	expectedCmd := exec.Command(zypper, append(zypperListPatchesArgs, "--all")...)

	data := []byte("SLE-Module-Basesystem15-SP1-Updates | SUSE-SLE-Module-Basesystem-15-SP1-2019-1258 | recommended | moderate  | ---         | needed     | Recommended update for postfix")
	mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).Return(data, []byte("stderr"), nil).Times(1)
	ret, err := ZypperPatches(testCtx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	want := []*ZypperPatch{{"SUSE-SLE-Module-Basesystem-15-SP1-2019-1258", "recommended", "moderate", "Recommended update for postfix"}}
	if !reflect.DeepEqual(ret, want) {
		t.Errorf("ZypperPatches() = %v, want %v", ret, want)
	}

	mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).Return([]byte("stdout"), []byte("stderr"), errors.New("error")).Times(1)
	if _, err := ZypperPatches(testCtx); err == nil {
		t.Errorf("did not get expected error")
	}
}

func TestZypperInstalledPatches(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	expectedCmd := exec.Command(zypper, append(zypperListPatchesArgs, "--all")...)

	data := []byte("SLE-Module-Basesystem15-SP1-Updates | SUSE-SLE-Module-Basesystem-15-SP1-2019-1258 | recommended | moderate  | ---         | applied     | Recommended update for postfix")
	mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).Return(data, []byte("stderr"), nil).Times(1)
	ret, err := ZypperInstalledPatches(testCtx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	want := []*ZypperPatch{{"SUSE-SLE-Module-Basesystem-15-SP1-2019-1258", "recommended", "moderate", "Recommended update for postfix"}}
	if !reflect.DeepEqual(ret, want) {
		t.Errorf("ZypperInstalledPatches() = %v, want %v", ret, want)
	}

	mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).Return([]byte("stdout"), []byte("stderr"), errors.New("error")).Times(1)
	if _, err := ZypperInstalledPatches(testCtx); err == nil {
		t.Errorf("did not get expected error")
	}
}

func TestParsePatchInfo(t *testing.T) {
	patchInfo := `
Loading repository data...
Reading installed packages...
Information for patch SUSE-SLE-SERVER-12-SP4-2019-2974:
-------------------------------------------------------
Repository  : SLES12-SP4-Updates
Name        : SUSE-SLE-SERVER-12-SP4-2019-2974
Version     : 1
Arch        : noarch
Vendor      : maint-coord@suse.de
Status      : needed
Category    : recommended
Severity    : important
Created On  : Thu Nov 14 13:17:48 2019
Interactive : ---
Summary     : Recommended update for irqbalance
Description :
    This update for irqbalance fixes the following issues:
    - Irqbalanced spreads the IRQs between the available virtual machines. (bsc#1119465, bsc#1154905)
Provides    : patch:SUSE-SLE-SERVER-12-SP4-2019-2974 = 1
Conflicts   : [4]
    irqbalance.src < 1.1.0-9.3.1
    irqbalance.x86_64 < 1.1.0-9.3.1
    common-package.src < 1.1.0-9.3.1
    common-package.x86_64 < 1.1.0-9.3.1
Information for patch SUSE-SLE-Module-Public-Cloud-12-2019-2026:
----------------------------------------------------------------
Repository  : SLE-Module-Public-Cloud12-Updates
Name        : SUSE-SLE-Module-Public-Cloud-12-2019-2026
Version     : 1
Arch        : noarch
Vendor      : maint-coord@suse.de
Status      : needed
Category    : recommended
Severity    : moderate
Created On  : Tue Jul 30 17:20:02 2019
Interactive : ---
Summary     : Recommended update for Azure Python SDK
Description :
    This update brings the following python modules for the Azure Python SDK:
    - python-Flask
    - python-Werkzeug
    - python-click
    - python-decorator
    - python-httpbin
    - python-idna
    - python-itsdangerous
    - python-py
    - python-pytest-httpbin
    - python-pytest-mock
    - python-requests
Provides    : patch:SUSE-SLE-Module-Public-Cloud-12-2019-2026 = 1
Conflicts   : [32]
    python-Flask.noarch < 0.12.1-7.4.2
    python-Flask.src < 0.12.1-7.4.2
    python-Werkzeug.noarch < 0.12.2-10.4.2
    python-Werkzeug.src < 0.12.2-10.4.2
    python-click.noarch < 6.7-2.4.2
    python-click.src < 6.7-2.4.2
    python-decorator.noarch < 4.1.2-4.4.2
    python-decorator.src < 4.1.2-4.4.2
    python-httpbin.noarch < 0.5.0-2.4.2
    python-httpbin.src < 0.5.0-2.4.2
    python-idna.noarch < 2.5-3.10.2
    python-idna.src < 2.5-3.10.2
    python-itsdangerous.noarch < 0.24-7.4.2
    python-itsdangerous.src < 0.24-7.4.2
    python-py.noarch < 1.5.2-8.8.2
    python-py.src < 1.5.2-8.8.2
    python-requests.noarch < 2.18.2-8.4.2
    python-requests.src < 2.18.2-8.4.2
    python-six.noarch < 1.11.0-9.21.2
    python-six.src < 1.11.0-9.21.2
    python3-Flask.noarch < 0.12.1-7.4.2
    python3-Werkzeug.noarch < 0.12.2-10.4.2
    python3-click.noarch < 6.7-2.4.2
    python3-decorator.noarch < 4.1.2-4.4.2
    python3-httpbin.noarch < 0.5.0-2.4.2
    python3-idna.noarch < 2.5-3.10.2
    python3-itsdangerous.noarch < 0.24-7.4.2
    python3-py.noarch < 1.5.2-8.8.2
    python3-requests.noarch < 2.18.2-8.4.2
    python3-six.noarch < 1.11.0-9.21.2
    common-package.src < 1.1.0-9.3.1
    common-package.x86_64 < 1.1.0-9.3.1

`
	ppMap, err := parseZypperPatchInfo([]byte(patchInfo))
	if err != nil {
		t.Errorf("unexpected error: %+v", err)
	}

	if _, ok := ppMap["python3-requests"]; !ok {
		t.Errorf("Unexpected result: expected a patch for python3-requests")
	}

	if _, ok := ppMap["random-package"]; ok {
		t.Errorf("Unexpected result: did not expect patch for random-package")
	}

	if _, ok := ppMap["random-package"]; ok {
		t.Errorf("Unexpected result: did not expect patch for random-package")
	}

	if patches, ok := ppMap["common-package"]; !ok {
		t.Errorf("Unexpected result: did not expect patch for common-package")
		for _, patch := range patches {
			if (strings.Compare(patch, "SUSE-SLE-Module-Public-Cloud-12-2019-2026") != 0) || (strings.Compare(patch, "SUSE-SLE-SERVER-12-SP4-2019-2974") != 0) {
				t.Errorf("Unexptected result: patch name should be one of SUSE-SLE-SERVER-12-SP4-2019-2974 or SUSE-SLE-Module-Public-Cloud-12-2019-2026")
			}
		}
	}

}
