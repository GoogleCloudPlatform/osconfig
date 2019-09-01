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

	"github.com/GoogleCloudPlatform/osconfig/common"
)

func TestInstallYumPackages(t *testing.T) {
	common.Run = getMockRun([]byte("TestInstallYumPackages"), nil)
	if err := InstallYumPackages(pkgs); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInstallYumPackagesReturnsError(t *testing.T) {
	common.Run = getMockRun([]byte("TestInstallYumPackagesReturnsError"), errors.New("Could not install package"))
	if err := InstallYumPackages(pkgs); err == nil {
		t.Errorf("did not get expected error")
	}
}

func TestRemoveYum(t *testing.T) {
	common.Run = getMockRun([]byte("TestRemoveYum"), nil)
	if err := RemoveYumPackages(pkgs); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRemoveYumReturnError(t *testing.T) {
	common.Run = getMockRun([]byte("TestRemoveYumReturnError"), errors.New("Could not find package"))
	if err := RemoveYumPackages(pkgs); err == nil {
		t.Errorf("did not get expected error")
	}
}

func TestYumUpdates(t *testing.T) {
	common.Run = getMockRun([]byte("TestYumUpdatesError"), errors.New("Bad error"))
	if _, err := YumUpdates(); err == nil {
		t.Errorf("did not get expected error")
	}
}

func TestYumUpdatesExitCode0(t *testing.T) {
	common.Run = getMockRun([]byte("TestYumUpdatesError"), nil)
	ret, err := YumUpdates()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if ret != nil {
		t.Errorf("unexpected return: %v", ret)
	}
}

func TestYumUpdatesExitCode100(t *testing.T) {
	if os.Getenv("EXIT100") == "1" {
		os.Exit(100)
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestYumUpdatesExitCode100")
	cmd.Env = append(os.Environ(), "EXIT100=1")

	common.Run = getMockRun([]byte("foo.noarch 2.0.0-1 repo"), cmd.Run())
	ret, err := YumUpdates()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	want := []PkgInfo{{"foo", "all", "2.0.0-1"}}
	if !reflect.DeepEqual(ret, want) {
		t.Errorf("YumUpdates() = %v, want %v", ret, want)
	}
}

func TestParseYumUpdates(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want []PkgInfo
	}{
		{"NormalCase", []byte(" \nfoo.noarch 2.0.0-1 repo\nbar.x86_64 2.0.0-1 repo\nObsoleting Packages\nbaz.noarch 2.0.0-1 repo"), []PkgInfo{{"foo", "all", "2.0.0-1"}, {"bar", "x86_64", "2.0.0-1"}}},
		{"NoPackages", []byte("nothing here"), nil},
		{"nil", nil, nil},
		{"UnrecognizedPackage", []byte("this.is.a bad package\nsomething we dont understand\n bar.noarch 1.2.3-4 repo"), []PkgInfo{{"bar", "all", "1.2.3-4"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseYumUpdates(tt.data); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseYumUpdates() = %v, want %v", got, tt.want)
			}
		})
	}
}
