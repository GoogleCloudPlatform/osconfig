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
	fakeOutput, empty := []byte("stdout"), []byte("")
	pkg1out := []byte("Inst pkg1 [1.0] (1.1 stable [amd64])")
	pkg12out := []byte("Inst pkg1 [1.0] (1.1 stable [amd64])\nInst pkg2 [1.0] (1.1 stable [amd64])")

	tests := []struct {
		name    string
		opts    []AptGetUpgradeOption
		mock    func()
		wantErr error
	}{
		{
			name: "default apt options, want nil",
			opts: nil,
			mock: func() {
				gomock.InOrder(
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(buildCmd(aptGet, env, "update"))).Return(fakeOutput, empty, nil),
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(buildCmd(aptGet, env, "--just-print", "-qq", "upgrade"))).Return(pkg1out, empty, nil),
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(buildCmd(aptGet, env, "install", "-y", "pkg1"))).Return(fakeOutput, empty, nil),
				)
			},
			wantErr: nil,
		},
		{
			name: "dry run option, want nil",
			opts: []AptGetUpgradeOption{AptGetDryRun(true)},
			mock: func() {
				gomock.InOrder(
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(buildCmd(aptGet, env, "update"))).Return(fakeOutput, empty, nil),
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(buildCmd(aptGet, env, "--just-print", "-qq", "upgrade"))).Return(pkg1out, empty, nil),
				)
			},
			wantErr: nil,
		},
		{
			name: "package excludes, want nil",
			opts: []AptGetUpgradeOption{
				AptGetExcludes([]*Exclude{CreateStringExclude(&pkg1)}),
			},
			mock: func() {
				gomock.InOrder(
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(buildCmd(aptGet, env, "update"))).Return(fakeOutput, empty, nil),
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(buildCmd(aptGet, env, "--just-print", "-qq", "upgrade"))).Return(pkg12out, empty, nil),
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(buildCmd(aptGet, env, "install", "-y", "pkg2"))).Return(fakeOutput, empty, nil),
				)
			},
			wantErr: nil,
		},
		{
			name: "exclusive packages, want nil",
			opts: []AptGetUpgradeOption{
				AptGetExclusivePackages([]string{pkg2}),
			},
			mock: func() {
				gomock.InOrder(
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(buildCmd(aptGet, env, "update"))).Return(fakeOutput, empty, nil),
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(buildCmd(aptGet, env, "--just-print", "-qq", "upgrade"))).Return(pkg12out, empty, nil),
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(buildCmd(aptGet, env, "install", "-y", "pkg2"))).Return(fakeOutput, empty, nil),
				)
			},
			wantErr: nil,
		},
		{
			name: "dist-upgrade type, want nil",
			opts: []AptGetUpgradeOption{
				AptGetUpgradeType(packages.AptGetDistUpgrade),
			},
			mock: func() {
				gomock.InOrder(
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(buildCmd(aptGet, env, "update"))).Return(fakeOutput, empty, nil),
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(buildCmd(aptGet, env, "--just-print", "-qq", "dist-upgrade"))).Return(pkg1out, empty, nil),
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(buildCmd(aptGet, env, "install", "-y", "pkg1"))).Return(fakeOutput, empty, nil),
				)
			},
			wantErr: nil,
		},
		{
			name: "full-upgrade type, want nil",
			opts: []AptGetUpgradeOption{
				AptGetUpgradeType(packages.AptGetFullUpgrade),
			},
			mock: func() {
				gomock.InOrder(
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(buildCmd(aptGet, env, "update"))).Return(fakeOutput, empty, nil),
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(buildCmd(aptGet, env, "--just-print", "-qq", "full-upgrade"))).Return(pkg1out, empty, nil),
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(buildCmd(aptGet, env, "install", "-y", "pkg1"))).Return(fakeOutput, empty, nil),
				)
			},
			wantErr: nil,
		},
		{
			name: "no packages to update, want nil",
			opts: nil,
			mock: func() {
				gomock.InOrder(
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(buildCmd(aptGet, env, "update"))).Return(fakeOutput, empty, nil),
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(buildCmd(aptGet, env, "--just-print", "-qq", "upgrade"))).Return(empty, empty, nil),
				)
			},
			wantErr: nil,
		},
		{
			name: "update with failure, want update fail error",
			opts: nil,
			mock: func() {
				mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(buildCmd(aptGet, env, "update"))).Return(empty, []byte("error"), errors.New("update fail"))
			},
			wantErr: errors.New("update fail"),
		},
		{
			name: "install with failure, want install fail",
			opts: nil,
			mock: func() {
				gomock.InOrder(
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(buildCmd(aptGet, env, "update"))).Return(fakeOutput, empty, nil),
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(buildCmd(aptGet, env, "--just-print", "-qq", "upgrade"))).Return(pkg1out, empty, nil),
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(buildCmd(aptGet, env, "install", "-y", "pkg1"))).Return(empty, []byte("install error"), errors.New("install fail")),
				)
			},
			wantErr: errors.New("error running /usr/bin/apt-get with args [\"install\" \"-y\" \"pkg1\"]: install fail, stdout: \"\", stderr: \"install error\""),
		},
		{
			name: "both exclusive packages and excludes, want 'exclusivePackages and excludes can not both be non 0' error",
			opts: []AptGetUpgradeOption{
				AptGetExclusivePackages([]string{pkg1}),
				AptGetExcludes([]*Exclude{CreateStringExclude(&pkg1)}),
			},
			mock: func() {
				gomock.InOrder(
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(buildCmd(aptGet, env, "update"))).Return(fakeOutput, empty, nil),
					mockCommandRunner.EXPECT().Run(gomock.Any(), utilmocks.EqCmd(buildCmd(aptGet, env, "--just-print", "-qq", "upgrade"))).Return(pkg1out, empty, nil),
				)
			},
			wantErr: errors.New("exclusivePackages and excludes can not both be non 0"),
    },:G
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
