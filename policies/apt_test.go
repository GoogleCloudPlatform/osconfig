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
	"path/filepath"
	"strings"
	"testing"

	"cloud.google.com/go/osconfig/agentendpoint/apiv1beta/agentendpointpb"
	"github.com/GoogleCloudPlatform/osconfig/osinfo"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/packet"
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

func TestIsArmoredGPGKey(t *testing.T) {
	tests := []struct {
		name     string
		keyData  []byte
		expected bool
	}{
		{
			name:     "valid armored key",
			keyData:  []byte("-----BEGIN PGP PUBLIC KEY BLOCK-----\n\nmQENBF2..."),
			expected: true,
		},
		{
			name:     "invalid armored key (not a key)",
			keyData:  []byte("-----BEGIN PGP MESSAGE-----\n\n..."),
			expected: true, // armor.Decode returns true for any valid armored block
		},
		{
			name:     "binary data",
			keyData:  []byte{0x99, 0x01, 0x02, 0x03},
			expected: false,
		},
		{
			name:     "empty data",
			keyData:  []byte{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			utiltest.AssertEquals(t, isArmoredGPGKey(tt.keyData), tt.expected)
		})
	}
}

func TestContainsEntity(t *testing.T) {
	e1 := &openpgp.Entity{PrimaryKey: &packet.PublicKey{Fingerprint: [20]byte{1}}}
	e2 := &openpgp.Entity{PrimaryKey: &packet.PublicKey{Fingerprint: [20]byte{2}}}
	e3 := &openpgp.Entity{PrimaryKey: &packet.PublicKey{Fingerprint: [20]byte{3}}}

	tests := []struct {
		name     string
		es       []*openpgp.Entity
		e        *openpgp.Entity
		expected bool
	}{
		{
			name:     "entity is present",
			es:       []*openpgp.Entity{e1, e2},
			e:        e1,
			expected: true,
		},
		{
			name:     "entity is not present",
			es:       []*openpgp.Entity{e1, e2},
			e:        e3,
			expected: false,
		},
		{
			name:     "empty entity list",
			es:       []*openpgp.Entity{},
			e:        e1,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			utiltest.AssertEquals(t, containsEntity(tt.es, tt.e), tt.expected)
		})
	}
}

func TestReadInstanceOsInfo(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		provider    osinfo.Provider
		wantName    string
		wantVersion float64
		wantErr     error
	}{
		{
			name: "successful read debian 11",
			provider: stubOsInfoProvider{nameVersionProvider: func() (string, string, string) {
				return "debian", "Debian", "11"
			}},
			wantName:    "debian",
			wantVersion: 11,
			wantErr:     nil,
		},
		{
			name:     "provider error",
			provider: errorOsInfoProvider{},
			wantErr:  errors.New("error getting osinfo: osinfo error"),
		},
		{
			name: "invalid version string",
			provider: stubOsInfoProvider{nameVersionProvider: func() (string, string, string) {
				return "debian", "Debian", "not-a-number"
			}},
			wantName:    "debian",
			wantVersion: 0,
			wantErr:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			osInfoProviderActual := osInfoProvider
			defer func() { osInfoProvider = osInfoProviderActual }()
			osInfoProvider = tt.provider

			gotName, gotVersion, gotErr := readInstanceOsInfo(ctx)
			utiltest.AssertErrorMatch(t, gotErr, tt.wantErr)
			utiltest.AssertEquals(t, gotName, tt.wantName)
			utiltest.AssertEquals(t, gotVersion, tt.wantVersion)
		})
	}
}

func TestShouldUseSignedBy(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		provider osinfo.Provider
		expected bool
	}{
		{
			name: "debian 12",
			provider: stubOsInfoProvider{nameVersionProvider: func() (string, string, string) {
				return "debian", "Debian", "12"
			}},
			expected: true,
		},
		{
			name: "debian 11",
			provider: stubOsInfoProvider{nameVersionProvider: func() (string, string, string) {
				return "debian", "Debian", "11"
			}},
			expected: false,
		},
		{
			name: "ubuntu 24",
			provider: stubOsInfoProvider{nameVersionProvider: func() (string, string, string) {
				return "ubuntu", "Ubuntu", "24"
			}},
			expected: true,
		},
		{
			name: "ubuntu 22",
			provider: stubOsInfoProvider{nameVersionProvider: func() (string, string, string) {
				return "ubuntu", "Ubuntu", "22"
			}},
			expected: false,
		},
		{
			name:     "error reading os info",
			provider: errorOsInfoProvider{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			osInfoProviderActual := osInfoProvider
			defer func() { osInfoProvider = osInfoProviderActual }()
			osInfoProvider = tt.provider

			utiltest.AssertEquals(t, shouldUseSignedBy(ctx), tt.expected)
		})
	}
}

type errorOsInfoProvider struct{}

func (e errorOsInfoProvider) GetOSInfo(ctx context.Context) (osinfo.OSInfo, error) {
	return osinfo.OSInfo{}, errors.New("osinfo error")
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
