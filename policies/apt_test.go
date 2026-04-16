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
	"strings"
	"testing"

	"cloud.google.com/go/osconfig/agentendpoint/apiv1beta/agentendpointpb"
	"github.com/GoogleCloudPlatform/osconfig/osinfo"
	"github.com/GoogleCloudPlatform/osconfig/packages"
	utilmocks "github.com/GoogleCloudPlatform/osconfig/util/mocks"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
	"github.com/golang/mock/gomock"
)

func runAptRepositories(ctx context.Context, repos []*agentendpointpb.AptRepository) (string, error) {
	td, err := ioutil.TempDir(os.TempDir(), "")
	if err != nil {
		return "", fmt.Errorf("error creating temp dir: %v", err)
	}
	defer os.RemoveAll(td)
	testRepo := filepath.Join(td, "testRepo")

	if err := aptRepositories(ctx, repos, testRepo); err != nil {
		return "", fmt.Errorf("error running aptRepositories: %v", err)
	}

	data, err := ioutil.ReadFile(testRepo)
	if err != nil {
		return "", fmt.Errorf("error reading testRepo: %v", err)
	}

	return string(data), nil
}

func TestAptRepositories(t *testing.T) {
	debian10 := func() (string, string, string) {
		return "debian", "Debian", "10"
	}

	debian12 := func() (string, string, string) {
		return "debian", "Debian", "12"
	}

	tests := []struct {
		name                   string
		repos                  []*agentendpointpb.AptRepository
		nameAndVersionProvider func() (string, string, string)
		want                   string
	}{
		{
			name:                   "No repositories",
			nameAndVersionProvider: debian10,
			repos:                  []*agentendpointpb.AptRepository{},
			want:                   "# Repo file managed by Google OSConfig agent\n"},
		{
			name:                   "1 repositoy, Debian 10",
			nameAndVersionProvider: debian10,
			repos: []*agentendpointpb.AptRepository{
				{Uri: "http://repo1-url/", Distribution: "distribution", Components: []string{"component1"}},
			},
			want: "# Repo file managed by Google OSConfig agent\n\ndeb http://repo1-url/ distribution component1\n",
		},
		{
			name:                   "1 repositoy, Debian 12",
			nameAndVersionProvider: debian12,
			repos: []*agentendpointpb.AptRepository{
				{Uri: "http://repo1-url/", Distribution: "distribution", Components: []string{"component1"}},
			},
			want: "# Repo file managed by Google OSConfig agent\n\ndeb [signed-by=/etc/apt/trusted.gpg.d/osconfig_agent_managed.gpg] http://repo1-url/ distribution component1\n",
		},
		{
			name:                   "2 repos, Debian 10",
			nameAndVersionProvider: debian10,
			repos: []*agentendpointpb.AptRepository{
				{Uri: "http://repo1-url/", Distribution: "distribution", Components: []string{"component1"}, ArchiveType: agentendpointpb.AptRepository_DEB_SRC},
				{Uri: "http://repo2-url/", Distribution: "distribution", Components: []string{"component1", "component2"}, ArchiveType: agentendpointpb.AptRepository_DEB},
			},
			want: "# Repo file managed by Google OSConfig agent\n\ndeb-src http://repo1-url/ distribution component1\n\ndeb http://repo2-url/ distribution component1 component2\n",
		},
		{
			name:                   "2 repos, Debian 12",
			nameAndVersionProvider: debian12,
			repos: []*agentendpointpb.AptRepository{
				{Uri: "http://repo1-url/", Distribution: "distribution", Components: []string{"component1"}, ArchiveType: agentendpointpb.AptRepository_DEB_SRC},
				{Uri: "http://repo2-url/", Distribution: "distribution", Components: []string{"component1", "component2"}, ArchiveType: agentendpointpb.AptRepository_DEB},
			},
			want: "# Repo file managed by Google OSConfig agent\n\ndeb-src [signed-by=/etc/apt/trusted.gpg.d/osconfig_agent_managed.gpg] http://repo1-url/ distribution component1\n\ndeb [signed-by=/etc/apt/trusted.gpg.d/osconfig_agent_managed.gpg] http://repo2-url/ distribution component1 component2\n",
		},
	}

	for _, tt := range tests {
		osInfoProviderActual := osInfoProvider
		defer func() { osInfoProvider = osInfoProviderActual }()

		osInfoStub := stubOsInfoProvider{nameVersionProvider: tt.nameAndVersionProvider}
		osInfoProvider = osInfoStub

		got, err := runAptRepositories(context.Background(), tt.repos)
		if err != nil {
			t.Fatal(err)
		}

		if got != tt.want {
			t.Errorf("%s: got:\n%q\nwant:\n%q", tt.name, got, tt.want)
		}
	}
}

