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
	"context"
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	utilmocks "github.com/GoogleCloudPlatform/osconfig/util/mocks"
	utiltest "github.com/GoogleCloudPlatform/osconfig/util/utiltest"
	"github.com/golang/mock/gomock"
)

func TestZypperInstalls(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	expectedCmd := utilmocks.EqCmd(exec.Command(zypper, append(zypperInstallArgs, pkgs...)...))

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
	expectedCmd := utilmocks.EqCmd(exec.Command(zypper, append(zypperRemoveArgs, pkgs...)...))

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
		{"NormalCase", []byte(normalCase), []*PkgInfo{{Name: "at", Arch: "x86_64", Version: "3.1.14-8.3.1"}, {Name: "autoyast2-installation", Arch: "all", Version: "3.2.22-2.9.2"}}},
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
	tests := []struct {
		name string
		pkgs []string

		expectedCommandsChain []expectedCommand
		expectedError         error
		expectedResults       []*PkgInfo
		expectedResultsFile   string
	}{
		{
			name: "empty output maps to nil result",
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command(zypper, zypperListUpdatesArgs...),
					stdout: []byte{},
					stderr: []byte("stderr"),
				},
			},
			expectedResults: nil,
		},
		{
			name: "`zypper list-updates` error propagates",
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command(zypper, zypperListUpdatesArgs...),
					stdout: []byte("stdout"),
					stderr: []byte("stderr"),
					err:    errors.New("error"),
				},
			},
			expectedError: errors.New("error running /usr/bin/zypper with args [\"--gpg-auto-import-keys\" \"-q\" \"list-updates\"]: error, stdout: \"stdout\", stderr: \"stderr\""),
		},
		{
			name: "single package maps correctly",
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command(zypper, zypperListUpdatesArgs...),
					stdout: []byte("v | SLES12-SP3-Updates  | at                     | 3.1.14-7.3      | 3.1.14-8.3.1      | x86_64"),
					stderr: []byte("stderr"),
				},
			},
			expectedResults: []*PkgInfo{{Name: "at", Arch: "x86_64", Version: "3.1.14-8.3.1"}},
		},
		{
			name: "sles-12-1 mapped list-updates stdout matches snapshot",
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command(zypper, zypperListUpdatesArgs...),
					stdout: utiltest.BytesFromFile(t, "./testdata/sles-12-1.zypper-list-updates.stdout"),
					stderr: []byte("stderr"),
				},
			},
			expectedResultsFile: "./testdata/sles-12-1.zypper-list-updates.expected",
		},
		{
			name: "sles-15-1 mapped list-updates stdout matches snapshot",
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command(zypper, zypperListUpdatesArgs...),
					stdout: utiltest.BytesFromFile(t, "./testdata/sles-15-1.zypper-list-updates.stdout"),
					stderr: []byte("stderr"),
				},
			},
			expectedResultsFile: "./testdata/sles-15-1.zypper-list-updates.expected",
		},
	}

	for _, tt := range tests {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
		SetCommandRunner(mockCommandRunner)

		t.Run(tt.name, func(t *testing.T) {
			setExpectations(mockCommandRunner, tt.expectedCommandsChain)

			pkgs, err := ZypperUpdates(testCtx)

			if !reflect.DeepEqual(err, tt.expectedError) {
				t.Errorf("unexpected err: expected %q, got %q", tt.expectedError, err)
			}
			if tt.expectedResultsFile != "" {
				utiltest.MatchSnapshot(t, pkgs, tt.expectedResultsFile)
			} else if !reflect.DeepEqual(pkgs, tt.expectedResults) {
				t.Errorf("unexpected pkgs, expected %v, got %v", tt.expectedResults, pkgs)
			}
		})
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

	//Recently new format of response was observed, the difference is additional "Since" field in the table.
	withSinceField := `Repository                                    | Name                                                  | Category    | Severity  | Interactive    | Status     | Since      | Summary
----------------------------------------------+-------------------------------------------------------+-------------+-----------+----------------+------------+------------------------------------------------------------
SLE-Module-Basesystem15-SP1-Updates           | SUSE-SLE-Module-Basesystem-15-SP1-2019-1206           | security    | low       | ---            | applied    | -          | Security update for bzip2
SLE-Module-Basesystem15-SP1-Updates           | SUSE-SLE-Module-Basesystem-15-SP1-2019-1221           | security    | moderate  | ---            | needed     | -          | Security update for libxslt
SLE-Module-Basesystem15-SP1-Updates           | SUSE-SLE-Module-Basesystem-15-SP1-2019-1229           | recommended | moderate  | ---            | not needed | -          | Recommended update for sensors
SLE-Module-Basesystem15-SP1-Updates           | SUSE-SLE-Module-Basesystem-15-SP1-2019-1258           | recommended | moderate  | ---            | needed     | -          | Recommended update for postfix
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
		{
			"WithSinceField",
			[]byte(withSinceField),
			[]*ZypperPatch{{"SUSE-SLE-Module-Basesystem-15-SP1-2019-1206", "security", "low", "Security update for bzip2"}},
			[]*ZypperPatch{{"SUSE-SLE-Module-Basesystem-15-SP1-2019-1221", "security", "moderate", "Security update for libxslt"}, {"SUSE-SLE-Module-Basesystem-15-SP1-2019-1258", "recommended", "moderate", "Recommended update for postfix"}},
		},
		{"NoPackages", []byte("nothing here"), nil, nil},
		{"nil", nil, nil, nil},
	}
	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIns, gotAvail := parseZypperPatches(ctx, tt.data)
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
	tests := []struct {
		name string
		pkgs []string

		expectedCommandsChain []expectedCommand
		expectedError         error
		expectedResults       []*ZypperPatch
		expectedResultsFile   string
	}{
		{
			name: "empty output maps to nil result",
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command(zypper, append(zypperListPatchesArgs, "--all")...),
					stdout: []byte{},
					stderr: []byte("stderr"),
				},
			},
			expectedResults: nil,
		},
		{
			name: "`zypper list-patches` error propagates",
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command(zypper, append(zypperListPatchesArgs, "--all")...),
					stdout: []byte("stdout"),
					stderr: []byte("stderr"),
					err:    errors.New("error"),
				},
			},
			expectedError: errors.New("error running /usr/bin/zypper with args [\"--gpg-auto-import-keys\" \"-q\" \"list-patches\" \"--all\"]: error, stdout: \"stdout\", stderr: \"stderr\""),
		},
		{
			name: "single package maps correctly",
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command(zypper, append(zypperListPatchesArgs, "--all")...),
					stdout: []byte("SLE-Module-Basesystem15-SP1-Updates | SUSE-SLE-Module-Basesystem-15-SP1-2019-1258 | recommended | moderate  | ---         | needed     | Recommended update for postfix"),
					stderr: []byte("stderr"),
				},
			},
			expectedResults: []*ZypperPatch{{"SUSE-SLE-Module-Basesystem-15-SP1-2019-1258", "recommended", "moderate", "Recommended update for postfix"}},
		},
		{
			name: "sles-12-1 mapped list-patches stdout matches snapshot",
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command(zypper, append(zypperListPatchesArgs, "--all")...),
					stdout: utiltest.BytesFromFile(t, "./testdata/sles-12-1.zypper-list-patches.stdout"),
					stderr: []byte("stderr"),
				},
			},
			expectedResultsFile: "./testdata/sles-12-1.zypper-list-patches.expected",
		},
		{
			name: "sles-15-1 mapped list-patches stdout matches snapshot",
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command(zypper, append(zypperListPatchesArgs, "--all")...),
					stdout: utiltest.BytesFromFile(t, "./testdata/sles-15-1.zypper-list-patches.stdout"),
					stderr: []byte("stderr"),
				},
			},
			expectedResultsFile: "./testdata/sles-15-1.zypper-list-patches.expected",
		},
	}

	for _, tt := range tests {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
		SetCommandRunner(mockCommandRunner)

		t.Run(tt.name, func(t *testing.T) {
			setExpectations(mockCommandRunner, tt.expectedCommandsChain)

			pkgs, err := ZypperPatches(testCtx)

			if !reflect.DeepEqual(err, tt.expectedError) {
				t.Errorf("unexpected err: expected %q, got %q", tt.expectedError, err)
			}
			if tt.expectedResultsFile != "" {
				utiltest.MatchSnapshot(t, pkgs, tt.expectedResultsFile)
			} else if !reflect.DeepEqual(pkgs, tt.expectedResults) {
				t.Errorf("unexpected pkgs, expected %v, got %v", tt.expectedResults, pkgs)
			}
		})
	}
}

func TestZypperInstalledPatches(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	expectedCmd := utilmocks.EqCmd(exec.Command(zypper, append(zypperListPatchesArgs, "--all")...))

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
    zypper.src < 1.14.46-13.1
    zypper.noarch < 1.14.46-13.1
    zypper.x86_64 < 1.14.46-13.1
    zypper-log < 1.14.46-13.1
    zypper-needs-restarting < 1.14.46-13.1

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

func TestParsePatchInfo_differentFormatsOfConflictPkgsVersions(t *testing.T) {
	patchInfo := `
Information for patch SUSE-SLE-Module-Basesystem-15-SP5-2023-4176:
------------------------------------------------------------------
Repository  : SLE-Module-Basesystem15-SP5-Updates
Name        : SUSE-SLE-Module-Basesystem-15-SP5-2023-4176
Version     : 1
Arch        : noarch
Vendor      : maint-coord@suse.de
Status      : needed
Category    : security
Severity    : important
Created On  : Tue Oct 24 13:35:58 2023
Interactive : ---
Summary     : Security update for ruby2.5
Description :
    This update for ruby2.5 fixes the following issues:

    - CVE-2023-28755: Fixed a ReDoS vulnerability in URI. (bsc#1209891)
    - CVE-2023-28756: Fixed an expensive regexp in the RFC2822 time parser. (bsc#1209967)
    - CVE-2021-41817: Fixed a Regular Expression Denial of Service Vulnerability of Date Parsing Methods. (bsc#1193035)
    - CVE-2021-33621: Fixed a HTTP response splitting vulnerability in CGI gem. (bsc#1205726)
Provides    : patch:SUSE-SLE-Module-Basesystem-15-SP5-2023-4176 = 1
Conflicts   : [11]
    libruby2_5-2_5.x86_64 < 2.5.9-150000.4.29.1
    libruby2_5-2_5.noarch < 2.5.9-150000.4.29.1
    srcpackage:ruby2.5 < 2.5.9-150000.4.29.1
    ruby2.5.noarch < 2.5.9-150000.4.29.1
    ruby2.5.x86_64 < 2.5.9-150000.4.29.1
    ruby2.5-devel.x86_64 < 2.5.9-150000.4.29.1
    ruby2.5-devel.noarch < 2.5.9-150000.4.29.1
    ruby2.5-devel-extra.x86_64 < 2.5.9-150000.4.29.1
    ruby2.5-devel-extra.noarch < 2.5.9-150000.4.29.1
    ruby2.5-stdlib.x86_64 < 2.5.9-150000.4.29.1
    ruby2.5-stdlib.noarch < 2.5.9-150000.4.29.1
Information for patch SUSE-SLE-Module-Basesystem-15-SP5-2023-4843:
------------------------------------------------------------------
Repository  : SLE-Module-Basesystem15-SP5-Updates
Name        : SUSE-SLE-Module-Basesystem-15-SP5-2023-4843
Version     : 1
Arch        : noarch
Vendor      : maint-coord@suse.de
Status      : needed
Category    : security
Severity    : moderate
Created On  : Thu Dec 14 11:23:04 2023
Interactive : ---
Summary     : Security update for python3-cryptography
Description :
    This update for python3-cryptography fixes the following issues:
    - CVE-2023-49083: Fixed a NULL pointer dereference when loading certificates from a PKCS#7 bundle (bsc#1217592).
Provides    : patch:SUSE-SLE-Module-Basesystem-15-SP5-2023-4843 = 1
Conflicts   : [3]
    srcpackage:python3-cryptography < 3.3.2-150400.23.1
    python3-cryptography.noarch < 3.3.2-150400.23.1
    python3-cryptography.x86_64 < 3.3.2-150400.23.1
Information for patch SUSE-SLE-Module-Basesystem-15-SP5-2023-3973:
------------------------------------------------------------------
Repository  : SLE-Module-Basesystem15-SP5-Updates
Name        : SUSE-SLE-Module-Basesystem-15-SP5-2023-3973
Version     : 1
Arch        : noarch
Vendor      : maint-coord@suse.de
Status      : needed
Category    : recommended
Severity    : moderate
Created On  : Thu Oct  5 08:17:12 2023
Interactive : restart
Summary     : Recommended update for zypper
Description :
    This update for zypper fixes the following issues:

    - Fix name of the bash completion script (bsc#1215007)
    - Update notes about failing signature checks (bsc#1214395)
    - Improve the SIGINT handler to be signal safe (bsc#1214292)
    - Update to version 1.14.64
    - Changed location of bash completion script (bsc#1213854).
Provides    : patch:SUSE-SLE-Module-Basesystem-15-SP5-2023-3973 = 1
Conflicts   : [5]
    srcpackage:zypper < 1.14.64-150400.3.32.1
    zypper.noarch < 1.14.64-150400.3.32.1
    zypper.x86_64 < 1.14.64-150400.3.32.1
    zypper-log < 1.14.64-150400.3.32.1
    zypper-needs-restarting < 1.14.64-150400.3.32.1
`
	ppMap, err := parseZypperPatchInfo([]byte(patchInfo))
	if err != nil {
		t.Errorf("unexpected error: %+v", err)
	}

	if _, ok := ppMap["libruby2_5-2_5"]; !ok {
		t.Errorf("Unexpected result: expected a patch for libruby2_5-2_5")
	}

	if _, ok := ppMap["ruby2.5"]; !ok {
		t.Errorf("Unexpected result: expected a patch for ruby2.5")
	}

	if _, ok := ppMap["python3-cryptography"]; !ok {
		t.Errorf("Unexpected result: expected a patch for python3-cryptography")
	}

	if _, ok := ppMap["zypper-log"]; !ok {
		t.Errorf("Unexpected result: expected a patch for zypper-log")
	}

	if _, ok := ppMap["zypper-needs-restarting"]; !ok {
		t.Errorf("Unexpected result: expected a patch for zypper-needs-restarting")
	}

	patches, ok := ppMap["python3-cryptography"]
	if !ok {
		t.Errorf("Unexpected result: expected a patch for python3-cryptography")
	} else {
		for _, patch := range patches {
			if strings.Compare(patch, "SUSE-SLE-Module-Basesystem-15-SP5-2023-4843") != 0 {
				t.Errorf("Unexptected result: patch name should be SUSE-SLE-Module-Basesystem-15-SP5-2023-4843")
			}
		}
	}
}

func TestZypperPackagesInPatch(t *testing.T) {
	ppMap, err := ZypperPackagesInPatch(testCtx, nil)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(ppMap) > 0 {
		t.Errorf("Unexpected result: expected no mappings, got = [%+v]", ppMap)
	}
}
