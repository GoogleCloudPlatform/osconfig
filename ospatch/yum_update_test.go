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

func TestRunYumUpdateWithSecurity(t *testing.T) {
	data := []byte(`
	=================================================================================================================================================================================
	Package                                      Arch                           Version                                              Repository                                Size
    =================================================================================================================================================================================
    Upgrading:
      foo                                       noarch                         2.0.0-1                           BaseOS                                   361 k
    blah
`)

	errExit100 := exec.Command("/bin/bash", "-c", "exit 100").Run()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	ctx := context.Background()
	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	packages.SetCommandRunner(mockCommandRunner)
	checkUpdateCall := mockCommandRunner.EXPECT().Run(ctx, utilmocks.EqCmd(exec.Command("/usr/bin/yum", []string{"check-update", "--assumeyes"}...))).Return([]byte("stdout"), []byte("stderr"), errExit100).Times(1)
	// yum install call to install package
	mockCommandRunner.EXPECT().Run(ctx, utilmocks.EqCmd(exec.Command("/usr/bin/yum", []string{"install", "--assumeyes", "foo.noarch"}...))).After(checkUpdateCall).Return([]byte("stdout"), []byte("stderr"), nil).Times(1)

	packages.SetPtyCommandRunner(mockCommandRunner)
	mockCommandRunner.EXPECT().Run(ctx, utilmocks.EqCmd(exec.Command("/usr/bin/yum", []string{"update", "--assumeno", "--color=never", "--security"}...))).Return(data, []byte("stderr"), nil).Times(1)

	err := RunYumUpdate(ctx, YumUpdateMinimal(false), YumUpdateSecurity(true))
	if err != nil {
		t.Errorf("did not expect error: %+v", err)
	}
}

func TestRunYumUpdateWithSecurityWithExclusives(t *testing.T) {
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
	exclusivePackages := []string{"foo", "bar"}

	errExit100 := exec.Command("/bin/bash", "-c", "exit 100").Run()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	packages.SetCommandRunner(mockCommandRunner)
	checkUpdateCall := mockCommandRunner.EXPECT().Run(ctx, utilmocks.EqCmd(exec.Command("/usr/bin/yum", []string{"check-update", "--assumeyes"}...))).Return([]byte("stdout"), []byte("stderr"), errExit100).Times(1)
	// yum install call to install package, make sure only 2 packages are installed.
	mockCommandRunner.EXPECT().Run(ctx, utilmocks.EqCmd(exec.Command("/usr/bin/yum", []string{"install", "--assumeyes", "foo.noarch", "bar.x86_64"}...))).After(checkUpdateCall).Return([]byte("stdout"), []byte("stderr"), nil).Times(1)

	packages.SetPtyCommandRunner(mockCommandRunner)
	mockCommandRunner.EXPECT().Run(ctx, utilmocks.EqCmd(exec.Command("/usr/bin/yum", []string{"update", "--assumeno", "--color=never", "--security"}...))).Return(data, []byte("stderr"), nil).Times(1)

	err := RunYumUpdate(ctx, YumUpdateMinimal(false), YumUpdateSecurity(true), YumExclusivePackages(exclusivePackages))
	if err != nil {
		t.Errorf("did not expect error: %+v", err)
	}
}

// TestRunYumUpdate_DryRun verifies that RunYumUpdate does not install packages when dryrun is true.
func TestRunYumUpdate_DryRun(t *testing.T) {
	data := []byte(`
	=================================================================================================================================================================================
	Package                                      Arch                           Version                                              Repository                                Size
	=================================================================================================================================================================================
	Upgrading:
	  foo                                       noarch                         2.0.0-1                           BaseOS                                   361 k
`)

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	ctx := context.Background()
	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	packages.SetCommandRunner(mockCommandRunner)
	packages.SetPtyCommandRunner(mockCommandRunner)

	errExit100 := exec.Command("/bin/bash", "-c", "exit 100").Run()

	checkUpdateCall := mockCommandRunner.EXPECT().Run(ctx, utilmocks.EqCmd(exec.Command("/usr/bin/yum", []string{"check-update", "--assumeyes"}...))).Return([]byte("stdout"), []byte("stderr"), errExit100).Times(1)
	mockCommandRunner.EXPECT().Run(ctx, utilmocks.EqCmd(exec.Command("/usr/bin/yum", []string{"update", "--assumeno", "--color=never"}...))).After(checkUpdateCall).Return(data, []byte("stderr"), nil).Times(1)

	err := RunYumUpdate(ctx, YumDryRun(true))
	if err != nil {
		t.Errorf("did not expect error: %+v", err)
	}
}

