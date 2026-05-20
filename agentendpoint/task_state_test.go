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

package agentendpoint

import (
	"encoding/json"
	"errors"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"

	"cloud.google.com/go/osconfig/agentendpoint/apiv1/agentendpointpb"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
)

var (
	testPatchTaskStateString = "{\"PatchTask\":{\"TaskID\":\"foo\",\"Task\":{\"patchConfig\":{\"apt\":{\"type\":\"DIST\",\"excludes\":[\"foo\",\"bar\"],\"exclusivePackages\":[\"foo\",\"bar\"]},\"windowsUpdate\":{\"classifications\":[\"CRITICAL\",\"SECURITY\"],\"excludes\":[\"foo\",\"bar\"],\"exclusivePatches\":[\"foo\",\"bar\"]}}},\"StartedAt\":\"0001-01-01T00:00:00Z\",\"PrePatchRebootCount\":2,\"PostPatchRebootCount\":1},\"Labels\":{\"foo\":\"bar\"}}"
	testPatchTaskState       = &taskState{
		Labels: map[string]string{"foo": "bar"},
		PatchTask: &patchTask{
			TaskID: "foo", Task: &applyPatchesTask{
				// This is not exhaustive but it's a good test for having multiple settings.
				&agentendpointpb.ApplyPatchesTask{
					PatchConfig: &agentendpointpb.PatchConfig{
						Apt:           &agentendpointpb.AptSettings{Type: agentendpointpb.AptSettings_DIST, Excludes: []string{"foo", "bar"}, ExclusivePackages: []string{"foo", "bar"}},
						WindowsUpdate: &agentendpointpb.WindowsUpdateSettings{Classifications: []agentendpointpb.WindowsUpdateSettings_Classification{agentendpointpb.WindowsUpdateSettings_CRITICAL, agentendpointpb.WindowsUpdateSettings_SECURITY}, Excludes: []string{"foo", "bar"}, ExclusivePatches: []string{"foo", "bar"}},
					},
				},
			},
			PrePatchRebootCount:  2,
			PostPatchRebootCount: 1,
		},
	}
)

func TestLoadState(t *testing.T) {
	td, err := ioutil.TempDir(os.TempDir(), "")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}
	defer os.RemoveAll(td)
	testState := filepath.Join(td, "testState")

	// test read error by attempting to read a directory
	if _, err := loadState(td); err == nil {
		t.Error("expected error loading state from a directory, got nil")
	}

	// test no state file
	if _, err := loadState(testState); err != nil {
		t.Errorf("no state file: unexpected error: %v", err)
	}

	// We don't test execTask as reboots during that task type is not supported.
	var tests = []struct {
		name    string
		state   []byte
		wantErr bool
		want    *taskState
	}{
		{
			"BlankState",
			[]byte("{}"),
			false,
			&taskState{},
		},
		{
			"BadState",
			[]byte("foo"),
			true,
			&taskState{},
		},
		{
			"PatchTask",
			[]byte(testPatchTaskStateString),
			false,
			testPatchTaskState,
		},
		{
			"IgnoresOldRebootFieldName",
			[]byte("{\"PatchTask\":{\"Task\":{},\"RebootCount\":1}}"),
			false,
			&taskState{
				PatchTask: &patchTask{
					Task: &applyPatchesTask{
						&agentendpointpb.ApplyPatchesTask{},
					},
					PrePatchRebootCount:  0,
					PostPatchRebootCount: 0,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ioutil.WriteFile(testState, tt.state, 0600); err != nil {
				t.Fatalf("error writing state: %v", err)
			}

			st, err := loadState(testState)
			if err != nil && !tt.wantErr {
				t.Fatalf("unexpected error: %v", err)
			}
			if err == nil && tt.wantErr {
				t.Fatalf("expected error")
			}

			if diff := cmp.Diff(tt.want, st, cmpopts.IgnoreUnexported(patchTask{}), protocmp.Transform()); diff != "" {
				t.Errorf("State does not match expectation: (-got +want)\n%s", diff)
			}
		})
	}
}

func TestLoadOldState(t *testing.T) {
	td, err := ioutil.TempDir(os.TempDir(), "")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}
	defer os.RemoveAll(td)
	testState := filepath.Join(td, "testState")
	oldTaskStateFile = testState

	if err := ioutil.WriteFile(testState, []byte(testPatchTaskStateString), 0600); err != nil {
		t.Fatalf("error writing state: %v", err)
	}

	st, err := loadState("/path/dne")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if diff := cmp.Diff(testPatchTaskState, st, cmpopts.IgnoreUnexported(patchTask{}), protocmp.Transform()); diff != "" {
		t.Errorf("State does not match expectation: (-got +want)\n%s", diff)
	}
}

