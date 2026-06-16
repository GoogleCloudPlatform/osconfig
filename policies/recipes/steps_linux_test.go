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

//go:build linux
// +build linux

package recipes

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
	"golang.org/x/sys/unix"
)

// Test_mkCharDevice tests the mkCharDevice function.
func Test_mkCharDevice(t *testing.T) {
	tmpDir, alreadyExistsPath := setupPrepareTempFile(t, "already_exists")
	var major, minor uint32 = 1, 3

	tests := []struct {
		name    string
		path    string
		wantErr error
	}{
		{
			name:    "valid path, want nil error",
			path:    filepath.Join(tmpDir, "char_dev"),
			wantErr: nil,
		},
		{
			name:    "invalid path, want ENOENT error",
			path:    filepath.Join(tmpDir, "non_existent_dir", "char_dev"),
			wantErr: unix.ENOENT,
		},
		{
			name:    "path already exists, want EEXIST error",
			path:    alreadyExistsPath,
			wantErr: unix.EEXIST,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotErr := mkCharDevice(tt.path, major, minor)

			utiltest.AssertErrorMatchAndSkip(t, gotErr, tt.wantErr)
			checkOsStat(t, tt.path)
		})
	}
}

// Test_mkBlockDevice tests the mkBlockDevice function.
func Test_mkBlockDevice(t *testing.T) {
	tmpDir, alreadyExistsPath := setupPrepareTempFile(t, "already_exists_block")
	var major, minor uint32 = 1, 3

	tests := []struct {
		name    string
		path    string
		wantErr error
	}{
		{
			name:    "valid path, want nil error",
			path:    filepath.Join(tmpDir, "block_dev"),
			wantErr: nil,
		},
		{
			name:    "invalid path, want ENOENT error",
			path:    filepath.Join(tmpDir, "non_existent_dir", "block_dev"),
			wantErr: unix.ENOENT,
		},
		{
			name:    "path already exists, want EEXIST error",
			path:    alreadyExistsPath,
			wantErr: unix.EEXIST,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotErr := mkBlockDevice(tt.path, major, minor)

			utiltest.AssertErrorMatchAndSkip(t, gotErr, tt.wantErr)
			checkOsStat(t, tt.path)
		})
	}
}

// Test_mkFifo tests the mkFifo function.
func Test_mkFifo(t *testing.T) {
	tmpDir, alreadyExistsPath := setupPrepareTempFile(t, "already_exists_fifo")
	var mode uint32 = 0666

	tests := []struct {
		name    string
		path    string
		wantErr error
	}{
		{
			name:    "valid path, want nil error",
			path:    filepath.Join(tmpDir, "test_fifo"),
			wantErr: nil,
		},
		{
			name:    "invalid path, want ENOENT error",
			path:    filepath.Join(tmpDir, "non_existent_dir", "test_fifo"),
			wantErr: unix.ENOENT,
		},
		{
			name:    "path already exists, want EEXIST error",
			path:    alreadyExistsPath,
			wantErr: unix.EEXIST,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotErr := mkFifo(tt.path, mode)

			utiltest.AssertErrorMatchAndSkip(t, gotErr, tt.wantErr)
			checkOsStat(t, tt.path)
		})
	}
}

// checkOsStat is a helper function that checks if a file was created.
func checkOsStat(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("failed to stat: %v", err)
	}
}

// setupPrepareTempFile writes content to a temporary file and schedules its removal.
func setupPrepareTempFile(t *testing.T, name string) (string, string) {
	t.Helper()
	tmpDir := t.TempDir()
	path := utiltest.WriteToTempFileMust(t, name, []byte("test"))
	t.Cleanup(func() { os.Remove(path) })
	return tmpDir, path
}
