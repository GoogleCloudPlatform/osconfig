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

package packages

import (
	"os"
	"os/exec"
	"testing"

	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
	"golang.org/x/sys/unix"
)

// CheckPtmxAvailability skips the test if /dev/ptmx is not available
func CheckPtmxAvailability(t *testing.T) {
	if _, err := os.Stat("/dev/ptmx"); os.IsNotExist(err) {
		t.Skip("/dev/ptmx not found, skipping PTY tests")
	}
}

// TestIoctlError ensures that the ioctl function correctly handles and returns errors
func TestIoctlError(t *testing.T) {
	// Use an invalid file descriptor to trigger an error in ioctl
	err := ioctl(^uintptr(0), unix.TIOCSPTLCK, 0)
	if err == nil {
		t.Error("ioctl() expected error with invalid fd, got nil")
	}
}

// TestRunWithPty tests the runWithPty function with various cases:
// successful command execution, commands returning non-zero exit codes, handling execution errors like missing binaries
func TestRunWithPty(t *testing.T) {
	CheckPtmxAvailability(t)

	tests := []struct {
		name       string
		cmd        *exec.Cmd
		wantOut    string
		wantStderr string
		wantErr    error
	}{
		{
			name:    "command with exit code 0 returns nil",
			cmd:     exec.Command("echo"),
			wantOut: "",
			wantErr: nil,
		},
		{
			name:    "command with exit code 1 returns output",
			cmd:     exec.Command("sh", "-c", "echo 'updates found'; exit 1"),
			wantOut: "updates found\r\n",
			wantErr: nil,
		},
		{
			name:       "command with stderr and exit code 1",
			cmd:        exec.Command("sh", "-c", "echo 'stdout message'; echo 'stderr message' >&2; exit 1"),
			wantOut:    "stdout message\r\n",
			wantStderr: "stderr message\n",
			wantErr:    nil,
		},
		{
			name:    "command with multiple lines of output",
			cmd:     exec.Command("sh", "-c", "printf 'line1\nline2\nline3\n'; exit 1"),
			wantOut: "line1\r\nline2\r\nline3\r\n",
			wantErr: nil,
		},
		{
			name:    "non existent command returns error",
			cmd:     exec.Command("this-command-does-not-exist"),
			wantOut: "",
			wantErr: &exec.Error{Name: "this-command-does-not-exist", Err: exec.ErrNotFound},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := runWithPty(tt.cmd)
			utiltest.AssertErrorMatch(t, err, tt.wantErr)
			utiltest.AssertEquals(t, string(stdout), tt.wantOut)
			utiltest.AssertEquals(t, string(stderr), tt.wantStderr)
		})
	}
}
