//  Copyright 2026 Google Inc. All Rights Reserved.
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
	"os"
	"os/exec"
	"testing"

	"github.com/GoogleCloudPlatform/osconfig/packages"
	utilmocks "github.com/GoogleCloudPlatform/osconfig/util/mocks"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
	"github.com/golang/mock/gomock"
)

// TestRunAptGetUpgrade verifies the functionality and error handling of RunAptGetUpgrade function
// apt behavior is mocked with gomock
func TestRunAptGetUpgrade(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	oldRunner := packages.GetCommandRunner()
	defer packages.SetCommandRunner(oldRunner)

	ctx := context.Background()
	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	packages.SetCommandRunner(mockCommandRunner)

	aptGet := "/usr/bin/apt-get"
	env := append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")

	pkg1, pkg2 := "pkg1", "pkg2"
	stdout, empty := []byte("stdout"), []byte("")
	pkg1out := []byte("Inst pkg1 [1.0] (1.1 stable [amd64])")
	pkg12out := []byte("Inst pkg1 [1.0] (1.1 stable [amd64])\nInst pkg2 [1.0] (1.1 stable [amd64])")

	updateCmd := buildCmd(aptGet, env, "update")
	upgradeCmd := buildCmd(aptGet, env, "--just-print", "-qq", "upgrade")
	distUpgradeCmd := buildCmd(aptGet, env, "--just-print", "-qq", "dist-upgrade")
	fullUpgradeCmd := buildCmd(aptGet, env, "--just-print", "-qq", "full-upgrade")
	installPkg1Cmd := buildCmd(aptGet, env, "install", "-y", "pkg1")
	installPkg2Cmd := buildCmd(aptGet, env, "install", "-y", "pkg2")

	tests := []struct {
		name    string
		opts    []AptGetUpgradeOption
		mock    func()
		wantErr error
	}{
		{
			name: "successful apt install with default options",
			opts: nil,
			mock: func() {
				gomock.InOrder(
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(updateCmd)).Return(stdout, empty, nil),
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(upgradeCmd)).Return(pkg1out, empty, nil),
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(installPkg1Cmd)).Return(stdout, empty, nil),
				)
			},
			wantErr: nil,
		},
		{
			name: "dry run mode, only check for updates",
			opts: []AptGetUpgradeOption{AptGetDryRun(true)},
			mock: func() {
        gomock.InOrder(
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(updateCmd)).Return(stdout, empty, nil),
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(upgradeCmd)).Return(pkg1out, empty, nil),
				)
			},
			wantErr: nil,
		},
		{
			name: "apt upgrade with package excludes should skip excluded packages",
			opts: []AptGetUpgradeOption{
				AptGetExcludes([]*Exclude{CreateStringExclude(&pkg1)}),
			},
			mock: func() {
				gomock.InOrder(
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(updateCmd)).Return(stdout, empty, nil),
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(upgradeCmd)).Return(pkg12out, empty, nil),
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(installPkg2Cmd)).Return(stdout, empty, nil),
				)
			},
			wantErr: nil,
		},
		{
			name: "apt upgrade with exclusive packages should only install specified packages",
			opts: []AptGetUpgradeOption{
				AptGetExclusivePackages([]string{pkg2}),
			},
			mock: func() {
				gomock.InOrder(
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(updateCmd)).Return(stdout, empty, nil),
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(upgradeCmd)).Return(pkg12out, empty, nil),
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(installPkg2Cmd)).Return(stdout, empty, nil),
				)
			},
			wantErr: nil,
		},
		{
			name: "dist-upgrade should be used when specified",
			opts: []AptGetUpgradeOption{
				AptGetUpgradeType(packages.AptGetDistUpgrade),
			},
			mock: func() {
				gomock.InOrder(
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(updateCmd)).Return(stdout, empty, nil),
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(distUpgradeCmd)).Return(pkg1out, empty, nil),
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(installPkg1Cmd)).Return(stdout, empty, nil),
				)
			},
			wantErr: nil,
		},
		{
			name: "full-upgrade should be used when specified",
			opts: []AptGetUpgradeOption{
				AptGetUpgradeType(packages.AptGetFullUpgrade),
			},
			mock: func() {
				gomock.InOrder(
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(updateCmd)).Return(stdout, empty, nil),
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(fullUpgradeCmd)).Return(pkg1out, empty, nil),
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(installPkg1Cmd)).Return(stdout, empty, nil),
				)
			},
			wantErr: nil,
		},
		{
			name: "no packages to update should result in no install command",
			opts: nil,
			mock: func() {
				gomock.InOrder(
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(updateCmd)).Return(stdout, empty, nil),
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(upgradeCmd)).Return(empty, empty, nil),
				)
			},
			wantErr: nil,
		},
		{
			name: "notify about update error",
			opts: nil,
			mock: func() {
				mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(updateCmd)).Return(empty, []byte("error"), errors.New("update fail"))
			},
			wantErr: errors.New("update fail"),
		},
		{
			name: "notify about install error with detailed output",
			opts: nil,
			mock: func() {
				gomock.InOrder(
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(updateCmd)).Return(stdout, empty, nil),
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(upgradeCmd)).Return(pkg1out, empty, nil),
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(installPkg1Cmd)).Return(empty, []byte("install error"), errors.New("install fail")),
				)
			},
			wantErr: errors.New("error running /usr/bin/apt-get with args [\"install\" \"-y\" \"pkg1\"]: install fail, stdout: \"\", stderr: \"install error\""),
		},
		{
			name: "providing both exclusive packages and excludes should return an error",
			opts: []AptGetUpgradeOption{
				AptGetExclusivePackages([]string{pkg1}),
				AptGetExcludes([]*Exclude{CreateStringExclude(&pkg1)}),
			},
			mock: func() {
				gomock.InOrder(
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(updateCmd)).Return(stdout, empty, nil),
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(upgradeCmd)).Return(pkg1out, empty, nil),
				)
			},
			wantErr: errors.New("exclusivePackages and excludes can not both be non 0"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mock()
			err := RunAptGetUpgrade(ctx, tt.opts...)
			utiltest.AssertErrorMatch(t, err, tt.wantErr)
		})
	}
}

// buildCmd builds an exec.Cmd with the given path, environment, and arguments.
func buildCmd(path string, env []string, args ...string) *exec.Cmd {
	cmd := exec.Command(path, args...)
	cmd.Env = env
	return cmd
}
