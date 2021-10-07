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
	"os"
	"os/exec"
	"testing"

	"github.com/GoogleCloudPlatform/osconfig/packages"
	utilmocks "github.com/GoogleCloudPlatform/osconfig/util/mocks"
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
	ctx := context.Background()

	if os.Getenv("EXIT100") == "1" {
		os.Exit(100)
	}

	cmd := exec.CommandContext(context.Background(), os.Args[0], "-test.run=TestRunYumUpdateWithSecurity")
	cmd.Env = append(os.Environ(), "EXIT100=1")
	err := cmd.Run()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	packages.SetCommandRunner(mockCommandRunner)
	checkUpdateCall := mockCommandRunner.EXPECT().Run(ctx, exec.CommandContext(context.Background(), "/usr/bin/yum", []string{"check-update", "--assumeyes"}...)).Return([]byte("stdout"), []byte("stderr"), err).Times(1)
	// yum install call to install package
	mockCommandRunner.EXPECT().Run(ctx, exec.CommandContext(context.Background(), "/usr/bin/yum", []string{"install", "--assumeyes", "foo"}...)).After(checkUpdateCall).Return([]byte("stdout"), []byte("stderr"), nil).Times(1)

	packages.SetPtyCommandRunner(mockCommandRunner)
	mockCommandRunner.EXPECT().Run(ctx, exec.CommandContext(context.Background(), "/usr/bin/yum", []string{"update", "--assumeno", "--cacheonly", "--color=never", "--security"}...)).Return(data, []byte("stderr"), nil).Times(1)

	err = RunYumUpdate(ctx, YumUpdateMinimal(false), YumUpdateSecurity(true))
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
	if os.Getenv("EXIT100") == "1" {
		os.Exit(100)
	}

	cmd := exec.CommandContext(context.Background(), os.Args[0], "-test.run=TestRunYumUpdateWithSecurityWithExclusives")
	cmd.Env = append(os.Environ(), "EXIT100=1")
	err := cmd.Run()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	packages.SetCommandRunner(mockCommandRunner)
	checkUpdateCall := mockCommandRunner.EXPECT().Run(ctx, exec.CommandContext(context.Background(), "/usr/bin/yum", []string{"check-update", "--assumeyes"}...)).Return([]byte("stdout"), []byte("stderr"), err).Times(1)
	// yum install call to install package, make sure only 2 packages are installed.
	mockCommandRunner.EXPECT().Run(ctx, exec.CommandContext(context.Background(), "/usr/bin/yum", []string{"install", "--assumeyes", "foo", "bar"}...)).After(checkUpdateCall).Return([]byte("stdout"), []byte("stderr"), nil).Times(1)

	packages.SetPtyCommandRunner(mockCommandRunner)
	mockCommandRunner.EXPECT().Run(ctx, exec.CommandContext(context.Background(), "/usr/bin/yum", []string{"update", "--assumeno", "--cacheonly", "--color=never", "--security"}...)).Return(data, []byte("stderr"), nil).Times(1)

	err = RunYumUpdate(ctx, YumUpdateMinimal(false), YumUpdateSecurity(true), YumExclusivePackages(exclusivePackages))
	if err != nil {
		t.Errorf("did not expect error: %+v", err)
	}
}
