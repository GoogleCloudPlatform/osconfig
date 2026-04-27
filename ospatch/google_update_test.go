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
	t.Cleanup(func() { mockCtrl.Finish() })

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	packages.SetCommandRunner(mockCommandRunner)

	tests := []struct {
		name             string
		opts             []GooGetUpdateOption
		expectedCommands []utiltest.ExpectedCommand
		wantErr          error
	}{
		{
			name: "available updates for foo and bar, installs both packages",
			expectedCommands: []utiltest.ExpectedCommand{
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
			name: "googet update returns an error, propagates the update error",
			expectedCommands: []utiltest.ExpectedCommand{
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
			name: "googet update returns no package updates, skips install command",
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command(googet, "update"),
					Stdout: []byte("nothing here"),
					Stderr: []byte("stderr"),
				},
			},
		},
		{
			name: "dry run is enabled with available updates, skips install command",
			opts: []GooGetUpdateOption{GooGetDryRun(true)},
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command(googet, "update"),
					Stdout: updatesData,
					Stderr: []byte("stderr"),
				},
			},
		},
		{
			name: "exclusive packages contains only foo, installs only foo",
			opts: []GooGetUpdateOption{GooGetExclusivePackages([]string{"foo"})},
			expectedCommands: []utiltest.ExpectedCommand{
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
			name: "excludes contains bar, installs only foo",
			opts: []GooGetUpdateOption{GooGetExcludes([]*Exclude{CreateStringExclude(&excludeName)})},
			expectedCommands: []utiltest.ExpectedCommand{
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
			name: "install command returns an error, propagates the install error",
			expectedCommands: []utiltest.ExpectedCommand{
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
			name: "exclusive packages and excludes are both set, returns a validation error",
			opts: []GooGetUpdateOption{GooGetExclusivePackages([]string{"foo"}), GooGetExcludes([]*Exclude{CreateStringExclude(&excludeName)})},
			expectedCommands: []utiltest.ExpectedCommand{
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
		t.Run(tt.name, func(t *testing.T) {
			utiltest.SetExpectedCommands(ctx, mockCommandRunner, tt.expectedCommands)

			err := RunGooGetUpdate(ctx, tt.opts...)

			utiltest.AssertErrorMatch(t, err, tt.wantErr)
		})
	}
}
