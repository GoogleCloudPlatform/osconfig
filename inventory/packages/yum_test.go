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

func TestInstallYumPackages(t *testing.T) {
	run = getMockRun([]byte("TestInstallYumPackages"), nil)
	if err := InstallYumPackages(pkgs); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInstallYumPackagesReturnsError(t *testing.T) {
	run = getMockRun([]byte("TestInstallYumPackagesReturnsError"), errors.New("Could not install package"))
	if err := InstallYumPackages(pkgs); err == nil {
		t.Errorf("did not get expected error")
	}
}

func TestRemoveYum(t *testing.T) {
	run = getMockRun([]byte("TestRemoveYum"), nil)
	if err := RemoveYumPackages(pkgs); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRemoveYumReturnError(t *testing.T) {
	run = getMockRun([]byte("TestRemoveYumReturnError"), errors.New("Could not find package"))
	if err := RemoveYumPackages(pkgs); err == nil {
		t.Errorf("did not get expected error")
	}
}

func TestYumUpdates(t *testing.T) {
	run = getMockRun([]byte("TestYumUpdatesError"), errors.New("Bad error"))
	if _, err := YumUpdates(); err == nil {
		t.Errorf("did not get expected error")
	}
}

func TestYumUpdatesExitCode0(t *testing.T) {
	run = getMockRun([]byte("TestYumUpdatesError"), nil)
	ret, err := YumUpdates()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if ret != nil {
		t.Errorf("unexpected return: %v", ret)
	}
}

func TestParseYumUpdates(t *testing.T) {
	data := []byte(`
	=================================================================================================================================================================================
	Package                                      Arch                           Version                                              Repository                                Size
	=================================================================================================================================================================================
	Installing:
      kernel                                    x86_64                         2.6.32-754.24.3.el6                                  updates                                   32 M
	    replacing kernel.x86_64 1.0.0-4
	Upgrading:
	  foo                                       noarch                         2.0.0-1                                              BaseOS                                   361 k
	  bar                                       x86_64                         2.0.0-1                                              repo                                      10 M
	Obsoleting:
	  baz                                       noarch                         2.0.0-1                                              repo                                      10 M
`)

	tests := []struct {
		name string
		data []byte
		want []PkgInfo
	}{
		{"NormalCase", data, []PkgInfo{{"kernel", "x86_64", "2.6.32-754.24.3.el6"}, {"foo", "all", "2.0.0-1"}, {"bar", "x86_64", "2.0.0-1"}}},
		{"NoPackages", []byte("nothing here"), nil},
		{"nil", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseYumUpdates(tt.data); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseYumUpdates() = %v, want %v", got, tt.want)
			}
		})
	}
}
