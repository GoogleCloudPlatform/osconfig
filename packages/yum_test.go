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
	"os"
	"os/exec"
	"reflect"
	"testing"

	"github.com/GoogleCloudPlatform/osconfig/util/mocks"
	"github.com/golang/mock/gomock"
)

func TestInstallYumPackages(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := mocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	mockCommandRunner.EXPECT().Run(ctx, exec.Command("/usr/bin/yum", []string{"install", "--assumeyes", "pkg1", "pkg2"}...)).Return([]byte("update successful"), nil).Times(1)

	if err := InstallYumPackages(ctx, pkgs); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInstallYumPackagesReturnsError(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := mocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	mockCommandRunner.EXPECT().Run(ctx, exec.Command("/usr/bin/yum", []string{"install", "--assumeyes", "pkg1", "pkg2"}...)).Return([]byte("update unsuccessful"), errors.New("could not update")).Times(1)

	if err := InstallYumPackages(ctx, pkgs); err == nil {
		t.Errorf("did not get expected error")
	}
}

func TestRemoveYum(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := mocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	mockCommandRunner.EXPECT().Run(ctx, exec.Command("/usr/bin/yum", []string{"remove", "--assumeyes", "pkg1", "pkg2"}...)).Return([]byte("removed successfully"), nil).Times(1)

	if err := RemoveYumPackages(ctx, pkgs); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRemoveYumReturnError(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := mocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	mockCommandRunner.EXPECT().Run(ctx, exec.Command("/usr/bin/yum", []string{"remove", "--assumeyes", "pkg1", "pkg2"}...)).Return([]byte("could not remove successfully"), errors.New("removal error")).Times(1)

	if err := RemoveYumPackages(ctx, pkgs); err == nil {
		t.Errorf("did not get expected error")
	}
}

func TestYumUpdates(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := mocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	mockCommandRunner.EXPECT().Run(ctx, exec.Command("/usr/bin/yum", []string{"check-update", "--assumeyes"}...)).Return([]byte("TestYumUpdatesError"), errors.New("Bad error")).Times(1)

	if _, err := YumUpdates(ctx); err == nil {
		t.Errorf("did not get expected error")
	}
}

func TestYumUpdatesMinimalWithSecurity(t *testing.T) {
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

	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := mocks.NewMockCommandRunner(mockCtrl)
	ptyrunner = mockCommandRunner
	mockCommandRunner.EXPECT().Run(ctx, exec.Command("/usr/bin/yum", []string{"update-minimal", "--assumeno", "--cacheonly", "--security"}...)).Return(data, nil).Times(1)

	ret, err := listAndParseYumPackages(ctx, YumUpdateMinimal(true), YumUpdateSecurity(true))
	if err != nil {
		t.Errorf("did not expect error: %+v", err)
	}

	if len(ret) <= 0 {
		t.Errorf("unexpected number of updates.")
	}

	allPackageNames := []string{"kernel", "foo", "bar"}
	for _, pkg := range ret {
		if !contains(allPackageNames, pkg.Name) {
			t.Errorf("package %s expected to be present.", pkg.Name)
		}
	}
}

func TestYumUpdatesWithSecurityWithExcludes(t *testing.T) {
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

	// the mock data returned by mockcommandrunner will not include this
	// package anyways. The purpose of this test is to make sure that
	// when customer specifies excluded packages, we set the --exclude flag
	// in the yum command.
	excludedPackages := []string{"ex-pkg1", "ex-pkg2"}
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := mocks.NewMockCommandRunner(mockCtrl)
	ptyrunner = mockCommandRunner
	mockCommandRunner.EXPECT().Run(ctx, exec.Command("/usr/bin/yum", []string{"update", "--assumeno", "--cacheonly", "--security", "--exclude", excludedPackages[0], "--exclude", excludedPackages[1]}...)).Return(data, nil).Times(1)

	ret, err := listAndParseYumPackages(ctx, YumUpdateMinimal(false), YumUpdateSecurity(true), YumExcludes(excludedPackages))
	if err != nil {
		t.Errorf("did not expect error: %+v", err)
	}

	if len(ret) != 3 {
		t.Errorf("unexpected number of updates.")
	}
}

func contains(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}

func TestYumUpdatesExitCode0(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := mocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	mockCommandRunner.EXPECT().Run(ctx, exec.Command("/usr/bin/yum", []string{"check-update", "--assumeyes"}...)).Return([]byte("TestYumUpdatesError"), nil).Times(1)

	ret, err := YumUpdates(ctx)
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

func TestYumUpdatesExitCode100(t *testing.T) {
	data := []byte(`
	=================================================================================================================================================================================
	Package                                      Arch                           Version                                              Repository                                Size
    =================================================================================================================================================================================
    Upgrading:
      foo                                       noarch                         2.0.0-1                           BaseOS                                   361 k
    blah
`)
	excludedPackages := []string{"ex-pkg1", "ex-pkg2"}
	ctx := context.Background()

	if os.Getenv("EXIT100") == "1" {
		os.Exit(100)
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestYumUpdatesExitCode100")
	cmd.Env = append(os.Environ(), "EXIT100=1")
	err := cmd.Run()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := mocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	mockCommandRunner.EXPECT().Run(ctx, exec.Command("/usr/bin/yum", []string{"check-update", "--assumeyes"}...)).Return([]byte("TestYumUpdatesError"), err).Times(1)

	ptyrunner = mockCommandRunner
	mockCommandRunner.EXPECT().Run(ctx, exec.Command("/usr/bin/yum", []string{"update", "--assumeno", "--cacheonly", "--exclude", excludedPackages[0], "--exclude", excludedPackages[1]}...)).Return(data, nil).Times(1)

	ret, err := YumUpdates(ctx, YumExcludes(excludedPackages))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	want := []PkgInfo{{"foo", "all", "2.0.0-1"}}
	if !reflect.DeepEqual(ret, want) {
		t.Errorf("YumUpdates() = %v, want %v", ret, want)
	}
}
