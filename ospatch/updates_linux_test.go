//  Copyright 2024 Google Inc. All Rights Reserved.
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
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
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

func mockRPM(t *testing.T, exists, rebootResult bool, rebootErr error) {
	t.Helper()

	originalRpmQuery := rpmquery
	originalRpmReboot := rpmReboot
	t.Cleanup(func() {
		rpmquery = originalRpmQuery
		rpmReboot = originalRpmReboot
	})

	if !exists {
		rpmquery = "/non_existing_file"
		return
	}

	rpmReboot = func() (bool, error) { return rebootResult, rebootErr }

	tmpFile, err := os.CreateTemp("", "rpmquery")
	if err != nil {
		t.Fatalf("Failed to create temp file for rpmquery mock: %v", err)
	}
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })
	rpmquery = tmpFile.Name()
}

func TestSystemRebootRequired(t *testing.T) {
	ctx := context.Background()
	errRpmReboot := errors.New("mock rpm reboot error")

	tests := []struct {
		desc            string
		aptExists       bool
		aptFileExists   bool
		aptFileReadErr  bool
		rpmExists       bool
		rpmRebootResult bool
		rpmRebootError  error
		wantReboot      bool
		wantErr         error
	}{
		{
			desc:       "APT system with no reboot required",
			aptExists:  true,
			wantReboot: false,
		},
		{
			desc:          "APT system with reboot required",
			aptExists:     true,
			aptFileExists: true,
			wantReboot:    true,
		},
		{
			desc:           "APT system with file read error",
			aptExists:      true,
			aptFileExists:  true,
			aptFileReadErr: true,
			wantReboot:     false,
			wantErr:        &os.PathError{Op: "open", Path: "/dev/null/invalid", Err: syscall.ENOTDIR},
		},
		{
			desc:            "RPM system with no reboot required",
			rpmExists:       true,
			rpmRebootResult: false,
			wantReboot:      false,
		},
		{
			desc:            "RPM system with reboot required",
			rpmExists:       true,
			rpmRebootResult: true,
			wantReboot:      true,
		},
		{
			desc:           "RPM system with evaluation error",
			rpmExists:      true,
			rpmRebootError: errRpmReboot,
			wantReboot:     false,
			wantErr:        errRpmReboot,
		},
		{
			desc:    "Unsupported package manager",
			wantErr: errors.New("no recognized package manager installed, can't determine if reboot is required"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			mockAPT(t, tt.aptExists, tt.aptFileExists, tt.aptFileReadErr)
			mockRPM(t, tt.rpmExists, tt.rpmRebootResult, tt.rpmRebootError)

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
