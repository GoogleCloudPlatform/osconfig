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
	"fmt"
	"os/exec"
	"testing"

	"github.com/GoogleCloudPlatform/osconfig/packages"
	utilmocks "github.com/GoogleCloudPlatform/osconfig/util/mocks"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
	"github.com/golang/mock/gomock"
)

func TestRunGooGetUpdate(t *testing.T) {
	ctx := context.Background()
	googet := "googet.exe"

	updatesData := []byte("foo.noarch, 1.0.0@1 --> 2.0.0@1 from repo\nbar.x86_64, 1.0.0@1 --> 2.0.0@1 from repo")
	updatesErr := errors.New("updates error")
	installErr := errors.New("install error")

	excludeName := "bar"
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	packages.SetCommandRunner(mockCommandRunner)

	tests := []struct {
		desc     string
		opts     []GooGetUpdateOption
		commands []utiltest.ExpectedCommand
		wantErr  error
	}{
		{
			desc: "googet update lists foo and bar then install succeeds",
			commands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command(googet, "update"),
					Stdout: updatesData,
					Stderr: []byte("stderr"),
				},
				{
					Cmd:    exec.Command(googet, "-noconfirm", "install", "foo", "bar"),
					Stderr: []byte("stderr"),
				},
			},
		},
		{
			desc: "googet update error is propagated",
			commands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command(googet, "update"),
					Stderr: []byte("stderr"),
					Err:    updatesErr,
				},
			},
			wantErr: fmt.Errorf("error running %s with args %q: %v, stdout: %q, stderr: %q",
				googet, []string{"update"}, updatesErr, "", "stderr"),
		},
		{
			desc: "no packages to update results in no install",
			commands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command(googet, "update"),
					Stdout: []byte("nothing here"),
					Stderr: []byte("stderr"),
				},
			},
		},
		{
			desc: "dryrun skips install command",
			opts: []GooGetUpdateOption{GooGetDryRun(true)},
			commands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command(googet, "update"),
					Stdout: updatesData,
					Stderr: []byte("stderr"),
				},
			},
		},
		{
			desc: "exclusive packages filters install to only foo",
			opts: []GooGetUpdateOption{GooGetExclusivePackages([]string{"foo"})},
			commands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command(googet, "update"),
					Stdout: updatesData,
					Stderr: []byte("stderr"),
				},
				{
					Cmd:    exec.Command(googet, "-noconfirm", "install", "foo"),
					Stderr: []byte("stderr"),
				},
			},
		},
		{
			desc: "excludes filters out bar from install",
			opts: []GooGetUpdateOption{GooGetExcludes([]*Exclude{CreateStringExclude(&excludeName)})},
			commands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command(googet, "update"),
					Stdout: updatesData,
					Stderr: []byte("stderr"),
				},
				{
					Cmd:    exec.Command(googet, "-noconfirm", "install", "foo"),
					Stderr: []byte("stderr"),
				},
			},
		},
		{
			desc: "install error is propagated",
			commands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command(googet, "update"),
					Stdout: updatesData,
					Stderr: []byte("stderr"),
				},
				{
					Cmd:    exec.Command(googet, "-noconfirm", "install", "foo", "bar"),
					Stderr: []byte("stderr"),
					Err:    installErr,
				},
			},
			wantErr: fmt.Errorf("error running %s with args %q: %v, stdout: %q, stderr: %q",
				googet, []string{"-noconfirm", "install", "foo", "bar"}, installErr, "", "stderr"),
		},
		{
			desc: "exclusive and excludes both set returns error without running commands",
			opts: []GooGetUpdateOption{GooGetExclusivePackages([]string{"foo"}), GooGetExcludes([]*Exclude{CreateStringExclude(&excludeName)})},
			commands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command(googet, "update"),
					Stdout: updatesData,
					Stderr: []byte("stderr"),
				},
			},
			wantErr: errors.New("exclusivePackages and excludes can not both be non 0"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			var prev *gomock.Call
			for _, ec := range tt.commands {
				cmd := utilmocks.EqCmd(ec.Cmd)
				call := mockCommandRunner.EXPECT().
					Run(ctx, cmd).
					Return(ec.Stdout, ec.Stderr, ec.Err).
					Times(1)
				if prev != nil {
					call.After(prev)
				}
				prev = call
			}

			err := RunGooGetUpdate(ctx, tt.opts...)

			utiltest.AssertErrorMatch(t, err, tt.wantErr)
		})
	}
}