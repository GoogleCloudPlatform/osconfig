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

package policies

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"cloud.google.com/go/osconfig/agentendpoint/apiv1beta/agentendpointpb"
	"github.com/GoogleCloudPlatform/osconfig/packages"
	utilmocks "github.com/GoogleCloudPlatform/osconfig/util/mocks"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
	"github.com/golang/mock/gomock"
)

func runGooGetRepositories(ctx context.Context, repos []*agentendpointpb.GooRepository) (string, error) {
	td, err := ioutil.TempDir(os.TempDir(), "")
	if err != nil {
		return "", fmt.Errorf("error creating temp dir: %v", err)
	}
	defer os.RemoveAll(td)
	testRepo := filepath.Join(td, "testRepo")

	if err := googetRepositories(ctx, repos, testRepo); err != nil {
		return "", fmt.Errorf("error running googetRepositories: %v", err)
	}

	data, err := ioutil.ReadFile(testRepo)
	if err != nil {
		return "", fmt.Errorf("error reading testRepo: %v", err)
	}

	return string(data), nil
}

func TestGooGetRepositories(t *testing.T) {
	tests := []struct {
		desc  string
		repos []*agentendpointpb.GooRepository
		want  string
	}{
		{"no repos", []*agentendpointpb.GooRepository{}, "# Repo file managed by Google OSConfig agent\n"},
		{
			"1 repo",
			[]*agentendpointpb.GooRepository{
				{Url: "http://repo1-url/", Name: "name"},
			},
			"# Repo file managed by Google OSConfig agent\n\n- name: name\n  url: http://repo1-url/\n",
		},
		{
			"2 repos",
			[]*agentendpointpb.GooRepository{
				{Url: "http://repo1-url/", Name: "name1"},
				{Url: "http://repo2-url/", Name: "name2"},
			},
			"# Repo file managed by Google OSConfig agent\n\n- name: name1\n  url: http://repo1-url/\n\n- name: name2\n  url: http://repo2-url/\n",
		},
	}

	for _, tt := range tests {
		got, err := runGooGetRepositories(context.Background(), tt.repos)
		if err != nil {
			t.Fatal(err)
		}

		if got != tt.want {
			t.Errorf("%s: got:\n%q\nwant:\n%q", tt.desc, got, tt.want)
		}
	}
}

// TestGooGetChanges tests the googetChanges function.
func TestGooGetChanges(t *testing.T) {
	ctx := context.Background()

	googet := filepath.Join(os.Getenv("GooGetRoot"), "googet.exe")

	mockCtrl := gomock.NewController(t)
	t.Cleanup(func() { mockCtrl.Finish() })

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	setupGooGetChangesTest(t, mockCommandRunner)

	tests := []struct {
		name             string
		gooInstalled     []*agentendpointpb.Package
		gooRemoved       []*agentendpointpb.Package
		gooUpdated       []*agentendpointpb.Package
		expectedCommands []utiltest.ExpectedCommand
		wantErr          error
	}{
		{
			name:         "no changes, want nil error",
			gooInstalled: []*agentendpointpb.Package{},
			wantErr:      nil,
		},
		{
			name:         "package p1 to install, want nil error",
			gooInstalled: []*agentendpointpb.Package{{Name: "p1"}},
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command(googet, "installed"),
					Stdout: []byte(""),
				},
				{
					Cmd: exec.Command(googet, "-noconfirm", "install", "p1"),
				},
			},
			wantErr: nil,
		},
		{
			name:         "package p1 to install with failure, want installed error",
			gooInstalled: []*agentendpointpb.Package{{Name: "p1"}},
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd: exec.Command(googet, "installed"),
					Err: errors.New("installed error"),
				},
			},
			wantErr: errors.New("error running googet.exe with args [\"installed\"]: installed error, stdout: \"\", stderr: \"\""),
		},
		{
			name:       "package p1 to update, want nil error",
			gooUpdated: []*agentendpointpb.Package{{Name: "p1"}},
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command(googet, "installed"),
					Stdout: []byte("p1.x86_64 1.0.0@1"),
				},
				{
					Cmd:    exec.Command(googet, "update"),
					Stdout: []byte("p1.x86_64, 1.0.0@1 --> 2.0.0@1 from repo"),
				},
				{
					Cmd: exec.Command(googet, "-noconfirm", "install", "p1"),
				},
			},
			wantErr: nil,
		},
		{
			name:       "package p1 to update with failure, want update error",
			gooUpdated: []*agentendpointpb.Package{{Name: "p1"}},
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command(googet, "installed"),
					Stdout: []byte("p1.x86_64 1.0.0@1"),
				},
				{
					Cmd: exec.Command(googet, "update"),
					Err: errors.New("update error"),
				},
			},
			wantErr: errors.New("error running googet.exe with args [\"update\"]: update error, stdout: \"\", stderr: \"\""),
		},
		{
			name:         "all packages operations failure, want combined error",
			gooInstalled: []*agentendpointpb.Package{{Name: "p1"}},
			gooUpdated:   []*agentendpointpb.Package{{Name: "p2"}},
			gooRemoved:   []*agentendpointpb.Package{{Name: "p3"}},
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command(googet, "installed"),
					Stdout: []byte("p2.x86_64 1.0.0@1\np3.x86_64 1.0.0@1"),
				},
				{
					Cmd:    exec.Command(googet, "update"),
					Stdout: []byte("p2.x86_64, 1.0.0@1 --> 2.0.0@1 from repo"),
				},
				{
					Cmd: exec.Command(googet, "-noconfirm", "install", "p1"),
					Err: errors.New("install error"),
				},
				{
					Cmd: exec.Command(googet, "-noconfirm", "install", "p2"),
					Err: errors.New("upgrade error"),
				},
				{
					Cmd: exec.Command(googet, "-noconfirm", "remove", "p3"),
					Err: errors.New("remove error"),
				},
			},
			wantErr: errors.New("error installing googet packages: error running googet.exe with args [\"-noconfirm\" \"install\" \"p1\"]: install error, stdout: \"\", stderr: \"\",\nerror upgrading googet packages: error running googet.exe with args [\"-noconfirm\" \"install\" \"p2\"]: upgrade error, stdout: \"\", stderr: \"\",\nerror removing googet packages: error running googet.exe with args [\"-noconfirm\" \"remove\" \"p3\"]: remove error, stdout: \"\", stderr: \"\""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			utiltest.SetExpectedCommands(ctx, mockCommandRunner, tt.expectedCommands)

			gotErr := googetChanges(context.Background(), tt.gooInstalled, tt.gooRemoved, tt.gooUpdated)
			utiltest.AssertErrorMatch(t, gotErr, tt.wantErr)
		})
	}
}

func setupGooGetChangesTest(t *testing.T, runner *utilmocks.MockCommandRunner) {
	utiltest.OverrideVariable(t, &packages.GooGetExists, true)
	packages.SetCommandRunner(runner)
}