// TestRunYumUpdate_Excludes verifies that packages matched by excludes are filtered out.
func TestRunYumUpdate_Excludes(t *testing.T) {
	data := []byte(`
	=================================================================================================================================================================================
	Package                                      Arch                           Version                                              Repository                                Size
	=================================================================================================================================================================================
	Upgrading:
	  foo                                       noarch                         2.0.0-1                           BaseOS                                   361 k
	  bar                                       x86_64                         2.0.0-1                           BaseOS                                   10 M
`)

	errExit100 := exec.Command("/bin/bash", "-c", "exit 100").Run()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	ctx := context.Background()
	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	packages.SetCommandRunner(mockCommandRunner)
	packages.SetPtyCommandRunner(mockCommandRunner)

	checkUpdateCall := mockCommandRunner.EXPECT().Run(ctx, utilmocks.EqCmd(exec.Command("/usr/bin/yum", []string{"check-update", "--assumeyes"}...))).Return([]byte("stdout"), []byte("stderr"), errExit100).Times(1)
	mockCommandRunner.EXPECT().Run(ctx, utilmocks.EqCmd(exec.Command("/usr/bin/yum", []string{"install", "--assumeyes", "foo.noarch"}...))).After(checkUpdateCall).Return([]byte("stdout"), []byte("stderr"), nil).Times(1)

	packages.SetPtyCommandRunner(mockCommandRunner)
	mockCommandRunner.EXPECT().Run(ctx, utilmocks.EqCmd(exec.Command("/usr/bin/yum", []string{"update", "--assumeno", "--color=never"}...))).Return(data, []byte("stderr"), nil).Times(1)

	excludeStr := "bar"
	err := RunYumUpdate(ctx, YumUpdateExcludes([]*Exclude{CreateStringExclude(&excludeStr)}))
	if err != nil {
		t.Errorf("did not expect error: %+v", err)
	}
}

// TestRunYumUpdate_Errors tests different error scenarios of RunYumUpdate.
func TestRunYumUpdate_Errors(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	t.Cleanup(func() { mockCtrl.Finish() })
	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)

	tests := []struct {
		name    string
		setup   func(t *testing.T, mockRunner *utilmocks.MockCommandRunner)
		opts    []YumUpdateOption
		wantErr error
	}{
		{
			name: "conflicting excludes and exclusives, want excludes error",
			setup: func(t *testing.T, mockRunner *utilmocks.MockCommandRunner) {
				packages.SetCommandRunner(mockRunner)
				packages.SetPtyCommandRunner(mockRunner)

				errExit100 := exec.Command("/bin/bash", "-c", "exit 100").Run()
				checkUpdateCall := mockRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(exec.Command("/usr/bin/yum", []string{"check-update", "--assumeyes"}...))).Return([]byte("stdout"), []byte("stderr"), errExit100).Times(1)
				mockRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(exec.Command("/usr/bin/yum", []string{"update", "--assumeno", "--color=never"}...))).After(checkUpdateCall).Return([]byte("Upgrading:\n foo noarch 2.0.0-1 BaseOS 361 k"), nil, nil).Times(1)
			},
			opts: []YumUpdateOption{
				YumExclusivePackages([]string{"foo"}),
				YumUpdateExcludes([]*Exclude{CreateStringExclude(new(string))}),
			},
			wantErr: errors.New("exclusivePackages and excludes can not both be non 0"),
		},
		{
			name: "no package upgrades available, want nil error",
			setup: func(t *testing.T, mockRunner *utilmocks.MockCommandRunner) {
				packages.SetCommandRunner(mockRunner)
				packages.SetPtyCommandRunner(mockRunner)

				mockRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(exec.Command("/usr/bin/yum", []string{"check-update", "--assumeyes"}...))).Return([]byte("stdout"), []byte("stderr"), nil).Times(1)
			},
			opts:    nil,
			wantErr: nil,
		},
		{
			name: "yum updates query fails, want updates error",
			setup: func(t *testing.T, mockRunner *utilmocks.MockCommandRunner) {
				packages.SetCommandRunner(mockRunner)
				packages.SetPtyCommandRunner(mockRunner)

				mockRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(exec.Command("/usr/bin/yum", []string{"check-update", "--assumeyes"}...))).Return(nil, nil, errors.New("yum updates error")).Times(1)
			},
			opts:    nil,
			wantErr: errors.New("error running /usr/bin/yum with args [\"check-update\" \"--assumeyes\"]: yum updates error, stdout: \"\", stderr: \"\""),
		},
		{
			name: "yum packages installation fails, want install error",
			setup: func(t *testing.T, mockRunner *utilmocks.MockCommandRunner) {
				packages.SetCommandRunner(mockRunner)
				packages.SetPtyCommandRunner(mockRunner)

				errExit100 := exec.Command("/bin/bash", "-c", "exit 100").Run()
				checkUpdateCall := mockRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(exec.Command("/usr/bin/yum", []string{"check-update", "--assumeyes"}...))).Return([]byte("stdout"), []byte("stderr"), errExit100).Times(1)
				updateCall := mockRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(exec.Command("/usr/bin/yum", []string{"update", "--assumeno", "--color=never"}...))).After(checkUpdateCall).Return([]byte("Upgrading:\n foo noarch 2.0.0-1 BaseOS 361 k"), nil, nil).Times(1)
				mockRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(exec.Command("/usr/bin/yum", []string{"install", "--assumeyes", "foo.noarch"}...))).After(updateCall).Return(nil, nil, errors.New("install error")).Times(1)
			},
			opts:    nil,
			wantErr: errors.New("error running /usr/bin/yum with args [\"install\" \"--assumeyes\" \"foo.noarch\"]: install error, stdout: \"\", stderr: \"\""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t, mockCommandRunner)

			gotErr := RunYumUpdate(ctx, tt.opts...)
			utiltest.AssertErrorMatch(t, gotErr, tt.wantErr)
		})
	}
}
