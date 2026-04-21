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

func runYumRepositories(ctx context.Context, repos []*agentendpointpb.YumRepository) (string, error) {
	td, err := ioutil.TempDir(os.TempDir(), "")
	if err != nil {
		return "", fmt.Errorf("error creating temp dir: %v", err)
	}
	defer os.RemoveAll(td)
	testRepo := filepath.Join(td, "testRepo")

	if err := yumRepositories(ctx, repos, testRepo); err != nil {
		return "", fmt.Errorf("error running yumRepositories: %v", err)
	}

	data, err := ioutil.ReadFile(testRepo)
	if err != nil {
		return "", fmt.Errorf("error reading testRepo: %v", err)
	}

	return string(data), nil
}

func TestYumRepositories(t *testing.T) {
	tests := []struct {
		desc  string
		repos []*agentendpointpb.YumRepository
		want  string
	}{
		{"no repos", []*agentendpointpb.YumRepository{}, "# Repo file managed by Google OSConfig agent\n"},
		{
			"1 repo",
			[]*agentendpointpb.YumRepository{
				{BaseUrl: "http://repo1-url/", Id: "id"},
			},
			"# Repo file managed by Google OSConfig agent\n\n[id]\nname=id\nbaseurl=http://repo1-url/\nenabled=1\ngpgcheck=1\n",
		},
		{
			"2 repos",
			[]*agentendpointpb.YumRepository{
				{BaseUrl: "http://repo1-url/", Id: "id1", DisplayName: "displayName1", GpgKeys: []string{"https://url/key"}},
				{BaseUrl: "http://repo1-url/", Id: "id2", DisplayName: "displayName2", GpgKeys: []string{"https://url/key1", "https://url/key2"}},
			},
			"# Repo file managed by Google OSConfig agent\n\n[id1]\nname=displayName1\nbaseurl=http://repo1-url/\nenabled=1\ngpgcheck=1\ngpgkey=https://url/key\n\n[id2]\nname=displayName2\nbaseurl=http://repo1-url/\nenabled=1\ngpgcheck=1\ngpgkey=https://url/key1\n       https://url/key2\n",
		},
	}

	for _, tt := range tests {
		got, err := runYumRepositories(context.Background(), tt.repos)
		if err != nil {
			t.Fatal(err)
		}

		if got != tt.want {
			t.Errorf("%s: got:\n%q\nwant:\n%q", tt.desc, got, tt.want)
		}
	}
}

