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
)

func TestZypperInstalls(t *testing.T) {
	run = getMockRun([]byte("TestZypperInstalls"), nil)
	if err := InstallZypperPackages(pkgs); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestZypperInstallsReturnError(t *testing.T) {
	run = getMockRun([]byte("TestZypperInstallsReturnError"), errors.New("Could not find package"))
	if err := InstallZypperPackages(pkgs); err == nil {
		t.Errorf("did not get expected error")
	}
}

func TestRemoveZypper(t *testing.T) {
	run = getMockRun([]byte("TestRemoveZypper"), nil)
	if err := RemoveZypperPackages(pkgs); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRemoveZypperReturnError(t *testing.T) {
	run = getMockRun([]byte("TestRemoveZypperReturnError"), errors.New("Could not find package"))
	if err := RemoveZypperPackages(pkgs); err == nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestParseZypperUpdates(t *testing.T) {
	normalCase := `S | Repository          | Name                   | Current Version | Available Version | Arch
--+---------------------+------------------------+-----------------+-------------------+-------
v | SLES12-SP3-Updates  | at                     | 3.1.14-7.3      | 3.1.14-8.3.1      | x86_64
v | SLES12-SP3-Updates  | autoyast2-installation | 3.2.17-1.3      | 3.2.22-2.9.2      | noarch`

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
	run = getMockRun([]byte("v | SLES12-SP3-Updates  | at                     | 3.1.14-7.3      | 3.1.14-8.3.1      | x86_64"), nil)
	ret, err := ZypperUpdates()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	want := []PkgInfo{{"at", "x86_64", "3.1.14-8.3.1"}}
	if !reflect.DeepEqual(ret, want) {
		t.Errorf("ZypperUpdates() = %v, want %v", ret, want)
	}

	run = getMockRun(nil, errors.New("bad error"))
	if _, err := ZypperUpdates(); err == nil {
		t.Errorf("did not get expected error")
	}
}
