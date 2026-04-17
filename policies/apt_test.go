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
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"cloud.google.com/go/osconfig/agentendpoint/apiv1beta/agentendpointpb"
	"github.com/GoogleCloudPlatform/osconfig/osinfo"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/packet"
)

func createTestGPGKey(t *testing.T) []byte {
	t.Helper()
	entity, err := openpgp.NewEntity("test", "test", "test@test.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := entity.Serialize(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestAptRepositories tests the adding of apt repository files.
func TestAptRepositories(t *testing.T) {
	ctx := context.Background()
	validKey := createTestGPGKey(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/valid-key":
			w.Write(validKey)
		default:
			w.Write([]byte("fake-gpg-key"))
		}
	}))
	defer srv.Close()

	tests := []struct {
		name     string
		repos    []*agentendpointpb.AptRepository
		provider osinfo.Provider
		want     string
	}{
		{
			name: "no repositories, want header only",
			provider: stubOsInfoProvider{nameVersionProvider: func() (string, string, string) {
				return "debian", "Debian", "10"
			}},
			repos: []*agentendpointpb.AptRepository{},
			want:  "# Repo file managed by Google OSConfig agent\n",
		},
		{
			name: "single deb repo on debian 10, want repo line without signed-by",
			provider: stubOsInfoProvider{nameVersionProvider: func() (string, string, string) {
				return "debian", "Debian", "10"
			}},
			repos: []*agentendpointpb.AptRepository{
				{Uri: "http://repo1-url/", Distribution: "distribution", Components: []string{"component1"}},
			},
			want: "# Repo file managed by Google OSConfig agent\n\ndeb http://repo1-url/ distribution component1\n",
		},
		{
			name: "single deb repo on debian 12, want repo line with signed-by",
			provider: stubOsInfoProvider{nameVersionProvider: func() (string, string, string) {
				return "debian", "Debian", "12"
			}},
			repos: []*agentendpointpb.AptRepository{
				{Uri: "http://repo1-url/", Distribution: "distribution", Components: []string{"component1"}},
			},
			want: "# Repo file managed by Google OSConfig agent\n\ndeb [signed-by=/etc/apt/trusted.gpg.d/osconfig_agent_managed.gpg] http://repo1-url/ distribution component1\n",
		},
		{
			name: "unknown archive type, want default to deb",
			provider: stubOsInfoProvider{nameVersionProvider: func() (string, string, string) {
				return "debian", "Debian", "10"
			}},
			repos: []*agentendpointpb.AptRepository{
				{Uri: "http://repo", Distribution: "dist", ArchiveType: agentendpointpb.AptRepository_ArchiveType(99)},
			},
			want: "# Repo file managed by Google OSConfig agent\n\ndeb http://repo dist\n",
		},
		{
			name: "multiple repos and components, want multiple repo lines",
			provider: stubOsInfoProvider{nameVersionProvider: func() (string, string, string) {
				return "debian", "Debian", "10"
			}},
			repos: []*agentendpointpb.AptRepository{
				{Uri: "http://repo1", Distribution: "dist1", Components: []string{"comp1"}, ArchiveType: agentendpointpb.AptRepository_DEB_SRC},
				{Uri: "http://repo2", Distribution: "dist2", Components: []string{"comp1", "comp2"}},
			},
			want: "# Repo file managed by Google OSConfig agent\n\ndeb-src http://repo1 dist1 comp1\n\ndeb http://repo2 dist2 comp1 comp2\n",
		},
		{
			name: "repo with valid gpg key, want repo line and gpg block coverage",
			provider: stubOsInfoProvider{nameVersionProvider: func() (string, string, string) {
				return "debian", "Debian", "10"
			}},
			repos: []*agentendpointpb.AptRepository{
				{Uri: "http://repo", Distribution: "dist", GpgKey: srv.URL + "/valid-key"},
			},
			want: "# Repo file managed by Google OSConfig agent\n\ndeb http://repo dist\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer utiltest.OverrideVariable(&osInfoProvider, tt.provider)()

			td, cleanup := utiltest.TempDir(t)
			defer cleanup()
			testRepo := filepath.Join(td, "testRepo")

			err := aptRepositories(ctx, tt.repos, testRepo)
			utiltest.AssertErrorMatch(t, err, nil)
			utiltest.AssertFileContents(t, testRepo, tt.want)
		})
	}
}

