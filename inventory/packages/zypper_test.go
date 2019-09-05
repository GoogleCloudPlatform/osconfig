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
	"reflect"
	"testing"

	"github.com/GoogleCloudPlatform/osconfig/common"
)

func TestZypperInstalls(t *testing.T) {
	common.Run = getMockRun([]byte("TestZypperInstalls"), nil)
	if err := InstallZypperPackages(pkgs); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestZypperInstallsReturnError(t *testing.T) {
	common.Run = getMockRun([]byte("TestZypperInstallsReturnError"), errors.New("Could not find package"))
	if err := InstallZypperPackages(pkgs); err == nil {
		t.Errorf("did not get expected error")
	}
}

func TestRemoveZypper(t *testing.T) {
	common.Run = getMockRun([]byte("TestRemoveZypper"), nil)
	if err := RemoveZypperPackages(pkgs); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRemoveZypperReturnError(t *testing.T) {
	common.Run = getMockRun([]byte("TestRemoveZypperReturnError"), errors.New("Could not find package"))
	if err := RemoveZypperPackages(pkgs); err == nil {
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
		want []PkgInfo
	}{
		{"NormalCase", []byte(normalCase), []PkgInfo{{"at", "x86_64", "3.1.14-8.3.1"}, {"autoyast2-installation", "all", "3.2.22-2.9.2"}}},
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
	common.Run = getMockRun([]byte("v | SLES12-SP3-Updates  | at                     | 3.1.14-7.3      | 3.1.14-8.3.1      | x86_64"), nil)
	ret, err := ZypperUpdates()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	want := []PkgInfo{{"at", "x86_64", "3.1.14-8.3.1"}}
	if !reflect.DeepEqual(ret, want) {
		t.Errorf("ZypperUpdates() = %v, want %v", ret, want)
	}

	common.Run = getMockRun(nil, errors.New("bad error"))
	if _, err := ZypperUpdates(); err == nil {
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
		wantIns   []ZypperPatch
		wantAvail []ZypperPatch
	}{
		{
			"NormalCase",
			[]byte(normalCase),
			[]ZypperPatch{{"SUSE-SLE-Module-Basesystem-15-SP1-2019-1206", "security", "low", "Security update for bzip2"}},
			[]ZypperPatch{{"SUSE-SLE-Module-Basesystem-15-SP1-2019-1221", "security", "moderate", "Security update for libxslt"}, {"SUSE-SLE-Module-Basesystem-15-SP1-2019-1258", "recommended", "moderate", "Recommended update for postfix"}},
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
	common.Run = getMockRun([]byte("SLE-Module-Basesystem15-SP1-Updates | SUSE-SLE-Module-Basesystem-15-SP1-2019-1258 | recommended | moderate  | ---         | needed     | Recommended update for postfix"), nil)
	ret, err := ZypperPatches()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	want := []ZypperPatch{{"SUSE-SLE-Module-Basesystem-15-SP1-2019-1258", "recommended", "moderate", "Recommended update for postfix"}}
	if !reflect.DeepEqual(ret, want) {
		t.Errorf("ZypperPatches() = %v, want %v", ret, want)
	}

	common.Run = getMockRun(nil, errors.New("bad error"))
	if _, err := ZypperPatches(); err == nil {
		t.Errorf("did not get expected error")
	}
}

func TestZypperInstalledPatches(t *testing.T) {
	common.Run = getMockRun([]byte("SLE-Module-Basesystem15-SP1-Updates | SUSE-SLE-Module-Basesystem-15-SP1-2019-1258 | recommended | moderate  | ---         | applied     | Recommended update for postfix"), nil)
	ret, err := ZypperInstalledPatches()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	want := []ZypperPatch{{"SUSE-SLE-Module-Basesystem-15-SP1-2019-1258", "recommended", "moderate", "Recommended update for postfix"}}
	if !reflect.DeepEqual(ret, want) {
		t.Errorf("ZypperInstalledPatches() = %v, want %v", ret, want)
	}

	common.Run = getMockRun(nil, errors.New("bad error"))
	if _, err := ZypperInstalledPatches(); err == nil {
		t.Errorf("did not get expected error")
	}
}