func TestStateSave(t *testing.T) {
	td, err := ioutil.TempDir(os.TempDir(), "")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}
	defer os.RemoveAll(td)
	testState := filepath.Join(td, "testState")
	invalidDir := filepath.Join(td, "invalidDir")
	if err := os.WriteFile(invalidDir, []byte(""), 0755); err != nil {
		t.Fatalf("error creating file: %v", err)
	}
	invalidPath := filepath.Join(invalidDir, "testState")

	roDir := filepath.Join(td, "roDir")
	if err := os.MkdirAll(roDir, 0444); err != nil {
		t.Fatalf("error creating read-only dir: %v", err)
	}
	defer os.Chmod(roDir, 0755)
	roPath := filepath.Join(roDir, "state.json")

	var tests = []struct {
		desc    string
		state   *taskState
		path    string
		want    string
		wantErr error
	}{
		{
			desc:    "NilState",
			state:   nil,
			path:    testState,
			want:    "{}",
			wantErr: nil,
		},
		{
			desc:    "BlankState",
			state:   &taskState{},
			path:    testState,
			want:    "{}",
			wantErr: nil,
		},
		{
			desc:    "PatchTask",
			state:   testPatchTaskState,
			path:    testState,
			want:    testPatchTaskStateString,
			wantErr: nil,
		},
		{
			desc:    "ExecTask",
			state:   &taskState{ExecTask: &execTask{TaskID: "foo"}},
			path:    testState,
			want:    "{\"ExecTask\":{\"StartedAt\":\"0001-01-01T00:00:00Z\",\"Task\":null,\"TaskID\":\"foo\"}}",
			wantErr: nil,
		},
		{
			desc:    "InvalidDirectoryError",
			state:   &taskState{},
			path:    invalidPath,
			want:    "",
			wantErr: &fs.PathError{Op: "mkdir", Path: invalidDir, Err: errors.New("not a directory")},
		},
		{
			// time.Time.MarshalJSON only supports years between 0 and 9999.
			desc: "MarshalError",
			state: &taskState{
				ExecTask: &execTask{StartedAt: time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)},
			},
			path:    testState,
			want:    "",
			wantErr: &json.MarshalerError{Type: reflect.TypeOf(time.Time{}), Err: errors.New("Time.MarshalJSON: year outside of range [0,9999]")},
		},
		{
			// TempFile inside writeFile will fail because the parent directory is read-only.
			desc:    "WriteFileTempFileError",
			state:   &taskState{},
			path:    roPath,
			want:    "",
			wantErr: &fs.PathError{Op: "open", Path: roPath, Err: errors.New("permission denied")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			err := tt.state.save(tt.path)
			// Update the expected PathError's path since it cannot be predicted ahead of time.
			if pe, ok := err.(*fs.PathError); ok && tt.desc == "WriteFileTempFileError" {
				if wantPe, ok := tt.wantErr.(*fs.PathError); ok {
					wantPe.Path = pe.Path
				}
			}
			utiltest.AssertErrorMatch(t, err, tt.wantErr)
			if err != nil {
				return
			}

			got, err := ioutil.ReadFile(tt.path)
			if err != nil {
				t.Errorf("error reading state: %v", err)
				return
			}

			if string(got) != tt.want {
				t.Errorf("got:\n%q\nwant:\n%q", string(got), tt.want)
			}
		})
	}
}

func TestSaveLoadState(t *testing.T) {
	td, err := ioutil.TempDir(os.TempDir(), "")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}
	defer os.RemoveAll(td)
	testState := filepath.Join(td, "testState")

	if err := testPatchTaskState.save(testState); err != nil {
		t.Errorf("Unexpected save error: %v", err)
	}

	st, err := loadState(testState)
	if err != nil {
		t.Fatalf("Unexpected load error: %v", err)
	}

	if diff := cmp.Diff(testPatchTaskState, st, cmpopts.IgnoreUnexported(patchTask{}), protocmp.Transform()); diff != "" {
		t.Errorf("State does not match expectation: (-got +want)\n%s", diff)
	}
}