// TestGetAptGPGKey tests the retrieval and validation of apt GPG keys.
func TestGetAptGPGKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/large":
			w.Header().Set("Content-Length", "2000000")
			w.Write(make([]byte, 100))
		case "/binary":
			w.Write([]byte{0x99, 0x01, 0x02})
		case "/empty_armored":
			w.Write([]byte("-----BEGIN PGP PUBLIC KEY BLOCK-----\n\n-----END PGP PUBLIC KEY BLOCK-----"))
		default:
			w.Write([]byte("invalid data"))
		}
	}))
	defer srv.Close()

	tests := []struct {
		name    string
		url     string
		wantErr error
	}{
		{
			name:    "empty armored key, want nil error",
			url:     srv.URL + "/empty_armored",
			wantErr: nil,
		},
		{
			name:    "invalid data, want invalid data error",
			url:     srv.URL + "/invalid",
			wantErr: errors.New("openpgp: invalid data: tag byte does not have MSB set"),
		},
		{
			name:    "binary key, want unexpected EOF error",
			url:     srv.URL + "/binary",
			wantErr: errors.New("unexpected EOF"),
		},
		{
			name:    "large key, want too large error",
			url:     srv.URL + "/large",
			wantErr: errors.New("key size of 2000000 too large"),
		},
		{
			name:    "invalid url, want parse error",
			url:     "http://invalid:url",
			wantErr: errors.New(`parse "http://invalid:url": invalid port ":url" after host`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := getAptGPGKey(tt.url)
			if err != nil {
				// openpgp returns custom error types
				// convert to standard error type for AssertErrorMatch
				err = errors.New(err.Error())
			}
			utiltest.AssertErrorMatch(t, err, tt.wantErr)
		})
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
		name    string
		keyData []byte
		want    bool
	}{
		{
			name:    "valid armored PGP public key block, expect true",
			keyData: []byte("-----BEGIN PGP PUBLIC KEY BLOCK-----\n\nmQENBF2..."),
			want:    true,
		},
		{
			name:    "valid armored PGP message block, expect true",
			keyData: []byte("-----BEGIN PGP MESSAGE-----\n\n..."),
			want:    true, // armor.Decode returns true for any valid armored block
		},
		{
			name:    "non-armored binary data, expect false",
			keyData: []byte{0x99, 0x01, 0x02, 0x03},
			want:    false,
		},
		{
			name:    "empty input, expect false",
			keyData: []byte{},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			utiltest.AssertEquals(t, isArmoredGPGKey(tt.keyData), tt.want)
		})
	}
}

func TestContainsEntity(t *testing.T) {
	entity1 := &openpgp.Entity{PrimaryKey: &packet.PublicKey{Fingerprint: [20]byte{1}}}
	entity2 := &openpgp.Entity{PrimaryKey: &packet.PublicKey{Fingerprint: [20]byte{2}}}
	entity3 := &openpgp.Entity{PrimaryKey: &packet.PublicKey{Fingerprint: [20]byte{3}}}

	tests := []struct {
		name       string
		entityList []*openpgp.Entity
		target     *openpgp.Entity
		want       bool
	}{
		{
			name:       "entity is present, expect true",
			entityList: []*openpgp.Entity{entity1, entity2},
			target:     entity1,
			want:       true,
		},
		{
			name:       "entity is not present, expect false",
			entityList: []*openpgp.Entity{entity1, entity2},
			target:     entity3,
			want:       false,
		},
		{
			name:       "empty entity list, expect false",
			entityList: []*openpgp.Entity{},
			target:     entity1,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			utiltest.AssertEquals(t, containsEntity(tt.entityList, tt.target), tt.want)
		})
	}
}

func TestShouldUseSignedBy(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		provider osinfo.Provider
		want     bool
	}{
		{
			name: "debian 12, expect true",
			provider: stubOsInfoProvider{nameVersionProvider: func() (string, string, string) {
				return "debian", "Debian", "12"
			}},
			want: true,
		},
		{
			name: "debian 11, expect false",
			provider: stubOsInfoProvider{nameVersionProvider: func() (string, string, string) {
				return "debian", "Debian", "11"
			}},
			want: false,
		},
		{
			name: "ubuntu 24, expect true",
			provider: stubOsInfoProvider{nameVersionProvider: func() (string, string, string) {
				return "ubuntu", "Ubuntu", "24"
			}},
			want: true,
		},
		{
			name: "ubuntu 22, expect false",
			provider: stubOsInfoProvider{nameVersionProvider: func() (string, string, string) {
				return "ubuntu", "Ubuntu", "22"
			}},
			want: false,
		},
		{
			name: "invalid version string on Debian, expect false",
			provider: stubOsInfoProvider{nameVersionProvider: func() (string, string, string) {
				return "debian", "Debian", "not-a-number"
			}},
			want: false,
		},
		{
			name:     "error reading os info, expect false",
			provider: stubOsInfoProvider{err: errors.New("osinfo error")},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer utiltest.OverrideVariable(&osInfoProvider, tt.provider)()
			utiltest.AssertEquals(t, shouldUseSignedBy(ctx), tt.want)
		})
	}
}

type stubOsInfoProvider struct {
	nameVersionProvider func() (string, string, string)
	err                 error
}

func (s stubOsInfoProvider) GetOSInfo(ctx context.Context) (osinfo.OSInfo, error) {
	if s.err != nil {
		return osinfo.OSInfo{}, s.err
	}
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
