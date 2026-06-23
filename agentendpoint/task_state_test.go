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
	"syscall"
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

	// test no state file
	if _, err := loadState(testState); err != nil {
		t.Errorf("no state file: unexpected error: %v", err)
	}

	// We don't test execTask as reboots during that task type is not supported.
	var tests = []struct {
		name    string
		setup   func(t *testing.T) string
		wantErr error
		want    *taskState
	}{
		{
			name: "empty json, want blank state",
			setup: func(t *testing.T) string {
				if err := ioutil.WriteFile(testState, []byte("{}"), 0600); err != nil {
					t.Fatalf("error writing state: %v", err)
				}
				return testState
			},
			wantErr: nil,
			want:    &taskState{},
		},
		{
			name: "invalid json, want json syntax error",
			setup: func(t *testing.T) string {
				if err := ioutil.WriteFile(testState, []byte("foo"), 0600); err != nil {
					t.Fatalf("error writing state: %v", err)
				}
				return testState
			},
			wantErr: json.Unmarshal([]byte("foo"), &taskState{}),
			want:    &taskState{},
		},
		{
			name: "valid patch task json, want patch task state",
			setup: func(t *testing.T) string {
				if err := ioutil.WriteFile(testState, []byte(testPatchTaskStateString), 0600); err != nil {
					t.Fatalf("error writing state: %v", err)
				}
				return testState
			},
			wantErr: nil,
			want:    testPatchTaskState,
		},
		{
			name: "json with old reboot field, want state with zero reboot counts",
			setup: func(t *testing.T) string {
				if err := ioutil.WriteFile(testState, []byte("{\"PatchTask\":{\"Task\":{},\"RebootCount\":1}}"), 0600); err != nil {
					t.Fatalf("error writing state: %v", err)
				}
				return testState
			},
			wantErr: nil,
			want: &taskState{
				PatchTask: &patchTask{
					Task: &applyPatchesTask{
						&agentendpointpb.ApplyPatchesTask{},
					},
					PrePatchRebootCount:  0,
					PostPatchRebootCount: 0,
				},
			},
		},
		{
			name: "directory path, want path error",
			setup: func(t *testing.T) string {
				return td
			},
			wantErr: &fs.PathError{Op: "read", Path: td, Err: syscall.EISDIR},
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup(t)
			st, err := loadState(path)
			utiltest.AssertErrorMatch(t, err, tt.wantErr)

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
	td := t.TempDir()
	testState := filepath.Join(td, "testState")
	invalidDir := utiltest.WriteToTempFileMust(t, "invalidDir", []byte(""))
	invalidPath := filepath.Join(invalidDir, "testState")

	var tests = []struct {
		desc    string
		state   *taskState
		path    string
		want    string
		wantErr error
	}{
		{
			desc:    "nil state, expect empty json",
			state:   nil,
			path:    testState,
			want:    "{}",
			wantErr: nil,
		},
		{
			desc:    "blank state, expect empty json",
			state:   &taskState{},
			path:    testState,
			want:    "{}",
			wantErr: nil,
		},
		{
			desc:    "patch task state, expect serialized patch task json",
			state:   testPatchTaskState,
			path:    testState,
			want:    testPatchTaskStateString,
			wantErr: nil,
		},
		{
			desc:    "exec task state, expect serialized exec task json",
			state:   &taskState{ExecTask: &execTask{TaskID: "foo"}},
			path:    testState,
			want:    "{\"ExecTask\":{\"StartedAt\":\"0001-01-01T00:00:00Z\",\"Task\":null,\"TaskID\":\"foo\"}}",
			wantErr: nil,
		},
		{
			desc:    "invalid directory path, expect path error",
			state:   &taskState{},
			path:    invalidPath,
			want:    "",
			wantErr: &fs.PathError{Op: "mkdir", Path: invalidDir, Err: errors.New("not a directory")},
		},
		{
			// time.Time.MarshalJSON only supports years between 0 and 9999.
			desc: "invalid input for json, expect marshal error",
			state: &taskState{
				ExecTask: &execTask{StartedAt: time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)},
			},
			path:    testState,
			want:    "",
			wantErr: &json.MarshalerError{Type: reflect.TypeOf(time.Time{}), Err: errors.New("Time.MarshalJSON: year outside of range [0,9999]")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			gotErr := tt.state.save(tt.path)

			utiltest.AssertErrorMatchAndSkip(t, gotErr, tt.wantErr)
			utiltest.AssertFileContents(t, tt.path, tt.want)
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