func TestGetAptGPGKey(t *testing.T) {
	key := "https://packages.cloud.google.com/apt/doc/apt-key.gpg"

	entityList, err := getAptGPGKey(key)
	if err != nil {
		t.Fatal(err)
	}

	// check if Artifact Regitry key exist or not
	artifactRegistryKeyFound := false
	for _, e := range entityList {
		for key := range e.Identities {
			if strings.Contains(key, "Artifact Registry") {
				artifactRegistryKeyFound = true
			}
		}
	}

	if !artifactRegistryKeyFound {
		t.Errorf("Expected to find Artifact Registry key in Google Cloud Public GPG key, but its missed.")
	}
}

func TestUseSignedBy(t *testing.T) {
	tests := []struct {
		name string
		repo *agentendpointpb.AptRepository
		want string
	}{
		{
			"1 repo",
			&agentendpointpb.AptRepository{Uri: "http://repo1-url/", Distribution: "distribution", Components: []string{"component1"}},
			"\ndeb [signed-by=/etc/apt/trusted.gpg.d/osconfig_agent_managed.gpg] http://repo1-url/ distribution component1",
		},
		{
			"2 components",
			&agentendpointpb.AptRepository{Uri: "http://repo2-url/", Distribution: "distribution", Components: []string{"component1", "component2"}, ArchiveType: agentendpointpb.AptRepository_DEB},
			"\ndeb [signed-by=/etc/apt/trusted.gpg.d/osconfig_agent_managed.gpg] http://repo2-url/ distribution component1 component2",
		},
	}

	useSignedBy := true
	for _, tt := range tests {
		aptRepoLine := getAptRepoLine(tt.repo, useSignedBy)

		if aptRepoLine != tt.want {
			t.Errorf("%s: got:\n%q\nwant:\n%q", tt.name, aptRepoLine, tt.want)
		}
	}
}

type stubOsInfoProvider struct {
	nameVersionProvider func() (string, string, string)
}

func (s stubOsInfoProvider) GetOSInfo(ctx context.Context) (osinfo.OSInfo, error) {
	short, long, version := s.nameVersionProvider()

	return osinfo.OSInfo{
		Hostname:      "test",
		LongName:      long,
		ShortName:     short,
		Version:       version,
		KernelVersion: "test",
		KernelRelease: "test",
		Architecture:  "x86_64",
	}, nil
}

