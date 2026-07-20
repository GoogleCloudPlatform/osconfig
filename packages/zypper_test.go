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
	"testing"

	utilmocks "github.com/GoogleCloudPlatform/osconfig/util/mocks"
	utiltest "github.com/GoogleCloudPlatform/osconfig/util/utiltest"
	"github.com/golang/mock/gomock"
)

func TestZypperInstalls(t *testing.T) {
	ctx := t.Context()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	expectedCmd := utilmocks.EqCmd(exec.Command(zypper, append(zypperInstallArgs, pkgs...)...))

	mockCommandRunner.EXPECT().Run(ctx, expectedCmd).Return([]byte("stdout"), []byte("stderr"), nil).Times(1)
	if err := InstallZypperPackages(ctx, pkgs); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	mockCommandRunner.EXPECT().Run(ctx, expectedCmd).Return([]byte("stdout"), []byte("stderr"), errors.New("error")).Times(1)
	if err := InstallZypperPackages(ctx, pkgs); err == nil {
		t.Errorf("did not get expected error")
	}
}

func TestRemoveZypper(t *testing.T) {
	ctx := t.Context()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	expectedCmd := utilmocks.EqCmd(exec.Command(zypper, append(zypperRemoveArgs, pkgs...)...))

	mockCommandRunner.EXPECT().Run(ctx, expectedCmd).Return([]byte("stdout"), []byte("stderr"), nil).Times(1)
	if err := RemoveZypperPackages(ctx, pkgs); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	mockCommandRunner.EXPECT().Run(ctx, expectedCmd).Return([]byte("stdout"), []byte("stderr"), errors.New("error")).Times(1)
	if err := RemoveZypperPackages(ctx, pkgs); err == nil {
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
		{"NormalCase", []byte(normalCase), []*PkgInfo{
			{Name: "at", Arch: "x86_64", Version: "3.1.14-8.3.1", Type: "rpm"},
			{Name: "autoyast2-installation", Arch: "all", Version: "3.2.22-2.9.2", Type: "rpm"}}},
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
			expectedResults: []*PkgInfo{{Name: "at", Arch: "x86_64", Version: "3.1.14-8.3.1", Type: "rpm"}},
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
			ctx := t.Context()
			setExpectations(mockCommandRunner, tt.expectedCommandsChain)

			pkgs, err := ZypperUpdates(ctx)

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
			[]*ZypperPatch{{"SUSE-SLE-Module-Basesystem-15-SP1-2019-1206", "security", "low", "Security update for bzip2" /*PURL: */, ""}},
			[]*ZypperPatch{{"SUSE-SLE-Module-Basesystem-15-SP1-2019-1221", "security", "moderate", "Security update for libxslt" /*PURL: */, ""}, {"SUSE-SLE-Module-Basesystem-15-SP1-2019-1258", "recommended", "moderate", "Recommended update for postfix" /*PURL: */, ""}},
		},
		{
			"WithSinceField",
			[]byte(withSinceField),
			[]*ZypperPatch{{"SUSE-SLE-Module-Basesystem-15-SP1-2019-1206", "security", "low", "Security update for bzip2" /*PURL: */, ""}},
			[]*ZypperPatch{{"SUSE-SLE-Module-Basesystem-15-SP1-2019-1221", "security", "moderate", "Security update for libxslt" /*PURL: */, ""}, {"SUSE-SLE-Module-Basesystem-15-SP1-2019-1258", "recommended", "moderate", "Recommended update for postfix", ""}},
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
			expectedResults: []*ZypperPatch{{"SUSE-SLE-Module-Basesystem-15-SP1-2019-1258", "recommended", "moderate", "Recommended update for postfix", ""}},
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
			ctx := t.Context()
			setExpectations(mockCommandRunner, tt.expectedCommandsChain)

			pkgs, err := ZypperPatches(ctx)

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
	ctx := t.Context()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	expectedCmd := utilmocks.EqCmd(exec.Command(zypper, append(zypperListPatchesArgs, "--all")...))

	data := []byte("SLE-Module-Basesystem15-SP1-Updates | SUSE-SLE-Module-Basesystem-15-SP1-2019-1258 | recommended | moderate  | ---         | applied     | Recommended update for postfix")
	mockCommandRunner.EXPECT().Run(ctx, expectedCmd).Return(data, []byte("stderr"), nil).Times(1)
	ret, err := ZypperInstalledPatches(ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	want := []*ZypperPatch{{"SUSE-SLE-Module-Basesystem-15-SP1-2019-1258", "recommended", "moderate", "Recommended update for postfix", ""}}
	if !reflect.DeepEqual(ret, want) {
		t.Errorf("ZypperInstalledPatches() = %v, want %v", ret, want)
	}

	mockCommandRunner.EXPECT().Run(ctx, expectedCmd).Return([]byte("stdout"), []byte("stderr"), errors.New("error")).Times(1)
	if _, err := ZypperInstalledPatches(ctx); err == nil {
		t.Errorf("did not get expected error")
	}
}

func TestParseZypperPatchInfo(t *testing.T) {
	patchInfo1 := `
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
    This update brings the following python modules:
Provides    : patch:SUSE-SLE-Module-Public-Cloud-12-2019-2026 = 1
Conflicts   : [3]
    python3-requests.noarch < 2.18.2-8.4.2
    common-package.src < 1.1.0-9.3.1
    common-package.x86_64 < 1.1.0-9.3.1
`

	patchInfo2 := `
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
Provides    : patch:SUSE-SLE-Module-Basesystem-15-SP5-2023-4176 = 1
Conflicts   : [3]
    libruby2_5-2_5.x86_64 < 2.5.9-150000.4.29.1
    srcpackage:ruby2.5 < 2.5.9-150000.4.29.1
    ruby2.5.noarch < 2.5.9-150000.4.29.1
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
Provides    : patch:SUSE-SLE-Module-Basesystem-15-SP5-2023-4843 = 1
Conflicts   : [4]
    srcpackage:python3-cryptography < 3.3.2-150400.23.1
    python3-cryptography.noarch < 3.3.2-150400.23.1
    python3-cryptography.x86_64 < 3.3.2-150400.23.1
    zypper-log < 1.14.64-150400.3.32.1
`

	tests := []struct {
		name    string
		input   string
		wantMap map[string][]string
		wantErr error
	}{
		{
			name:  "multiple patch blobs, want parsed mapping",
			input: patchInfo1,
			wantMap: map[string][]string{
				"irqbalance":       {"SUSE-SLE-SERVER-12-SP4-2019-2974", "SUSE-SLE-SERVER-12-SP4-2019-2974"},
				"common-package":   {"SUSE-SLE-SERVER-12-SP4-2019-2974", "SUSE-SLE-SERVER-12-SP4-2019-2974", "SUSE-SLE-Module-Public-Cloud-12-2019-2026", "SUSE-SLE-Module-Public-Cloud-12-2019-2026"},
				"python3-requests": {"SUSE-SLE-Module-Public-Cloud-12-2019-2026"},
			},
			wantErr: nil,
		},
		{
			name:  "various package name and version formats, want parsed mapping",
			input: patchInfo2,
			wantMap: map[string][]string{
				"libruby2_5-2_5":       {"SUSE-SLE-Module-Basesystem-15-SP5-2023-4176"},
				"ruby2.5":              {"SUSE-SLE-Module-Basesystem-15-SP5-2023-4176", "SUSE-SLE-Module-Basesystem-15-SP5-2023-4176"},
				"python3-cryptography": {"SUSE-SLE-Module-Basesystem-15-SP5-2023-4843", "SUSE-SLE-Module-Basesystem-15-SP5-2023-4843", "SUSE-SLE-Module-Basesystem-15-SP5-2023-4843"},
				"zypper-log":           {"SUSE-SLE-Module-Basesystem-15-SP5-2023-4843"},
			},
			wantErr: nil,
		},
		{
			name:    "empty input, want invalid patch information error",
			input:   "",
			wantMap: nil,
			wantErr: errors.New("invalid patch information, did not find patch blobs"),
		},
		{
			name: "invalid name line format, want invalid name output error",
			input: `
Information for patch SUSE-SLE-SERVER-12-SP4-2019-2974:
Name        : foo : bar
Conflicts   : [1]
    pkg1 < 1.0
`,
			wantMap: nil,
			wantErr: errors.New("invalid name output"),
		},
		{
			name: "no conflicts line, want nil result",
			input: `
Information for patch SUSE-SLE-SERVER-12-SP4-2019-2974:
Name        : patch1
`,
			wantMap: nil,
			wantErr: nil,
		},
		{
			name: "invalid conflicts format (empty brackets), want invalid patch info error",
			input: `
Information for patch SUSE-SLE-SERVER-12-SP4-2019-2974:
Name        : patch1
Conflicts   : []
    pkg1 < 1.0
`,
			wantMap: nil,
			wantErr: errors.New("invalid patch info"),
		},
		{
			name: "invalid package info (no '<'), want invalid package info error",
			input: `
Information for patch SUSE-SLE-SERVER-12-SP4-2019-2974:
Name        : patch1
Conflicts   : [1]
    pkg1 1.0
`,
			wantMap: nil,
			wantErr: errors.New("invalid package info, can't parse line:     pkg1 1.0"),
		},
		{
			name: "invalid srcpackage format, want invalid package info error",
			input: `
Information for patch SUSE-SLE-SERVER-12-SP4-2019-2974:
Name        : patch1
Conflicts   : [1]
    srcpackage: < 1.0
`,
			wantMap: nil,
			wantErr: errors.New("invalid package info, can't parse line:     srcpackage: < 1.0"),
		},
		{
			name: "overflow conflicts count, want invalid conflict info error",
			input: `
Information for patch SUSE-SLE-SERVER-12-SP4-2019-2974:
Name        : patch1
Conflicts   : [999999999999999999999999999999]
    pkg1 < 1.0
`,
			wantMap: nil,
			wantErr: errors.New("invalid patch info: invalid conflict info"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMap, gotErr := parseZypperPatchInfo([]byte(tt.input))
			utiltest.AssertErrorMatch(t, gotErr, tt.wantErr)
			utiltest.AssertEquals(t, gotMap, tt.wantMap)
		})
	}
}

func TestZypperPackagesInPatch(t *testing.T) {
	ctx := t.Context()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner

	wantZypperPackagesInPatchErr := errors.New(`error running /usr/bin/zypper with args ["info" "-t" "patch" "patch1"]: generic error, stdout: "", stderr: "stderr"`)

	tests := []struct {
		name             string
		patches          []*ZypperPatch
		expectedCommands []utiltest.ExpectedCommand
		wantErr          error
		wantMappings     map[string][]string
	}{
		{
			name:         "nil patches, want empty map",
			patches:      nil,
			wantMappings: map[string][]string{},
		},
		{
			name:    "successful query, want mappings",
			patches: []*ZypperPatch{{Name: "patch1"}},
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command(zypper, "info", "-t", "patch", "patch1"),
					Stdout: []byte("\nInformation for patch patch1:\n-----------------------------\nName        : patch1\nConflicts   : [1]\n    pkg1 < 1.0\n"),
					Stderr: []byte("stderr"),
				},
			},
			wantMappings: map[string][]string{"pkg1": {"patch1"}},
		},
		{
			name:    "generic error, want wrapped error",
			patches: []*ZypperPatch{{Name: "patch1"}},
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command(zypper, "info", "-t", "patch", "patch1"),
					Stderr: []byte("stderr"),
					Err:    errors.New("generic error"),
				},
			},
			wantErr: wantZypperPackagesInPatchErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			utiltest.SetExpectedCommands(ctx, mockCommandRunner, tt.expectedCommands)

			gotMappings, gotErr := ZypperPackagesInPatch(ctx, tt.patches)
			utiltest.AssertErrorMatchAndSkip(t, gotErr, tt.wantErr)
			utiltest.AssertEquals(t, gotMappings, tt.wantMappings)
		})
	}
}

