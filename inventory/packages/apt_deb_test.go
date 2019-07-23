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
	"testing"
)

func TestInstallAptPackage(t *testing.T) {
	run = getMockRun([]byte("TestInstallAptPackage"), nil)
	if err := InstallAptPackage(pkgs[0]); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInstallAptPackagesReturnsError(t *testing.T) {
	run = getMockRun([]byte("TestInstallAptPackagesReturnsError"), errors.New("Could not install package"))
	err := InstallAptPackage(pkgs[0])
	if err == nil {
		t.Errorf("did not get expected error")
	}
}

func TestRemoveApt(t *testing.T) {
	run = getMockRun([]byte("TestRemoveApt"), nil)
	if err := RemoveAptPackage(pkgs[0]); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRemoveAptReturnError(t *testing.T) {
	run = getMockRun([]byte("TestRemoveAptReturnError"), errors.New("Could not find package"))
	if err := RemoveAptPackage(pkgs[0]); err == nil {
		t.Errorf("did not get expected error")
	}
}
