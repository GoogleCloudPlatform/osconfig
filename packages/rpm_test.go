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

func TestParseInstalledRPMPackages(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want []PkgInfo
	}{
		{"NormalCase", []byte("foo x86_64 1.2.3-4\nbar noarch 1.2.3-4"), []PkgInfo{{"foo", "x86_64", "1.2.3-4"}, {"bar", "all", "1.2.3-4"}}},
		{"NoPackages", []byte("nothing here"), nil},
		{"nil", nil, nil},
		{"UnrecognizedPackage", []byte("foo.x86_64 1.2.3-4\nsomething we dont understand\n bar noarch 1.2.3-4 "), []PkgInfo{{"bar", "all", "1.2.3-4"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseInstalledRPMPackages(tt.data)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("installedRPMPackages() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInstalledRPMPackages(t *testing.T) {
	run = getMockRun([]byte("foo x86_64 1.2.3-4"), nil)
	ret, err := InstalledRPMPackages()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	want := []PkgInfo{{"foo", "x86_64", "1.2.3-4"}}
	if !reflect.DeepEqual(ret, want) {
		t.Errorf("InstalledRPMPackages() = %v, want %v", ret, want)
	}

	run = getMockRun(nil, errors.New("bad error"))
	if _, err := InstalledRPMPackages(); err == nil {
		t.Errorf("did not get expected error")
	}
}