func TestZypperInstall(t *testing.T) {
	ctx := t.Context()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner

	exitErr := exec.Command("false").Run()
	exitErr102 := exec.Command("bash", "-c", "exit 102").Run()
	wantZypperInstallErr := errors.New(`error running /usr/bin/zypper with args ["--gpg-auto-import-keys" "--non-interactive" "install" "--auto-agree-with-licenses" "patch:patch1" "package:pkg1"]: generic error, stdout: "stdout", stderr: "stderr"`)
	patches := []*ZypperPatch{{Name: "patch1"}}
	pkgs := []*PkgInfo{{Name: "pkg1"}}
	expectedCmd := exec.Command(zypper, append(zypperInstallArgs, "patch:patch1", "package:pkg1")...)

	tests := []struct {
		name             string
		expectedCommands []utiltest.ExpectedCommand
		wantErr          error
	}{
		{
			name: "successful run, want nil error",
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    expectedCmd,
					Stderr: []byte("stderr"),
				},
			},
			wantErr: nil,
		},
		{
			name: "exit code 102 (reboot needed), want nil error",
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    expectedCmd,
					Stderr: []byte("stderr"),
					Err:    exitErr102,
				},
			},
			wantErr: nil,
		},
		{
			name: "exit code 1, want exit error propagated",
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    expectedCmd,
					Stderr: []byte("stderr"),
					Err:    exitErr,
				},
			},
			wantErr: exitErr,
		},
		{
			name: "generic error, want wrapped error",
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    expectedCmd,
					Stdout: []byte("stdout"),
					Stderr: []byte("stderr"),
					Err:    errors.New("generic error"),
				},
			},
			wantErr: wantZypperInstallErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			utiltest.SetExpectedCommands(ctx, mockCommandRunner, tt.expectedCommands)

			gotErr := ZypperInstall(ctx, patches, pkgs)
			utiltest.AssertErrorMatch(t, gotErr, tt.wantErr)
		})
	}
}

