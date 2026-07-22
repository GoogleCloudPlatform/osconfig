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

package osinfo

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
	"golang.org/x/sys/unix"
)

// TestDefaultProvider_GetOSInfo tests the GetOSInfo method of the defaultProvider.
func TestDefaultProvider_GetOSInfo(t *testing.T) {
	tmpDir := t.TempDir()
	doesNotExist := filepath.Join(tmpDir, "does_not_exist")
	utiltest.OverrideVariable(t, &oracleReleaseFilepath, doesNotExist)
	utiltest.OverrideVariable(t, &redHatReleaseFilepath, doesNotExist)

	tests := []struct {
		name      string
		setupFunc func(t *testing.T)
		wantInfo  OSInfo
		wantErr   error
	}{
		{
			name: "valid release file, want expected OSInfo and nil error",
			setupFunc: func(t *testing.T) {
				debianReleaseFile := filepath.Join(tmpDir, "debian_release_file")
				enforceFileWithContent(t, debianReleaseFile, []byte(debianReleaseFileContent))
				utiltest.OverrideVariable(t, &defaultReleaseFilepath, debianReleaseFile)
			},
			wantInfo: OSInfo{
				ShortName: "debian",
				LongName:  "Debian buster",
				Version:   "10",
			},
			wantErr: nil,
		},
		{
			name: "release file is a directory causing read error, want fallback OSInfo and nil error",
			setupFunc: func(t *testing.T) {
				utiltest.OverrideVariable(t, &defaultReleaseFilepath, tmpDir)
			},
			wantInfo: OSInfo{
				ShortName: "linux",
				LongName:  "",
				Version:   "",
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupFunc(t)
			populateHostFields(t, &tt.wantInfo)
			p := NewProvider()
			gotInfo, gotErr := p.GetOSInfo(context.Background())

			utiltest.AssertErrorMatch(t, gotErr, tt.wantErr)
			utiltest.AssertEquals(t, gotInfo, tt.wantInfo)
		})
	}
}

// populateHostFields fills Hostname, KernelRelease, KernelVersion, and Architecture using uname.
func populateHostFields(t *testing.T, oi *OSInfo) {
	t.Helper()
	var uts unix.Utsname
	if err := unix.Uname(&uts); err != nil {
		t.Fatalf("unable to get unix.Uname, err: %v", err)
	}
	oi.Hostname = stringFromUtsField(uts.Nodename)
	oi.KernelRelease = stringFromUtsField(uts.Release)
	oi.KernelVersion = stringFromUtsField(uts.Version)
	oi.Architecture = NormalizeArchitecture(stringFromUtsField(uts.Machine))
}
