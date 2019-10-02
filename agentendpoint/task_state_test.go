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

	"github.com/kylelemons/godebug/pretty"
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
			&taskState{PatchTask: nil, ExecTask: nil},
		},
		{
			"BadState",
			[]byte("foo"),
			true,
			&taskState{PatchTask: nil, ExecTask: nil},
		},
		{
			"PatchTask",
			[]byte(`{"PatchTask": {"TaskID": "foo"}}`),
			false,
			&taskState{PatchTask: &patchTask{TaskID: "foo"}, ExecTask: nil},
		},
		{
			"ExecTask",
			[]byte(`{"ExecTask": {"TaskID": "foo"}}`),
			false,
			&taskState{PatchTask: nil, ExecTask: &execTask{TaskID: "foo"}},
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
			if diff := pretty.Compare(tt.want, st); diff != "" {
				t.Errorf("patchWindow does not match expectation: (-got +want)\n%s", diff)
			}
		})
	}
}

func TestStateSave(t *testing.T) {
	td, err := ioutil.TempDir(os.TempDir(), "")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}
	defer os.RemoveAll(td)
	testState := filepath.Join(td, "testState")

	var tests = []struct {
		desc  string
		state *taskState
		want  string
	}{
		{
			"NilState",
			nil,
			"{}",
		},
		{
			"BlankState",
			&taskState{},
			"{}",
		},
		{
			"PatchTask",
			&taskState{PatchTask: &patchTask{TaskID: "foo"}, ExecTask: nil},
			"{\"PatchTask\":{\"TaskID\":\"foo\",\"Task\":null,\"StartedAt\":\"0001-01-01T00:00:00Z\",\"RebootCount\":0}}",
		},
		{
			"ExecTask",
			&taskState{ExecTask: &execTask{TaskID: "foo"}, PatchTask: nil},
			"{\"ExecTask\":{\"TaskID\":\"foo\",\"Task\":null,\"StartedAt\":\"0001-01-01T00:00:00Z\"}}",
		},
	}
	for _, tt := range tests {
		err := saveState(tt.state, testState)
		if err != nil {
			t.Errorf("%s: unexpected save error: %v", tt.desc, err)
			continue
		}

		got, err := ioutil.ReadFile(testState)
		if err != nil {
			t.Errorf("%s: error reading state: %v", tt.desc, err)
			continue
		}

		if string(got) != tt.want {
			t.Errorf("%s:\ngot:\n%q\nwant:\n%q", tt.desc, got, tt.want)
		}
	}
}
