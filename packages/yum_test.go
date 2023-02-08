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

	utilmocks "github.com/GoogleCloudPlatform/osconfig/util/mocks"
	"github.com/golang/mock/gomock"
)

func TestInstallYumPackages(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	expectedCmd := utilmocks.EqCmd(exec.Command(yum, append(yumInstallArgs, pkgs...)...))

	mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).Return([]byte("stdout"), []byte("stderr"), nil).Times(1)
	if err := InstallYumPackages(testCtx, pkgs); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).Return([]byte("stdout"), []byte("stderr"), errors.New("could not update")).Times(1)
	if err := InstallYumPackages(testCtx, pkgs); err == nil {
		t.Errorf("did not get expected error")
	}
}

func TestRemoveYum(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	expectedCmd := utilmocks.EqCmd(exec.Command(yum, append(yumRemoveArgs, pkgs...)...))

	mockCommandRunner.EXPECT().Run(ctx, expectedCmd).Return([]byte("stdout"), []byte("stderr"), nil).Times(1)
	if err := RemoveYumPackages(ctx, pkgs); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).Return([]byte("stdout"), []byte("stderr"), errors.New("removal error")).Times(1)
	if err := RemoveYumPackages(testCtx, pkgs); err == nil {
		t.Errorf("did not get expected error")
	}
}

func TestYumUpdates(t *testing.T) {
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

	if os.Getenv("EXIT100") == "1" {
		os.Exit(100)
	}

	cmd := exec.CommandContext(context.Background(), os.Args[0], "-test.run=TestYumUpdates")
	cmd.Env = append(os.Environ(), "EXIT100=1")
	errExit100 := cmd.Run()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	ptyrunner = mockCommandRunner
	expectedCheckUpdate := utilmocks.EqCmd(exec.Command(yum, yumCheckUpdateArgs...))

	// Test Error
	t.Run("Error", func(t *testing.T) {
		mockCommandRunner.EXPECT().Run(testCtx, expectedCheckUpdate).Return(data, []byte("stderr"), errors.New("Bad error")).Times(1)
		if _, err := YumUpdates(testCtx); err == nil {
			t.Errorf("did not get expected error")
		}
	})

	// yum check-updates exit code 0
	t.Run("ExitCode0", func(t *testing.T) {
		mockCommandRunner.EXPECT().Run(testCtx, expectedCheckUpdate).Return([]byte("stdout"), []byte("stderr"), nil).Times(1)
		ret, err := YumUpdates(testCtx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if ret != nil {
			t.Errorf("unexpected return: %v", ret)
		}
	})

	// Test no options
	t.Run("NoOptions", func(t *testing.T) {
		expectedCmd := utilmocks.EqCmd(exec.Command(yum, yumListUpdatesArgs...))

		first := mockCommandRunner.EXPECT().Run(testCtx, expectedCheckUpdate).Return(data, []byte("stderr"), errExit100).Times(1)
		mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).After(first).Return(data, []byte("stderr"), nil).Times(1)
		ret, err := YumUpdates(testCtx)
		if err != nil {
			t.Errorf("did not expect error: %v", err)
		}

		allPackageNames := []string{"kernel", "foo", "bar"}
		for _, pkg := range ret {
			if !contains(allPackageNames, pkg.Name) {
				t.Errorf("package %s expected to be present.", pkg.Name)
			}
		}
	})

	// Test MinimalWithSecurity
	t.Run("MinimalWithSecurity", func(t *testing.T) {
		expectedCmd := utilmocks.EqCmd(exec.Command(yum, append(yumListUpdateMinimalArgs, "--security")...))

		first := mockCommandRunner.EXPECT().Run(testCtx, expectedCheckUpdate).Return(data, []byte("stderr"), errExit100).Times(1)
		mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).After(first).Return(data, []byte("stderr"), nil).Times(1)
		ret, err := YumUpdates(testCtx, YumUpdateMinimal(true), YumUpdateSecurity(true))
		if err != nil {
			t.Errorf("did not expect error: %v", err)
		}

		allPackageNames := []string{"kernel", "foo", "bar"}
		for _, pkg := range ret {
			if !contains(allPackageNames, pkg.Name) {
				t.Errorf("package %s expected to be present.", pkg.Name)
			}
		}
	})

	/*	// Test WithSecurityWithExcludes
		t.Run("WithSecurityWithExcludes", func(t *testing.T) {
			// the mock data returned by mockcommandrunner will not include this
			// package anyways. The purpose of this test is to make sure that
			// when customer specifies excluded packages, we set the --exclude flag
			// in the yum command.
			expectedCmd := exec.CommandContext(context.Background(), yum, append(yumListUpdatesArgs, "--security")...)

			first := mockCommandRunner.EXPECT().Run(testCtx, expectedCheckUpdate).Return(data, []byte("stderr"), errExit100).Times(1)
			mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).After(first).Return(data, []byte("stderr"), nil).Times(1)
			ret, err := YumUpdates(testCtx, YumUpdateMinimal(false), YumUpdateSecurity(true))
			if err != nil {
				t.Errorf("did not expect error: %v", err)
			}

			allPackageNames := []string{"kernel", "foo", "bar"}
			for _, pkg := range ret {
				if !contains(allPackageNames, pkg.Name) {
					t.Errorf("package %s expected to be present.", pkg.Name)
				}
			}
		})*/
}

func contains(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
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
		want []*PkgInfo
	}{
		{"NormalCase", data, []*PkgInfo{{"kernel", "x86_64", "2.6.32-754.24.3.el6"}, {"foo", "all", "2.0.0-1"}, {"bar", "x86_64", "2.0.0-1"}}},
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

func TestGetYumTX(t *testing.T) {
	dataWithTX := []byte(`
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

	Transaction Summary
	=================================================================================================================================================================================
	Upgrade  11 Packages

	Total download size: 106 M
	Exiting on user command
	Your transaction was saved, rerun it with:
	 yum load-transaction /tmp/yum_save_tx.abcdef.yumtx
	`)
	dataNoTX := []byte(`
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

	Transaction Summary
	=================================================================================================================================================================================
	Upgrade  11 Packages
	`)
	dataUnexpected := []byte(`
	Exiting on user command
	Your transaction was saved, rerun it with:
	 yum load-transaction /tmp/otherfilename
	`)

	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"Transaction created", dataWithTX, "/tmp/yum_save_tx.abcdef.yumtx"},
		{"Transaction not created", dataNoTX, ""},
		{"Unexpected Filename", dataUnexpected, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getYumTXFile(tt.data); got != tt.want {
				t.Errorf("%s: getYumTXFile() = %v, want %v", tt.name, got, tt.want)
			}
		})
	}

}
