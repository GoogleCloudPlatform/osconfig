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

func TestInstallGooGetPackages(t *testing.T) {
	run = getMockRun([]byte("TestInstallGooGetPackages"), nil)
	if err := InstallGooGetPackages(testCtx, pkgs); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInstallGooGetPackagesReturnsError(t *testing.T) {
	run = getMockRun([]byte("TestInstallGooGetPackagesReturnsError"), errors.New("Could not install package"))
	if err := InstallGooGetPackages(testCtx, pkgs); err == nil {
		t.Errorf("did not get expected error")
	}
}

func TestRemoveGooGet(t *testing.T) {
	run = getMockRun([]byte("TestRemoveGooGet"), nil)
	if err := RemoveGooGetPackages(testCtx, pkgs); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRemoveGooGetReturnError(t *testing.T) {
	run = getMockRun([]byte("TestRemoveGooGetReturnError"), errors.New("Could not find package"))
	if err := RemoveGooGetPackages(testCtx, pkgs); err == nil {
		t.Errorf("did not get expected error")
	}
}

func TestParseInstalledGooGetPackages(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want []PkgInfo
	}{
		{"NormalCase", []byte(" Installed Packages:\nfoo.x86_64 1.2.3@4\nbar.noarch 1.2.3@4"), []PkgInfo{{"foo", "x86_64", "1.2.3@4"}, {"bar", "noarch", "1.2.3@4"}}},
		{"NoPackages", []byte("nothing here"), nil},
		{"nil", nil, nil},
		{"UnrecognizedPackage", []byte("Inst something we dont understand\n foo.x86_64 1.2.3@4"), []PkgInfo{{"foo", "x86_64", "1.2.3@4"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseInstalledGooGetPackages(tt.data); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseInstalledGooGetPackages() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInstalledGooGetPackages(t *testing.T) {
	run = getMockRun([]byte("foo.x86_64 1.2.3@4"), nil)
	ret, err := InstalledGooGetPackages(testCtx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	want := []PkgInfo{{"foo", "x86_64", "1.2.3@4"}}
	if !reflect.DeepEqual(ret, want) {
		t.Errorf("InstalledGooGetPackages() = %v, want %v", ret, want)
	}

	run = getMockRun(nil, errors.New("bad error"))
	if _, err := InstalledGooGetPackages(testCtx); err == nil {
		t.Errorf("did not get expected error")
	}
}

func TestParseGooGetUpdates(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want []PkgInfo
	}{
		{"NormalCase", []byte("Searching for available updates...\nfoo.noarch, 3.5.4@1 --> 3.6.7@1 from repo\nbar.x86_64, 1.0.0@1 --> 2.0.0@1 from repo\nPerform update? (y/N):"), []PkgInfo{{"foo", "noarch", "3.6.7@1"}, {"bar", "x86_64", "2.0.0@1"}}},
		{"NoPackages", []byte("nothing here"), nil},
		{"nil", nil, nil},
		{"UnrecognizedPackage", []byte("Inst something we dont understand\n foo.noarch, 3.5.4@1 --> 3.6.7@1 from repo"), []PkgInfo{{"foo", "noarch", "3.6.7@1"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseGooGetUpdates(tt.data); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseGooGetUpdates() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGooGetUpdates(t *testing.T) {
	run = getMockRun([]byte("foo.noarch, 3.5.4@1 --> 3.6.7@1 from repo"), nil)
	ret, err := GooGetUpdates(testCtx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	want := []PkgInfo{{"foo", "noarch", "3.6.7@1"}}
	if !reflect.DeepEqual(ret, want) {
		t.Errorf("GooGetUpdates() = %v, want %v", ret, want)
	}

	run = getMockRun(nil, errors.New("bad error"))
	if _, err := GooGetUpdates(testCtx); err == nil {
		t.Errorf("did not get expected error")
	}
}
