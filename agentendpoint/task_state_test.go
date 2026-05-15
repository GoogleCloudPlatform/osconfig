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
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"

	"cloud.google.com/go/osconfig/agentendpoint/apiv1/agentendpointpb"
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
		wantErr bool
	}{
		{
			"NilState",
			nil,
			testState,
			"{}",
			false,
		},
		{
			"BlankState",
			&taskState{},
			testState,
			"{}",
			false,
		},
		{
			"PatchTask",
			testPatchTaskState,
			testState,
			testPatchTaskStateString,
			false,
		},
		{
			"ExecTask",
			&taskState{ExecTask: &execTask{TaskID: "foo"}},
			testState,
			"{\"ExecTask\":{\"StartedAt\":\"0001-01-01T00:00:00Z\",\"Task\":null,\"TaskID\":\"foo\"}}",
			false,
		},
		{
			"InvalidDirectoryError",
			&taskState{},
			invalidPath,
			"",
			true,
		},
		{
			// time.Time.MarshalJSON only supports years between 0 and 9999.
			"MarshalError",
			&taskState{
				ExecTask: &execTask{StartedAt: time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)},
			},
			testState,
			"",
			true,
		},
		{
			// TempFile inside writeFile will fail because the parent directory is read-only.
			"WriteFileTempFileError",
			&taskState{},
			roPath,
			"",
			true,
		},
	}
	for _, tt := range tests {
		err := tt.state.save(tt.path)
		if (err != nil) != tt.wantErr {
			t.Errorf("%s: unexpected save error state: %v, wantErr: %v", tt.desc, err, tt.wantErr)
			continue
		}
		if err != nil {
			continue
		}

		got, err := ioutil.ReadFile(tt.path)
		if err != nil {
			t.Errorf("%s: error reading state: %v", tt.desc, err)
			continue
		}

		if string(got) != tt.want {
			t.Errorf("%s:\ngot:\n%q\nwant:\n%q", tt.desc, got, tt.want)
		}
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