func TestZypperPatchesWithOptions(t *testing.T) {
	ctx := t.Context()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner

	tests := []struct {
		name             string
		options          []ZypperListOption
		expectedCommands []utiltest.ExpectedCommand
	}{
		{
			name:    "category filter, want category flags in command",
			options: []ZypperListOption{ZypperListPatchCategories([]string{"security", "recommended"})},
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd: exec.Command(zypper, append(zypperListPatchesArgs, "--category=security", "--category=recommended")...),
				},
			},
		},
		{
			name:    "severity filter, want severity flags in command",
			options: []ZypperListOption{ZypperListPatchSeverities([]string{"critical", "important"})},
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd: exec.Command(zypper, append(zypperListPatchesArgs, "--severity=critical", "--severity=important")...),
				},
			},
		},
		{
			name:    "with optional, want with-optional and all flags in command",
			options: []ZypperListOption{ZypperListPatchWithOptional(true)},
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd: exec.Command(zypper, append(zypperListPatchesArgs, "--with-optional", "--all")...),
				},
			},
		},
		{
			name:    "all, want all flag in command",
			options: []ZypperListOption{ZypperListPatchAll(true)},
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd: exec.Command(zypper, append(zypperListPatchesArgs, "--all")...),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			utiltest.SetExpectedCommands(ctx, mockCommandRunner, tt.expectedCommands)

			_, _ = ZypperPatches(ctx, tt.options...)
		})
	}
}
