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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cloud.google.com/go/osconfig/agentendpoint/apiv1beta/agentendpointpb"
	"golang.org/x/sys/unix"
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
uts:
	unix.Utsname{
		Nodename: toUtsField("testhost"),
		Machine:  toUtsField("i386"),
		Release:  toUtsField("6.1.0-29-cloud-i386"),
		Version:  toUtsField("test"),
	}
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
			nameAndVersionProvider: debian10,
			repos: []*agentendpointpb.AptRepository{
				{Uri: "http://repo1-url/", Distribution: "distribution", Components: []string{"component1"}},
			},
			want: "# Repo file managed by Google OSConfig agent\n\ndeb http://repo1-url/ distribution component1\n",
		},
		{
			name: "2 repos, Debian 10",
			repos: []*agentendpointpb.AptRepository{
				{Uri: "http://repo1-url/", Distribution: "distribution", Components: []string{"component1"}, ArchiveType: agentendpointpb.AptRepository_DEB_SRC},
				{Uri: "http://repo2-url/", Distribution: "distribution", Components: []string{"component1", "component2"}, ArchiveType: agentendpointpb.AptRepository_DEB},
			},
			want: "# Repo file managed by Google OSConfig agent\n\ndeb-src http://repo1-url/ distribution component1\n\ndeb http://repo2-url/ distribution component1 component2\n",
		},
		{
			name: "2 repos, Debian 12",
			repos: []*agentendpointpb.AptRepository{
				{Uri: "http://repo1-url/", Distribution: "distribution", Components: []string{"component1"}, ArchiveType: agentendpointpb.AptRepository_DEB_SRC},
				{Uri: "http://repo2-url/", Distribution: "distribution", Components: []string{"component1", "component2"}, ArchiveType: agentendpointpb.AptRepository_DEB},
			},
			want: "# Repo file managed by Google OSConfig agent\n\ndeb-src http://repo1-url/ distribution component1\n\ndeb http://repo2-url/ distribution component1 component2\n",
		},
	}

	for _, tt := range tests {
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
