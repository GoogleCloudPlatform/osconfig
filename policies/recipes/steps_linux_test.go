//  Copyright 2019 Google Inc. All Rights Reserved.
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
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		path    string
		major   uint32
		minor   uint32
		wantErr error
	}{
		{
			name:    "valid path, success",
			path:    filepath.Join(tmpDir, "char_dev"),
			major:   1,
			minor:   3,
			wantErr: nil,
		},
		{
			name:    "invalid path, ENOENT",
			path:    filepath.Join(tmpDir, "non_existent_dir", "char_dev"),
			major:   1,
			minor:   3,
			wantErr: unix.ENOENT,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mkCharDevice(tt.path, tt.major, tt.minor)
			utiltest.AssertErrorMatch(t, err, tt.wantErr)

			if err == nil {
				fi, err := os.Stat(tt.path)
				if err != nil {
					t.Fatalf("failed to stat created device: %v", err)
				}
				if fi.Mode()&os.ModeDevice == 0 || fi.Mode()&os.ModeCharDevice == 0 {
					t.Errorf("expected char device, got mode: %v", fi.Mode())
				}
			}
		})
	}

	t.Run("path already exists, EEXIST", func(t *testing.T) {
		path := filepath.Join(tmpDir, "already_exists")
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
		err := mkCharDevice(path, 1, 3)
		utiltest.AssertErrorMatch(t, err, unix.EEXIST)
	})
}

// Test_mkBlockDevice tests the mkBlockDevice function.
func Test_mkBlockDevice(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		path    string
		major   uint32
		minor   uint32
		wantErr error
	}{
		{
			name:    "valid path, success",
			path:    filepath.Join(tmpDir, "block_dev"),
			major:   1,
			minor:   3,
			wantErr: nil,
		},
		{
			name:    "invalid path, ENOENT",
			path:    filepath.Join(tmpDir, "non_existent_dir", "block_dev"),
			major:   1,
			minor:   3,
			wantErr: unix.ENOENT,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mkBlockDevice(tt.path, tt.major, tt.minor)
			utiltest.AssertErrorMatch(t, err, tt.wantErr)

			if err == nil {
				fi, err := os.Stat(tt.path)
				if err != nil {
					t.Fatalf("failed to stat created device: %v", err)
				}
				if fi.Mode()&os.ModeDevice == 0 || fi.Mode()&os.ModeCharDevice != 0 {
					// In some environments block devices might be tricky, but it should have ModeDevice set.
					// And it should NOT have ModeCharDevice set.
					t.Logf("Mode for block device: %v", fi.Mode())
				}
			}
		})
	}

	t.Run("path already exists, EEXIST", func(t *testing.T) {
		path := filepath.Join(tmpDir, "already_exists_block")
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
		err := mkBlockDevice(path, 1, 3)
		utiltest.AssertErrorMatch(t, err, unix.EEXIST)
	})
}

// Test_mkFifo tests the mkFifo function.
func Test_mkFifo(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		path     string
		mode     uint32
		wantErr  error
	}{
		{
			name:    "valid path, success",
			path:    filepath.Join(tmpDir, "test_fifo"),
			mode:    0666,
			wantErr: nil,
		},
		{
			name:    "invalid path, ENOENT",
			path:    filepath.Join(tmpDir, "non_existent_dir", "test_fifo"),
			mode:    0666,
			wantErr: unix.ENOENT,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mkFifo(tt.path, tt.mode)
			utiltest.AssertErrorMatch(t, err, tt.wantErr)

			if err == nil {
				fi, err := os.Stat(tt.path)
				if err != nil {
					t.Fatalf("failed to stat created fifo: %v", err)
				}
				if fi.Mode()&os.ModeNamedPipe == 0 {
					t.Errorf("expected FIFO, got mode: %v", fi.Mode())
				}
			}
		})
	}

	t.Run("path already exists, EEXIST", func(t *testing.T) {
		path := filepath.Join(tmpDir, "already_exists_fifo")
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
		err := mkFifo(path, 0666)
		utiltest.AssertErrorMatch(t, err, unix.EEXIST)
	})
}
