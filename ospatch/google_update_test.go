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
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/GoogleCloudPlatform/osconfig/packages"
	utilmocks "github.com/GoogleCloudPlatform/osconfig/util/mocks"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
	"github.com/golang/mock/gomock"
)

func TestRunGooGetUpdate(t *testing.T) {
	ctx := context.Background()
	googet := filepath.Join(os.Getenv("GooGetRoot"), "googet.exe")

	updatesData := []byte("foo.noarch, 1.0.0@1 --> 2.0.0@1 from repo\nbar.x86_64, 1.0.0@1 --> 2.0.0@1 from repo")
	updatesErr := errors.New("updates error")
	installErr := errors.New("install error")

	excludeName := "bar"

	tests := []struct {
		desc            string
		opts            []GooGetUpdateOption
		updatesOut      []byte
		updatesErr      error
		installOut      []byte
		installErr      error
		expectInstall   bool
		installPkgNames []string
		wantErr         error
	}{
		{
			desc:            "success",
			updatesOut:      updatesData,
			expectInstall:   true,
			installPkgNames: []string{"foo", "bar"},
		},
		{
			desc:       "updates error",
			updatesErr: updatesErr,
			wantErr: fmt.Errorf("error running %s with args %q: %v, stdout: %q, stderr: %q",
				googet, []string{"update"}, updatesErr, "", "stderr"),
		},
		{
			desc:       "no packages to update",
			updatesOut: []byte("nothing here"),
		},
		{
			desc:          "dryrun skips install",
			opts:          []GooGetUpdateOption{GooGetDryRun(true)},
			updatesOut:    updatesData,
			expectInstall: false,
		},
		{
			desc:            "exclusive packages",
			opts:            []GooGetUpdateOption{GooGetExclusivePackages([]string{"foo"})},
			updatesOut:      updatesData,
			expectInstall:   true,
			installPkgNames: []string{"foo"},
		},
		{
			desc:            "excludes",
			opts:            []GooGetUpdateOption{GooGetExcludes([]*Exclude{CreateStringExclude(&excludeName)})},
			updatesOut:      updatesData,
			expectInstall:   true,
			installPkgNames: []string{"foo"},
		},
		{
			desc:            "install error",
			updatesOut:      updatesData,
			expectInstall:   true,
			installPkgNames: []string{"foo", "bar"},
			installErr:      installErr,
			wantErr: fmt.Errorf("error running %s with args %q: %v, stdout: %q, stderr: %q",
				googet, []string{"-noconfirm", "install", "foo", "bar"}, installErr, "", "stderr"),
		},
		{
			desc:       "exclusive and excludes error",
			opts:       []GooGetUpdateOption{GooGetExclusivePackages([]string{"foo"}), GooGetExcludes([]*Exclude{CreateStringExclude(&excludeName)})},
			updatesOut: updatesData,
			wantErr:    errors.New("exclusivePackages and excludes can not both be non 0"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
			packages.SetCommandRunner(mockCommandRunner)

			updatesCmd := utilmocks.EqCmd(exec.Command(googet, "update"))
			updatesCall := mockCommandRunner.EXPECT().
				Run(ctx, updatesCmd).
				Return(tt.updatesOut, []byte("stderr"), tt.updatesErr).
				Times(1)

			if tt.expectInstall {
				installArgs := append([]string{"-noconfirm", "install"}, tt.installPkgNames...)
				installCmd := utilmocks.EqCmd(exec.Command(googet, installArgs...))
				mockCommandRunner.EXPECT().
					Run(ctx, installCmd).
					After(updatesCall).
					Return(tt.installOut, []byte("stderr"), tt.installErr).
					Times(1)
			}

			err := RunGooGetUpdate(ctx, tt.opts...)

			utiltest.AssertErrorMatch(t, err, tt.wantErr)
		})
	}
}
