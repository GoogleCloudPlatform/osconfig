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
	utiltest "github.com/GoogleCloudPlatform/osconfig/util/utiltest"
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

	errExit100 := exec.Command("/bin/bash", "-c", "exit 100").Run()

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

	tests := []struct {
		name                  string
		expectedCommandsChain []expectedCommand
		expectedError         error
		expectedResultsFile   string
	}{
		{
			name: "centos-7-1 mapped output matches snapshot",
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command(yum, yumCheckUpdateArgs...),
					stdout: utiltest.BytesFromFile(t, "./testdata/centos-7-1.yum-check-update.stdout"),
					stderr: utiltest.BytesFromFile(t, "./testdata/centos-7-1.yum-check-update.stderr"),
					err:    errExit100,
				},
				{
					cmd:    exec.Command(yum, yumListUpdatesArgs...),
					stdout: utiltest.BytesFromFile(t, "./testdata/centos-7-1.yum-update.stdout"),
					stderr: utiltest.BytesFromFile(t, "./testdata/centos-7-1.yum-update.stderr"),
					err:    nil,
				},
			},
			expectedResultsFile: "./testdata/centos-7-1.yum-update.expected",
		},
		{
			name: "oracle-linux-8 mapped output matches snapshot",
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command(yum, yumCheckUpdateArgs...),
					stdout: utiltest.BytesFromFile(t, "./testdata/oracle-linux-8.yum-check-update.stdout"),
					stderr: utiltest.BytesFromFile(t, "./testdata/oracle-linux-8.yum-check-update.stderr"),
					err:    errExit100,
				},
				{
					cmd:    exec.Command(yum, yumListUpdatesArgs...),
					stdout: utiltest.BytesFromFile(t, "./testdata/oracle-linux-8.yum-update.stdout"),
					stderr: utiltest.BytesFromFile(t, "./testdata/oracle-linux-8.yum-update.stderr"),
					err:    nil,
				},
			},
			expectedResultsFile: "./testdata/oracle-linux-8.yum-update.expected",
		},
		{
			name: "rhel-7-1 mapped output matches snapshot",
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command(yum, yumCheckUpdateArgs...),
					stdout: utiltest.BytesFromFile(t, "./testdata/rhel-7-1.yum-check-update.stdout"),
					stderr: utiltest.BytesFromFile(t, "./testdata/rhel-7-1.yum-check-update.stderr"),
					err:    errExit100,
				},
				{
					cmd:    exec.Command(yum, yumListUpdatesArgs...),
					stdout: utiltest.BytesFromFile(t, "./testdata/rhel-7-1.yum-update.stdout"),
					stderr: utiltest.BytesFromFile(t, "./testdata/rhel-7-1.yum-update.stderr"),
					err:    nil,
				},
			},
			expectedResultsFile: "./testdata/rhel-7-1.yum-update.expected",
		},
		{
			name: "rocky8-8 mapped output matches snapshot",
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command(yum, yumCheckUpdateArgs...),
					stdout: utiltest.BytesFromFile(t, "./testdata/rocky8-8.yum-check-update.stdout"),
					stderr: utiltest.BytesFromFile(t, "./testdata/rocky8-8.yum-check-update.stderr"),
					err:    errExit100,
				},
				{
					cmd:    exec.Command(yum, yumListUpdatesArgs...),
					stdout: utiltest.BytesFromFile(t, "./testdata/rocky8-8.yum-update.stdout"),
					stderr: utiltest.BytesFromFile(t, "./testdata/rocky8-8.yum-update.stderr"),
					err:    nil,
				},
			},
			expectedResultsFile: "./testdata/rocky8-8.yum-update.expected",
		},
	}

	for _, tt := range tests {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
		SetCommandRunner(mockCommandRunner)
		SetPtyCommandRunner(mockCommandRunner)

		t.Run(tt.name, func(t *testing.T) {
			setExpectations(mockCommandRunner, tt.expectedCommandsChain)

			pkgs, err := YumUpdates(testCtx)

			if !reflect.DeepEqual(err, tt.expectedError) {
				t.Errorf("unexpected err: expected %v, got %q", tt.expectedError, err)
			} else {
				utiltest.MatchSnapshot(t, pkgs, tt.expectedResultsFile)
			}
		})
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
		{"NormalCase", data, []*PkgInfo{{Name: "kernel", Arch: "x86_64", RawArch: "x86_64", Version: "2.6.32-754.24.3.el6"}, {Name: "foo", Arch: "all", RawArch: "noarch", Version: "2.0.0-1"}, {Name: "bar", Arch: "x86_64", RawArch: "x86_64", Version: "2.0.0-1"}}},
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

func TestParseYumUpdatesWithInstallingDependenciesKeywords(t *testing.T) {
	data := []byte(`
	=================================================================================================================================================================================
	Package                                      Arch                           Version                                              Repository                                Size
	=================================================================================================================================================================================
	Installing dependencies:
	  kernel                                    x86_64                         2.6.32-754.24.3.el6                                  updates                                   32 M
	Installing weak dependencies:
	  foo                                       noarch                         2.0.0-1                                              BaseOS                                   361 k
	  bar                                       x86_64                         2.0.0-1                                              repo                                      10 M
`)

	tests := []struct {
		name string
		data []byte
		want []*PkgInfo
	}{
		{"NormalCase", data, []*PkgInfo{{Name: "kernel", Arch: "x86_64", RawArch: "x86_64", Version: "2.6.32-754.24.3.el6"}, {Name: "foo", Arch: "all", RawArch: "noarch", Version: "2.0.0-1"}, {Name: "bar", Arch: "x86_64", RawArch: "x86_64", Version: "2.0.0-1"}}},
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
