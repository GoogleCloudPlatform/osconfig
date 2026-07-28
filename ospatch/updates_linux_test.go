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
	"syscall"
	"testing"

	"github.com/GoogleCloudPlatform/osconfig/packages"
	"github.com/GoogleCloudPlatform/osconfig/util"
	utilmocks "github.com/GoogleCloudPlatform/osconfig/util/mocks"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
	"github.com/golang/mock/gomock"
)

func createTempFile(t *testing.T) string {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "reboot-required")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })
	return tmpFile.Name()
}

func setAptExists(t *testing.T, exists bool) {
	t.Helper()
	original := packages.AptExists
	packages.AptExists = exists
	t.Cleanup(func() { packages.AptExists = original })
}

func setRebootRequiredFile(t *testing.T, path string) {
	t.Helper()
	original := rebootRequiredFile
	rebootRequiredFile = path
	t.Cleanup(func() { rebootRequiredFile = original })
}

func setRpmquery(t *testing.T, path string) {
	t.Helper()
	original := rpmquery
	rpmquery = path
	t.Cleanup(func() { rpmquery = original })
}

// getExitError returns an *exec.ExitError instance by running a failing command.
func getExitError(t *testing.T) error {
	t.Helper()
	cmd := exec.Command("false")
	err := cmd.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr
	}
	t.Fatalf("failed to get ExitError: %v", err)
	return nil
}

func TestSystemRebootRequiredApt(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		desc       string
		setup      func(t *testing.T)
		wantReboot bool
		wantErr    error
	}{
		{
			desc: "no reboot required when reboot file does not exist",
			setup: func(t *testing.T) {
				setAptExists(t, true)
				setRebootRequiredFile(t, "/non_existing_reboot_file")
			},
			wantReboot: false,
		},
		{
			desc: "reboot required when reboot file exists",
			setup: func(t *testing.T) {
				setAptExists(t, true)
				setRebootRequiredFile(t, createTempFile(t))
			},
			wantReboot: true,
		},
		{
			desc: "file read error is propagated",
			setup: func(t *testing.T) {
				setAptExists(t, true)
				setRebootRequiredFile(t, "/dev/null/invalid")
			},
			wantReboot: false,
			wantErr:    &os.PathError{Op: "open", Path: "/dev/null/invalid", Err: syscall.ENOTDIR},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			tt.setup(t)

			gotReboot, gotErr := SystemRebootRequired(ctx)

			utiltest.AssertEquals(t, gotReboot, tt.wantReboot)
			utiltest.AssertErrorMatch(t, gotErr, tt.wantErr)
		})
	}
}

func TestSystemRebootRequiredRpm(t *testing.T) {
	ctx := context.Background()

	mockCtrl := gomock.NewController(t)
	t.Cleanup(func() { mockCtrl.Finish() })
	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	setAptExists(t, false)

	provides := []string{
		"kernel", "glibc", "gnutls",
		"linux-firmware", "openssl-libs", "dbus",
		"kernel-firmware", "libopenssl1_1", "libopenssl1_0_0", "dbus-1",
	}
	args := append([]string{"--queryformat", "%{INSTALLTIME}\n", "--whatprovides"}, provides...)

	tests := []struct {
		desc             string
		rpmqueryPath     string
		procStatPath     string
		expectedCommands []utiltest.ExpectedCommand
		wantReboot       bool
		wantErr          error
	}{
		{
			desc:         "rpm reboot check succeeds",
			rpmqueryPath: "/dev/null",
			procStatPath: "/proc/stat",
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command("/dev/null", args...),
					Stdout: []byte("1000\n"),
				},
			},
			wantReboot: false,
		},
		{
			desc:         "unsupported package manager returns error",
			rpmqueryPath: "/non_existing_file",
			procStatPath: "/proc/stat",
			wantErr:      errors.New("no recognized package manager installed, can't determine if reboot is required"),
		},
		{
			desc:         "rpmquery returns exit error, want no reboot and nil error",
			rpmqueryPath: "/dev/null",
			procStatPath: "/proc/stat",
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command("/dev/null", args...),
					Stdout: []byte("1000\n"),
					Err:    getExitError(t),
				},
			},
			wantReboot: false,
			wantErr:    nil,
		},
		{
			desc:         "rpmquery returns runner error, want running error",
			rpmqueryPath: "/dev/null",
			procStatPath: "/proc/stat",
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd: exec.Command("/dev/null", args...),
					Err: errors.New("runner error"),
				},
			},
			wantErr: errors.New("error running /dev/null: runner error"),
		},
		{
			desc:         "proc stat file is missing, want no file error",
			rpmqueryPath: "/dev/null",
			procStatPath: "/nonexistent/stat",
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command("/dev/null", args...),
					Stdout: []byte("1000\n"),
				},
			},
			wantErr: errors.New("error opening /nonexistent/stat: open /nonexistent/stat: no such file or directory"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			setRpmquery(t, tt.rpmqueryPath)
			utiltest.OverrideVariable(t, &procStatPath, tt.procStatPath)
			utiltest.OverrideVariable[util.CommandRunner](t, &runner, mockCommandRunner)
			utiltest.SetExpectedCommands(ctx, mockCommandRunner, tt.expectedCommands)
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
