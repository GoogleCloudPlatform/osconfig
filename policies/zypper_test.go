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

func runZypperRepositories(ctx context.Context, repos []*agentendpointpb.ZypperRepository) (string, error) {
	td, err := ioutil.TempDir(os.TempDir(), "")
	if err != nil {
		return "", fmt.Errorf("error creating temp dir: %v", err)
	}
	defer os.RemoveAll(td)
	testRepo := filepath.Join(td, "testRepo")

	if err := zypperRepositories(ctx, repos, testRepo); err != nil {
		return "", fmt.Errorf("error running zypperRepositories: %v", err)
	}

	data, err := ioutil.ReadFile(testRepo)
	if err != nil {
		return "", fmt.Errorf("error reading testRepo: %v", err)
	}

	return string(data), nil
}

func TestZypperRepositories(t *testing.T) {
	tests := []struct {
		desc  string
		repos []*agentendpointpb.ZypperRepository
		want  string
	}{
		{"no repos", []*agentendpointpb.ZypperRepository{}, "# Repo file managed by Google OSConfig agent\n"},
		{
			"1 repo",
			[]*agentendpointpb.ZypperRepository{
				{BaseUrl: "http://repo1-url/", Id: "id"},
			},
			"# Repo file managed by Google OSConfig agent\n\n[id]\nname=id\nbaseurl=http://repo1-url/\nenabled=1\ngpgcheck=1\n",
		},
		{
			"2 repos",
			[]*agentendpointpb.ZypperRepository{
				{BaseUrl: "http://repo1-url/", Id: "id1", DisplayName: "displayName1", GpgKeys: []string{"https://url/key"}},
				{BaseUrl: "http://repo1-url/", Id: "id2", DisplayName: "displayName2", GpgKeys: []string{"https://url/key1", "https://url/key2"}},
			},
			"# Repo file managed by Google OSConfig agent\n\n[id1]\nname=displayName1\nbaseurl=http://repo1-url/\nenabled=1\ngpgcheck=1\ngpgkey=https://url/key\n\n[id2]\nname=displayName2\nbaseurl=http://repo1-url/\nenabled=1\ngpgcheck=1\ngpgkey=https://url/key1\n       https://url/key2\n",
		},
	}

	for _, tt := range tests {
		got, err := runZypperRepositories(context.Background(), tt.repos)
		if err != nil {
			t.Fatal(err)
		}

		if got != tt.want {
			t.Errorf("%s: got:\n%q\nwant:\n%q", tt.desc, got, tt.want)
		}
	}
}

