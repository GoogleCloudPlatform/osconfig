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
	"github.com/golang/mock/gomock"
)

func TestInstallGooGetPackages(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	expectedCmd := exec.CommandContext(context.Background(), googet, append(googetInstallArgs, pkgs...)...)

	mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).Return([]byte("stdout"), []byte("stderr"), nil).Times(1)
	if err := InstallGooGetPackages(testCtx, pkgs); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).Return([]byte("stdout"), []byte("stderr"), errors.New("Could not install package")).Times(1)
	if err := InstallGooGetPackages(testCtx, pkgs); err == nil {
		t.Errorf("did not get expected error")
	}
}

func TestRemoveGooGet(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	expectedCmd := exec.CommandContext(context.Background(), googet, append(googetRemoveArgs, pkgs...)...)

	mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).Return([]byte("stdout"), []byte("stderr"), nil).Times(1)
	if err := RemoveGooGetPackages(testCtx, pkgs); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).Return([]byte("stdout"), []byte("stderr"), errors.New("Could not remove package")).Times(1)
	if err := RemoveGooGetPackages(testCtx, pkgs); err == nil {
		t.Errorf("did not get expected error")
	}
}

func TestParseInstalledGooGetPackages(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want []*PkgInfo
	}{
		{"NormalCase", []byte(" Installed Packages:\nfoo.x86_64 1.2.3@4\nbar.noarch 1.2.3@4"), []*PkgInfo{{"foo", "x86_64", "1.2.3@4"}, {"bar", "noarch", "1.2.3@4"}}},
		{"NoPackages", []byte("nothing here"), nil},
		{"nil", nil, nil},
		{"UnrecognizedPackage", []byte("Inst something we dont understand\n foo.x86_64 1.2.3@4"), []*PkgInfo{{"foo", "x86_64", "1.2.3@4"}}},
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
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	expectedCmd := exec.CommandContext(context.Background(), googet, googetInstalledQueryArgs...)

	mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).Return([]byte("foo.x86_64 1.2.3@4"), []byte("stderr"), nil).Times(1)
	ret, err := InstalledGooGetPackages(testCtx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	want := []*PkgInfo{{"foo", "x86_64", "1.2.3@4"}}
	if !reflect.DeepEqual(ret, want) {
		t.Errorf("InstalledGooGetPackages() = %v, want %v", ret, want)
	}

	mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).Return(nil, nil, errors.New("bad error")).Times(1)
	if _, err := InstalledGooGetPackages(testCtx); err == nil {
		t.Errorf("did not get expected error")
	}
}

func TestParseGooGetUpdates(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want []*PkgInfo
	}{
		{"NormalCase", []byte("Searching for available updates...\nfoo.noarch, 3.5.4@1 --> 3.6.7@1 from repo\nbar.x86_64, 1.0.0@1 --> 2.0.0@1 from repo\nPerform update? (y/N):"), []*PkgInfo{{"foo", "noarch", "3.6.7@1"}, {"bar", "x86_64", "2.0.0@1"}}},
		{"NoPackages", []byte("nothing here"), nil},
		{"nil", nil, nil},
		{"UnrecognizedPackage", []byte("Inst something we dont understand\n foo.noarch, 3.5.4@1 --> 3.6.7@1 from repo"), []*PkgInfo{{"foo", "noarch", "3.6.7@1"}}},
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
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	expectedCmd := exec.CommandContext(context.Background(), googet, googetUpdateQueryArgs...)

	mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).Return([]byte("foo.noarch, 3.5.4@1 --> 3.6.7@1 from repo"), []byte("stderr"), nil).Times(1)
	ret, err := GooGetUpdates(testCtx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	want := []*PkgInfo{{"foo", "noarch", "3.6.7@1"}}
	if !reflect.DeepEqual(ret, want) {
		t.Errorf("GooGetUpdates() = %v, want %v", ret, want)
	}

	mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).Return([]byte("stdout"), []byte("stderr"), errors.New("bad error")).Times(1)
	if _, err := GooGetUpdates(testCtx); err == nil {
		t.Errorf("did not get expected error")
	}
}
