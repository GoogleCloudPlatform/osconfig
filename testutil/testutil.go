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

// Package testutil provides common testing utility functions for the osconfig agent.
package testutil

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// EnsureEquals checks if got and want are deeply equal. If not, it fails the test.
func EnsureEquals(t *testing.T, got interface{}, want interface{}) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got != want, got = %q want = %q", got, want)
	}
}

// AssertErrorMatch verifies that the gotErr matches the wantErr type and message.
func AssertErrorMatch(t *testing.T, gotErr, wantErr error) {
	t.Helper()
	if gotErr == nil && wantErr == nil {
		return
	}
	if gotErr == nil || wantErr == nil {
		t.Errorf("Errors mismatch, want %v, got %v", wantErr, gotErr)
		return
	}
	if reflect.TypeOf(gotErr) != reflect.TypeOf(wantErr) || gotErr.Error() != wantErr.Error() {
		t.Errorf("Unexpected error, want %v, got %v", wantErr, gotErr)
	}
}

// AssertFilePath verifies that the file path base matches the expected path base.
func AssertFilePath(t *testing.T, pathType string, gotPath string, wantPath string) {
	t.Helper()
	if wantPath == "" {
		if gotPath != "" {
			t.Errorf("unexpected %s path: got %q, want empty", pathType, gotPath)
		}
		return
	}
	if wantPath != filepath.Base(gotPath) {
		t.Errorf("unexpected %s path: got %q, want %q", pathType, filepath.Base(gotPath), wantPath)
	}
}

// AssertFileContents verifies that the file at filePath matches the expected contents.
func AssertFileContents(t *testing.T, filePath string, wantContents string) {
	t.Helper()
	if filePath == "" {
		return
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file %q: %v", filePath, err)
	}
	if string(data) != wantContents {
		t.Errorf("File contents = %q, want %q", string(data), wantContents)
	}
}
