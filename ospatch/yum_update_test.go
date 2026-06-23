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

package ospatch

import (
	"context"
	"errors"
	"os/exec"
	"testing"

	"github.com/GoogleCloudPlatform/osconfig/packages"
	utilmocks "github.com/GoogleCloudPlatform/osconfig/util/mocks"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
	"github.com/golang/mock/gomock"
)

func stringPtr(s string) *string {
	return &s
}

func setupYumUpdateTestData() (dataSecurity, dataExclusives, dataDryRun, dataExcludes []byte) {
	dataSecurity = []byte(`
	=================================================================================================================================================================================
	Package                                      Arch                           Version                                              Repository                                Size
	=================================================================================================================================================================================
	Upgrading:
	  foo                                       noarch                         2.0.0-1                           BaseOS                                   361 k
	blah
`)

	dataExclusives = []byte(`
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

	dataDryRun = []byte(`
	=================================================================================================================================================================================
	Package                                      Arch                           Version                                              Repository                                Size
	=================================================================================================================================================================================
	Upgrading:
	  foo                                       noarch                         2.0.0-1                           BaseOS                                   361 k
`)

	dataExcludes = []byte(`
	=================================================================================================================================================================================
	Package                                      Arch                           Version                                              Repository                                Size
	=================================================================================================================================================================================
	Upgrading:
	  foo                                       noarch                         2.0.0-1                           BaseOS                                   361 k
	  bar                                       x86_64                         2.0.0-1                           BaseOS                                   10 M
`)

	return
}

// TestRunYumUpdate tests different scenarios of RunYumUpdate using table-driven tests.
func TestRunYumUpdate(t *testing.T) {
	ctx := context.Background()
	errExit100 := exec.Command("/bin/bash", "-c", "exit 100").Run()

	mockCtrl := gomock.NewController(t)
	t.Cleanup(func() { mockCtrl.Finish() })

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	packages.SetCommandRunner(mockCommandRunner)
	packages.SetPtyCommandRunner(mockCommandRunner)

	dataSecurity, dataExclusives, dataDryRun, dataExcludes := setupYumUpdateTestData()

	tests := []struct {
		name             string
		expectedCommands []utiltest.ExpectedCommand
		opts             []YumUpdateOption
		wantErr          error
	}{
		{
			name: "security flag set, want security packages upgraded and nil error",
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command("/usr/bin/yum", "check-update", "--assumeyes"),
					Stdout: []byte("stdout"),
					Stderr: []byte("stderr"),
					Err:    errExit100,
				},
				{
					Cmd:    exec.Command("/usr/bin/yum", "update", "--assumeno", "--color=never", "--security"),
					Stdout: dataSecurity,
					Stderr: []byte("stderr"),
				},
				{
					Cmd:    exec.Command("/usr/bin/yum", "install", "--assumeyes", "foo.noarch"),
					Stdout: []byte("stdout"),
					Stderr: []byte("stderr"),
				},
			},
			opts: []YumUpdateOption{
				YumUpdateMinimal(false),
				YumUpdateSecurity(true),
			},
			wantErr: nil,
		},
		{
			name: "security flag and exclusive packages set, want only exclusive packages installed and nil error",
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command("/usr/bin/yum", "check-update", "--assumeyes"),
					Stdout: []byte("stdout"),
					Stderr: []byte("stderr"),
					Err:    errExit100,
				},
				{
					Cmd:    exec.Command("/usr/bin/yum", "update", "--assumeno", "--color=never", "--security"),
					Stdout: dataExclusives,
					Stderr: []byte("stderr"),
				},
				{
					Cmd:    exec.Command("/usr/bin/yum", "install", "--assumeyes", "foo.noarch", "bar.x86_64"),
					Stdout: []byte("stdout"),
					Stderr: []byte("stderr"),
				},
			},
			opts: []YumUpdateOption{
				YumUpdateMinimal(false),
				YumUpdateSecurity(true),
				YumExclusivePackages([]string{"foo", "bar"}),
			},
			wantErr: nil,
		},
		{
			name: "dry run set, want check and update run without installation and nil error",
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command("/usr/bin/yum", "check-update", "--assumeyes"),
					Stdout: []byte("stdout"),
					Stderr: []byte("stderr"),
					Err:    errExit100,
				},
				{
					Cmd:    exec.Command("/usr/bin/yum", "update", "--assumeno", "--color=never"),
					Stdout: dataDryRun,
					Stderr: []byte("stderr"),
				},
			},
			opts: []YumUpdateOption{
				YumDryRun(true),
			},
			wantErr: nil,
		},
		{
			name: "excludes set, want excluded packages filtered out and nil error",
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command("/usr/bin/yum", "check-update", "--assumeyes"),
					Stdout: []byte("stdout"),
					Stderr: []byte("stderr"),
					Err:    errExit100,
				},
				{
					Cmd:    exec.Command("/usr/bin/yum", "update", "--assumeno", "--color=never"),
					Stdout: dataExcludes,
					Stderr: []byte("stderr"),
				},
				{
					Cmd:    exec.Command("/usr/bin/yum", "install", "--assumeyes", "foo.noarch"),
					Stdout: []byte("stdout"),
					Stderr: []byte("stderr"),
				},
			},
			opts: []YumUpdateOption{
				YumUpdateExcludes([]*Exclude{CreateStringExclude(stringPtr("bar"))}),
			},
			wantErr: nil,
		},
		{
			name: "conflicting excludes and exclusives, want excludes error",
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command("/usr/bin/yum", "check-update", "--assumeyes"),
					Stdout: []byte("stdout"),
					Stderr: []byte("stderr"),
					Err:    errExit100,
				},
				{
					Cmd:    exec.Command("/usr/bin/yum", "update", "--assumeno", "--color=never"),
					Stdout: []byte("Upgrading:\n foo noarch 2.0.0-1 BaseOS 361 k"),
				},
			},
			opts: []YumUpdateOption{
				YumExclusivePackages([]string{"foo"}),
				YumUpdateExcludes([]*Exclude{CreateStringExclude(new(string))}),
			},
			wantErr: errors.New("exclusivePackages and excludes can not both be non 0"),
		},
		{
			name: "no package upgrades available, want nil error",
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command("/usr/bin/yum", "check-update", "--assumeyes"),
					Stdout: []byte("stdout"),
					Stderr: []byte("stderr"),
				},
			},
			opts:    nil,
			wantErr: nil,
		},
		{
			name: "yum updates query fails, want updates error",
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd: exec.Command("/usr/bin/yum", "check-update", "--assumeyes"),
					Err: errors.New("yum updates error"),
				},
			},
			opts:    nil,
			wantErr: errors.New("error running /usr/bin/yum with args [\"check-update\" \"--assumeyes\"]: yum updates error, stdout: \"\", stderr: \"\""),
		},
		{
			name: "yum packages installation fails, want install error",
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command("/usr/bin/yum", "check-update", "--assumeyes"),
					Stdout: []byte("stdout"),
					Stderr: []byte("stderr"),
					Err:    errExit100,
				},
				{
					Cmd:    exec.Command("/usr/bin/yum", "update", "--assumeno", "--color=never"),
					Stdout: []byte("Upgrading:\n foo noarch 2.0.0-1 BaseOS 361 k"),
				},
				{
					Cmd: exec.Command("/usr/bin/yum", "install", "--assumeyes", "foo.noarch"),
					Err: errors.New("install error"),
				},
			},
			opts:    nil,
			wantErr: errors.New("error running /usr/bin/yum with args [\"install\" \"--assumeyes\" \"foo.noarch\"]: install error, stdout: \"\", stderr: \"\""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			utiltest.SetExpectedCommands(ctx, mockCommandRunner, tt.expectedCommands)

			gotErr := RunYumUpdate(ctx, tt.opts...)
			utiltest.AssertErrorMatch(t, gotErr, tt.wantErr)
		})
	}
}
