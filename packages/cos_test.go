//  Copyright 2020 Google Inc. All Rights Reserved.
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

func TestParseInstalledCosPackages(t *testing.T) {
	readMachineArch = func() (string, error) {
		return "", errors.New("failed to obtain machine architecture")
	}
	if _, err := parseInstalledCosPackages([]byte{}); err == nil {
		t.Errorf("did not get expected error")
	}

	readMachineArch = func() (string, error) {
		return "x86_64", nil
	}
	tests := []struct {
		name string
		data []byte
		want []PkgInfo
	}{
		{"NormalCase",
			[]byte("  dev-util/foo-x     1.2.3-r4\napp-admin/bar 0.1-r1"),
			[]PkgInfo{{"dev-util/foo-x", "x86_64", "1.2.3-r4"}, {"app-admin/bar", "x86_64", "0.1-r1"}}},
		{"NoPackages", []byte("no packages here"), nil},
		{"nil", nil, nil},
		{"UnrecognizedPackage",
			[]byte("foo zzz 1.2.3-r4\nsomething we dont understand\n bar 0.1-r1 "),
			[]PkgInfo{{"bar", "x86_64", "0.1-r1"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseInstalledCosPackages(tt.data)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseInstalledCosPackages() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInstalledCosPackages(t *testing.T) {
	readMachineArch = func() (string, error) {
		return "", errors.New("failed to obtain machine architecture")
	}
	readCosPackageList = func() ([]byte, error) {
		return []byte("sys-boot/foo-bar 1.2.3-r4"), nil
	}
	if _, err := InstalledCosPackages(); err == nil {
		t.Errorf("did not get expected error from readMachineArch")
	}

	readMachineArch = func() (string, error) {
		return "x86_64", nil
	}
	readCosPackageList = func() ([]byte, error) {
		return nil, errors.New("failed to read package list")
	}
	if _, err := InstalledCosPackages(); err == nil {
		t.Errorf("did not get expected error fro readCosPackageList")
	}

	readMachineArch = func() (string, error) {
		return "x86_64", nil
	}
	readCosPackageList = func() ([]byte, error) {
		return []byte("sys-boot/foo-bar 1.2.3-r4"), nil
	}
	ret, err := InstalledCosPackages()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	want := []PkgInfo{{"sys-boot/foo-bar", "x86_64", "1.2.3-r4"}}
	if !reflect.DeepEqual(ret, want) {
		t.Errorf("InstalledCosPackages() = %v, want %v", ret, want)
	}

}