func TestAptChanges(t *testing.T) {
	dpkgQueryArgs := []string{"-W", "-f", `\{"architecture":"${Architecture}","package":"${Package}","source_name":"${source:Package}","source_version":"${source:Version}","status":"${db:Status-Status}","version":"${Version}"\}` + "\n"}
	aptUpgradableArgs := []string{"--just-print", "-qq", "dist-upgrade"}
	aptEnv := []string{"DEBIAN_FRONTEND=noninteractive"}

	tests := []struct {
		name          string
		aptInstalled  []*agentendpointpb.Package
		aptRemoved    []*agentendpointpb.Package
		aptUpdated    []*agentendpointpb.Package
		expectations  []expectedCommand
		wantErr       error
	}{
		{
			name: "no changes needed",
		},
		{
			name:         "failed to get installed packages",
			aptInstalled: []*agentendpointpb.Package{{Name: "p1"}},
			expectations: []expectedCommand{
				{cmd: exec.Command("/usr/bin/dpkg-query", dpkgQueryArgs...), err: errors.New("dpkg-query error")},
			},
			wantErr: errors.New("error running /usr/bin/dpkg-query with args [\"-W\" \"-f\" \"\\\\{\\\"architecture\\\":\\\"${Architecture}\\\",\\\"package\\\":\\\"${Package}\\\",\\\"source_name\\\":\\\"${source:Package}\\\",\\\"source_version\\\":\\\"${source:Version}\\\",\\\"status\\\":\\\"${db:Status-Status}\\\",\\\"version\\\":\\\"${Version}\\\"\\\\}\\n\"]: dpkg-query error, stdout: \"\", stderr: \"\""),
		},
		{
			name:       "failed to get updates",
			aptUpdated: []*agentendpointpb.Package{{Name: "p1"}},
			expectations: []expectedCommand{
				{cmd: exec.Command("/usr/bin/dpkg-query", dpkgQueryArgs...), stdout: []byte(`{"package":"p1","status":"installed"}`)},
				{cmd: exec.Command("/usr/bin/apt-get", "update"), envs: aptEnv},
				{cmd: exec.Command("/usr/bin/apt-get", aptUpgradableArgs...), envs: aptEnv, err: errors.New("apt-get updates error")},
			},
			wantErr: errors.New("apt-get updates error"),
		},
		{
			name:         "successful install",
			aptInstalled: []*agentendpointpb.Package{{Name: "p1"}},
			expectations: []expectedCommand{
				{cmd: exec.Command("/usr/bin/dpkg-query", dpkgQueryArgs...), stdout: []byte("")},
				{cmd: exec.Command("/usr/bin/apt-get", "update"), envs: aptEnv},
				{cmd: exec.Command("/usr/bin/apt-get", "install", "-y", "p1"), envs: aptEnv},
			},
		},
		{
			name:         "install fallback success",
			aptInstalled: []*agentendpointpb.Package{{Name: "p1"}, {Name: "p2"}},
			expectations: []expectedCommand{
				{cmd: exec.Command("/usr/bin/dpkg-query", dpkgQueryArgs...), stdout: []byte("")},
				{cmd: exec.Command("/usr/bin/apt-get", "update"), envs: aptEnv},
				{cmd: exec.Command("/usr/bin/apt-get", "install", "-y", "p1", "p2"), envs: aptEnv, err: errors.New("bulk install error")},
				{cmd: exec.Command("/usr/bin/apt-get", "install", "-y", "p1"), envs: aptEnv},
				{cmd: exec.Command("/usr/bin/apt-get", "install", "-y", "p2"), envs: aptEnv},
			},
		},
		{
			name:         "install fallback failure",
			aptInstalled: []*agentendpointpb.Package{{Name: "p1"}},
			expectations: []expectedCommand{
				{cmd: exec.Command("/usr/bin/dpkg-query", dpkgQueryArgs...), stdout: []byte("")},
				{cmd: exec.Command("/usr/bin/apt-get", "update"), envs: aptEnv},
				{cmd: exec.Command("/usr/bin/apt-get", "install", "-y", "p1"), envs: aptEnv, err: errors.New("bulk install error")},
				{cmd: exec.Command("/usr/bin/apt-get", "install", "-y", "p1"), envs: aptEnv, err: errors.New("individual install error")},
			},
			wantErr: errors.New("error installing apt packages: Error installing apt package: p1. Error details: error running /usr/bin/apt-get with args [\"install\" \"-y\" \"p1\"]: individual install error, stdout: \"\", stderr: \"\""),
		},
		{
			name:       "successful upgrade",
			aptUpdated: []*agentendpointpb.Package{{Name: "p1"}},
			expectations: []expectedCommand{
				{cmd: exec.Command("/usr/bin/dpkg-query", dpkgQueryArgs...), stdout: []byte(`{"package":"p1","status":"installed"}`)},
				{cmd: exec.Command("/usr/bin/apt-get", "update"), envs: aptEnv},
				{cmd: exec.Command("/usr/bin/apt-get", aptUpgradableArgs...), envs: aptEnv, stdout: []byte("Inst p1 [1.0] (2.0 repo [amd64])\n")},
				{cmd: exec.Command("/usr/bin/apt-get", "install", "-y", "p1"), envs: aptEnv},
			},
		},
		{
			name:       "upgrade failure",
			aptUpdated: []*agentendpointpb.Package{{Name: "p1"}},
			expectations: []expectedCommand{
				{cmd: exec.Command("/usr/bin/dpkg-query", dpkgQueryArgs...), stdout: []byte(`{"package":"p1","status":"installed"}`)},
				{cmd: exec.Command("/usr/bin/apt-get", "update"), envs: aptEnv},
				{cmd: exec.Command("/usr/bin/apt-get", aptUpgradableArgs...), envs: aptEnv, stdout: []byte("Inst p1 [1.0] (2.0 repo [amd64])\n")},
				{cmd: exec.Command("/usr/bin/apt-get", "install", "-y", "p1"), envs: aptEnv, err: errors.New("upgrade error")},
			},
			wantErr: errors.New("error upgrading apt packages: error running /usr/bin/apt-get with args [\"install\" \"-y\" \"p1\"]: upgrade error, stdout: \"\", stderr: \"\""),
		},
		{
			name:       "successful remove",
			aptRemoved: []*agentendpointpb.Package{{Name: "p1"}},
			expectations: []expectedCommand{
				{cmd: exec.Command("/usr/bin/dpkg-query", dpkgQueryArgs...), stdout: []byte(`{"package":"p1","status":"installed"}`)},
				{cmd: exec.Command("/usr/bin/apt-get", "remove", "-y", "p1"), envs: aptEnv},
			},
		},
		{
			name:       "remove fallback success",
			aptRemoved: []*agentendpointpb.Package{{Name: "p1"}, {Name: "p2"}},
			expectations: []expectedCommand{
				{cmd: exec.Command("/usr/bin/dpkg-query", dpkgQueryArgs...), stdout: []byte(`{"package":"p1","status":"installed"}` + "\n" + `{"package":"p2","status":"installed"}`)},
				{cmd: exec.Command("/usr/bin/apt-get", "remove", "-y", "p1", "p2"), envs: aptEnv, err: errors.New("bulk remove error")},
				{cmd: exec.Command("/usr/bin/apt-get", "remove", "-y", "p1"), envs: aptEnv},
				{cmd: exec.Command("/usr/bin/apt-get", "remove", "-y", "p2"), envs: aptEnv},
			},
		},
		{
			name:       "remove fallback failure",
			aptRemoved: []*agentendpointpb.Package{{Name: "p1"}},
			expectations: []expectedCommand{
				{cmd: exec.Command("/usr/bin/dpkg-query", dpkgQueryArgs...), stdout: []byte(`{"package":"p1","status":"installed"}`)},
				{cmd: exec.Command("/usr/bin/apt-get", "remove", "-y", "p1"), envs: aptEnv, err: errors.New("bulk remove error")},
				{cmd: exec.Command("/usr/bin/apt-get", "remove", "-y", "p1"), envs: aptEnv, err: errors.New("individual remove error")},
			},
			wantErr: errors.New("error removing apt packages: Error removing apt package: p1. Error details: error running /usr/bin/apt-get with args [\"remove\" \"-y\" \"p1\"]: individual remove error, stdout: \"\", stderr: \"\""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
			setupAptChangesTest(t, mockCommandRunner)
			setExpectations(mockCommandRunner, tt.expectations)

			err := aptChanges(context.Background(), tt.aptInstalled, tt.aptRemoved, tt.aptUpdated)
			utiltest.AssertErrorMatch(t, err, tt.wantErr)
		})
	}
}

// setupAptChangesTest sets up the environment for aptChanges tests by mocking the command runner.
func setupAptChangesTest(t *testing.T, runner *utilmocks.MockCommandRunner) {
	oldApt := packages.AptExists

	packages.AptExists = true
	packages.SetCommandRunner(runner)
	packages.SetPtyCommandRunner(runner)

	t.Cleanup(func() {
		packages.AptExists = oldApt
	})
}