// TestYumChanges tests the yumChanges function, ensuring it correctly handles package installations, removals, and updates.
func TestYumChanges(t *testing.T) {
	rpmQueryArgs := []string{"--queryformat", `\{"architecture":"%{ARCH}","package":"%{NAME}","source_name":"%{SOURCERPM}","version":"%|EPOCH?{%{EPOCH}:}:{}|%{VERSION}-%{RELEASE}"\}` + "\n", "-a"}
	yumCheckUpdateArgs := []string{"check-update", "--assumeyes"}
	yumListUpdatesArgs := []string{"update", "--assumeno", "--color=never"}

	yumCheckUpdateErr := exec.Command("/bin/bash", "-c", "exit 100").Run()

	tests := []struct {
		name         string
		yumInstalled []*agentendpointpb.Package
		yumRemoved   []*agentendpointpb.Package
		yumUpdated   []*agentendpointpb.Package
		expectations []expectedCommand
		wantErr      error
	}{
		{
			name: "no changes, want nil",
		},
		{
			name:         "rpmquery failure, want error",
			yumInstalled: []*agentendpointpb.Package{{Name: "p1"}},
			expectations: []expectedCommand{
				{cmd: exec.Command("/usr/bin/rpmquery", rpmQueryArgs...), err: errors.New("rpmquery error")},
			},
			wantErr: errors.New("error running /usr/bin/rpmquery with args [\"--queryformat\" \"\\\\{\\\"architecture\\\":\\\"%{ARCH}\\\",\\\"package\\\":\\\"%{NAME}\\\",\\\"source_name\\\":\\\"%{SOURCERPM}\\\",\\\"version\\\":\\\"%|EPOCH?{%{EPOCH}:}:{}|%{VERSION}-%{RELEASE}\\\"\\\\}\\n\" \"-a\"]: rpmquery error, stdout: \"\", stderr: \"\""),
		},
		{
			name:       "yum check-update failure, want check-update error",
			yumUpdated: []*agentendpointpb.Package{{Name: "p1"}},
			expectations: []expectedCommand{
				{cmd: exec.Command("/usr/bin/rpmquery", rpmQueryArgs...), stdout: []byte(`{"package":"p1","status":"installed"}`)},
				{cmd: exec.Command("/usr/bin/yum", yumCheckUpdateArgs...), err: errors.New("yum check-update error")},
			},
			wantErr: errors.New("error running /usr/bin/yum with args [\"check-update\" \"--assumeyes\"]: yum check-update error, stdout: \"\", stderr: \"\""),
		},
		{
			name:         "p1 to install, want nil",
			yumInstalled: []*agentendpointpb.Package{{Name: "p1"}},
			expectations: []expectedCommand{
				{cmd: exec.Command("/usr/bin/rpmquery", rpmQueryArgs...), stdout: []byte("")},
				{cmd: exec.Command("/usr/bin/yum", "install", "--assumeyes", "p1")},
			},
		},
		{
			name:         "p1 to install with failure, want installing error",
			yumInstalled: []*agentendpointpb.Package{{Name: "p1"}},
			expectations: []expectedCommand{
				{cmd: exec.Command("/usr/bin/rpmquery", rpmQueryArgs...), stdout: []byte("")},
				{cmd: exec.Command("/usr/bin/yum", "install", "--assumeyes", "p1"), err: errors.New("install error")},
			},
			wantErr: errors.New("error installing yum packages: error running /usr/bin/yum with args [\"install\" \"--assumeyes\" \"p1\"]: install error, stdout: \"\", stderr: \"\""),
		},
		{
			name:       "p1 to upgrade, want nil",
			yumUpdated: []*agentendpointpb.Package{{Name: "p1"}},
			expectations: []expectedCommand{
				{cmd: exec.Command("/usr/bin/rpmquery", rpmQueryArgs...), stdout: []byte(`{"package":"p1","status":"installed"}`)},
				{cmd: exec.Command("/usr/bin/yum", yumCheckUpdateArgs...), err: yumCheckUpdateErr},
				{cmd: exec.Command("/usr/bin/yum", yumListUpdatesArgs...), stdout: []byte("Updating:\n p1 x86_64 2.0 updates 100 k\n")},
				{cmd: exec.Command("/usr/bin/yum", "install", "--assumeyes", "p1")},
			},
		},
		{
			name:       "p1 to upgrade with failure, want upgrading error",
			yumUpdated: []*agentendpointpb.Package{{Name: "p1"}},
			expectations: []expectedCommand{
				{cmd: exec.Command("/usr/bin/rpmquery", rpmQueryArgs...), stdout: []byte(`{"package":"p1","status":"installed"}`)},
				{cmd: exec.Command("/usr/bin/yum", yumCheckUpdateArgs...), err: yumCheckUpdateErr},
				{cmd: exec.Command("/usr/bin/yum", yumListUpdatesArgs...), stdout: []byte("Updating:\n p1 x86_64 2.0 updates 100 k\n")},
				{cmd: exec.Command("/usr/bin/yum", "install", "--assumeyes", "p1"), err: errors.New("upgrade error")},
			},
			wantErr: errors.New("error upgrading yum packages: error running /usr/bin/yum with args [\"install\" \"--assumeyes\" \"p1\"]: upgrade error, stdout: \"\", stderr: \"\""),
		},
		{
			name:       "p1 to remove, want nil",
			yumRemoved: []*agentendpointpb.Package{{Name: "p1"}},
			expectations: []expectedCommand{
				{cmd: exec.Command("/usr/bin/rpmquery", rpmQueryArgs...), stdout: []byte(`{"package":"p1","status":"installed"}`)},
				{cmd: exec.Command("/usr/bin/yum", "remove", "--assumeyes", "p1")},
			},
		},
		{
			name:       "p1 to remove with failure, want removing error",
			yumRemoved: []*agentendpointpb.Package{{Name: "p1"}},
			expectations: []expectedCommand{
				{cmd: exec.Command("/usr/bin/rpmquery", rpmQueryArgs...), stdout: []byte(`{"package":"p1","status":"installed"}`)},
				{cmd: exec.Command("/usr/bin/yum", "remove", "--assumeyes", "p1"), err: errors.New("remove error")},
			},
			wantErr: errors.New("error removing yum packages: error running /usr/bin/yum with args [\"remove\" \"--assumeyes\" \"p1\"]: remove error, stdout: \"\", stderr: \"\""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
			setupYumChangesTest(t, mockCommandRunner)
			setExpectations(mockCommandRunner, tt.expectations)

			err := yumChanges(context.Background(), tt.yumInstalled, tt.yumRemoved, tt.yumUpdated)
			utiltest.AssertErrorMatch(t, err, tt.wantErr)
		})
	}
}

// setupYumChangesTest sets up the environment for yumChanges tests by mocking the command runner.
func setupYumChangesTest(t *testing.T, runner *utilmocks.MockCommandRunner) {
	t.Cleanup(utiltest.OverrideVariable(&packages.YumExists, true))
	packages.SetCommandRunner(runner)
	packages.SetPtyCommandRunner(runner)
}