// TestZypperChanges tests the zypperChanges function, ensuring it correctly handles package installations, removals, and updates.
func TestZypperChanges(t *testing.T) {
	rpmQueryArgs := []string{"--queryformat", `\{"architecture":"%{ARCH}","package":"%{NAME}","source_name":"%{SOURCERPM}","version":"%|EPOCH?{%{EPOCH}:}:{}|%{VERSION}-%{RELEASE}"\}` + "\n", "-a"}
	zypperListUpdatesArgs := []string{"--gpg-auto-import-keys", "-q", "list-updates"}

	tests := []struct {
		name            string
		zypperInstalled []*agentendpointpb.Package
		zypperRemoved   []*agentendpointpb.Package
		zypperUpdated   []*agentendpointpb.Package
		expectations    []expectedCommand
		wantErr         error
	}{
		{
			name: "no changes needed",
		},
		{
			name:            "failed to get installed packages",
			zypperInstalled: []*agentendpointpb.Package{{Name: "p1"}},
			expectations: []expectedCommand{
				{cmd: exec.Command("/usr/bin/rpmquery", rpmQueryArgs...), err: errors.New("rpmquery error")},
			},
			wantErr: errors.New("error running /usr/bin/rpmquery with args [\"--queryformat\" \"\\\\{\\\"architecture\\\":\\\"%{ARCH}\\\",\\\"package\\\":\\\"%{NAME}\\\",\\\"source_name\\\":\\\"%{SOURCERPM}\\\",\\\"version\\\":\\\"%|EPOCH?{%{EPOCH}:}:{}|%{VERSION}-%{RELEASE}\\\"\\\\}\\n\" \"-a\"]: rpmquery error, stdout: \"\", stderr: \"\""),
		},
		{
			name:          "failed to get updates",
			zypperUpdated: []*agentendpointpb.Package{{Name: "p1"}},
			expectations: []expectedCommand{
				{cmd: exec.Command("/usr/bin/rpmquery", rpmQueryArgs...), stdout: []byte(`{"package":"p1","status":"installed"}`)},
				{cmd: exec.Command("/usr/bin/zypper", zypperListUpdatesArgs...), err: errors.New("zypper list-updates error")},
			},
			wantErr: errors.New("error running /usr/bin/zypper with args [\"--gpg-auto-import-keys\" \"-q\" \"list-updates\"]: zypper list-updates error, stdout: \"\", stderr: \"\""),
		},
		{
			name:            "successful install",
			zypperInstalled: []*agentendpointpb.Package{{Name: "p1"}},
			expectations: []expectedCommand{
				{cmd: exec.Command("/usr/bin/rpmquery", rpmQueryArgs...), stdout: []byte("")},
				{cmd: exec.Command("/usr/bin/zypper", "--gpg-auto-import-keys", "--non-interactive", "install", "--auto-agree-with-licenses", "p1")},
			},
		},
		{
			name:            "install failure",
			zypperInstalled: []*agentendpointpb.Package{{Name: "p1"}},
			expectations: []expectedCommand{
				{cmd: exec.Command("/usr/bin/rpmquery", rpmQueryArgs...), stdout: []byte("")},
				{cmd: exec.Command("/usr/bin/zypper", "--gpg-auto-import-keys", "--non-interactive", "install", "--auto-agree-with-licenses", "p1"), err: errors.New("install error")},
			},
			wantErr: errors.New("error installing zypper packages: error running /usr/bin/zypper with args [\"--gpg-auto-import-keys\" \"--non-interactive\" \"install\" \"--auto-agree-with-licenses\" \"p1\"]: install error, stdout: \"\", stderr: \"\""),
		},
		{
			name:          "successful upgrade",
			zypperUpdated: []*agentendpointpb.Package{{Name: "p1"}},
			expectations: []expectedCommand{
				{cmd: exec.Command("/usr/bin/rpmquery", rpmQueryArgs...), stdout: []byte(`{"package":"p1","status":"installed"}`)},
				{cmd: exec.Command("/usr/bin/zypper", zypperListUpdatesArgs...), stdout: []byte("v | Repo | p1 | 1.0 | 2.0 | x86_64\n")},
				{cmd: exec.Command("/usr/bin/zypper", "--gpg-auto-import-keys", "--non-interactive", "install", "--auto-agree-with-licenses", "p1")},
			},
		},
		{
			name:          "upgrade failure",
			zypperUpdated: []*agentendpointpb.Package{{Name: "p1"}},
			expectations: []expectedCommand{
				{cmd: exec.Command("/usr/bin/rpmquery", rpmQueryArgs...), stdout: []byte(`{"package":"p1","status":"installed"}`)},
				{cmd: exec.Command("/usr/bin/zypper", zypperListUpdatesArgs...), stdout: []byte("v | Repo | p1 | 1.0 | 2.0 | x86_64\n")},
				{cmd: exec.Command("/usr/bin/zypper", "--gpg-auto-import-keys", "--non-interactive", "install", "--auto-agree-with-licenses", "p1"), err: errors.New("upgrade error")},
			},
			wantErr: errors.New("error upgrading zypper packages: error running /usr/bin/zypper with args [\"--gpg-auto-import-keys\" \"--non-interactive\" \"install\" \"--auto-agree-with-licenses\" \"p1\"]: upgrade error, stdout: \"\", stderr: \"\""),
		},
		{
			name:          "successful remove",
			zypperRemoved: []*agentendpointpb.Package{{Name: "p1"}},
			expectations: []expectedCommand{
				{cmd: exec.Command("/usr/bin/rpmquery", rpmQueryArgs...), stdout: []byte(`{"package":"p1","status":"installed"}`)},
				{cmd: exec.Command("/usr/bin/zypper", "--non-interactive", "remove", "p1")},
			},
		},
		{
			name:          "remove failure",
			zypperRemoved: []*agentendpointpb.Package{{Name: "p1"}},
			expectations: []expectedCommand{
				{cmd: exec.Command("/usr/bin/rpmquery", rpmQueryArgs...), stdout: []byte(`{"package":"p1","status":"installed"}`)},
				{cmd: exec.Command("/usr/bin/zypper", "--non-interactive", "remove", "p1"), err: errors.New("remove error")},
			},
			wantErr: errors.New("error removing zypper packages: error running /usr/bin/zypper with args [\"--non-interactive\" \"remove\" \"p1\"]: remove error, stdout: \"\", stderr: \"\""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
			setupZypperChangesTest(t, mockCommandRunner)
			setExpectations(mockCommandRunner, tt.expectations)

			err := zypperChanges(context.Background(), tt.zypperInstalled, tt.zypperRemoved, tt.zypperUpdated)
			utiltest.AssertErrorMatch(t, err, tt.wantErr)
		})
	}
}

// setupZypperChangesTest sets up the environment for zypperChanges tests by mocking the command runner.
func setupZypperChangesTest(t *testing.T, runner *utilmocks.MockCommandRunner) {
	oldZypper := packages.ZypperExists

	packages.ZypperExists = true
	packages.SetCommandRunner(runner)
	packages.SetPtyCommandRunner(runner)

	t.Cleanup(func() {
		packages.ZypperExists = oldZypper
	})
}
