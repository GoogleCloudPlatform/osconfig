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
	"syscall"
	"testing"

	"github.com/GoogleCloudPlatform/osconfig/packages"
	utilmocks "github.com/GoogleCloudPlatform/osconfig/util/mocks"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
	"github.com/golang/mock/gomock"
)

func mockAPT(t *testing.T, exists, fileExists, fileReadErr bool) {
	t.Helper()

	originalAptExists := packages.AptExists
	originalRebootFile := rebootRequiredFile
	t.Cleanup(func() {
		packages.AptExists = originalAptExists
		rebootRequiredFile = originalRebootFile
	})

	packages.AptExists = exists
	if !exists {
		return
	}
	if !fileExists {
		rebootRequiredFile = "/non_existing_reboot_file"
		return
	}
	if fileReadErr {
		rebootRequiredFile = "/dev/null/invalid"
		return
	}

	tmpFile, err := os.CreateTemp("", "reboot-required")
	if err != nil {
		t.Fatalf("Failed to create temp file for reboot-required mock: %v", err)
	}
	tmpFile.Close()
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })

	rebootRequiredFile = tmpFile.Name()
}

func mockRPM(t *testing.T, ctx context.Context, exists bool) {
	t.Helper()

	originalRpmQuery := rpmquery
	originalProcStatPath := procStatPath
	originalRunner := runner
	t.Cleanup(func() {
		rpmquery = originalRpmQuery
		procStatPath = originalProcStatPath
		runner = originalRunner
	})

	if !exists {
		rpmquery = "/non_existing_file"
		return
	}

	tmpFile, err := os.CreateTemp("", "rpmquery")
	if err != nil {
		t.Fatalf("Failed to create temp file for rpmquery mock: %v", err)
	}
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })
	rpmquery = tmpFile.Name()

	mockCtrl := gomock.NewController(t)
	t.Cleanup(func() { mockCtrl.Finish() })
	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	mockCommandRunner.EXPECT().Run(ctx, gomock.Any()).Return([]byte("1000\n"), nil, nil).Times(1)
}

func TestSystemRebootRequiredApt(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		desc           string
		aptFileExists  bool
		aptFileReadErr bool
		wantReboot     bool
		wantErr        error
	}{
		{
			desc:       "no reboot required",
			wantReboot: false,
		},
		{
			desc:          "reboot required",
			aptFileExists: true,
			wantReboot:    true,
		},
		{
			desc:           "file read error",
			aptFileExists:  true,
			aptFileReadErr: true,
			wantReboot:     false,
			wantErr:        &os.PathError{Op: "open", Path: "/dev/null/invalid", Err: syscall.ENOTDIR},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			mockAPT(t, true, tt.aptFileExists, tt.aptFileReadErr)
			mockRPM(t, ctx, false)

			gotReboot, gotErr := SystemRebootRequired(ctx)

			utiltest.AssertEquals(t, gotReboot, tt.wantReboot)
			utiltest.AssertErrorMatch(t, gotErr, tt.wantErr)
		})
	}
}

func TestSystemRebootRequiredRpm(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		desc       string
		rpmExists  bool
		wantReboot bool
		wantErr    error
	}{
		{
			desc:       "reboot check",
			rpmExists:  true,
			wantReboot: false,
		},
		{
			desc:    "unsupported package manager",
			wantErr: errors.New("no recognized package manager installed, can't determine if reboot is required"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			mockAPT(t, false, false, false)
			mockRPM(t, ctx, tt.rpmExists)

			gotReboot, gotErr := SystemRebootRequired(ctx)

			utiltest.AssertEquals(t, gotReboot, tt.wantReboot)
			utiltest.AssertErrorMatch(t, gotErr, tt.wantErr)
		})
	}
}

func TestInstallWUAUpdates(t *testing.T) {
	if err := InstallWUAUpdates(context.Background()); err != nil {
		t.Errorf("InstallWUAUpdates() on linux stub should not return an error, got: %v", err)
	}
}
